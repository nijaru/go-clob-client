package rtds

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	json "github.com/go-json-experiment/json"
)

// This fixture models RTDS's replacement semantics: each subscribe replaces
// the filter for its topic/type, rather than adding another symbol interest.
func TestRTDSSharedSubscriptionLifecycle(t *testing.T) {
	for _, topic := range []string{"crypto_prices", "crypto_prices_chainlink"} {
		t.Run(topic, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			type peer struct {
				conn   *websocket.Conn
				mu     sync.Mutex
				active map[string]Subscription
				frames chan SubscriptionRequest
			}
			peers := make(chan *peer, 8)
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					conn, err := websocket.Accept(w, r, nil)
					if err != nil {
						return
					}
					defer conn.CloseNow()
					p := &peer{
						conn:   conn,
						active: make(map[string]Subscription),
						frames: make(chan SubscriptionRequest, 32),
					}
					peers <- p
					for {
						_, data, err := conn.Read(ctx)
						if err != nil {
							return
						}
						if string(data) == "PING" {
							_ = conn.Write(ctx, websocket.MessageText, []byte("PONG"))
							continue
						}
						var frame SubscriptionRequest
						if err := json.Unmarshal(data, &frame); err != nil {
							return
						}
						p.mu.Lock()
						for _, sub := range frame.Subscriptions {
							key := sub.Topic + "/" + sub.Type
							if frame.Action == ActionSubscribe {
								p.active[key] = sub
							}
							if frame.Action == ActionUnsubscribe {
								delete(p.active, key)
							}
						}
						p.mu.Unlock()
						p.frames <- frame
					}
				}),
			)
			defer server.Close()
			client := NewClient(strings.Replace(server.URL, "http", "ws", 1), nil)
			defer client.Close()
			nextPeer := func() *peer {
				select {
				case p := <-peers:
					return p
				case <-ctx.Done():
					t.Fatal("connection timeout")
					return nil
				}
			}
			expectFrames := func(p *peer, action Action, topics ...string) {
				t.Helper()
				// A fixture-only probe establishes a deterministic wire barrier:
				// every prior delta must precede it, including unwanted duplicates.
				if err := client.sendJSON(ctx, SubscriptionRequest{Action: "probe"}); err != nil {
					t.Fatal(err)
				}
				var got []string
				for {
					select {
					case frame := <-p.frames:
						if frame.Action == "probe" {
							if !slices.Equal(got, topics) {
								t.Fatalf("wire topics = %v, want %v", got, topics)
							}
							return
						}
						if frame.Action != action {
							t.Fatalf("wire action = %s, want %s", frame.Action, action)
						}
						for _, sub := range frame.Subscriptions {
							if sub.Filters != nil {
								t.Fatalf("server filter would replace shared interests: %+v", sub)
							}
							got = append(got, sub.Topic)
						}
					case <-ctx.Done():
						t.Fatal("wire frame timeout")
					}
				}
			}
			btc := Subscription{
				Topic:   topic,
				Type:    "update",
				Filters: map[string]string{"symbol": "btc/usd"},
			}
			eth := Subscription{
				Topic:   topic,
				Type:    "update",
				Filters: map[string]string{"symbol": "eth/usd"},
			}
			if topic == "crypto_prices" {
				btc.Filters = []string{"btc/usd"}
				eth.Filters = []string{"eth/usd"}
			}
			other := Subscription{Topic: "equity_prices", Type: "update"}
			if err := client.Connect(ctx); err != nil {
				t.Fatal(err)
			}
			p := nextPeer()
			if err := client.Subscribe(ctx, btc); err != nil {
				t.Fatal(err)
			}
			expectFrames(p, ActionSubscribe, topic)
			var registrations sync.WaitGroup
			registrationErrors := make(chan error, 2)
			for _, sub := range []Subscription{eth, btc} {
				registrations.Go(func() { registrationErrors <- client.Subscribe(ctx, sub) })
			}
			registrations.Wait()
			close(registrationErrors)
			for err := range registrationErrors {
				if err != nil {
					t.Fatal(err)
				}
			}
			expectFrames(p, ActionSubscribe) // no redundant/overwriting wire frames
			if err := client.Subscribe(ctx, other); err != nil {
				t.Fatal(err)
			}
			expectFrames(p, ActionSubscribe, "equity_prices")

			emit := func(p *peer, want ...string) {
				t.Helper()
				p.mu.Lock()
				sub, hasPrices := p.active[topic+"/update"]
				_, hasOther := p.active["equity_prices/update"]
				p.mu.Unlock()
				var batch []*RtdsMessage
				if hasPrices {
					for _, symbol := range []string{"doge/usd", "btc/usd", "eth/usd"} {
						payload, _ := json.Marshal(map[string]any{"symbol": symbol, "value": 1})
						m := &RtdsMessage{Topic: topic, Type: "update", Payload: payload}
						allowed := sub.Filters == nil
						switch filter := sub.Filters.(type) {
						case []any:
							allowed = slices.Contains(filter, any(symbol))
						case map[string]any:
							allowed = filter["symbol"] == symbol
						case string:
							var fields map[string]string
							_ = json.Unmarshal([]byte(filter), &fields)
							allowed = fields["symbol"] == symbol
						}
						if allowed {
							batch = append(batch, m)
						}
					}
					// RTDS may emit event kinds other than the requested type.
					batch = append(
						batch,
						&RtdsMessage{
							Topic:   topic,
							Type:    "snapshot",
							Payload: []byte(`{"symbol":"eth/usd"}`),
						},
					)
				}
				if !hasOther {
					t.Fatal("independent equity feed lost")
				}
				batch = append(
					batch,
					&RtdsMessage{
						Topic:   "equity_prices",
						Type:    "update",
						Payload: []byte(`{"symbol":"SPY"}`),
					},
				)
				data, _ := json.Marshal(batch)
				data = append([]byte(" \n"), data...)
				if err := p.conn.Write(ctx, websocket.MessageText, data); err != nil {
					t.Fatal(err)
				}
				var got []string
				for {
					select {
					case m := <-client.Messages():
						if m.Topic == "equity_prices" {
							if !slices.Equal(got, want) {
								t.Fatalf("delivered = %v, want %v", got, want)
							}
							return
						}
						if m.Type != "update" {
							t.Fatalf("unmatched type delivered: %s", m.Type)
						}
						var payload struct {
							Symbol string `json:"symbol"`
						}
						if err := json.Unmarshal(m.Payload, &payload); err != nil {
							t.Fatal(err)
						}
						got = append(got, payload.Symbol)
					case err := <-client.Errors():
						t.Fatal(err)
					case <-ctx.Done():
						t.Fatal("delivery timeout")
					}
				}
			}
			reconnect := func() {
				t.Helper()
				if err := p.conn.CloseNow(); err != nil {
					t.Fatal(err)
				}
				p = nextPeer()
				// Wait for replay before probing the newly published connection.
				select {
				case frame := <-p.frames:
					if frame.Action != ActionSubscribe || len(frame.Subscriptions) != 2 ||
						frame.Subscriptions[0].Topic != topic || frame.Subscriptions[1].Topic != "equity_prices" ||
						frame.Subscriptions[0].Filters != nil || frame.Subscriptions[1].Filters != nil {
						t.Fatalf("bad replay: %+v", frame)
					}
				case <-ctx.Done():
					t.Fatal("replay timeout")
				}
				expectFrames(p, ActionSubscribe) // exactly one entry per topic/type
			}
			emit(p, "btc/usd", "eth/usd")
			reconnect()
			emit(p, "btc/usd", "eth/usd")
			if err := client.Unsubscribe(ctx, btc); err != nil {
				t.Fatal(err)
			}
			expectFrames(p, ActionUnsubscribe)
			emit(p, "btc/usd", "eth/usd") // second BTC owner survives
			if err := client.Unsubscribe(ctx, btc); err != nil {
				t.Fatal(err)
			}
			expectFrames(p, ActionUnsubscribe)
			emit(p, "eth/usd")
			reconnect()
			emit(p, "eth/usd") // released BTC is not resurrected by replay
			if err := client.Unsubscribe(ctx, eth); err != nil {
				t.Fatal(err)
			}
			expectFrames(p, ActionUnsubscribe, topic)
			emit(p) // releasing crypto must not release equity
			if err := client.Unsubscribe(ctx, eth); err != nil {
				t.Fatal(err)
			}
			expectFrames(p, ActionUnsubscribe) // absent interest is a no-op
			if err := client.Unsubscribe(ctx, other); err != nil {
				t.Fatal(err)
			}
			expectFrames(p, ActionUnsubscribe, "equity_prices")
			if client.SubscriptionCount() != 0 {
				t.Fatal("registrations leaked")
			}
		})
	}
}

