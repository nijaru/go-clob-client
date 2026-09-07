package clob

import (
	"testing"

	json "github.com/go-json-experiment/json"
)

func TestOrderBookSummaryAcceptsLegacyTokenIDKey(t *testing.T) {
	t.Parallel()

	// Primary asset_id key wins.
	var book OrderBookSummary
	payload := `{"market":"m","asset_id":"100","timestamp":"1","bids":[],"asks":[],"min_order_size":"1","tick_size":"0.01","neg_risk":false,"last_trade_price":"0.5","hash":"h"}`
	if err := json.Unmarshal([]byte(payload), &book); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if book.AssetID != "100" {
		t.Fatalf("asset ID = %q, want 100", book.AssetID)
	}

	// Legacy token_id spelling is accepted when asset_id is absent
	// (py-sdk test_order_book_accepts_token_id_without_asset_id parity).
	var legacy OrderBookSummary
	legacyPayload := `{"market":"m","token_id":"legacy-token","timestamp":"1","bids":[],"asks":[],"min_order_size":"1","tick_size":"0.01","neg_risk":false,"last_trade_price":"0.5","hash":"h"}`
	if err := json.Unmarshal([]byte(legacyPayload), &legacy); err != nil {
		t.Fatalf("unmarshal legacy: %v", err)
	}
	if legacy.AssetID != "legacy-token" {
		t.Fatalf("asset ID = %q, want legacy-token", legacy.AssetID)
	}

	// When both keys are present but asset_id is empty, the fallback fills it:
	// an empty asset identifier is treated as absent across all alias decoders.
	var both OrderBookSummary
	bothPayload := `{"market":"m","asset_id":"","token_id":"fallback","timestamp":"1","bids":[],"asks":[],"min_order_size":"1","tick_size":"0.01","neg_risk":false,"last_trade_price":"0.5","hash":"h"}`
	if err := json.Unmarshal([]byte(bothPayload), &both); err != nil {
		t.Fatalf("unmarshal both: %v", err)
	}
	if both.AssetID != "fallback" {
		t.Fatalf("asset ID = %q, want fallback (empty asset_id falls back)", both.AssetID)
	}
}

func TestTradeRecordsAcceptLegacyTokenIDKey(t *testing.T) {
	t.Parallel()

	var trade Trade
	payload := `{"id":"t1","market":"m","token_id":"7","side":"BUY","size":"10","price":"0.5","status":"MATCHED","match_time":"1","last_update":"1","outcome":"Yes","bucket_index":0}`
	if err := json.Unmarshal([]byte(payload), &trade); err != nil {
		t.Fatalf("unmarshal trade: %v", err)
	}
	if trade.AssetID != "7" {
		t.Fatalf("trade asset ID = %q, want 7", trade.AssetID)
	}

	var maker MakerOrder
	makerPayload := `{"order_id":"o1","owner":"0xabc","matched_amount":"5","price":"0.5","fee_rate_bps":"0","token_id":"9","outcome":"Yes","side":"BUY"}`
	if err := json.Unmarshal([]byte(makerPayload), &maker); err != nil {
		t.Fatalf("unmarshal maker order: %v", err)
	}
	if maker.AssetID != "9" {
		t.Fatalf("maker asset ID = %q, want 9", maker.AssetID)
	}
}

func TestOpenOrderAcceptsLegacyTokenIDKey(t *testing.T) {
	t.Parallel()

	var order OpenOrder
	payload := `{"id":"o1","status":"LIVE","owner":"0xabc","maker_address":"0xabc","market":"m","token_id":"11","side":"BUY","original_size":"10","size_matched":"0","price":"0.5","associate_trades":[],"outcome":"Yes","created_at":1,"expiration":"0","order_type":"GTC"}`
	if err := json.Unmarshal([]byte(payload), &order); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if order.AssetID != "11" {
		t.Fatalf("asset ID = %q, want 11", order.AssetID)
	}
	if order.CreatedAtTime == nil {
		t.Fatal("timestamp normalization must survive the alias handling")
	}
}

func TestOutcomeTokenAndLastTradePricesAcceptAssetIDKey(t *testing.T) {
	t.Parallel()

	var token OutcomeToken
	if err := json.Unmarshal([]byte(`{"asset_id":"21","outcome":"Yes","price":"0.5","winner":false}`), &token); err != nil {
		t.Fatalf("unmarshal outcome token: %v", err)
	}
	if token.TokenID != "21" {
		t.Fatalf("token ID = %q, want 21", token.TokenID)
	}

	var lastTrade LastTradesPricesResponse
	if err := json.Unmarshal([]byte(`{"asset_id":"31","price":"0.5","side":"BUY"}`), &lastTrade); err != nil {
		t.Fatalf("unmarshal last trade: %v", err)
	}
	if lastTrade.TokenID != "31" {
		t.Fatalf("token ID = %q, want 31", lastTrade.TokenID)
	}
	// Legacy token_id key still primary.
	if err := json.Unmarshal([]byte(`{"token_id":"32","price":"0.5","side":"BUY"}`), &lastTrade); err != nil {
		t.Fatalf("unmarshal last trade legacy: %v", err)
	}
	if lastTrade.TokenID != "32" {
		t.Fatalf("token ID = %q, want 32", lastTrade.TokenID)
	}
}

func TestNotificationPayloadsAcceptBothAssetKeys(t *testing.T) {
	t.Parallel()

	var order OrderNotificationPayload
	if err := json.Unmarshal([]byte(`{"token_id":"41","market":"0xcond","order_id":"o1","side":"BUY","price":"0.5","original_size":"10","matched_size":"0","remaining_size":"10","outcome":"Yes","outcome_index":0}`), &order); err != nil {
		t.Fatalf("unmarshal order notification: %v", err)
	}
	if order.AssetID != "41" {
		t.Fatalf("asset ID = %q, want 41", order.AssetID)
	}

	var token MarketNotificationToken
	if err := json.Unmarshal([]byte(`{"asset_id":"51","outcome":"Yes","winner":true}`), &token); err != nil {
		t.Fatalf("unmarshal market token: %v", err)
	}
	if token.TokenID != "51" {
		t.Fatalf("token ID = %q, want 51", token.TokenID)
	}
}

func TestBuilderTradeAcceptsAllAssetKeySpellings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload string
		want    string
	}{
		{"assetId", `{"id":"b1","tradeType":"TAKER","assetId":"61"}`, "61"},
		{"asset_id", `{"id":"b1","tradeType":"TAKER","asset_id":"62"}`, "62"},
		{"token_id", `{"id":"b1","tradeType":"TAKER","token_id":"63"}`, "63"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var trade BuilderTrade
			if err := json.Unmarshal([]byte(tc.payload), &trade); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if trade.AssetID != tc.want {
				t.Fatalf("asset ID = %q, want %q", trade.AssetID, tc.want)
			}
		})
	}
}
