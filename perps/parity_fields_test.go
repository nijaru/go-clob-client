package perps

import (
	"encoding/json"
	"testing"
)

// The fields below were added upstream after the initial perps inventory. They
// are decoded from raw wire payloads so absent, null, and named values stay
// distinguishable from invented defaults.
func TestPortfolioRiskAndMarginFields(t *testing.T) {
	var portfolio PerpsPortfolio
	if err := json.Unmarshal([]byte(`{
		"positions":[{
			"instrument_id":1,"symbol":"btcusd","size":"1","entry_price":"10",
			"leverage":5,"cross":true,"initial_margin":"2","maintenance_margin":"1",
			"position_value":"10","liquidation_price":"8","unrealized_pnl":"0.5",
			"return_on_equity":"0.25","cumulative_funding":"-0.01","adl_index":2
		}],
		"margin":{
			"total_account_value":"100","available_order_margin":"40",
			"total_initial_margin":"20","total_maintenance_margin":"10",
			"total_position_value":"60"
		},
		"withdrawable":"30","in_liquidation":false,"fee_tier":1,"timestamp":1791547994123
	}`), &portfolio); err != nil {
		t.Fatal(err)
	}
	if portfolio.Positions[0].ADLIndex != 2 {
		t.Fatalf("adl_index = %d", portfolio.Positions[0].ADLIndex)
	}
	if portfolio.Margin.AvailableOrderMargin != "40" {
		t.Fatalf("available_order_margin = %q", portfolio.Margin.AvailableOrderMargin)
	}
	if portfolio.FeeTier != 1 {
		t.Fatalf("fee_tier = %d", portfolio.FeeTier)
	}
	if portfolio.Margin.TotalAccountValue != "100" ||
		portfolio.Positions[0].CumulativeFunding != "-0.01" {
		t.Fatalf("exact amounts lost: %+v", portfolio)
	}
}

func TestInstrumentVisibilityAndLogo(t *testing.T) {
	var instrument PerpsInstrument
	if err := json.Unmarshal([]byte(`{
		"instrument_id":1,"symbol":"btcusd","ui_live_time":1791547994123,
		"logo":{"light":"https://cdn/light.svg","dark":"https://cdn/dark.svg"},
		"risk_tiers":[],"price_bounds":"0"
	}`), &instrument); err != nil {
		t.Fatal(err)
	}
	if instrument.UILiveTime == nil || *instrument.UILiveTime != 1791547994123 {
		t.Fatalf("ui_live_time = %v", instrument.UILiveTime)
	}
	if instrument.Logo == nil || instrument.Logo.Light != "https://cdn/light.svg" ||
		instrument.Logo.Dark != "https://cdn/dark.svg" {
		t.Fatalf("logo = %+v", instrument.Logo)
	}
	// Absent visibility metadata means hidden, not epoch zero.
	var hidden PerpsInstrument
	if err := json.Unmarshal([]byte(`{"instrument_id":1}`), &hidden); err != nil {
		t.Fatal(err)
	}
	if hidden.UILiveTime != nil || hidden.Logo != nil {
		t.Fatalf("absent metadata invented values: %+v", hidden)
	}
}

func TestFillLiquidationAndADL(t *testing.T) {
	var fill PerpsAccountFill
	if err := json.Unmarshal([]byte(`{
		"trade_id":1,"order_id":2,"instrument_id":3,"side":"long","price":"10",
		"quantity":"1","fee":"0.01","total_fee":"0.01","liquidation":true,"adl":false,
		"liquidation_details":{"liquidated_user":"0xabc","mark":"9.5","method":"backstop"},
		"timestamp":1791547994123
	}`), &fill); err != nil {
		t.Fatal(err)
	}
	if fill.ADL || fill.LiquidationDetails == nil {
		t.Fatalf("fill = %+v", fill)
	}
	details := fill.LiquidationDetails
	if details.Method != PerpsLiquidationBackstop ||
		details.Mark != "9.5" ||
		details.LiquidatedUser == nil || *details.LiquidatedUser != "0xabc" {
		t.Fatalf("liquidation details = %+v", details)
	}
	// The accounting price stays separate from the liquidation mark.
	if fill.Price != "10" {
		t.Fatalf("fill price = %q", fill.Price)
	}
	// Ordinary order-book liquidations omit the details.
	var ordinary PerpsAccountFill
	if err := json.Unmarshal(
		[]byte(`{"fee":"0.01","total_fee":"0.01","liquidation":true,"adl":true,"liquidation_details":null}`),
		&ordinary,
	); err != nil {
		t.Fatal(err)
	}
	if !ordinary.ADL || ordinary.LiquidationDetails != nil {
		t.Fatalf("ordinary fill = %+v", ordinary)
	}
}
