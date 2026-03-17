package clob

import (
	json "github.com/go-json-experiment/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quagmt/udecimal"
)

// These tests verify CalculateMarketPrice matches the official SDKs'
// (Rust/TypeScript/Python) logic:
//   - For BUY: accumulate size*price across asks (descending), return price when sum >= amount
//   - For SELL: accumulate size across bids (ascending), return price when sum >= amount
//   - On FOK with insufficient liquidity, return error
//   - On GTC with insufficient liquidity, return worst price (first element)

func TestCalculateMarketPriceBuyFOK(t *testing.T) {
	t.Parallel()

	// Asks sorted descending (worst to best): 0.50@30, 0.45@20, 0.40@10
	server := newOrderBookServer(t, []OrderSummary{
		{Price: "0.50", Size: "30"},
		{Price: "0.45", Size: "20"},
		{Price: "0.40", Size: "10"},
	}, nil)
	defer server.Close()

	clientRaw, _ := New(Config{Host: server.URL})
	client := clientRaw.(*Client)
	ctx := t.Context()

	// amount=5: 10*0.4=4 < 5, next: 4+20*0.45=13 >= 5, return 0.45
	price, err := client.CalculateMarketPrice(
		ctx,
		"tok",
		SideBuy,
		udecimal.MustParse("5"),
		OrderTypeFOK,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price.Cmp(udecimal.MustParse("0.45")) != 0 {
		t.Errorf("BUY amount=5: got %s, want 0.45", price.String())
	}

	// amount=20: 4 < 20, 4+9=13 < 20, 13+30*0.5=28 >= 20, return 0.50
	price, err = client.CalculateMarketPrice(
		ctx,
		"tok",
		SideBuy,
		udecimal.MustParse("20"),
		OrderTypeFOK,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price.Cmp(udecimal.MustParse("0.5")) != 0 {
		t.Errorf("BUY amount=20: got %s, want 0.5", price.String())
	}
}

func TestCalculateMarketPriceSellFOK(t *testing.T) {
	t.Parallel()

	// Bids sorted ascending (best to worst): 0.25@35, 0.30@25, 0.35@15
	server := newOrderBookServer(t, nil, []OrderSummary{
		{Price: "0.25", Size: "35"},
		{Price: "0.30", Size: "25"},
		{Price: "0.35", Size: "15"},
	})
	defer server.Close()

	clientRaw, _ := New(Config{Host: server.URL})
	client := clientRaw.(*Client)
	ctx := t.Context()

	// amount=10: 15 >= 10 (best bid 0.35), return 0.35
	price, err := client.CalculateMarketPrice(
		ctx,
		"tok",
		SideSell,
		udecimal.MustParse("10"),
		OrderTypeFOK,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price.Cmp(udecimal.MustParse("0.35")) != 0 {
		t.Errorf("SELL amount=10: got %s, want 0.35", price.String())
	}

	// amount=30: 15 < 30, 15+25=40 >= 30, return 0.30
	price, err = client.CalculateMarketPrice(
		ctx,
		"tok",
		SideSell,
		udecimal.MustParse("30"),
		OrderTypeFOK,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price.Cmp(udecimal.MustParse("0.3")) != 0 {
		t.Errorf("SELL amount=30: got %s, want 0.3", price.String())
	}
}

func TestCalculateMarketPriceInsufficientLiquidity(t *testing.T) {
	t.Parallel()

	// Only 10 shares available at best ask 0.40
	server := newOrderBookServer(t, []OrderSummary{
		{Price: "0.50", Size: "30"},
		{Price: "0.40", Size: "10"},
	}, nil)
	defer server.Close()

	clientRaw, _ := New(Config{Host: server.URL})
	client := clientRaw.(*Client)
	ctx := t.Context()

	// FOK: should fail when total < amount
	_, err := client.CalculateMarketPrice(
		ctx,
		"tok",
		SideBuy,
		udecimal.MustParse("100"),
		OrderTypeFOK,
	)
	if err == nil {
		t.Fatal("FOK should fail with insufficient liquidity")
	}

	// GTC: should return worst available price (first element)
	price, err := client.CalculateMarketPrice(
		ctx,
		"tok",
		SideBuy,
		udecimal.MustParse("100"),
		OrderTypeGTC,
	)
	if err != nil {
		t.Fatalf("GTC should not fail: %v", err)
	}
	if price.Cmp(udecimal.MustParse("0.5")) != 0 {
		t.Errorf("GTC fallback: got %s, want 0.5", price.String())
	}
}

func TestCalculateMarketPriceEdgeCases(t *testing.T) {
	t.Parallel()

	server := newOrderBookServer(t, []OrderSummary{{Price: "0.50", Size: "10"}}, nil)
	defer server.Close()

	clientRaw, _ := New(Config{Host: server.URL})
	client := clientRaw.(*Client)
	ctx := t.Context()

	// Zero amount should error
	_, err := client.CalculateMarketPrice(ctx, "tok", SideBuy, udecimal.Zero, OrderTypeFOK)
	if err == nil {
		t.Fatal("zero amount should error")
	}

	// Exact match: amount equals level size * price
	// 10 * 0.50 = 5, amount=5 should return 0.50
	price, err := client.CalculateMarketPrice(
		ctx,
		"tok",
		SideBuy,
		udecimal.MustParse("5"),
		OrderTypeFOK,
	)
	if err != nil {
		t.Fatalf("exact match: %v", err)
	}
	if price.Cmp(udecimal.MustParse("0.5")) != 0 {
		t.Errorf("exact match: got %s, want 0.5", price.String())
	}
}

func newOrderBookServer(t *testing.T, asks, bids []OrderSummary) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data, _ := json.Marshal(OrderBookSummary{
			Market:  "test",
			AssetID: "tok",
			Asks:    asks,
			Bids:    bids,
		})
		w.Write(data)
	}))
}
