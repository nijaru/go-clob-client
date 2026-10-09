package gamma

import (
	"net/http"
	"testing"
)

func TestClient_GetMarket(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/markets/123" {
			t.Errorf("path = %s, want /markets/123", r.URL.Path)
		}
		writeJSON(w, Market{ID: "123", Question: ptr("Will it rain?")})
	})

	m, err := client.GetMarket(t.Context(), "123")
	if err != nil {
		t.Fatalf("GetMarket: %v", err)
	}
	if m.ID != "123" || (m.Question == nil || *m.Question != "Will it rain?") {
		t.Errorf("market = %+v", m)
	}
}

func TestClient_GetMarketDecodesRustStringifiedArrays(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"id":            "123",
			"outcomePrices": `["0.55","0.45"]`,
			"outcomes":      `["Yes","No"]`,
			"clobTokenIds":  `["11","22"]`,
		})
	})

	market, err := client.GetMarket(t.Context(), "123")
	if err != nil {
		t.Fatalf("GetMarket: %v", err)
	}
	if len(market.OutcomePrices) != 2 || market.OutcomePrices[0] != "0.55" {
		t.Fatalf("outcome prices = %#v", market.OutcomePrices)
	}
	if len(market.Outcomes) != 2 || market.Outcomes[1] != "No" {
		t.Fatalf("outcomes = %#v", market.Outcomes)
	}
	if len(market.CLOBTokenIDs) != 2 || market.CLOBTokenIDs[1] != "22" {
		t.Fatalf("clob token ids = %#v", market.CLOBTokenIDs)
	}
}

func TestClient_GetMarketBySlug(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/markets/slug/will-it-rain" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if raw := r.URL.RawQuery; raw != "" {
			t.Errorf("expected no query, got %q", raw)
		}
		writeJSON(w, Market{ID: "123", Slug: ptr("will-it-rain")})
	})

	m, err := client.GetMarketBySlug(t.Context(), "will-it-rain")
	if err != nil {
		t.Fatalf("GetMarketBySlug: %v", err)
	}
	if m.Slug == nil || *m.Slug != "will-it-rain" {
		t.Errorf("slug = %v", m.Slug)
	}
}

func TestClient_GetMarkets(t *testing.T) {
	active := true
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/markets" {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("active") != "true" {
			t.Errorf("active = %q, want true", q.Get("active"))
		}
		if q.Get("limit") != "10" {
			t.Errorf("limit = %q, want 10", q.Get("limit"))
		}
		if q.Get("order") != "volumeNum" {
			t.Errorf("order = %q, want volumeNum", q.Get("order"))
		}
		// Multi-value params
		ids := q["clob_token_ids"]
		if len(ids) != 2 || ids[0] != "a" || ids[1] != "b" {
			t.Errorf("clob_token_ids = %v", ids)
		}
		writeJSON(w, []Market{{ID: "1"}})
	})

	markets, err := client.GetMarkets(t.Context(), MarketFilterParams{
		Active:       &active,
		Limit:        10,
		Order:        "volume",
		ClobTokenIDs: []string{"a", "b"},
	})
	if err != nil {
		t.Fatalf("GetMarkets: %v", err)
	}
	if len(markets) != 1 {
		t.Errorf("got %d markets", len(markets))
	}
}

func TestClient_GetMarkets_AllFilters(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		// Verify a sampling of filter params are wired
		checks := map[string]string{
			"closed":            "true",
			"archived":          "false",
			"negative_risk":     "true",
			"accepting_orders":  "true",
			"slug":              "test-slug",
			"event_id":          "evt-1",
			"tag_id":            "tag-1",
			"ascending":         "true",
			"liquidity_num_min": "100",
			"volume_num_max":    "5000",
			"start_date_min":    "2025-01-01",
			"end_date_max":      "2025-12-31",
		}
		for k, want := range checks {
			if got := q.Get(k); got != want {
				t.Errorf("%s = %q, want %q", k, got, want)
			}
		}
		// Multi-value
		if n := len(q["condition_ids"]); n != 1 {
			t.Errorf("condition_ids count = %d, want 1", n)
		}
		if n := len(q["market_maker_address"]); n != 2 {
			t.Errorf("market_maker_address count = %d, want 2", n)
		}
		writeJSON(w, []Market{})
	})

	_, err := client.GetMarkets(t.Context(), MarketFilterParams{
		Closed:             boolPtr(true),
		Archived:           boolPtr(false),
		NegativeRisk:       boolPtr(true),
		AcceptingOrders:    boolPtr(true),
		Slug:               "test-slug",
		EventID:            "evt-1",
		TagID:              "tag-1",
		Ascending:          boolPtr(true),
		ConditionIDs:       []string{"cid-1"},
		MarketMakerAddress: []string{"addr-1", "addr-2"},
		LiquidityNumMin:    "100",
		VolumeNumMax:       "5000",
		StartDateMin:       "2025-01-01",
		EndDateMax:         "2025-12-31",
	})
	if err != nil {
		t.Fatalf("GetMarkets: %v", err)
	}
}

func TestClient_IterMarkets(t *testing.T) {
	call := 0
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		call++
		switch call {
		case 1:
			// Return full page (limit=2)
			writeJSON(w, []Market{{ID: "1"}, {ID: "2"}})
		case 2:
			// Return partial page — signals end
			writeJSON(w, []Market{{ID: "3"}})
		default:
			t.Error("iterator did not stop after partial page")
		}
	})

	var ids []string
	for m, err := range client.IterMarkets(t.Context(), MarketFilterParams{Limit: 2}) {
		if err != nil {
			t.Fatalf("IterMarkets: %v", err)
		}
		ids = append(ids, m.ID)
	}
	if len(ids) != 3 || ids[0] != "1" || ids[2] != "3" {
		t.Errorf("iter collected = %v", ids)
	}
}

func TestClient_IterMarkets_Empty(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []Market{})
	})

	var count int
	for range client.IterMarkets(t.Context(), MarketFilterParams{}) {
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 markets, got %d", count)
	}
}

func TestClient_IterMarkets_Error(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"boom"}`))
	})

	for _, err := range client.IterMarkets(t.Context(), MarketFilterParams{}) {
		if err == nil {
			t.Fatal("expected error from iterator")
		}
		return
	}
	t.Fatal("expected at least one yield with error")
}

func TestClient_GetMarketTags(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/markets/m-1/tags" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, []Tag{{ID: "t-1"}})
	})

	tags, err := client.GetMarketTags(t.Context(), "m-1")
	if err != nil {
		t.Fatalf("GetMarketTags: %v", err)
	}
	if len(tags) != 1 {
		t.Errorf("got %d tags", len(tags))
	}
}

func TestClient_GetMarketTypes(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sports/market-types" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, SportsMarketTypesResponse{
			MarketTypes: []string{"moneyline", "spread"},
		})
	})

	resp, err := client.GetMarketTypes(t.Context())
	if err != nil {
		t.Fatalf("GetMarketTypes: %v", err)
	}
	if len(resp.MarketTypes) != 2 || resp.MarketTypes[0] != "moneyline" {
		t.Errorf("market types = %+v", resp)
	}
}
