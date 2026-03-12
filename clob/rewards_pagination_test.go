package clob

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

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

	client, err := New(Config{
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

	earningsPage, err := client.GetEarningsForUserForDayPage(context.Background(), "2026-03-12", "")
	if err != nil || len(earningsPage.Data) != 1 || earningsPage.NextCursor != "cursor-2" {
		t.Fatalf("unexpected earnings page: %+v %v", earningsPage, err)
	}
	earnings, err := client.GetEarningsForUserForDay(context.Background(), "2026-03-12")
	if err != nil || len(earnings) != 2 {
		t.Fatalf("unexpected earnings: %+v %v", earnings, err)
	}

	userRewardsPage, err := client.GetUserRewardsAndMarketsConfigPage(
		context.Background(),
		UserRewardsFilterParams{Date: "2026-03-12"},
		"",
	)
	if err != nil || len(userRewardsPage.Data) != 1 || userRewardsPage.NextCursor != "cursor-2" {
		t.Fatalf("unexpected user rewards page: %+v %v", userRewardsPage, err)
	}
	userRewards, err := client.GetUserRewardsAndMarketsConfig(
		context.Background(),
		UserRewardsFilterParams{Date: "2026-03-12"},
	)
	if err != nil || len(userRewards) != 2 {
		t.Fatalf("unexpected user rewards: %+v %v", userRewards, err)
	}

	currentRewardsPage, err := client.GetCurrentRewardsPage(context.Background(), "")
	if err != nil || len(currentRewardsPage.Data) != 1 ||
		currentRewardsPage.NextCursor != "cursor-2" {
		t.Fatalf("unexpected current rewards page: %+v %v", currentRewardsPage, err)
	}
	currentRewards, err := client.GetCurrentRewards(context.Background())
	if err != nil || len(currentRewards) != 2 {
		t.Fatalf("unexpected current rewards: %+v %v", currentRewards, err)
	}

	marketRewardsPage, err := client.GetRewardsForMarketPage(context.Background(), "cond-1", "")
	if err != nil || len(marketRewardsPage.Data) != 1 ||
		marketRewardsPage.NextCursor != "cursor-2" {
		t.Fatalf("unexpected market rewards page: %+v %v", marketRewardsPage, err)
	}
	marketRewards, err := client.GetRewardsForMarket(context.Background(), "cond-1")
	if err != nil || len(marketRewards) != 2 {
		t.Fatalf("unexpected market rewards: %+v %v", marketRewards, err)
	}
}
