package rtds

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	json "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"

	"github.com/coder/websocket"
)

func TestRTDSNumericPricePayloads(t *testing.T) {
	for _, tc := range []struct {
		name  string
		topic string
		want  string
	}{
		{name: "crypto", topic: "crypto_prices", want: "3456.78"},
		{name: "chainlink", topic: "crypto_prices_chainlink", want: "3456.78"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var message RtdsMessage
			if err := json.Unmarshal(
				[]byte(
					`{"topic":"`+tc.topic+`","type":"update","timestamp":1,"payload":{"symbol":"eth/usd","timestamp":2,"value":3456.78}}`,
				),
				&message,
			); err != nil {
				t.Fatalf("decode RTDS message: %v", err)
			}
			if tc.topic == "crypto_prices" {
				price, err := message.AsCryptoPrice()
				if err != nil {
					t.Fatalf("decode crypto price: %v", err)
				}
				if price.Value != tc.want {
					t.Fatalf("value = %q, want %q", price.Value, tc.want)
				}
				return
			}
			price, err := message.AsChainlinkPrice()
			if err != nil {
				t.Fatalf("decode Chainlink price: %v", err)
			}
			if price.Value != tc.want {
				t.Fatalf("value = %q, want %q", price.Value, tc.want)
			}
		})
	}
}

func TestChainlinkTWAPPriceUsesExactE18Value(t *testing.T) {
	t.Parallel()

	var message RtdsMessage
	if err := json.Unmarshal([]byte(`{
		"topic":"crypto_prices_twap_thirty",
		"type":"update",
		"timestamp":1772752581815,
		"payload":{
			"symbol":"btc/usd",
			"value":65000.12,
			"full_accuracy_value":"65000123456789012345678",
			"timestamp":1772752581815,
			"window_s":30
		}
	}`), &message); err != nil {
		t.Fatalf("decode RTDS message: %v", err)
	}
	price, err := message.AsChainlinkTWAPPrice()
	if err != nil {
		t.Fatalf("decode Chainlink TWAP price: %v", err)
	}
	if price.Symbol != "btc/usd" || price.Timestamp != 1772752581815 ||
		price.Value != "65000.123456789012345678" ||
		price.WindowSeconds != ChainlinkTWAP30Seconds {
		t.Fatalf("price = %+v", price)
	}
}

