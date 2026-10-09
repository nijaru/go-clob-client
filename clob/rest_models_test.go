package clob

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	json "github.com/go-json-experiment/json"
)

// Independent fixtures from the pinned Rust 561830b, TS 087f944, py ed8d04ca
// and standalone TS 8046a89 wire contracts, not marshaled Go model fixtures.
func TestRESTFinancialLexemes(t *testing.T) {
	const precise = "0.1234567890123456789012345678"
	type fixture struct {
		name, wire string
		target     any
		check      func() bool
	}
	cases := []fixture{}
	var market Market
	var book OrderBookSummary
	var tick TickSizeResponse
	var midpoint MidpointResponse
	var price PriceResponse
	var spread SpreadResponse
	var midpoints MidpointsResponse
	var prices PricesResponse
	var last LastTradePriceResponse
	var batch LastTradesPricesResponse
	var order OpenOrder
	var maker MakerOrder
	var trade Trade
	var builder BuilderTrade
	cases = append(
		cases,
		fixture{
			"market",
			`{"minimum_order_size":5,"minimum_tick_size":0.001,"maker_base_fee":1,"taker_base_fee":2}`,
			&market,
			func() bool {
				return market.MinimumOrderSize == "5" && market.MinimumTickSize == "0.001" &&
					market.MakerBaseFee == "1" &&
					market.TakerBaseFee == "2"
			},
		},
	)
	// Each row protects an independently decoded REST shape. A number with >19
	// fractional digits deliberately cannot pass through the order-math decimal.
	cases = append(cases, []fixture{
		{
			"book",
			`{"asset_id":123456789012345678901234567890,"min_order_size":5,"tick_size":0.01,"last_trade_price":` + precise + `,"bids":[{"price":` + precise + `,"size":2}],"asks":[]}`,
			&book,
			func() bool {
				return book.AssetID == "123456789012345678901234567890" &&
					book.MinOrderSize == "5" &&
					book.TickSize == "0.01" &&
					book.LastTradePrice == precise &&
					book.Bids[0].Price == precise &&
					book.Bids[0].Size == "2"
			},
		},
		{
			"tick",
			`{"minimum_tick_size":0.001}`,
			&tick,
			func() bool { return tick.MinimumTickSize == "0.001" },
		},
		{
			"mid",
			`{"mid":` + precise + `}`,
			&midpoint,
			func() bool { return midpoint.Mid == precise },
		},
		{"price", `{"price":1e-25}`, &price, func() bool { return price.Price == "1e-25" }},
		{"spread", `{"spread":2}`, &spread, func() bool { return spread.Spread == "2" }},
		{
			"midpoints",
			`{"123":` + precise + `}`,
			&midpoints,
			func() bool { return midpoints["123"] == precise },
		},
		{
			"prices",
			`{"123":{"BUY":` + precise + `,"SELL":"0.20"}}`,
			&prices,
			func() bool { return prices["123"][SideBuy] == precise && prices["123"][SideSell] == "0.20" },
		},
		{
			"last",
			`{"price":` + precise + `,"side":"BUY"}`,
			&last,
			func() bool { return last.Price == precise },
		},
		{
			"last batch",
			`{"token_id":123456789012345678901234567890,"price":` + precise + `}`,
			&batch,
			func() bool { return batch.TokenID == "123456789012345678901234567890" && batch.Price == precise },
		},
		{
			"order",
			`{"asset_id":123456789012345678901234567890,"original_size":10,"size_matched":2,"price":` + precise + `,"created_at":"2026-03-12","expiration":0}`,
			&order,
			func() bool {
				return order.AssetID == "123456789012345678901234567890" &&
					order.OriginalSize == "10" &&
					order.SizeMatched == "2" &&
					order.Price == precise &&
					order.CreatedAtTime.Format("2006-01-02") == "2026-03-12"
			},
		},
		{
			"maker",
			`{"token_id":123456789012345678901234567890,"matched_amount":2,"price":` + precise + `,"fee_rate_bps":12,"builder_fee":1e-25,"builder_code":"code"}`,
			&maker,
			func() bool {
				return maker.AssetID == "123456789012345678901234567890" &&
					maker.MatchedAmount == "2" &&
					maker.Price == precise &&
					maker.FeeRateBps == "12" &&
					maker.BuilderFee.String() == "1e-25" &&
					*maker.BuilderCode == "code"
			},
		},
		{
			"trade",
			`{"asset_id":123456789012345678901234567890,"size":2,"price":` + precise + `,"fee_rate_bps":12,"match_time":1710000000,"last_update":1710000000123,"match_time_nano":"1710000000123456789","err_msg":"failed"}`,
			&trade,
			func() bool {
				return trade.AssetID == "123456789012345678901234567890" && trade.Size == "2" &&
					trade.Price == precise &&
					trade.FeeRateBps == "12" &&
					trade.MatchTime == "2024-03-09T16:00:00Z" &&
					trade.LastUpdate == "2024-03-09T16:00:00.123Z" &&
					trade.MatchTimeNano == "1710000000123456789" &&
					trade.ErrorMsg == "failed"
			},
		},
		{
			"builder",
			`{"assetId":123456789012345678901234567890,"size":2,"sizeUsdc":3,"price":` + precise + `,"fee":4,"feeUsdc":5,"builderFee":1e-25,"builderCode":"code","matchTime":1710000000,"createdAt":1710000000123,"updatedAt":"2026-03-12T00:00:00Z","errMsg":"failed"}`,
			&builder,
			func() bool {
				return builder.AssetID == "123456789012345678901234567890" && builder.Size == "2" &&
					builder.SizeUSDC == "3" &&
					builder.Price == precise &&
					builder.Fee == "4" &&
					builder.FeeUSDC == "5" &&
					builder.BuilderFee.String() == "1e-25" &&
					*builder.BuilderCode == "code" &&
					builder.MatchTime == "2024-03-09T16:00:00Z" &&
					builder.CreatedAt == "2024-03-09T16:00:00.123Z" &&
					builder.UpdatedAt == "2026-03-12T00:00:00Z" &&
					builder.Error == "failed"
			},
		},
	}...)
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(tt.wire), tt.target); err != nil {
				t.Fatal(err)
			}
			if !tt.check() {
				t.Fatalf("decoded %#v", tt.target)
			}
		})
	}
}

