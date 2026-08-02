package clob

import (
	"net/http"
	"net/http/httptest"
	"testing"

	json "github.com/go-json-experiment/json"
)

func TestRewardsDecodeRustNumericFields(t *testing.T) {
	t.Parallel()

	var reward MarketReward
	if err := json.Unmarshal([]byte(`{
		"condition_id":"cond-1",
		"market_competitiveness":0.05,
		"rewards_config":[{"id":"1","total_days":10,"rate_per_day":"1.25","total_rewards":"400.0"}]
	}`), &reward); err != nil {
		t.Fatalf("decode reward: %v", err)
	}
	if reward.MarketCompetitiveness != "0.05" {
		t.Fatalf("market competitiveness = %q", reward.MarketCompetitiveness)
	}
	if len(reward.RewardsConfig) != 1 || reward.RewardsConfig[0].TotalDays != "10" {
		t.Fatalf("rewards config = %#v", reward.RewardsConfig)
	}
}

func TestRewardsDecodeAllRustDecimalShapes(t *testing.T) {
	t.Parallel()

	var earning UserEarning
	if err := json.Unmarshal([]byte(`{"earnings":1,"asset_rate":0.1}`), &earning); err != nil {
		t.Fatalf("decode user earning: %v", err)
	}
	if earning.Earnings != "1" || earning.AssetRate != "0.1" {
		t.Fatalf("user earning = %+v", earning)
	}

	var current CurrentReward
	if err := json.Unmarshal([]byte(`{"rewards_max_spread":0.05,"rewards_min_size":10,"rewards_config":[{"rate_per_day":2,"total_rewards":5}]}`), &current); err != nil {
		t.Fatalf("decode current reward: %v", err)
	}
	if current.RewardsMaxSpread != "0.05" || current.RewardsMinSize != "10" ||
		current.RewardsConfig[0].RatePerDay != "2" {
		t.Fatalf("current reward = %+v", current)
	}

	var userReward UserRewardsEarning
	if err := json.Unmarshal([]byte(`{"market_competitiveness":0.2,"earning_percentage":0.3,"earnings":[{"earnings":1,"asset_rate":0.4}],"tokens":[{"token_id":123,"price":0.5}]}`), &userReward); err != nil {
		t.Fatalf("decode user reward: %v", err)
	}
	if userReward.MarketCompetitiveness != "0.2" || userReward.EarningPercentage != "0.3" ||
		userReward.Earnings[0].Earnings != "1" || userReward.Tokens[0].TokenID != "123" ||
		userReward.Tokens[0].Price != "0.5" {
		t.Fatalf("user reward = %+v", userReward)
	}
}

func TestRewardsPercentagesDecodeRustNumbers(t *testing.T) {
	t.Parallel()

	var percentages RewardsPercentages
	if err := json.Unmarshal([]byte(`{"cond-1":0.25,"cond-2":"0.5"}`), &percentages); err != nil {
		t.Fatalf("decode reward percentages: %v", err)
	}
	if percentages["cond-1"] != "0.25" || percentages["cond-2"] != "0.5" {
		t.Fatalf("percentages = %#v", percentages)
	}
}

