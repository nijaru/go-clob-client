package clob

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	json "github.com/go-json-experiment/json"
)

func BenchmarkCalculateMarketPrice(b *testing.B) {
	asks := []OrderSummary{
		{Price: "0.65", Size: "80"},
		{Price: "0.60", Size: "60"},
		{Price: "0.55", Size: "40"},
		{Price: "0.50", Size: "30"},
		{Price: "0.45", Size: "20"},
		{Price: "0.40", Size: "10"},
	}
	bids := []OrderSummary{
		{Price: "0.25", Size: "35"},
		{Price: "0.30", Size: "25"},
		{Price: "0.35", Size: "15"},
		{Price: "0.40", Size: "10"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data, _ := json.Marshal(OrderBookSummary{
			Market:  "test",
			AssetID: "tok",
			Asks:    asks,
			Bids:    bids,
		})
		_, _ = w.Write(data)
	}))
	b.Cleanup(server.Close)

	client, err := NewClient(Config{Host: server.URL})
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()

	b.Run("buy_usdc", func(b *testing.B) {
		for b.Loop() {
			price, err := client.CalculateMarketPrice(
				ctx, "tok", SideBuy, MustDec("100"), OrderTypeFOK,
			)
			if err != nil {
				b.Fatal(err)
			}
			if price.IsZero() {
				b.Fatal("expected non-zero price")
			}
		}
	})

	b.Run("sell_shares", func(b *testing.B) {
		for b.Loop() {
			price, err := client.CalculateMarketPrice(
				ctx, "tok", SideSell, MustDec("50"), OrderTypeFOK,
			)
			if err != nil {
				b.Fatal(err)
			}
			if price.IsZero() {
				b.Fatal("expected non-zero price")
			}
		}
	})
}
