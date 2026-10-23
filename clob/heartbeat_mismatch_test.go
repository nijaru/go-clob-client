package clob

import (
	stdjson "encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHeartbeatMismatchError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != heartbeatsEndpoint {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		// A mismatch names the ID the service expects; it can be empty after a
		// legacy heartbeat, so presence rather than non-emptiness matters.
		_, _ = w.Write([]byte(`{"error_msg":"Invalid Heartbeat ID","heartbeat_id":""}`))
	}))
	defer server.Close()

	client := newHeartbeatTestClient(t, server.URL)
	_, err := client.PostHeartbeat(t.Context(), "stale")
	mismatch, ok := errors.AsType[*HeartbeatMismatchError](err)
	if !ok {
		t.Fatalf("expected *HeartbeatMismatchError, got %v", err)
	}
	if mismatch.HeartbeatID != "" {
		t.Fatalf("expected ID = %q, want empty", mismatch.HeartbeatID)
	}
	if mismatch.StatusCode != http.StatusBadRequest || !errors.Is(err, ErrBadRequest) {
		t.Fatalf("mismatch lost the API error: %+v", mismatch.APIError)
	}

	// A 400 without the mismatch envelope stays an ordinary API error.
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"something else"}`))
	}))
	defer plain.Close()
	if _, err := newHeartbeatTestClient(t, plain.URL).PostHeartbeat(t.Context(), "id"); err == nil {
		t.Fatal("expected an API error")
	} else if _, ok := errors.AsType[*HeartbeatMismatchError](err); ok {
		t.Fatalf("unrelated 400 reported as a heartbeat mismatch: %v", err)
	}
}

func TestHeartbeatLoopRecoversFromStaleID(t *testing.T) {
	t.Parallel()

	accepted := make(chan string, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != heartbeatsEndpoint {
			http.NotFound(w, r)
			return
		}
		var body struct {
			HeartbeatID string `json:"heartbeat_id"`
		}
		if err := stdjson.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch body.HeartbeatID {
		case "hb-1":
			// The second send raced another owner of the same API key.
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error_msg":"Invalid Heartbeat ID","heartbeat_id":"hb-2"}`))
		case "hb-2":
			accepted <- body.HeartbeatID
			_, _ = w.Write([]byte(`{"heartbeat_id":"hb-3"}`))
		default:
			accepted <- body.HeartbeatID
			_, _ = w.Write([]byte(`{"heartbeat_id":"hb-1"}`))
		}
	}))
	defer server.Close()

	client := newHeartbeatTestClient(t, server.URL)
	if err := client.StartHeartbeats(t.Context()); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(2 * time.Second)
	for {
		select {
		case id := <-accepted:
			if id == "hb-2" {
				if err := client.Close(); err != nil {
					t.Fatal(err)
				}
				return
			}
		case <-deadline:
			t.Fatal("heartbeat loop never adopted the expected ID")
		}
	}
}

func newHeartbeatTestClient(t *testing.T, host string) *AuthenticatedClient {
	t.Helper()
	client, err := NewAuthenticatedClient(Config{
		Host:              host,
		PrivateKey:        "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		Credentials:       &Credentials{Key: "key", Secret: "c2VjcmV0", Passphrase: "pass"},
		HeartbeatInterval: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new authenticated client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}