func TestRewardsPaginationHelpers(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("POLY_API_KEY"); got != "api-key" {
			t.Fatalf("unexpected api key header: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case rewardsUserEndpoint:
			switch r.URL.Query().Get("next_cursor") {
			case initialCursor:
				_, _ = w.Write([]byte(`{
					"limit": 1,
					"count": 1,
					"next_cursor": "cursor-2",
					"data": [{"date":"2026-03-12","condition_id":"cond-1","asset_address":"0xabc","maker_address":"0xdef","earnings":"1.23","asset_rate":"0.5"}]
				}`))
			case "cursor-2":
				_, _ = w.Write([]byte(`{
					"limit": 1,
					"count": 1,
					"next_cursor": "LTE=",
					"data": [{"date":"2026-03-12","condition_id":"cond-2","asset_address":"0xghi","maker_address":"0xjkl","earnings":"2.34","asset_rate":"0.7"}]
				}`))
			default:
				t.Fatalf("unexpected earnings cursor: %q", r.URL.Query().Get("next_cursor"))
			}
		case rewardsUserMarketsEndpoint:
			switch r.URL.Query().Get("next_cursor") {
			case initialCursor:
				_, _ = w.Write([]byte(`{
					"limit": 1,
					"count": 1,
					"next_cursor": "cursor-2",
					"data": [{"condition_id":"cond-1","question":"Q1","market_slug":"m1","event_slug":"e1","image":"i1","rewards_max_spread":"0.02","rewards_min_size":"10","market_competitiveness":"0.5","tokens":[],"rewards_config":[],"maker_address":"0xdef","earning_percentage":"0.25","earnings":[]}]
				}`))
			case "cursor-2":
				_, _ = w.Write([]byte(`{
					"limit": 1,
					"count": 1,
					"next_cursor": "LTE=",
					"data": [{"condition_id":"cond-2","question":"Q2","market_slug":"m2","event_slug":"e2","image":"i2","rewards_max_spread":"0.03","rewards_min_size":"20","market_competitiveness":"0.6","tokens":[],"rewards_config":[],"maker_address":"0xabc","earning_percentage":"0.50","earnings":[]}]
				}`))
			default:
				t.Fatalf("unexpected user rewards cursor: %q", r.URL.Query().Get("next_cursor"))
			}
		case rewardsMarketsCurrentEndpoint:
			switch r.URL.Query().Get("next_cursor") {
			case initialCursor:
				_, _ = w.Write([]byte(`{
					"limit": 1,
					"count": 1,
					"next_cursor": "cursor-2",
					"data": [{"condition_id":"cond-1","rewards_config":[],"rewards_max_spread":"0.02","rewards_min_size":"10"}]
				}`))
			case "cursor-2":
				_, _ = w.Write([]byte(`{
					"limit": 1,
					"count": 1,
					"next_cursor": "LTE=",
					"data": [{"condition_id":"cond-2","rewards_config":[],"rewards_max_spread":"0.03","rewards_min_size":"20"}]
				}`))
			default:
				t.Fatalf("unexpected current rewards cursor: %q", r.URL.Query().Get("next_cursor"))
			}
		case rewardsMarketsEndpoint + "cond-1":
			switch r.URL.Query().Get("next_cursor") {
			case initialCursor:
				_, _ = w.Write([]byte(`{
					"limit": 1,
					"count": 1,
					"next_cursor": "cursor-2",
					"data": [{"condition_id":"cond-1","question":"Q1","market_slug":"m1","event_slug":"e1","image":"i1","rewards_max_spread":"0.02","rewards_min_size":"10","tokens":[],"rewards_config":[]}]
				}`))
			case "cursor-2":
				_, _ = w.Write([]byte(`{
					"limit": 1,
					"count": 1,
					"next_cursor": "LTE=",
					"data": [{"condition_id":"cond-1","question":"Q2","market_slug":"m2","event_slug":"e2","image":"i2","rewards_max_spread":"0.03","rewards_min_size":"20","tokens":[],"rewards_config":[]}]
				}`))
			default:
				t.Fatalf("unexpected market rewards cursor: %q", r.URL.Query().Get("next_cursor"))
			}
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewAuthenticatedClient(Config{
		Host:       server.URL,
		PrivateKey: "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		Credentials: &Credentials{
			Key:        "api-key",
			Secret:     "c2VjcmV0",
			Passphrase: "pass",
		},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	earningsPage, err := client.GetEarningsForUserForDayPage(t.Context(), "2026-03-12", "")
	if err != nil || len(earningsPage.Data) != 1 || earningsPage.NextCursor != "cursor-2" {
		t.Fatalf("unexpected earnings page: %+v %v", earningsPage, err)
	}
	earnings, err := client.GetEarningsForUserForDay(t.Context(), "2026-03-12")
	if err != nil || len(earnings) != 2 {
		t.Fatalf("unexpected earnings: %+v %v", earnings, err)
	}

	userRewardsPage, err := client.GetUserEarningsAndMarketsConfigPage(
		t.Context(),
		UserRewardsFilterParams{Date: "2026-03-12"},
		"",
	)
	if err != nil || len(userRewardsPage.Data) != 1 || userRewardsPage.NextCursor != "cursor-2" {
		t.Fatalf("unexpected user rewards page: %+v %v", userRewardsPage, err)
	}
	userRewards, err := client.GetUserEarningsAndMarketsConfig(
		t.Context(),
		UserRewardsFilterParams{Date: "2026-03-12"},
	)
	if err != nil || len(userRewards) != 2 {
		t.Fatalf("unexpected user rewards: %+v %v", userRewards, err)
	}

	currentRewardsPage, err := client.GetCurrentRewardsPage(t.Context(), "")
	if err != nil || len(currentRewardsPage.Data) != 1 ||
		currentRewardsPage.NextCursor != "cursor-2" {
		t.Fatalf("unexpected current rewards page: %+v %v", currentRewardsPage, err)
	}
	currentRewards, err := client.GetCurrentRewards(t.Context())
	if err != nil || len(currentRewards) != 2 {
		t.Fatalf("unexpected current rewards: %+v %v", currentRewards, err)
	}

	marketRewardsPage, err := client.GetRewardsForMarketPage(t.Context(), "cond-1", "")
	if err != nil || len(marketRewardsPage.Data) != 1 ||
		marketRewardsPage.NextCursor != "cursor-2" {
		t.Fatalf("unexpected market rewards page: %+v %v", marketRewardsPage, err)
	}
	marketRewards, err := client.GetRewardsForMarket(t.Context(), "cond-1")
	if err != nil || len(marketRewards) != 2 {
		t.Fatalf("unexpected market rewards: %+v %v", marketRewards, err)
	}
}
