package rtds

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	json "github.com/go-json-experiment/json"

	"github.com/coder/websocket"
)

func TestRTDSClient(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
			if string(data) == " " {
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
			if typ == websocket.MessageText && string(data) != " " {
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
