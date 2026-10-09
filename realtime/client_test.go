package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// These fixtures follow the v1 envelopes in TS polybolt.ts (087f9443) and
// Python realtime/protocol.py (ed8d04ca), not the Go encoder/decoder.
func priceFixture(channel, symbol string, snapshot bool) string {
	if snapshot {
		return fmt.Sprintf(
			`{"v":1,"channel":%q,"seq":1,"ts":123456,"snapshot":true,"payload":{"symbol":%q,"source":"newvendor","window_seconds":60,"data":[{"timestamp":123455,"value":123.45,"full_accuracy_value":"123.450000000000000001"}]}}`,
			channel,
			symbol,
		)
	}
	return fmt.Sprintf(
		`{"v":1,"channel":%q,"seq":2,"ts":123457,"dropped":3,"payload":{"symbol":%q,"source":"newvendor","window_seconds":60,"timestamp":123456,"value":123.45,"full_accuracy_value":"123.450000000000000002","received_at":123457,"is_carried_forward":false}}`,
		channel,
		symbol,
	)
}

type serverFrame struct {
	Op            string            `json:"op"`
	RID           string            `json:"rid"`
	Auth          map[string]string `json:"auth"`
	Subscriptions []struct {
		Channel string         `json:"channel"`
		Filter  map[string]any `json:"filter"`
	} `json:"subscriptions"`
}
type observation struct {
	peer  *testPeer
	frame serverFrame
	at    time.Time
}
type testPeer struct {
	id   int
	conn *websocket.Conn
	ctx  context.Context
	done chan struct{}
}

func (p *testPeer) write(t *testing.T, raw string) {
	t.Helper()
	if err := p.conn.Write(p.ctx, websocket.MessageText, []byte(raw)); err != nil &&
		p.ctx.Err() == nil {
		t.Errorf("server write: %v", err)
	}
}

func (p *testPeer) acknowledge(t *testing.T, f serverFrame, provider string) {
	t.Helper()
	switch f.Op {
	case "auth":
		p.write(t, fmt.Sprintf(`{"op":"authed","rid":%q}`, f.RID))
	case "ping":
		p.write(t, `{"op":"pong"}`)
	case "subscribe", "unsubscribe":
		op := "subscribed"
		if f.Op == "unsubscribe" {
			op = "unsubscribed"
		}
		for _, sub := range f.Subscriptions {
			extra := ""
			if provider != "" && f.Op == "subscribe" {
				extra = fmt.Sprintf(`,"provider":%q`, provider)
			}
			p.write(
				t,
				fmt.Sprintf(`{"op":%q,"channel":%q,"rid":%q%s}`, op, sub.Channel, f.RID, extra),
			)
		}
	}
}

type testFeed struct {
	url          string
	frames       chan observation
	peers        chan *testPeer
	ctx          context.Context
	mu           sync.Mutex
	observations []observation
}

func startFeed(t *testing.T, hook func(*testPeer, serverFrame)) *testFeed {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	f := &testFeed{frames: make(chan observation, 1024), peers: make(chan *testPeer, 128), ctx: ctx}
	var wg sync.WaitGroup
	var id atomic.Int64
	var connections sync.Map
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		p := &testPeer{int(id.Add(1)), conn, ctx, make(chan struct{})}
		connections.Store(p, true)
		wg.Add(1)
		defer wg.Done()
		defer close(p.done)
		defer conn.CloseNow()
		defer connections.Delete(p)
		f.peers <- p
		for {
			_, raw, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var frame serverFrame
			if err := json.Unmarshal(raw, &frame); err != nil {
				t.Errorf("invalid client JSON: %v", err)
				return
			}
			obs := observation{p, frame, time.Now()}
			f.mu.Lock()
			f.observations = append(f.observations, obs)
			f.mu.Unlock()
			select {
			case f.frames <- obs:
			case <-ctx.Done():
				return
			}
			if hook != nil {
				hook(p, frame)
			} else {
				p.acknowledge(t, frame, "")
			}
		}
	}))
	f.url = "ws" + strings.TrimPrefix(srv.URL, "http")
	t.Cleanup(func() {
		cancel()
		connections.Range(func(k, v any) bool { k.(*testPeer).conn.CloseNow(); return true })
		srv.Close()
		wg.Wait()
	})
	return f
}

