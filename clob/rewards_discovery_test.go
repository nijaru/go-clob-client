package clob

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestRewardMarketsDiscovery(t *testing.T) {
	t.Parallel()

	var queries []map[string][]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != rewardsMarketsMultiEndpoint {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("POLY_API_KEY") != "" {
			t.Error("public reward discovery sent credentials")
		}
		queries = append(queries, r.URL.Query())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("next_cursor") {
		case initialCursor:
			_, _ = w.Write([]byte(`{
				"limit":1,"count":1,"next_cursor":"cursor-2",
				"data":[{
					"condition_id":"cond-1",
					"market_id":"101",
					"market_slug":"market-1",
					"question":"Will it?",
					"image":"image-1",
					"market_competitiveness":0.05,
					"rewards_config":[{"id":1,"asset_address":"0xasset","start_date":1791547994123,"end_date":"","rate_per_day":"1.25","total_rewards":5}],
					"rewards_max_spread":0.02,
					"rewards_min_size":10,
					"spread":"0.01",
					"tokens":[{"token_id":"123","outcome":"Yes","price":0.5}],
					"group_item_title":"Group",
					"volume_24hr":9007199254740993.125,
					"event_id":"7",
					"event_slug":"event-1",
					"created_at":1791547994123,
					"one_day_price_change":-0.5,
					"end_date":1791634394123
				}]
			}`))
		case "cursor-2":
			_, _ = w.Write([]byte(`{
				"limit":1,"count":1,"next_cursor":"LTE=",
				"data":[{
					"condition_id":"cond-2","market_id":"102","market_slug":"market-2",
					"question":"And?","image":"image-2","market_competitiveness":0.06,
					"rewards_config":[],"rewards_max_spread":0.03,"rewards_min_size":20,
					"spread":"0.02","tokens":[],"group_item_title":"Group",
					"volume_24hr":1,"event_id":"8","event_slug":"event-2",
					"created_at":1791547994123,"one_day_price_change":0,
					"end_date":""
				}]
			}`))
		default:
			t.Errorf("unexpected cursor %q", r.URL.Query().Get("next_cursor"))
		}
	}))
	defer server.Close()

	client := newRewardClient(t, server.URL)
	markets, err := client.GetRewardMarkets(t.Context(), RewardMarketsParams{
		Query:         "sports",
		TagSlugs:      []string{"nba"},
		MarketID:      ptrOf(int64(101)),
		EventIDs:      []int64{7, 8},
		EventTitle:    "Final",
		OrderBy:       RewardMarketSortVolume24hr,
		SortDirection: RewardMarketSortDescending,
		MinPrice:      ptrOf[DecimalString]("0.1"),
		MaxPrice:      ptrOf[DecimalString]("0.9"),
		PageSize:      2,
	})
	if err != nil {
		t.Fatalf("GetRewardMarkets: %v", err)
	}
	if len(markets) != 2 {
		t.Fatalf("markets = %d", len(markets))
	}
	first := markets[0]
	if first.ConditionID != "cond-1" || first.MarketID != "101" ||
		first.MarketCompetitiveness != "0.05" || first.RewardsMaxSpread != "0.02" ||
		first.RewardsMinSize != "10" || first.Spread != "0.01" ||
		first.Volume24hr != "9007199254740993.125" ||
		first.OneDayPriceChange != "-0.5" ||
		first.EventID != "7" || first.EventSlug != "event-1" ||
		first.CreatedAt != "2026-10-09T12:13:14.123Z" ||
		first.EndDate != "2026-10-10T12:13:14.123Z" {
		t.Fatalf("first market lost wire values: %+v", first)
	}
	if len(first.RewardsConfig) != 1 ||
		first.RewardsConfig[0].RatePerDay != "1.25" ||
		first.RewardsConfig[0].TotalRewards != "5" ||
		first.RewardsConfig[0].StartDate != "2026-10-09T12:13:14.123Z" ||
		first.RewardsConfig[0].EndDate != "" {
		t.Fatalf("reward config = %+v", first.RewardsConfig)
	}
	if len(first.Tokens) != 1 || first.Tokens[0].TokenID != "123" {
		t.Fatalf("tokens = %+v", first.Tokens)
	}
	// An empty reward end date stays absent rather than becoming epoch zero.
	if markets[1].EndDate != "" {
		t.Fatalf("empty end date = %q", markets[1].EndDate)
	}

	if len(queries) != 2 {
		t.Fatalf("requests = %d", len(queries))
	}
	firstQuery := queries[0]
	for key, want := range map[string]string{
		"q":           "sports",
		"event_title": "Final",
		"market_id":   "101",
		"order_by":    "volume_24hr",
		"position":    "DESC",
		"min_price":   "0.1",
		"max_price":   "0.9",
		"page_size":   "2",
		"next_cursor": initialCursor,
	} {
		if got := firstQuery[key]; !slices.Equal(got, []string{want}) {
			t.Errorf("%s = %v, want %q", key, got, want)
		}
	}
	if !slices.Equal(firstQuery["tag_slug"], []string{"nba"}) ||
		!slices.Equal(firstQuery["event_id"], []string{"7", "8"}) {
		t.Errorf("selectors = %v", firstQuery)
	}
	if queries[1]["next_cursor"][0] != "cursor-2" {
		t.Errorf("second cursor = %v", queries[1]["next_cursor"])
	}
}