func TestRESTDatesAndOptionalRewards(t *testing.T) {
	for _, date := range []string{`1710000000123`, `"1710000000123"`, `"2024-03-09"`, `"2024-03-09T16:00:00.123Z"`} {
		want := "2024-03-09T16:00:00.123Z"
		if date == `"2024-03-09"` {
			want = "2024-03-09"
		}
		var user UserEarning
		var total TotalUserEarning
		var config RewardsConfig
		var market MarketRewardsConfig
		for _, target := range []any{&user, &total} {
			if err := json.Unmarshal([]byte(`{"date":`+date+`}`), target); err != nil {
				t.Fatal(err)
			}
		}
		for _, target := range []any{&config, &market} {
			if err := json.Unmarshal([]byte(`{"start_date":`+date+`,"end_date":`+date+`}`), target); err != nil {
				t.Fatal(err)
			}
		}
		if user.Date != want || total.Date != want || config.StartDate != want ||
			config.EndDate != want ||
			market.StartDate != want ||
			market.EndDate != want {
			t.Fatalf("date %s: %+v %+v %+v %+v", date, user, total, config, market)
		}
	}
	var builder BuilderTrade
	if err := json.Unmarshal([]byte(`{"matchTime":1000,"createdAt":1000,"updatedAt":"2000"}`), &builder); err != nil {
		t.Fatal(err)
	}
	if builder.MatchTime != "1970-01-01T00:16:40Z" || builder.CreatedAt != "1970-01-01T00:00:01Z" ||
		builder.UpdatedAt != "1970-01-01T00:00:02Z" {
		t.Fatalf("builder timestamp units %+v", builder)
	}
	var key BuilderAPIKey
	if err := json.Unmarshal([]byte(`{"key":"k","createdAt":1000,"revokedAt":"2000"}`), &key); err != nil {
		t.Fatal(err)
	}
	if key.CreatedAt != "1970-01-01T00:00:01Z" || key.RevokedAt != "1970-01-01T00:00:02Z" {
		t.Fatalf("key %+v", key)
	}
	var reward CurrentReward
	if err := json.Unmarshal([]byte(`{"sponsored_daily_rate":1e-25,"sponsors_count":0,"native_daily_rate":null,"total_daily_rate":"1.000000000000000000001","rewards_config":[{"id":0,"start_date":1000,"end_date":null}]}`), &reward); err != nil {
		t.Fatal(err)
	}
	if reward.SponsoredDailyRate.String() != "1e-25" || reward.SponsorsCount == nil ||
		*reward.SponsorsCount != 0 ||
		reward.NativeDailyRate != nil ||
		reward.TotalDailyRate.String() != "1.000000000000000000001" ||
		*reward.RewardsConfig[0].ID != 0 ||
		reward.RewardsConfig[0].StartDate != "1970-01-01T00:00:01Z" {
		t.Fatalf("reward %+v", reward)
	}
}