func testConfig(f *testFeed) Config {
	return Config{
		URL:               f.url,
		Credentials:       Credentials{"test-key", "test-secret", "test-pass"},
		ReconnectMin:      time.Millisecond,
		ReconnectMax:      time.Millisecond,
		AckTimeout:        time.Second,
		AcceptanceTimeout: 5 * time.Second,
	}
}

func testClient(t *testing.T, cfg Config) *Client {
	t.Helper()
	c, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case value, ok := <-ch:
		if !ok {
			t.Fatal("channel closed unexpectedly")
		}
		return value
	case <-timer.C:
		t.Fatal("timed out waiting for channel")
		var zero T
		return zero
	}
}

func waitClosed(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("resource did not close")
	}
}

func (f *testFeed) next(t *testing.T, op string) observation {
	t.Helper()
	for {
		o := receive(t, f.frames)
		if o.frame.Op == op {
			return o
		}
	}
}

func subscribe(t *testing.T, c *Client, ctx context.Context, r Request) *Subscription {
	t.Helper()
	s, err := c.Subscribe(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestWireChannelsAndProviders(t *testing.T) {
	for _, row := range []struct {
		channel           Channel
		symbol, canonical string
		window            bool
	}{
		{Crypto, "btcusd", "btcusd", false}, {CryptoTWAP, "btcusd", "btcusd", true}, {Equity, " AAPL ", "aapl", false}, {EquityTWAP, "UsDjPy", "usdjpy", true},
	} {
		for _, provider := range []Provider{"", Pyth, Chainlink} {
			t.Run(string(row.channel)+"/"+string(provider), func(t *testing.T) {
				f := startFeed(t, func(p *testPeer, frame serverFrame) {
					p.acknowledge(t, frame, "massive")
					if frame.Op == "subscribe" {
						p.write(t, priceFixture(string(row.channel), row.canonical, true))
						p.write(t, priceFixture(string(row.channel), row.canonical, false))
					}
				})
				c := testClient(t, testConfig(f))
				s := subscribe(
					t,
					c,
					t.Context(),
					Request{
						Channel:  row.channel,
						Symbols:  []string{row.symbol},
						Provider: provider,
					},
				)
				auth := f.next(t, "auth")
				if !reflect.DeepEqual(
					auth.frame.Auth,
					map[string]string{
						"apiKey":     "test-key",
						"secret":     "test-secret",
						"passphrase": "test-pass",
					},
				) {
					t.Fatalf("auth = %#v", auth.frame.Auth)
				}
				wire := f.next(t, "subscribe")
				if wire.at.Sub(auth.at) < operationInterval-5*time.Millisecond {
					t.Fatal("operation pacing violated")
				}
				expected := map[string]any{"symbol": row.canonical}
				if row.window {
					expected["window_seconds"] = float64(60)
				}
				if provider != "" {
					expected["provider"] = string(provider)
				}
				if len(wire.frame.Subscriptions) != 1 ||
					wire.frame.Subscriptions[0].Channel != string(row.channel) ||
					!reflect.DeepEqual(wire.frame.Subscriptions[0].Filter, expected) {
					t.Fatalf("wire = %#v", wire.frame)
				}
				confirm := receive(t, s.Events())
				if confirm.Type != Accepted || confirm.Confirmation.Provider == nil ||
					*confirm.Confirmation.Provider != SourceMassive {
					t.Fatalf("confirmation = %+v", confirm)
				}
				snap := receive(t, s.Events())
				if snap.Snapshot == nil ||
					snap.Snapshot.Data[0].Value != "123.450000000000000001" ||
					snap.Source != "newvendor" {
					t.Fatalf("snapshot = %+v", snap)
				}
				update := receive(t, s.Events())
				if update.Update == nil || update.Update.Value != "123.450000000000000002" ||
					update.Dropped == nil ||
					*update.Dropped != 3 ||
					update.Timestamp.UnixMilli() != 123457 ||
					update.Update.Timestamp.UnixMilli() != 123456 {
					t.Fatalf("update = %+v", update)
				}
				if !row.window &&
					(update.Update.ReceivedAt == nil || update.Update.ReceivedAt.UnixMilli() != 123457 || update.Update.IsCarriedForward == nil || *update.Update.IsCarriedForward) {
					t.Fatalf("receipt metadata = %+v", update.Update)
				}
				if row.window && update.WindowSeconds != 60 {
					t.Fatal("TWAP window missing")
				}
				s.Close()
				waitClosed(t, s.Done())
				waitClosed(t, wire.peer.done)
				if s.Err() != nil {
					t.Fatal(s.Err())
				}
			})
		}
	}
}

func TestConcurrentSharingAndReferenceCounts(t *testing.T) {
	f := startFeed(t, func(p *testPeer, frame serverFrame) {
		p.acknowledge(t, frame, "")
		if frame.Op == "subscribe" {
			for _, sub := range frame.Subscriptions {
				p.write(t, priceFixture(sub.Channel, sub.Filter["symbol"].(string), true))
			}
		}
	})
	c := testClient(t, testConfig(f))
	keeper := subscribe(t, c, t.Context(), Request{Channel: Crypto, Symbols: []string{"ethusd"}})
	first := f.next(t, "subscribe")
	receive(t, keeper.Events())
	receive(t, keeper.Events())
	const count = 20
	results := make(chan *Subscription, count)
	var wg sync.WaitGroup
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := c.Subscribe(
				t.Context(),
				Request{Channel: Crypto, Symbols: []string{"btcusd", "btcusd"}},
			)
			if err != nil {
				t.Error(err)
				return
			}
			results <- s
		}()
	}
	wg.Wait()
	handles := make([]*Subscription, 0, count)
	for range count {
		handles = append(handles, receive(t, results))
	}
	added := f.next(t, "subscribe")
	if added.peer != first.peer || len(added.frame.Subscriptions) != 1 {
		t.Fatalf("shared subscription = %+v", added)
	}
	// After all joins complete, closing every BTC reference yields one unsubscribe.
	for _, s := range handles {
		s.Close()
	}
	removed := f.next(t, "unsubscribe")
	if removed.peer != first.peer || len(removed.frame.Subscriptions) != 1 ||
		removed.frame.Subscriptions[0].Filter["symbol"] != "btcusd" {
		t.Fatalf("unsubscribe = %+v", removed)
	}
	// A new subscription for a just-removed key must be ordered after removal.
	replacement := subscribe(
		t,
		c,
		t.Context(),
		Request{Channel: Crypto, Symbols: []string{"btcusd"}},
	)
	replaced := f.next(t, "subscribe")
	if replaced.peer != first.peer {
		t.Fatal("unexpected new socket")
	}
	replacement.Close()
	f.next(t, "unsubscribe")
	keeper.Close()
	waitClosed(t, first.peer.done)
	f.mu.Lock()
	defer f.mu.Unlock()
	subscribes := 0
	for _, obs := range f.observations {
		if obs.frame.Op == "subscribe" {
			subscribes += len(obs.frame.Subscriptions)
		}
	}
	if subscribes != 3 {
		t.Fatalf("expected ETH, shared BTC, replacement BTC; got %d", subscribes)
	}
}

