package clob

import "testing"

func TestGetOrderBookHash(t *testing.T) {
	t.Parallel()

	client, err := New(Config{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	hash, err := client.GetOrderBookHash(OrderBookSummary{
		Market:    "0xaabbcc",
		AssetID:   "100",
		Timestamp: "123456789",
		Bids: []OrderSummary{
			{Price: "0.3", Size: "100"},
			{Price: "0.4", Size: "100"},
		},
		Asks: []OrderSummary{
			{Price: "0.6", Size: "100"},
			{Price: "0.7", Size: "100"},
		},
		MinOrderSize:   "100",
		TickSize:       "0.01",
		NegRisk:        false,
		LastTradePrice: "0.5",
		Hash:           "ignored",
	})
	if err != nil {
		t.Fatalf("get orderbook hash: %v", err)
	}

	if hash != "0458ea5755c9f73d64a14636fa5c36ed460ec394" {
		t.Fatalf("unexpected orderbook hash: %s", hash)
	}
}

func TestGetOrderBookHashEmptyBook(t *testing.T) {
	t.Parallel()

	client, err := New(Config{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	hash, err := client.GetOrderBookHash(OrderBookSummary{
		Market:         "0xaabbcc",
		AssetID:        "100",
		Timestamp:      "",
		Bids:           []OrderSummary{},
		Asks:           []OrderSummary{},
		MinOrderSize:   "100",
		TickSize:       "0.01",
		NegRisk:        false,
		LastTradePrice: "0.5",
	})
	if err != nil {
		t.Fatalf("get orderbook hash: %v", err)
	}

	if hash != "74c6a7c81c1d572f1c877b7d3e25b80c336d8a6e" {
		t.Fatalf("unexpected empty-book hash: %s", hash)
	}
}
