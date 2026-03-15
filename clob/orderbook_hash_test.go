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

	if hash != "40c73ef796bc06f9c6b6f40ac40726dd0584cad8" {
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

	if hash != "b83e2ea87740aba865fbca0077679e8a606da38d" {
		t.Fatalf("unexpected empty-book hash: %s", hash)
	}
}