func TestPoolCapacityProviderIsolationAndReconnect(t *testing.T) {
	f := startFeed(t, nil)
	c := testClient(t, testConfig(f))
	symbols := make([]string, 65)
	for i := range symbols {
		symbols[i] = fmt.Sprintf("asset%dusd", i)
	}
	many := subscribe(t, c, t.Context(), Request{Channel: Crypto, Symbols: symbols})
	a, b := f.next(t, "subscribe"), f.next(t, "subscribe")
	if a.peer == b.peer || len(a.frame.Subscriptions)+len(b.frame.Subscriptions) != 65 ||
		len(a.frame.Subscriptions) > 64 ||
		len(b.frame.Subscriptions) > 64 {
		t.Fatalf("pool capacities: %d %d", len(a.frame.Subscriptions), len(b.frame.Subscriptions))
	}
	pinned := subscribe(
		t,
		c,
		t.Context(),
		Request{Channel: Crypto, Symbols: []string{symbols[0]}, Provider: Pyth},
	)
	pin := f.next(t, "subscribe")
	if pin.peer == a.peer || pin.peer == b.peer {
		t.Fatal("provider groups mixed")
	}
	defaultJoin := subscribe(
		t,
		c,
		t.Context(),
		Request{Channel: Crypto, Symbols: []string{symbols[0]}},
	)
	joinConfirmation := receive(t, defaultJoin.Events())
	pinConfirmation := receive(t, pinned.Events())
	if joinConfirmation.ConnectionID == pinConfirmation.ConnectionID ||
		pinConfirmation.Confirmation.Provider != nil {
		t.Fatal("pin conflated with actual provider")
	}
	// Close a real socket with a recoverable overload code. Active filters must
	// authenticate and replay on a new socket without affecting other sockets.
	a.peer.conn.Close(websocket.StatusCode(4002), "Slow consumer")
	replayAuth := f.next(t, "auth")
	replay := f.next(t, "subscribe")
	if replay.peer != replayAuth.peer || replay.peer == a.peer ||
		len(replay.frame.Subscriptions) != len(a.frame.Subscriptions) {
		t.Fatalf("replay = %+v", replay)
	}
	want := map[string]bool{}
	for _, sub := range a.frame.Subscriptions {
		want[sub.Filter["symbol"].(string)] = true
	}
	for _, sub := range replay.frame.Subscriptions {
		if !want[sub.Filter["symbol"].(string)] {
			t.Fatal("wrong replayed filter")
		}
	}
	many.Close()
	pinned.Close()
	defaultJoin.Close()
	c.Close()
	waitClosed(t, replay.peer.done)
	waitClosed(t, b.peer.done)
	waitClosed(t, pin.peer.done)
}

