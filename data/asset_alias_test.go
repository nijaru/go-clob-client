package data

import (
	"testing"

	json "github.com/go-json-experiment/json"
)

func TestPositionModelsAcceptAssetIDKeySpellings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload string
		want    string
	}{
		{"asset primary key", `{"proxyWallet":"0xabc","asset":"70","conditionId":"0xc","size":"1","avgPrice":"0.5","currentValue":"0.5"}`, "70"},
		{"asset_id alias", `{"proxyWallet":"0xabc","asset_id":"71","conditionId":"0xc","size":"1","avgPrice":"0.5","currentValue":"0.5"}`, "71"},
		{"token_id alias", `{"proxyWallet":"0xabc","token_id":"72","conditionId":"0xc","size":"1","avgPrice":"0.5","currentValue":"0.5"}`, "72"},
	}
	for _, tc := range cases {
		t.Run("position "+tc.name, func(t *testing.T) {
			var position Position
			if err := json.Unmarshal([]byte(tc.payload), &position); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if position.Asset != tc.want {
				t.Fatalf("asset = %q, want %q", position.Asset, tc.want)
			}
		})
	}

	var closed ClosedPosition
	closedPayload := `{"proxyWallet":"0xabc","token_id":"73","conditionId":"0xc","avgPrice":"0.5","totalBought":"1","realizedPnl":"0","curPrice":"0.5","timestamp":1,"title":"t","slug":"s","icon":"i","eventSlug":"e","outcome":"Yes","outcomeIndex":0,"oppositeOutcome":"No","oppositeAsset":"74","endDate":"2026"}`
	if err := json.Unmarshal([]byte(closedPayload), &closed); err != nil {
		t.Fatalf("unmarshal closed: %v", err)
	}
	if closed.Asset != "73" {
		t.Fatalf("closed asset = %q, want 73", closed.Asset)
	}
	if closed.OppositeAsset != "74" {
		t.Fatalf("opposite asset must pass through unchanged, got %q", closed.OppositeAsset)
	}
}

func TestTradeDecodesAllAssetKeys(t *testing.T) {
	t.Parallel()

	var trade Trade
	payload := `{"proxyWallet":"0xabc","side":"BUY","asset_id":"75","conditionId":"0xc","size":"1","price":"0.5","timestamp":1,"title":"t","slug":"s","icon":"i","eventSlug":"e","outcome":"Yes","outcomeIndex":0,"transactionHash":"0xh"}`
	if err := json.Unmarshal([]byte(payload), &trade); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if trade.Asset != "75" {
		t.Fatalf("asset = %q, want 75", trade.Asset)
	}
}

func TestActivityDecodesAssetIDKey(t *testing.T) {
	t.Parallel()

	var activity Activity
	payload := `{"proxyWallet":"0xabc","timestamp":1,"type":"TRADE","asset_id":"76","size":"1","usdcSize":"0.5","transactionHash":"0xh","side":"BUY","outcome":"Yes","title":"t","slug":"s","icon":"i","eventSlug":"e"}`
	if err := json.Unmarshal([]byte(payload), &activity); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if activity.Asset != "76" {
		t.Fatalf("asset = %q, want 76", activity.Asset)
	}
}

func TestHolderModelsAcceptAssetIDKeys(t *testing.T) {
	t.Parallel()

	var holder Holder
	payload := `{"proxyWallet":"0xabc","asset_id":"77","amount":"1","outcomeIndex":0}`
	if err := json.Unmarshal([]byte(payload), &holder); err != nil {
		t.Fatalf("unmarshal holder: %v", err)
	}
	if holder.Asset != "77" {
		t.Fatalf("holder asset = %q, want 77", holder.Asset)
	}

	var meta MetaHolder
	metaPayload := `{"asset_id":"78","holders":[{"proxyWallet":"0xabc","asset":"77","amount":"1","outcomeIndex":0}]}`
	if err := json.Unmarshal([]byte(metaPayload), &meta); err != nil {
		t.Fatalf("unmarshal meta holder: %v", err)
	}
	if meta.Token != "78" {
		t.Fatalf("meta token = %q, want 78", meta.Token)
	}
}

func TestMarketPositionModelsAcceptAssetIDKeys(t *testing.T) {
	t.Parallel()

	var detail MarketPositionDetail
	payload := `{"proxyWallet":"0xabc","asset_id":"79","conditionId":"0xc","avgPrice":"0.5","size":"1","currPrice":"0.5","currentValue":"0.5","cashPnl":"0","totalBought":"1","realizedPnl":"0","totalPnl":"0","outcome":"Yes","outcomeIndex":0}`
	if err := json.Unmarshal([]byte(payload), &detail); err != nil {
		t.Fatalf("unmarshal detail: %v", err)
	}
	if detail.TokenID != "79" {
		t.Fatalf("detail token = %q, want 79", detail.TokenID)
	}

	var meta MetaMarketPosition
	metaPayload := `{"asset_id":"80","positions":[]}`
	if err := json.Unmarshal([]byte(metaPayload), &meta); err != nil {
		t.Fatalf("unmarshal meta: %v", err)
	}
	if meta.Token != "80" {
		t.Fatalf("meta token = %q, want 80", meta.Token)
	}
}
