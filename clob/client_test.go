package clob

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	json "github.com/go-json-experiment/json"
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

		data, _ := json.Marshal(OrderBookSummary{
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
		w.Write(data)
	}))
	defer server.Close()

	client, err := NewClient(Config{Host: server.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	book, err := client.GetOrderBook(t.Context(), "123")
	if err != nil {
		t.Fatalf("get order book: %v", err)
	}

	if book.AssetID != "123" {
		t.Fatalf("unexpected asset id: %s", book.AssetID)
	}
}

func TestAuthenticatedClientShutdown(t *testing.T) {
	t.Parallel()
	entered := make(chan struct{})
	canceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(entered)
		<-r.Context().Done()
		close(canceled)
	}))
	defer server.Close()
	client, err := NewAuthenticatedClient(Config{
		Host:              server.URL,
		PrivateKey:        "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		Credentials:       &Credentials{Key: "key", Secret: "c2VjcmV0", Passphrase: "pass"},
		HeartbeatInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if client.HeartbeatsActive() {
		t.Fatal("constructor started background work")
	}
	ctx, cancel := context.WithCancel(t.Context())
	if err := client.StartHeartbeats(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("heartbeat did not start")
	}
	cancel()
	join, stop := context.WithTimeout(t.Context(), time.Second)
	defer stop()
	if err := client.StopHeartbeats(join); err != nil {
		t.Fatal(err)
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("request context was not canceled")
	}
	if client.HeartbeatsActive() {
		t.Fatal("loop still active after join")
	}
	if err := client.Shutdown(join); err != nil {
		t.Fatal(err)
	}
	if err := client.Shutdown(join); err != nil {
		t.Fatal(err)
	}
}

func TestNewAuthenticatedClientDecodesAPISecret(t *testing.T) {
	t.Parallel()

	privateKey := "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c"
	client, err := NewAuthenticatedClient(Config{
		PrivateKey: privateKey,
		Credentials: &Credentials{
			Key:        "key",
			Secret:     "c2VjcmV0",
			Passphrase: "pass",
		},
	})
	if err != nil {
		t.Fatalf("new authenticated client: %v", err)
	}
	if got := string(client.decodedSecret); got != "secret" {
		t.Fatalf("decoded secret = %q, want secret", got)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	_, err = NewAuthenticatedClient(Config{
		PrivateKey: privateKey,
		Credentials: &Credentials{
			Key:        "key",
			Secret:     "*",
			Passphrase: "pass",
		},
	})
	if err == nil {
		t.Fatal("expected invalid API secret error")
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
			data, _ := json.Marshal(apiKeyRaw{
				APIKey:     "key",
				Secret:     "c2VjcmV0",
				Passphrase: "pass",
			})
			w.Write(data)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewSignerClient(Config{Host: server.URL, PrivateKey: privateKey})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	creds, err := client.CreateOrDeriveAPIKey(t.Context(), 0)
	if err != nil {
		t.Fatalf("create or derive: %v", err)
	}

	if creds.Key != "key" {
		t.Fatalf("unexpected api key: %s", creds.Key)
	}
}

func TestPostJSONDoesNotRetryDecodeFailures(t *testing.T) {
	t.Parallel()

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != heartbeatsEndpoint {
			http.NotFound(w, r)
			return
		}
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"heartbeat_id":`))
	}))
	defer server.Close()

	client, err := NewAuthenticatedClient(Config{
		Host:       server.URL,
		PrivateKey: "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		Credentials: &Credentials{
			Key:        "key",
			Secret:     "c2VjcmV0",
			Passphrase: "pass",
		},
		RetryMax: 3,
	})
	if err != nil {
		t.Fatalf("new authenticated client: %v", err)
	}

	_, err = client.PostHeartbeat(t.Context(), "")
	if err == nil {
		t.Fatal("expected decode error")
	}
	if calls != 1 {
		t.Fatalf("post heartbeat retried %d times, want 1", calls)
	}
}

func TestGetJSONRetriesTransientServerFailures(t *testing.T) {
	t.Parallel()

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != orderBookEndpoint {
			http.NotFound(w, r)
			return
		}
		calls++
		if calls == 1 {
			http.Error(w, `{"error":"temporary"}`, http.StatusInternalServerError)
			return
		}

		data, _ := json.Marshal(OrderBookSummary{
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
		w.Write(data)
	}))
	defer server.Close()

	client, err := NewClient(Config{
		Host:         server.URL,
		RetryMax:     1,
		RetryBackoff: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	book, err := client.GetOrderBook(t.Context(), "123")
	if err != nil {
		t.Fatalf("get order book: %v", err)
	}
	if book.AssetID != "123" {
		t.Fatalf("unexpected asset id: %s", book.AssetID)
	}
	if calls != 2 {
		t.Fatalf("get order book calls = %d, want 2", calls)
	}
}

func TestCreateAPIKeyUsesNoncePOSTWithoutRetry(t *testing.T) {
	t.Parallel()

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != createAPIKeyEndpoint {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		calls++
		http.Error(w, `{"error":"temporary"}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := NewSignerClient(Config{
		Host:         server.URL,
		PrivateKey:   "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		RetryMax:     3,
		RetryBackoff: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new signer client: %v", err)
	}

	_, err = client.CreateAPIKey(t.Context(), 7)
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Fatalf("create api key calls = %d, want 1", calls)
	}
}