func TestContextCancellationDuringAcceptanceAndClientShutdown(t *testing.T) {
	gate := make(chan struct{})
	f := startFeed(t, func(p *testPeer, frame serverFrame) {
		if frame.Op == "subscribe" {
			select {
			case <-gate:
			case <-p.ctx.Done():
				return
			}
		}
		p.acknowledge(t, frame, "")
	})
	c := testClient(t, testConfig(f))
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() {
		_, err := c.Subscribe(ctx, Request{Channel: Crypto, Symbols: []string{"btcusd"}})
		result <- err
	}()
	wire := f.next(t, "subscribe")
	cancel()
	if err := receive(t, result); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error = %v", err)
	}
	close(gate)
	waitClosed(t, wire.peer.done)
	// Close races with new registration: no network goroutine survives Close,
	// and Subscribe never returns a live handle from a closed client.
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := c.Subscribe(
				t.Context(),
				Request{Channel: Crypto, Symbols: []string{"ethusd"}},
			)
			if err == nil {
				s.Close()
			} else if !errors.Is(err, ErrClosed) {
				t.Errorf("subscribe: %v", err)
			}
		}()
	}
	c.Close()
	wg.Wait()
	if _, err := c.Subscribe(t.Context(), Request{Channel: Crypto, Symbols: []string{"ethusd"}}); !errors.Is(
		err,
		ErrClosed,
	) {
		t.Fatalf("closed subscribe = %v", err)
	}
	var closes sync.WaitGroup
	for range 10 {
		closes.Add(1)
		go func() { defer closes.Done(); c.Close() }()
	}
	closes.Wait()
}