func TestChainlinkE18Conversion(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		input string
		want  string
	}{
		{input: "1", want: "0.000000000000000001"},
		{input: "-1230000000000000000", want: "-1.23"},
		{input: "-0", want: "0"},
		{input: "100000000000000000000", want: "100"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got, err := chainlinkE18ToDecimalString(tc.input)
			if err != nil || got != tc.want {
				t.Fatalf("conversion = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestChainlinkTWAPPriceValidation(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		topic   string
		payload string
	}{
		{
			name:    "topic window mismatch",
			topic:   "crypto_prices_twap_thirty",
			payload: `{"symbol":"btc/usd","value":1,"full_accuracy_value":"1000000000000000000","timestamp":1,"window_s":60}`,
		},
		{
			name:    "missing exact value",
			topic:   "crypto_prices_twap_thirty",
			payload: `{"symbol":"btc/usd","value":1,"timestamp":1,"window_s":30}`,
		},
		{
			name:    "non integer exact value",
			topic:   "crypto_prices_twap_thirty",
			payload: `{"symbol":"btc/usd","value":1,"full_accuracy_value":"1.2","timestamp":1,"window_s":30}`,
		},
		{
			name:    "unknown topic",
			topic:   "crypto_prices_twap_other",
			payload: `{"symbol":"btc/usd","value":1,"full_accuracy_value":"1","timestamp":1,"window_s":30}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			message := RtdsMessage{
				Topic:   tc.topic,
				Type:    "update",
				Payload: jsontext.Value(tc.payload),
			}
			if _, err := message.AsChainlinkTWAPPrice(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestChainlinkTWAPSubscriptions(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		window ChainlinkTWAPWindowSeconds
		topic  string
	}{
		{window: ChainlinkTWAP30Seconds, topic: "crypto_prices_twap_thirty"},
		{window: ChainlinkTWAP60Seconds, topic: "crypto_prices_twap_sixty"},
	} {
		t.Run(tc.topic, func(t *testing.T) {
			got, err := chainlinkTWAPTopic(tc.window)
			if err != nil || got != tc.topic {
				t.Fatalf("topic = %q, %v; want %q", got, err, tc.topic)
			}
		})
	}
	if _, err := chainlinkTWAPTopic(45); err == nil {
		t.Fatal("expected invalid window error")
	}
}

func TestRTDSClient(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	// Mock RTDS Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusInternalError, "closing")

		for {
			typ, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}

			if typ != websocket.MessageText {
				continue
			}

			// Heartbeat
			if string(data) == "PING" {
				_ = conn.Write(r.Context(), websocket.MessageText, []byte("PONG"))
				continue
			}

			var req SubscriptionRequest
			if err := json.Unmarshal(data, &req); err != nil {
				continue
			}

			if req.Action == ActionSubscribe {
				for _, sub := range req.Subscriptions {
					// Send a mock response based on the topic
					var payload any
					switch sub.Topic {
					case "crypto_prices":
						payload = CryptoPrice{
							Symbol:    "btcusdt",
							Timestamp: time.Now().UnixMilli(),
							Value:     "65000.50",
						}
					case "crypto_prices_chainlink":
						payload = ChainlinkPrice{
							Symbol:    "eth/usd",
							Timestamp: time.Now().UnixMilli(),
							Value:     "3500.00",
						}
					case "comments":
						payload = Comment{
							ID:   "123",
							Body: "hello world",
						}
					}

					resp := RtdsMessage{
						Topic:     sub.Topic,
						Type:      "update",
						Timestamp: time.Now().UnixMilli(),
					}
					resp.Payload, _ = json.Marshal(payload)

					respData, _ := json.Marshal(resp)
					conn.Write(r.Context(), websocket.MessageText, respData)
				}
			}
		}
	}))
	defer server.Close()

	url := strings.Replace(server.URL, "http", "ws", 1)
	client := NewClient(url, nil)
	t.Cleanup(func() { client.Close() })

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("failed to connect: %v", err)
	}

	// Test Crypto Prices Subscription
	if err := client.SubscribeCryptoPrices(ctx, []string{"btcusdt"}); err != nil {
		t.Fatalf("failed to subscribe to crypto prices: %v", err)
	}

	// Test Chainlink Prices Subscription
	if err := client.SubscribeChainlinkPrices(ctx, "eth/usd"); err != nil {
		t.Fatalf("failed to subscribe to chainlink prices: %v", err)
	}

	// Test Comments Subscription
	if err := client.SubscribeComments(ctx, CommentCreated, nil); err != nil {
		t.Fatalf("failed to subscribe to comments: %v", err)
	}

	// Wait for messages
	receivedTopics := make(map[string]bool)
	for i := 0; i < 3; i++ {
		select {
		case msg := <-client.Messages():
			receivedTopics[msg.Topic] = true

			// Verify payload extraction helpers
			switch msg.Topic {
			case "crypto_prices":
				p, err := msg.AsCryptoPrice()
				if err != nil {
					t.Errorf("AsCryptoPrice failed: %v", err)
				}
				if p.Symbol != "btcusdt" {
					t.Errorf("expected symbol btcusdt, got %s", p.Symbol)
				}
			case "crypto_prices_chainlink":
				p, err := msg.AsChainlinkPrice()
				if err != nil {
					t.Errorf("AsChainlinkPrice failed: %v", err)
				}
				if p.Symbol != "eth/usd" {
					t.Errorf("expected symbol eth/usd, got %s", p.Symbol)
				}
			case "comments":
				p, err := msg.AsComment()
				if err != nil {
					t.Errorf("AsComment failed: %v", err)
				}
				if p.ID != "123" {
					t.Errorf("expected comment id 123, got %s", p.ID)
				}
			}
		case err := <-client.Errors():
			t.Fatalf("received error: %v", err)
		case <-ctx.Done():
			t.Fatalf("timed out waiting for messages, received: %v", receivedTopics)
		}
	}

	if len(receivedTopics) < 3 {
		t.Errorf("expected 3 topics, got %d: %v", len(receivedTopics), receivedTopics)
	}
}

func TestRTDSReconnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	connCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connCount++
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}

		// If it's the first connection, close it immediately to trigger reconnect
		if connCount == 1 {
			conn.Close(websocket.StatusGoingAway, "bye")
			return
		}

		// Second connection: stay open and handle one sub
		for {
			typ, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			if typ == websocket.MessageText && string(data) != "PING" {
				var req SubscriptionRequest
				if err := json.Unmarshal(data, &req); err == nil {
					resp := RtdsMessage{
						Topic: req.Subscriptions[0].Topic,
						Type:  "reconnected",
					}
					respData, _ := json.Marshal(resp)
					conn.Write(r.Context(), websocket.MessageText, respData)
				}
			}
		}
	}))
	defer server.Close()

	url := strings.Replace(server.URL, "http", "ws", 1)
	client := NewClient(url, nil)
	t.Cleanup(func() { client.Close() })

	// Pre-add a subscription so it resubscribes on reconnect
	client.subs = append(client.subs, Subscription{Topic: "reconnect_test", Type: "test"})

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("initial connect failed: %v", err)
	}

	// Wait for reconnection and message
	select {
	case msg := <-client.Messages():
		if msg.Topic != "reconnect_test" {
			t.Errorf("expected topic reconnect_test, got %s", msg.Topic)
		}
		if msg.Type != "reconnected" {
			t.Errorf("expected type reconnected, got %s", msg.Type)
		}
	case <-ctx.Done():
		t.Fatalf("timed out waiting for reconnect message, connCount: %d", connCount)
	}

	if connCount < 2 {
		t.Errorf("expected at least 2 connections, got %d", connCount)
	}
}

func TestRTDSHeartbeatTimeoutTriggersReconnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	reconnected := make(chan struct{}, 1)
	connCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connCount++
		current := connCount

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		for {
			typ, data, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			if typ != websocket.MessageText || string(data) != "PING" {
				continue
			}

			if current == 1 {
				continue
			}

			select {
			case reconnected <- struct{}{}:
			default:
			}
			_ = conn.Write(r.Context(), websocket.MessageText, []byte("PONG"))
		}
	}))
	defer server.Close()

	url := strings.Replace(server.URL, "http", "ws", 1)
	client := NewClient(url, nil)
	client.heartbeatInterval = 25 * time.Millisecond
	client.heartbeatTimeout = 25 * time.Millisecond
	t.Cleanup(func() { client.Close() })

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect failed: %v", err)
	}

	select {
	case <-reconnected:
	case <-ctx.Done():
		t.Fatalf("timed out waiting for reconnect, connCount=%d", connCount)
	}

	if connCount < 2 {
		t.Fatalf("expected reconnect after heartbeat timeout, connCount=%d", connCount)
	}
}