func TestRTDSRegistrationSnapshotsAndValidation(t *testing.T) {
	client := NewClient("", nil)
	defer client.Close()
	filters := map[string]string{"symbol": "btc/usd"}
	creds := &Credentials{Key: "original"}
	sub := Subscription{Topic: "prices", Type: "*", Filters: filters, CLOBAuth: creds}
	if err := client.Subscribe(t.Context(), sub); err != nil {
		t.Fatal(err)
	}
	filters["symbol"] = "eth/usd"
	creds.Key = "changed"
	client.handleData(
		t.Context(),
		[]byte(
			`[{"topic":"prices","type":"update","payload":{"symbol":"eth/usd"}},{"topic":"prices","type":"update","payload":{"symbol":"btc/usd"}}]`,
		),
	)
	select {
	case m := <-client.Messages():
		if string(m.Payload) != `{"symbol":"btc/usd"}` {
			t.Fatalf("caller mutation changed matching: %s", m.Payload)
		}
	default:
		t.Fatal("matching message not delivered")
	}
	if err := client.Subscribe(t.Context(), sub); err == nil {
		t.Fatal("conflicting credentials accepted")
	}
	for _, bad := range []Subscription{
		{Topic: "prices"},
		{Topic: "prices", Type: "*", Filters: "opaque server substring"},
		{Topic: "prices", Type: "*", Filters: []int{1}},
	} {
		if err := client.Subscribe(t.Context(), bad); err == nil {
			t.Fatalf("invalid subscription accepted: %+v", bad)
		}
	}
	original := Subscription{
		Topic:    "prices",
		Type:     "*",
		Filters:  map[string]string{"symbol": "btc/usd"},
		CLOBAuth: &Credentials{Key: "original"},
	}
	if err := client.Unsubscribe(t.Context(), original); err != nil {
		t.Fatal(err)
	}
	if client.SubscriptionCount() != 0 {
		t.Fatal("snapshot ownership could not be released")
	}
}