func TestRewardEndpointSponsoredFilter(t *testing.T) {
	t.Parallel()

	seen := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen[r.URL.Path] = r.URL.Query().Get("sponsored")
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case rewardsUserTotalEndpoint:
			_, _ = w.Write(
				[]byte(
					`[{"date":"2026-03-12","asset_address":"0xa","maker_address":"0xm","earnings":1,"asset_rate":1,"native_earnings":2,"sponsored_earnings":3}]`,
				),
			)
		default:
			_, _ = w.Write([]byte(`{"limit":1,"count":0,"next_cursor":"LTE=","data":[]}`))
		}
	}))
	defer server.Close()

	client := newRewardClient(t, server.URL)
	sponsored := true
	native := false
	total, err := client.GetTotalEarningsForUserForDay(t.Context(), TotalEarningsParams{
		Date:      "2026-03-12",
		Sponsored: &sponsored,
	})
	if err != nil {
		t.Fatalf("GetTotalEarningsForUserForDay: %v", err)
	}
	if total[0].Earnings != "1" || total[0].NativeEarnings == nil ||
		*total[0].NativeEarnings != "2" || total[0].SponsoredEarnings == nil ||
		*total[0].SponsoredEarnings != "3" {
		t.Fatalf("earnings breakdown lost: %+v", total[0])
	}
	if _, err := client.GetCurrentRewards(t.Context(), CurrentRewardsParams{
		Sponsored: &native,
	}); err != nil {
		t.Fatalf("GetCurrentRewards: %v", err)
	}
	if _, err := client.GetRewardsForMarket(t.Context(), MarketRewardsParams{
		ConditionID: "cond-1",
		Sponsored:   &sponsored,
	}); err != nil {
		t.Fatalf("GetRewardsForMarket: %v", err)
	}
	if seen[rewardsUserTotalEndpoint] != "true" ||
		seen[rewardsMarketsCurrentEndpoint] != "false" ||
		seen[rewardsMarketsEndpoint+"cond-1"] != "true" {
		t.Fatalf("sponsored filters = %v", seen)
	}
}

func TestCurrentRebates(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != rebatesCurrentEndpoint {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("POLY_API_KEY") != "" {
			t.Error("public rebates lookup sent credentials")
		}
		if r.URL.Query().Get("date") != "2026-10-06" ||
			r.URL.Query().Get("maker_address") != "0x00000000000000000000000000000000000000Ab" {
			t.Errorf("query = %v", r.URL.Query())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{
			"date":"2026-10-06",
			"condition_id":"0x` + repeatByte("ab", 32) + `",
			"asset_address":"0x0000000000000000000000000000000000000001",
			"maker_address":"0x00000000000000000000000000000000000000Ab",
			"rebated_fees_usdc":"0.000001"
		}]`))
	}))
	defer server.Close()

	client := newRewardClient(t, server.URL)
	rebates, err := client.GetCurrentRebates(t.Context(), RebatesParams{
		Date:         "2026-10-06",
		MakerAddress: "0x00000000000000000000000000000000000000Ab",
	})
	if err != nil {
		t.Fatalf("GetCurrentRebates: %v", err)
	}
	if len(rebates) != 1 || rebates[0].Date != "2026-10-06" ||
		rebates[0].RebatedFeesUSDC != "0.000001" ||
		rebates[0].AssetAddress != "0x0000000000000000000000000000000000000001" {
		t.Fatalf("rebates = %+v", rebates)
	}
}

func TestRewardRequestValidation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("invalid request reached the service: %s", r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"limit":1,"count":0,"next_cursor":"LTE=","data":[]}`))
	}))
	defer server.Close()
	client := newRewardClient(t, server.URL)

	for name, params := range map[string]RewardMarketsParams{
		"page size":      {PageSize: 501},
		"negative page":  {PageSize: -1},
		"market id":      {MarketID: ptrOf(int64(0))},
		"event id":       {EventIDs: []int64{0}},
		"sort field":     {OrderBy: "unknown"},
		"sort direction": {SortDirection: "UP"},
		"empty tag":      {TagSlugs: []string{""}},
		"negative bound": {MinPrice: ptrOf[DecimalString]("-0.1")},
		"infinite bound": {MaxSpread: ptrOf[DecimalString]("Infinity")},
	} {
		if _, err := client.GetRewardMarkets(t.Context(), params); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
	for name, params := range map[string]RebatesParams{
		"calendar date": {Date: "2026-10-6", MakerAddress: "0x00000000000000000000000000000000000000Ab"},
		"epoch date":    {Date: "1791547994", MakerAddress: "0x00000000000000000000000000000000000000Ab"},
		"short address": {Date: "2026-10-06", MakerAddress: "0xAb"},
		"zero address":  {Date: "2026-10-06", MakerAddress: "0x0000000000000000000000000000000000000000"},
	} {
		if _, err := client.GetCurrentRebates(t.Context(), params); err == nil {
			t.Fatalf("%s: accepted", name)
		}
	}
}

func newRewardClient(t *testing.T, host string) *AuthenticatedClient {
	t.Helper()
	client, err := NewAuthenticatedClient(Config{
		Host:        host,
		PrivateKey:  "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		Credentials: &Credentials{Key: "api-key", Secret: "c2VjcmV0", Passphrase: "pass"},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

func ptrOf[T any](value T) *T { return &value }

func repeatByte(value string, count int) string {
	result := ""
	for range count {
		result += value
	}
	return result
}
