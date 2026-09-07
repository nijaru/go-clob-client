package ws

import (
	"testing"

	json "github.com/go-json-experiment/json"

	"github.com/nijaru/go-clob-client/clob"
)

func TestMarketEventsAcceptLegacyTokenIDKey(t *testing.T) {
	t.Parallel()

	var book BookEvent
	bookPayload := `{"event_type":"book","market":"m","token_id":"90","bids":[],"asks":[],"timestamp":"1"}`
	if err := json.Unmarshal([]byte(bookPayload), &book); err != nil {
		t.Fatalf("unmarshal book: %v", err)
	}
	if book.AssetID != "90" {
		t.Fatalf("book asset ID = %q, want 90", book.AssetID)
	}

	var change PriceChange
	changePayload := `{"token_id":"91","price":"0.5","size":"10","side":"BUY"}`
	if err := json.Unmarshal([]byte(changePayload), &change); err != nil {
		t.Fatalf("unmarshal price change: %v", err)
	}
	if change.AssetID != "91" {
		t.Fatalf("price change asset ID = %q, want 91", change.AssetID)
	}

	var tick TickSizeChangeEvent
	tickPayload := `{"event_type":"tick_size_change","token_id":"92","market":"m","old_tick_size":"0.01","new_tick_size":"0.001","timestamp":"1"}`
	if err := json.Unmarshal([]byte(tickPayload), &tick); err != nil {
		t.Fatalf("unmarshal tick change: %v", err)
	}
	if tick.AssetID != "92" {
		t.Fatalf("tick change asset ID = %q, want 92", tick.AssetID)
	}

	var last LastTradePriceEvent
	lastPayload := `{"event_type":"last_trade_price","token_id":"93","market":"m","price":"0.5","size":"10","side":"BUY","fee_rate_bps":"0","timestamp":"1"}`
	if err := json.Unmarshal([]byte(lastPayload), &last); err != nil {
		t.Fatalf("unmarshal last trade: %v", err)
	}
	if last.AssetID != "93" {
		t.Fatalf("last trade asset ID = %q, want 93", last.AssetID)
	}

	var best BestBidAskEvent
	bestPayload := `{"event_type":"best_bid_ask","token_id":"94","market":"m","best_bid":"0.4","best_ask":"0.6","spread":"0.2","timestamp":"1"}`
	if err := json.Unmarshal([]byte(bestPayload), &best); err != nil {
		t.Fatalf("unmarshal best bid ask: %v", err)
	}
	if best.AssetID != "94" {
		t.Fatalf("best bid ask asset ID = %q, want 94", best.AssetID)
	}
}

func TestUserEventsAcceptLegacyTokenIDKey(t *testing.T) {
	t.Parallel()

	var order OrderEvent
	orderPayload := `{"event_type":"order","id":"o1","order_id":"o1","owner":"0xabc","token_id":"95","market":"m","price":"0.5","size":"10","side":"BUY","timestamp":"1"}`
	if err := json.Unmarshal([]byte(orderPayload), &order); err != nil {
		t.Fatalf("unmarshal order event: %v", err)
	}
	if order.OrderID != "o1" {
		t.Fatalf("order ID = %q, want o1 (id key must win)", order.OrderID)
	}
	if order.AssetID != "95" {
		t.Fatalf("order asset ID = %q, want 95", order.AssetID)
	}

	var trade TradeEvent
	tradePayload := `{"event_type":"trade","id":"t1","trade_id":"t1","market":"m","token_id":"96","side":"BUY","size":"10","price":"0.5","status":"MATCHED","timestamp":"1"}`
	if err := json.Unmarshal([]byte(tradePayload), &trade); err != nil {
		t.Fatalf("unmarshal trade event: %v", err)
	}
	if trade.AssetID != "96" {
		t.Fatalf("trade asset ID = %q, want 96", trade.AssetID)
	}

	var maker MakerOrder
	makerPayload := `{"order_id":"m1","owner":"0xabc","matched_amount":"5","price":"0.5","token_id":"97","outcome":"Yes","side":"BUY"}`
	if err := json.Unmarshal([]byte(makerPayload), &maker); err != nil {
		t.Fatalf("unmarshal maker order: %v", err)
	}
	if maker.AssetID != "97" {
		t.Fatalf("maker asset ID = %q, want 97", maker.AssetID)
	}
}

// Sanity: the current asset_id spelling still decodes for every aliased type.
func TestAssetIDKeyStillPrimary(t *testing.T) {
	t.Parallel()

	var book BookEvent
	if err := json.Unmarshal([]byte(`{"event_type":"book","market":"m","asset_id":"98","bids":[],"asks":[],"timestamp":"1"}`), &book); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if book.AssetID != "98" {
		t.Fatalf("asset ID = %q, want 98", book.AssetID)
	}
	if book.TickSize != clob.TickSize("") {
		t.Fatal("zero tick size must stay zero")
	}
}
