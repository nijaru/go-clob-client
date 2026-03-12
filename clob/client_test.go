package clob

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetOrderBook(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != orderBookEndpoint {
			t.Fatalf("unexpected path: %s", got)
		}
		if got := r.URL.Query().Get("token_id"); got != "123" {
			t.Fatalf("unexpected token_id: %s", got)
		}

		_ = json.NewEncoder(w).Encode(OrderBookSummary{
			Market:         "market-1",
			AssetID:        "123",
			Timestamp:      "1710000000",
			Bids:           []OrderSummary{{Price: "0.45", Size: "10"}},
			Asks:           []OrderSummary{{Price: "0.55", Size: "12"}},
			MinOrderSize:   "5",
			TickSize:       "0.01",
			NegRisk:        false,
			LastTradePrice: "0.50",
			Hash:           "abc",
		})
	}))
	defer server.Close()

	client, err := New(Config{Host: server.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	book, err := client.GetOrderBook(context.Background(), "123")
	if err != nil {
		t.Fatalf("get order book: %v", err)
	}

	if book.AssetID != "123" {
		t.Fatalf("unexpected asset id: %s", book.AssetID)
	}
}

func TestCreateOrDeriveAPIKeyFallsBackToDerive(t *testing.T) {
	t.Parallel()

	privateKey := "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case createAPIKeyEndpoint:
			http.Error(w, `{"error":"exists"}`, http.StatusConflict)
		case deriveAPIKeyEndpoint:
			_ = json.NewEncoder(w).Encode(apiKeyRaw{
				APIKey:     "key",
				Secret:     "c2VjcmV0",
				Passphrase: "pass",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := New(Config{Host: server.URL, PrivateKey: privateKey})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	creds, err := client.CreateOrDeriveAPIKey(context.Background(), 0)
	if err != nil {
		t.Fatalf("create or derive: %v", err)
	}

	if creds.Key != "key" {
		t.Fatalf("unexpected api key: %s", creds.Key)
	}
}