func TestRejectionsAndTerminalClose(t *testing.T) {
	for _, code := range []string{"auth_invalid", "future_auth_error"} {
		t.Run(code, func(t *testing.T) {
			f := startFeed(t, func(p *testPeer, frame serverFrame) {
				if frame.Op == "auth" {
					p.write(t, fmt.Sprintf(`{"op":"error","rid":%q,"code":%q}`, frame.RID, code))
				}
			})
			c := testClient(t, testConfig(f))
			_, err := c.Subscribe(
				t.Context(),
				Request{Channel: Crypto, Symbols: []string{"btcusd"}},
			)
			var rejection *RejectionError
			if !errors.As(err, &rejection) || rejection.Code != code {
				t.Fatalf("rejection = %v", err)
			}
			c.Close()
			peer := receive(t, f.peers)
			waitClosed(t, peer.done)
			f.mu.Lock()
			defer f.mu.Unlock()
			if len(f.observations) != 1 {
				t.Fatal("terminal auth retried")
			}
		})
	}
	for _, code := range []int{4001, 4008} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			f := startFeed(t, func(p *testPeer, frame serverFrame) { p.acknowledge(t, frame, "") })
			c := testClient(t, testConfig(f))
			s := subscribe(t, c, t.Context(), Request{Channel: Crypto, Symbols: []string{"btcusd"}})
			peer := f.next(t, "subscribe").peer
			peer.conn.Close(websocket.StatusCode(code), "terminal")
			waitClosed(t, s.Done())
			var closed *ConnectionError
			if !errors.As(s.Err(), &closed) || closed.Code != code || closed.Reason != "terminal" {
				t.Fatalf("close error=%v", s.Err())
			}
			c.Close()
			waitClosed(t, peer.done)
		})
	}
}

func TestPartialRejectionKeepsUnrelatedSharedListeners(t *testing.T) {
	gate := make(chan struct{})
	f := startFeed(t, func(p *testPeer, frame serverFrame) {
		if frame.Op != "subscribe" {
			p.acknowledge(t, frame, "")
			return
		}
		for _, sub := range frame.Subscriptions {
			if sub.Filter["symbol"] == "badusd" {
				select {
				case <-gate:
				case <-p.ctx.Done():
					return
				}
				p.write(
					t,
					fmt.Sprintf(
						`{"op":"error","rid":%q,"channel":"price.crypto","code":"new_rejection"}`,
						frame.RID,
					),
				)
			} else {
				p.write(t, fmt.Sprintf(`{"op":"subscribed","rid":%q,"channel":%q}`, frame.RID, sub.Channel))
			}
		}
	})
	c := testClient(t, testConfig(f))
	good := subscribe(t, c, t.Context(), Request{Channel: Crypto, Symbols: []string{"btcusd"}})
	f.next(t, "subscribe")
	receive(t, good.Events())
	result := make(chan error, 1)
	go func() {
		_, err := c.Subscribe(
			t.Context(),
			Request{Channel: Crypto, Symbols: []string{"btcusd", "badusd", "ethusd"}},
		)
		result <- err
	}()
	pending := f.next(t, "subscribe")
	close(gate)
	var rejection *RejectionError
	if err := receive(t, result); !errors.As(err, &rejection) || rejection.Code != "new_rejection" {
		t.Fatalf("partial rejection=%v", err)
	}
	pending.peer.write(t, priceFixture("price.crypto", "btcusd", false))
	event := receive(t, good.Events())
	if event.Update == nil {
		t.Fatal("unrelated shared listener failed")
	}
	good.Close()
	waitClosed(t, pending.peer.done)
}