func TestRESTRejectsFractionalAssetIDs(t *testing.T) {
	for _, wire := range []string{`{"asset_id":1.5}`, `{"asset_id":1e3}`, `{"asset_id":-1}`} {
		for _, target := range []any{new(OrderBookSummary), new(OpenOrder), new(MakerOrder), new(Trade)} {
			if err := json.Unmarshal([]byte(wire), target); err == nil {
				t.Fatalf("accepted %s in %T", wire, target)
			}
		}
	}
	var book OrderBookSummary
	if err := json.Unmarshal([]byte(`{"asset_id":"native:position:opaque"}`), &book); err != nil ||
		book.AssetID != "native:position:opaque" {
		t.Fatalf("opaque ID: %+v %v", book, err)
	}
}

func TestRESTDecimalExactConversion(t *testing.T) {
	for _, tt := range []struct{ wire, want string }{{"1e-19", "0.0000000000000000001"}, {"1.000000000000000000000", "1"}, {"2.5e2", "250"}, {"0", "0"}, {"-0.01", "-0.01"}} {
		var d DecimalString
		if err := json.Unmarshal([]byte(tt.wire), &d); err != nil {
			t.Fatal(err)
		}
		got, err := d.Decimal()
		if err != nil || got.String() != tt.want || d.String() != tt.wire {
			t.Fatalf("%s -> %s, %v", tt.wire, got, err)
		}
	}
	for _, wire := range []string{"1e-25", "0.1234567890123456789012345678", "1e100000000", "999999999999999999999999999999999999999999"} {
		var d DecimalString
		if err := json.Unmarshal([]byte(wire), &d); err != nil {
			t.Fatal(err)
		}
		if _, err := d.Decimal(); err == nil {
			t.Fatalf("inexact/out of range accepted: %s", wire)
		}
	}
}

