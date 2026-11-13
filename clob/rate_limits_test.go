package clob

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

func TestRequestBudgetsKeepCancelAndReadResponsive(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
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
	if err := client.postTradingJSON(waitCtx, "/order", nil, polyhttp.AuthNone, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second placement error = %v; want deadline", err)
	}
	if err := client.cancelTradingJSON(t.Context(), "/cancel", nil, polyhttp.AuthNone, nil); err != nil {
		t.Fatalf("cancel blocked by placement budget: %v", err)
	}
	var result struct{}
	if err := client.getJSON(t.Context(), "/read", nil, polyhttp.AuthNone, &result); err != nil {
		t.Fatalf("read blocked by trading budget: %v", err)
	}
	if calls != 3 {
		t.Fatalf("server calls = %d, want 3 (order, cancel, read)", calls)
	}
}
