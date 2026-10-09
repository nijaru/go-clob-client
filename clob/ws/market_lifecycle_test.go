package ws

import (
	"slices"
	"testing"

	json "github.com/go-json-experiment/json"
)

func TestMarketLifecycleAliases(t *testing.T) {
	for _, tc := range []struct {
		name   string
		fields string
		assets []string
		winner string
	}{
		{"legacy token keys", `"token_ids":["1","2"],"winning_token_id":"2"`, []string{"1", "2"}, "2"},
		{"asset_ids alias", `"asset_ids":["3"],"token_ids":["4"],"winning_asset_id":"3","winning_token_id":"4"`, []string{"3"}, "3"},
		{"current keys win", `"assets_ids":["5"],"asset_ids":["6"],"token_ids":["7"],"winning_asset_id":"5","winning_token_id":"7"`, []string{"5"}, "5"},
		{"explicit empty wins", `"assets_ids":[],"asset_ids":["6"],"winning_asset_id":"","winning_token_id":"7"`, []string{}, ""},
		{"explicit null wins", `"assets_ids":null,"token_ids":["6"],"winning_asset_id":null,"winning_token_id":"7"`, nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Decode via the actual event dispatch entry point, not only a helper.
			for _, eventType := range []string{"new_market", "market_resolved"} {
				payload := []byte(
					`{"event_type":"` + eventType + `","id":"m","market":"condition",` + tc.fields + `}`,
				)
				client := NewClient("")
				defer client.Close()
				client.handleMessage(t.Context(), payload)
				var event Event
				select {
				case event = <-client.Events():
				case err := <-client.Errors():
					t.Fatal(err)
				default:
					t.Fatal("event not delivered")
				}
				var assets []string
				switch e := event.(type) {
				case *NewMarketEvent:
					assets = e.AssetIDs
				case *MarketResolvedEvent:
					assets = e.AssetIDs
					if e.WinningAssetID != tc.winner {
						t.Fatalf("winner = %q, want %q", e.WinningAssetID, tc.winner)
					}
				default:
					t.Fatalf("unexpected event %T", event)
				}
				if !slices.Equal(assets, tc.assets) || (assets == nil) != (tc.assets == nil) {
					t.Fatalf("assets = %#v, want %#v", assets, tc.assets)
				}
			}
		})
	}
}

func TestNewMarketRoutingAndFinancialFields(t *testing.T) {
	var event NewMarketEvent
	if err := json.Unmarshal([]byte(`{
		"event_type":"new_market","id":"m","market":"condition",
		"condition_id":"condition-id","clob_token_ids":["yes","no"],
		"active":false,"tags":["sports"],"sports_market_type":"spread",
		"line":-1.5,"game_start_time":1772752581815,"group_item_title":"Team A",
		"order_price_min_tick_size":0.001,"taker_base_fee":"0.123456789012345678901",
		"fees_enabled":false,"fee_schedule":{"rate":0.025,"exponent":2}
	}`), &event); err != nil {
		t.Fatal(err)
	}
	if event.ConditionID != "condition-id" ||
		!slices.Equal(event.CLOBTokenIDs, []string{"yes", "no"}) ||
		event.Active == nil ||
		*event.Active ||
		!slices.Equal(event.Tags, []string{"sports"}) ||
		event.SportsMarketType != "spread" ||
		event.GroupItemTitle != "Team A" ||
		event.GameStartTime != "1772752581815" {
		t.Fatalf("routing metadata lost: %+v", event)
	}
	if event.Line == nil || *event.Line != "-1.5" || event.OrderPriceMinTickSize == nil ||
		*event.OrderPriceMinTickSize != "0.001" || event.TakerBaseFee == nil ||
		*event.TakerBaseFee != "0.123456789012345678901" || event.FeesEnabled == nil || *event.FeesEnabled ||
		string(event.FeeSchedule) != `{"rate":0.025,"exponent":2}` {
		t.Fatalf("financial metadata lost: %+v", event)
	}
}
