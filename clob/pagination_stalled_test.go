package clob

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPaginateRefusesRepeatedCursor(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		// The service reports more rows but never advances the cursor.
		id := "m1"
		if r.URL.Query().Get("next_cursor") == "cursor-2" {
			id = "m2"
		}
		_, _ = w.Write([]byte(
			`{"limit":1,"count":1,"next_cursor":"cursor-2","data":[{"id":"` + id + `"}]}`,
		))
	}))
	defer server.Close()

	client, err := NewClient(Config{Host: server.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	var yieldErr error
	count := 0
	for market, err := range client.IterMarkets(t.Context()) {
		_ = market
		if err != nil {
			yieldErr = err
			break
		}
		count++
	}
	if !errors.Is(yieldErr, ErrPaginationStalled) {
		t.Fatalf("error = %v, want ErrPaginationStalled", yieldErr)
	}
	// Each page's rows are yielded, then the repeated cursor is caught instead
	// of looping forever.
	if count != 2 || requests != 2 {
		t.Fatalf("count=%d requests=%d", count, requests)
	}
}

func TestPaginateStopsAtEndSentinel(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"limit":1,"count":1,"next_cursor":"LTE=","data":[{"id":"m"}]}`))
	}))
	defer server.Close()

	client, err := NewClient(Config{Host: server.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	markets, err := Collect(client.IterMarkets(t.Context()))
	if err != nil || len(markets) != 1 {
		t.Fatalf("markets=%d error=%v", len(markets), err)
	}
}
