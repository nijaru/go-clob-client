package data

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkGetPositions(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]Position{
			{
				ProxyWallet:  "0x123",
				Asset:        "100",
				ConditionID:  "market-1",
				Size:         udecimalMustParse("12.5"),
				AvgPrice:     udecimalMustParse("0.45"),
				CurrentValue: udecimalMustParse("5.625"),
				Title:        "Test Market",
			},
		})
	}))
	defer server.Close()

	client := New(Config{Host: server.URL})
	ctx := b.Context()

	for b.Loop() {
		items, err := client.GetPositions(ctx, PositionParams{User: "0x123"})
		if err != nil {
			b.Fatalf("GetPositions: %v", err)
		}
		if len(items) != 1 {
			b.Fatalf("unexpected item count: %d", len(items))
		}
	}
}

func BenchmarkIterPositionsSinglePage(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("offset") != "" {
			_ = json.NewEncoder(w).Encode([]Position{})
			return
		}
		_ = json.NewEncoder(w).Encode([]Position{
			{Asset: "100", Title: "A", Size: udecimalMustParse("1")},
			{Asset: "101", Title: "B", Size: udecimalMustParse("2")},
			{Asset: "102", Title: "C", Size: udecimalMustParse("3")},
		})
	}))
	defer server.Close()

	client := New(Config{Host: server.URL})
	ctx := b.Context()

	for b.Loop() {
		count := 0
		for _, err := range client.IterPositions(ctx, PositionParams{User: "0x123", Limit: 3}) {
			if err != nil {
				b.Fatalf("IterPositions: %v", err)
			}
			count++
		}
		if count != 3 {
			b.Fatalf("unexpected count: %d", count)
		}
	}
}

func BenchmarkGetTradesWithFilter(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("filterType"); got != "CASH" {
			b.Fatalf("filterType = %q", got)
		}
		if got := r.URL.Query().Get("filterAmount"); got != "100" {
			b.Fatalf("filterAmount = %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]Trade{
			{
				Side:  SideBuy,
				Size:  udecimalMustParse("10"),
				Price: udecimalMustParse("0.55"),
			},
		})
	}))
	defer server.Close()

	client := New(Config{Host: server.URL})
	ctx := b.Context()

	for b.Loop() {
		items, err := client.GetTrades(ctx, TradeParams{
			User:        "0x123",
			TradeFilter: &TradeFilter{FilterType: FilterTypeCash, FilterAmount: udecimalMustParse("100")},
		})
		if err != nil {
			b.Fatalf("GetTrades: %v", err)
		}
		if len(items) != 1 {
			b.Fatalf("unexpected item count: %d", len(items))
		}
	}
}
