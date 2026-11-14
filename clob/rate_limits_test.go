package clob

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

func TestRequestBudgetsKeepCancelAndReadResponsive(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{
		Host: server.URL,
		Rates: RateLimits{
			Orders:  RateBudget{PerSecond: 1, Burst: 1},
			Cancels: RateBudget{PerSecond: 1, Burst: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.postTradingJSON(t.Context(), "/order", nil, polyhttp.AuthNone, nil); err != nil {
		t.Fatal(err)
	}
	waitCtx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()
	if err := client.postTradingJSON(waitCtx, "/order", nil, polyhttp.AuthNone, nil); !errors.Is(
		err,
		context.DeadlineExceeded,
	) {
		t.Fatalf("second placement error = %v; want deadline", err)
	}
	if err := client.cancelTradingJSON(t.Context(), "/cancel", nil, polyhttp.AuthNone, nil); err != nil {
		t.Fatalf("cancel blocked by placement budget: %v", err)
	}
	var result struct{}
	if err := client.getJSON(t.Context(), "/read", nil, polyhttp.AuthNone, &result); err != nil {
		t.Fatalf("read blocked by trading budget: %v", err)
	}
	if calls.Load() != 3 {
		t.Fatalf("server calls = %d, want 3 (order, cancel, read)", calls.Load())
	}
}

// Market-scoped cancellation and account heartbeats must be able to proceed
// while unrelated API requests exhaust their optional local budget.
func TestAccountControlLanesIgnoreOtherRateBudget(t *testing.T) {
	t.Parallel()
	var paths []string
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case cancelMarketOrdersEndpoint:
			_, _ = w.Write([]byte(`{"canceled":[]}`))
		case heartbeatsEndpoint:
			_, _ = w.Write([]byte(`{"heartbeat_id":"next"}`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer server.Close()

	client, err := NewAuthenticatedClient(Config{
		Host:        server.URL,
		PrivateKey:  orderResolutionPrivateKey,
		Credentials: &Credentials{Key: "key", Secret: "c2VjcmV0", Passphrase: "pass"},
		Rates:       RateLimits{Other: RateBudget{PerSecond: 1, Burst: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if err := client.postJSON(t.Context(), "/probe", nil, polyhttp.AuthNone, nil); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
	defer cancel()
	if _, err := client.CancelMarketOrders(ctx, CancelMarketOrdersRequest{Market: "market"}); err != nil {
		t.Fatalf("cancel delayed by unrelated requests: %v", err)
	}
	heartbeat, err := client.PostHeartbeat(ctx, "")
	if err != nil {
		t.Fatalf("heartbeat delayed by unrelated requests: %v", err)
	}
	if heartbeat.HeartbeatID != "next" {
		t.Fatalf("unexpected heartbeat: %+v", heartbeat)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(paths) != 3 {
		t.Fatalf("request paths = %v, want probe, cancel market, heartbeat", paths)
	}
}
