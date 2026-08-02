package clob

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	json "github.com/go-json-experiment/json"
	"github.com/quagmt/udecimal"
)

func TestTypedReadOnlySurfaces(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("OK"))
		case marketEndpoint + "cond-1":
			_, _ = w.Write([]byte(`{
				"enable_order_book": true,
				"active": true,
				"closed": false,
				"archived": false,
				"accepting_orders": true,
				"accepting_order_timestamp": "2026-03-12T18:00:00Z",
				"minimum_order_size": "5",
				"minimum_tick_size": "0.01",
				"condition_id": "cond-1",
				"question_id": "question-1",
				"question": "Will this ship?",
				"description": "Ship it.",
				"market_slug": "ship-it",
				"end_date_iso": "2026-03-13T00:00:00Z",
				"game_start_time": null,
				"seconds_delay": 0,
				"fpmm": null,
				"maker_base_fee": "0",
				"taker_base_fee": "0",
				"notifications_enabled": true,
				"neg_risk": false,
				"neg_risk_market_id": null,
				"neg_risk_request_id": null,
				"icon": "icon.png",
				"image": "image.png",
				"rewards": {"rates": [], "min_size": "10", "max_spread": "0.02"},
				"is_50_50_outcome": true,
				"tokens": [{"token_id": "123", "outcome": "Yes", "price": "0.55", "winner": false}],
				"tags": ["featured"]
			}`))
		case marketsEndpoint:
			_, _ = w.Write([]byte(`{
				"limit": 1,
				"count": 1,
				"next_cursor": "LTE=",
				"data": [{
					"enable_order_book": true,
					"active": true,
					"closed": false,
					"archived": false,
					"accepting_orders": true,
					"minimum_order_size": "5",
					"minimum_tick_size": "0.01",
					"condition_id": "cond-1",
					"question_id": "question-1",
					"question": "Will this ship?",
					"description": "Ship it.",
					"market_slug": "ship-it",
					"seconds_delay": 0,
					"maker_base_fee": "0",
					"taker_base_fee": "0",
					"notifications_enabled": true,
					"neg_risk": false,
					"icon": "icon.png",
					"image": "image.png",
					"rewards": {"rates": [], "min_size": "10", "max_spread": "0.02"},
					"is_50_50_outcome": true,
					"tokens": [],
					"tags": []
				}]
			}`))
		case priceHistoryEndpoint:
			if got := r.URL.Query().Get("market"); got != "123" {
				t.Fatalf("unexpected market query: %s", got)
			}
			if got := r.URL.Query().Get("interval"); got != "1d" {
				t.Fatalf("unexpected interval query: %s", got)
			}
			_, _ = w.Write([]byte(`{"history":[{"t":1710000000,"p":0.42}]}`))
		case marketTradesEventsEndpoint + "cond-1":
			_, _ = w.Write([]byte(`[
				{
					"event_type": "trade",
					"market": {
						"condition_id": "cond-1",
						"asset_id": "123",
						"question": "Will this ship?",
						"icon": "icon.png",
						"slug": "ship-it"
					},
					"user": {
						"address": "0x1",
						"username": "nick",
						"profile_picture": "p.png",
						"optimized_profile_picture": "op.png",
						"pseudonym": "n"
					},
					"side": "BUY",
					"size": "10",
					"fee_rate_bps": "0",
					"price": "0.42",
					"outcome": "Yes",
					"outcome_index": 0,
					"transaction_hash": "0xabc",
					"timestamp": "2026-03-12T18:00:00Z"
				}
			]`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(Config{Host: server.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	health, err := client.GetOk(t.Context())
	if err != nil {
		t.Fatalf("get ok check: %v", err)
	}
	if health != "OK" {
		t.Fatalf("unexpected health: %q", health)
	}

	market, err := client.GetMarket(t.Context(), "cond-1")
	if err != nil {
		t.Fatalf("get market: %v", err)
	}
	if market.Question != "Will this ship?" {
		t.Fatalf("unexpected market question: %s", market.Question)
	}

	page, err := client.GetMarketsPage(t.Context(), "")
	if err != nil {
		t.Fatalf("get markets page: %v", err)
	}
	if len(page.Data) != 1 || page.Data[0].MarketSlug != "ship-it" {
		t.Fatalf("unexpected market page: %+v", page)
	}

	history, err := client.GetPricesHistory(t.Context(), PriceHistoryFilterParams{
		Market:   "123",
		Interval: PriceHistoryIntervalOneDay,
	})
	if err != nil {
		t.Fatalf("get price history: %v", err)
	}
	if len(history) != 1 || history[0].P.Cmp(udecimal.MustParse("0.42")) != 0 {
		t.Fatalf("unexpected price history: %+v", history)
	}

	events, err := client.GetMarketTradesEvents(t.Context(), "cond-1")
	if err != nil {
		t.Fatalf("get market trade events: %v", err)
	}
	if len(events) != 1 || events[0].User.Username != "nick" {
		t.Fatalf("unexpected trade events: %+v", events)
	}
}

func TestTypedAuthenticatedSurfaces(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("POLY_API_KEY"); got != "api-key" {
			t.Fatalf("unexpected api key header: %s", got)
		}
		if got := r.Header.Get("POLY_SIGNATURE"); got == "" {
			t.Fatal("expected poly signature header")
		}

		w.Header().Set("Content-Type", "application/json")

		switch r.Method + " " + r.URL.Path {
		case http.MethodGet + " " + getAPIKeysEndpoint:
			_, _ = w.Write([]byte(`{"apiKeys":["api-key","other-key"]}`))
		case http.MethodPost + " " + createReadonlyAPIKeyEndpoint:
			_, _ = w.Write([]byte(`{"apiKey":"readonly-1"}`))
		case http.MethodGet + " " + getReadonlyAPIKeysEndpoint:
			_, _ = w.Write([]byte(`{"readonly_api_keys":["readonly-1","readonly-2"]}`))
		case http.MethodDelete + " " + deleteReadonlyAPIKeyEndpoint:
			var payload DeleteReadonlyAPIKeyRequest
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode readonly delete payload: %v", err)
			}
			if payload.Key != "readonly-1" {
				t.Fatalf("unexpected readonly key payload: %+v", payload)
			}
			_, _ = w.Write([]byte(`true`))
		case http.MethodGet + " " + notificationsEndpoint,
			http.MethodDelete + " " + notificationsEndpoint:
			if r.Method == http.MethodGet {
				if got := r.URL.Query().Get("signature_type"); got != "0" {
					t.Fatalf("unexpected notification signature type: %s", got)
				}
				_, _ = w.Write([]byte(`[
					{
						"type": 1,
						"owner": "api-key",
						"payload": {
							"asset_id": "123",
							"condition_id": "cond-1",
							"eventSlug": "ship-it",
							"icon": "icon.png",
							"image": "image.png",
							"market": "cond-1",
							"market_slug": "ship-it",
							"matched_size": "10",
							"order_id": "order-1",
							"original_size": "20",
							"outcome": "Yes",
							"outcome_index": 0,
							"owner": "api-key",
							"price": "0.42",
							"question": "Will this ship?",
							"remaining_size": "10",
							"seriesSlug": "series",
							"side": "BUY",
							"trade_id": "trade-1",
							"transaction_hash": "0xabc",
							"type": "GTC"
						}
					}
				]`))
				return
			}
			if got := r.URL.Query().Get("ids"); got != "n1,n2" {
				t.Fatalf("unexpected notification delete query: %s", got)
			}
			w.WriteHeader(http.StatusOK)
		case http.MethodGet + " " + balanceAllowanceEndpoint:
			if got := r.URL.Query().Get("asset_type"); got != "CONDITIONAL" {
				t.Fatalf("unexpected asset type: %s", got)
			}
			if got := r.URL.Query().Get("token_id"); got != "123" {
				t.Fatalf("unexpected token id: %s", got)
			}
			if got := r.URL.Query().Get("signature_type"); got != "0" {
				t.Fatalf("unexpected signature type: %s", got)
			}
			_, _ = w.Write([]byte(`{"balance":"100","allowances":{"0xabc":"250"}}`))
		case http.MethodGet + " " + updateBalanceAllowanceEndpoint:
			w.WriteHeader(http.StatusOK)
		case http.MethodGet + " " + orderScoringEndpoint:
			if got := r.URL.Query().Get("order_id"); got != "order-1" {
				t.Fatalf("unexpected order scoring query: %s", got)
			}
			_, _ = w.Write([]byte(`{"scoring":true}`))
		case http.MethodPost + " " + ordersScoringEndpoint:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read orders scoring body: %v", err)
			}
			if string(body) != `["order-1","order-2"]` {
				t.Fatalf("unexpected orders scoring body: %s", string(body))
			}
			_, _ = w.Write([]byte(`{"order-1":true,"order-2":false}`))
		case http.MethodDelete + " " + cancelMarketOrdersEndpoint:
			var payload CancelMarketOrdersRequest
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode cancel market payload: %v", err)
			}
			if payload.Market != "cond-1" || payload.AssetID != "123" {
				t.Fatalf("unexpected cancel market payload: %+v", payload)
			}
			_, _ = w.Write([]byte(`{"canceled":["order-1"],"not_canceled":{}}`))
		case http.MethodGet + " " + rewardsPercentagesEndpoint:
			if got := r.URL.Query().Get("signature_type"); got != "0" {
				t.Fatalf("unexpected reward percentages signature type: %s", got)
			}
			_, _ = w.Write([]byte(`{"cond-1":"0.25"}`))
		case http.MethodGet + " " + rewardsMarketsCurrentEndpoint:
			_, _ = w.Write([]byte(`{
				"limit": 1,
				"count": 1,
				"next_cursor": "LTE=",
				"data": [{
					"condition_id": "cond-1",
					"rewards_config": [],
					"rewards_max_spread": "0.02",
					"rewards_min_size": "10"
				}]
			}`))
		case http.MethodGet + " " + rewardsMarketsEndpoint + "cond-1":
			_, _ = w.Write([]byte(`{
				"limit": 1,
				"count": 1,
				"next_cursor": "LTE=",
				"data": [{
					"condition_id": "cond-1",
					"question": "Will this ship?",
					"market_slug": "ship-it",
					"event_slug": "ship-it",
					"image": "image.png",
					"rewards_max_spread": "0.02",
					"rewards_min_size": "10",
					"market_competitiveness": "0.5",
					"tokens": [],
					"rewards_config": []
				}]
			}`))
		case http.MethodGet + " " + rewardsUserEndpoint:
			_, _ = w.Write([]byte(`{
				"limit": 1,
				"count": 1,
				"next_cursor": "LTE=",
				"data": [{
					"date": "2026-03-12",
					"condition_id": "cond-1",
					"asset_address": "0xabc",
					"maker_address": "0xdef",
					"earnings": "1.23",
					"asset_rate": "0.5"
				}]
			}`))
		case http.MethodGet + " " + rewardsUserTotalEndpoint:
			_, _ = w.Write([]byte(`[{
				"date": "2026-03-12",
				"asset_address": "0xabc",
				"maker_address": "0xdef",
				"earnings": "1.23",
				"asset_rate": "0.5"
			}]`))
		case http.MethodGet + " " + rewardsUserMarketsEndpoint:
			_, _ = w.Write([]byte(`{
				"limit": 1,
				"count": 1,
				"next_cursor": "LTE=",
				"data": [{
					"condition_id": "cond-1",
					"question": "Will this ship?",
					"market_slug": "ship-it",
					"event_slug": "ship-it",
					"image": "image.png",
					"rewards_max_spread": "0.02",
					"rewards_min_size": "10",
					"market_competitiveness": "0.5",
					"tokens": [],
					"rewards_config": [],
					"maker_address": "0xdef",
					"earning_percentage": "0.25",
					"earnings": []
				}]
			}`))
		default:
			t.Fatalf("unexpected endpoint: %s %s", r.Method, r.URL.Path)
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

	apiKeys, err := client.GetAPIKeys(t.Context())
	if err != nil || len(apiKeys.APIKeys) != 2 || apiKeys.APIKeys[0] != "api-key" {
		t.Fatalf("get api keys: %+v %v", apiKeys, err)
	}

	readonly, err := client.CreateReadonlyAPIKey(t.Context())
	if err != nil || readonly.APIKey != "readonly-1" {
		t.Fatalf("create readonly api key: %+v %v", readonly, err)
	}

	readonlyKeys, err := client.GetReadonlyAPIKeys(t.Context())
	if err != nil || len(readonlyKeys) != 2 {
		t.Fatalf("get readonly api keys: %+v %v", readonlyKeys, err)
	}

	deleted, err := client.DeleteReadonlyAPIKey(t.Context(), "readonly-1")
	if err != nil || !deleted {
		t.Fatalf("delete readonly api key: %v %v", deleted, err)
	}

	notifications, err := client.GetNotifications(t.Context())
	if err != nil || len(notifications) != 1 {
		t.Fatalf("get notifications: %+v %v", notifications, err)
	}

	if err := client.DeleteNotifications(
		t.Context(),
		DeleteNotificationsParams{IDs: []string{"n1", "n2"}},
	); err != nil {
		t.Fatalf("delete notifications: %v", err)
	}

	allowance, err := client.GetBalanceAllowance(t.Context(), BalanceAllowanceParams{
		AssetType: AssetTypeConditional,
		TokenID:   "123",
	})
	if err != nil || allowance.Allowances["0xabc"] != "250" {
		t.Fatalf("get balance allowance: %+v %v", allowance, err)
	}

	if err := client.UpdateBalanceAllowance(t.Context(), BalanceAllowanceParams{
		AssetType: AssetTypeConditional,
		TokenID:   "123",
	}); err != nil {
		t.Fatalf("update balance allowance: %v", err)
	}

	scoring, err := client.IsOrderScoring(
		t.Context(),
		OrderScoringParams{OrderID: "order-1"},
	)
	if err != nil || !scoring.Scoring {
		t.Fatalf("is order scoring: %+v %v", scoring, err)
	}

	ordersScoring, err := client.AreOrdersScoring(t.Context(), OrdersScoringParams{
		OrderIDs: []string{"order-1", "order-2"},
	})
	if err != nil || !ordersScoring["order-1"] || ordersScoring["order-2"] {
		t.Fatalf("are orders scoring: %+v %v", ordersScoring, err)
	}

	cancelled, err := client.CancelMarketOrders(t.Context(), CancelMarketOrdersRequest{
		Market:  "cond-1",
		AssetID: "123",
	})
	if err != nil || len(cancelled.Canceled) != 1 {
		t.Fatalf("cancel market orders: %+v %v", cancelled, err)
	}

	percentages, err := client.GetRewardPercentages(t.Context())
	if err != nil || percentages["cond-1"] != "0.25" {
		t.Fatalf("get reward percentages: %+v %v", percentages, err)
	}

	currentRewardsPage, err := client.GetCurrentRewardsPage(t.Context(), "")
	if err != nil || len(currentRewardsPage.Data) != 1 {
		t.Fatalf("get current rewards page: %+v %v", currentRewardsPage, err)
	}

	currentRewards, err := client.GetCurrentRewards(t.Context())
	if err != nil || len(currentRewards) != 1 {
		t.Fatalf("get current rewards: %+v %v", currentRewards, err)
	}

	marketRewardsPage, err := client.GetRewardsForMarketPage(t.Context(), "cond-1", "")
	if err != nil || len(marketRewardsPage.Data) != 1 {
		t.Fatalf("get rewards for market page: %+v %v", marketRewardsPage, err)
	}

	marketRewards, err := client.GetRewardsForMarket(t.Context(), "cond-1")
	if err != nil || len(marketRewards) != 1 {
		t.Fatalf("get rewards for market: %+v %v", marketRewards, err)
	}

	earningsPage, err := client.GetEarningsForUserForDayPage(t.Context(), "2026-03-12", "")
	if err != nil || len(earningsPage.Data) != 1 {
		t.Fatalf("get earnings for user page: %+v %v", earningsPage, err)
	}

	earnings, err := client.GetEarningsForUserForDay(t.Context(), "2026-03-12")
	if err != nil || len(earnings) != 1 {
		t.Fatalf("get earnings for user: %+v %v", earnings, err)
	}

	totalEarnings, err := client.GetTotalEarningsForUserForDay(t.Context(), "2026-03-12")
	if err != nil || len(totalEarnings) != 1 {
		t.Fatalf("get total earnings for user: %+v %v", totalEarnings, err)
	}

	userRewardsPage, err := client.GetUserEarningsAndMarketsConfigPage(
		t.Context(),
		UserRewardsFilterParams{
			Date:          "2026-03-12",
			NoCompetition: true,
		},
		"",
	)
	if err != nil || len(userRewardsPage.Data) != 1 {
		t.Fatalf("get user rewards and markets config page: %+v %v", userRewardsPage, err)
	}

	userRewards, err := client.GetUserEarningsAndMarketsConfig(
		t.Context(),
		UserRewardsFilterParams{
			Date:          "2026-03-12",
			NoCompetition: true,
		},
	)
	if err != nil || len(userRewards) != 1 {
		t.Fatalf("get user rewards and markets config: %+v %v", userRewards, err)
	}
}