func TestRESTPublicGETWireContracts(t *testing.T) {
	spreadBody := `{"spreads":{"TOKEN":2}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case orderBookEndpoint:
			_, _ = w.Write(
				[]byte(
					`{"asset_id":123456789012345678901234567890,"bids":[{"price":0.1234567890123456789012345678,"size":2}],"min_order_size":5,"tick_size":0.01}`,
				),
			)
		case priceHistoryEndpoint:
			_, _ = w.Write(
				[]byte(
					`{"history":[{"t":1710000000,"p":0.1234567890123456789012345678},{"t":1710000001,"p":1e-25}]}`,
				),
			)
		case spreadsEndpoint:
			_, _ = w.Write([]byte(spreadBody))
		case builderFeeRateEndpoint + "code":
			_, _ = w.Write(
				[]byte(
					`{"builderMakerFeeRateBps":12,"builderTakerFeeRateBps":25,"builder_taker_fee_rate_bps":99}`,
				),
			)
		case marketsByTokenEndpoint + "TOKEN":
			_, _ = w.Write([]byte(`{"condition_id":"cid"}`))
		case clobMarketEndpoint + "/cid":
			_, _ = w.Write(
				[]byte(
					`{"c":"cid","t":[{"t":"TOKEN","o":"YES"}],"mts":0.01,"fd":{"r":1e-25,"e":2},"mbf":0.1234567890123456789012345678,"tbf":1e-25,"r":{"mi":1e-25,"ma":2,"e":true,"smoa":false,"moas":10},"ao":true,"sd":3,"gst":"2026-03-12","cbos":false,"aot":null,"itode":true,"ibce":false}`,
				),
			)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client, err := NewClient(Config{Host: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	book, err := client.GetOrderBook(t.Context(), "TOKEN")
	if err != nil || book.AssetID != "123456789012345678901234567890" ||
		book.Bids[0].Price != "0.1234567890123456789012345678" {
		t.Fatalf("numeric public book %+v %v", book, err)
	}
	history, err := client.GetPricesHistory(t.Context(), PriceHistoryFilterParams{Market: "TOKEN"})
	if err != nil || len(history) != 2 ||
		history[0].P.String() != "0.1234567890123456789012345678" ||
		history[1].P.String() != "1e-25" {
		t.Fatalf("lossless public history %+v %v", history, err)
	}
	for _, body := range []string{`{"TOKEN":2}`, `{"spreads":{"TOKEN":2}}`, `{"spreads":null}`} {
		spreadBody = body
		out, err := client.GetSpreads(t.Context(), []BookParams{{TokenID: "TOKEN"}})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(body, "null") {
			if out != nil {
				t.Fatalf("nullable spreads %+v", out)
			}
		} else if out["TOKEN"] != "2" {
			t.Fatalf("spreads %+v", out)
		}
	}
	fee, err := client.GetBuilderFeeRate(t.Context(), "code")
	if err != nil || fee.BuilderMakerFeeRateBps != 12 || fee.BuilderTakerFeeRateBps != 25 {
		t.Fatalf("BPS %+v %v", fee, err)
	}
	market, err := client.GetClobMarket(t.Context(), "cid")
	if err != nil {
		t.Fatal(err)
	}
	if market.FeeDetails.Rate != "1e-25" ||
		market.MakerBaseFee.String() != "0.1234567890123456789012345678" ||
		market.TakerBaseFee.String() != "1e-25" ||
		market.Rewards.MinSize.String() != "1e-25" ||
		market.Rewards.MaxSpread.String() != "2" ||
		!*market.Rewards.Enabled ||
		*market.Rewards.SkipMinOrderAge ||
		*market.Rewards.MinOrderAgeSeconds != 10 ||
		!*market.AcceptingOrders ||
		*market.SecondsDelay != 3 ||
		*market.GameStartTime != "2026-03-12" ||
		*market.ClearBookOnStart ||
		market.AcceptingOrderTime != nil ||
		!*market.TakerOrderDelayEnabled ||
		*market.BlockaidCheckEnabled {
		t.Fatalf("compact fields %+v", market)
	}
	if _, err := client.GetFeeInfo(t.Context(), "TOKEN"); err == nil {
		t.Fatal("getter silently narrowed fee")
	}
	if _, err := client.fetchOrderMarketMetadata(t.Context(), "TOKEN", "cid"); err == nil {
		t.Fatal("order cache silently narrowed fee")
	}
}
