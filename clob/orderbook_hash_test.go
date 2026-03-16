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

func TestGetOrderBookHashNilSlices(t *testing.T) {
	t.Parallel()

	client, err := New(Config{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	// nil bids/asks should produce the same hash as empty slices
	hashNil, err := client.GetOrderBookHash(OrderBookSummary{
		Market:         "0xaabbcc",
		AssetID:        "100",
		Timestamp:      "123",
		MinOrderSize:   "100",
		TickSize:       "0.01",
		NegRisk:        false,
		LastTradePrice: "0.5",
	})
	if err != nil {
		t.Fatalf("get orderbook hash (nil): %v", err)
	}

	hashEmpty, err := client.GetOrderBookHash(OrderBookSummary{
		Market:         "0xaabbcc",
		AssetID:        "100",
		Timestamp:      "123",
		Bids:           []OrderSummary{},
		Asks:           []OrderSummary{},
		MinOrderSize:   "100",
		TickSize:       "0.01",
		NegRisk:        false,
		LastTradePrice: "0.5",
	})
	if err != nil {
		t.Fatalf("get orderbook hash (empty): %v", err)
	}

	if hashNil != hashEmpty {
		t.Errorf(
			"nil and empty slices should produce same hash: nil=%s empty=%s",
			hashNil,
			hashEmpty,
		)
	}
}

func TestOrderBookHashMatchesPython(t *testing.T) {
	t.Parallel()

	// Computed by Python SDK for the same payload:
	// python3 -c "import hashlib,json; p={'market':'0x123','asset_id':'456','timestamp':'789','hash':'',
	//   'bids':[{'price':'0.45','size':'10'}],'asks':[{'price':'0.55','size':'20'}],
	//   'min_order_size':'5','tick_size':'0.01','neg_risk':False,'last_trade_price':'0.50'};
	//   print(hashlib.sha1(json.dumps(p,separators=(',',':')).encode()).hexdigest())"
	expectedHash := "6d87cb5de10f3372dcd425c5dd5b90dd20ca3a7b"

	client, err := New(Config{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	hash, err := client.GetOrderBookHash(OrderBookSummary{
		Market:         "0x123",
		AssetID:        "456",
		Timestamp:      "789",
		Bids:           []OrderSummary{{Price: "0.45", Size: "10"}},
		Asks:           []OrderSummary{{Price: "0.55", Size: "20"}},
		MinOrderSize:   "5",
		TickSize:       "0.01",
		NegRisk:        false,
		LastTradePrice: "0.50",
	})
	if err != nil {
		t.Fatalf("get orderbook hash: %v", err)
	}

	if hash != expectedHash {
		t.Errorf("hash mismatch with Python SDK:\n  got:  %s\n  want: %s", hash, expectedHash)
	}
}
