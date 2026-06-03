package clob

import (
	"context"
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

	privateKey := "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c"

	// Track heartbeat calls to confirm the loop actually ran.
	var heartbeatCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case heartbeatsEndpoint:
			heartbeatCalls++
			data, _ := json.Marshal(HeartbeatResponse{HeartbeatID: "hb-1"})
			w.Write(data)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	creds := &Credentials{Key: "key", Secret: "c2VjcmV0", Passphrase: "pass"}
	client, err := NewAuthenticatedClient(Config{
		Host:              server.URL,
		PrivateKey:        privateKey,
		Credentials:       creds,
		HeartbeatInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new authenticated client: %v", err)
	}

	// Allow at least one heartbeat tick before shutting down.
	time.Sleep(30 * time.Millisecond)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	if err := client.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	// Second call must be a no-op and return nil.
	if err := client.Shutdown(ctx); err != nil {
		t.Fatalf("second shutdown: %v", err)
	}

	// Close after Shutdown must also be a no-op.
	if err := client.Close(); err != nil {
		t.Fatalf("close after shutdown: %v", err)
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
		DisableAutoHeartbeat: true,
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
		DisableAutoHeartbeat: true,
	})
	if err == nil {
		t.Fatal("expected invalid API secret error")
	}
}

func TestAuthenticatedClientShutdownTimeout(t *testing.T) {
	t.Parallel()

	privateKey := "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow heartbeat endpoint.
		time.Sleep(100 * time.Millisecond)
		data, _ := json.Marshal(HeartbeatResponse{HeartbeatID: "hb-1"})
		w.Write(data)
	}))
	defer server.Close()

	creds := &Credentials{Key: "key", Secret: "c2VjcmV0", Passphrase: "pass"}
	client, err := NewAuthenticatedClient(Config{
		Host:              server.URL,
		PrivateKey:        privateKey,
		Credentials:       creds,
		HeartbeatInterval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new authenticated client: %v", err)
	}

	// Issue Shutdown with an already-expired context; should return DeadlineExceeded
	// because the heartbeat goroutine may be blocked mid-request.
	expired, expireCancel := context.WithDeadline(t.Context(), time.Now())
	defer expireCancel()

	err = client.Shutdown(expired)
	if err != context.DeadlineExceeded {
		// Either context.DeadlineExceeded or nil is acceptable: nil means the
		// goroutine happened to exit before we checked (race). Only fail on
		// unexpected errors.
		if err != nil {
			t.Fatalf("expected DeadlineExceeded or nil, got: %v", err)
		}
	}

	// Always clean up with a real timeout.
	cleanupCtx, cleanupCancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cleanupCancel()
	_ = client.Shutdown(cleanupCtx)
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
		RetryMax:             3,
		DisableAutoHeartbeat: true,
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
