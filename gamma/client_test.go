package gamma

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	json "github.com/go-json-experiment/json"
)

// writeJSON marshals v and writes it to w. Panics on marshal failure (test-only).
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	w.Write(data)
}

// newTestServer creates an httptest server that validates the request path and
// query, then writes the provided response body. Returns the server and a
// Client wired to it.
func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, New(Config{Host: srv.URL})
}

func TestClient_GetMarket(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/markets/123" {
			t.Errorf("path = %s, want /markets/123", r.URL.Path)
		}
		writeJSON(w, Market{ID: "123", Question: "Will it rain?"})
	})

	m, err := client.GetMarket(t.Context(), "123")
	if err != nil {
		t.Fatalf("GetMarket: %v", err)
	}
	if m.ID != "123" || m.Question != "Will it rain?" {
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

func TestClient_RustResourceOptions(t *testing.T) {
	boolPtr := func(value bool) *bool { return &value }
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch r.URL.Path {
		case "/markets/m-1":
			if q.Get("include_tag") != "true" {
				t.Errorf("include_tag = %q", q.Get("include_tag"))
			}
			writeJSON(w, Market{ID: "m-1"})
		case "/events/e-1":
			if q.Get("include_chat") != "true" || q.Get("include_template") != "false" {
				t.Errorf("event options = %v", q)
			}
			writeJSON(w, Event{ID: "e-1"})
		case "/series/s-1":
			if q.Get("include_chat") != "true" {
				t.Errorf("include_chat = %q", q.Get("include_chat"))
			}
			writeJSON(w, Series{ID: "s-1"})
		case "/tags/t-1":
			if q.Get("include_template") != "true" {
				t.Errorf("include_template = %q", q.Get("include_template"))
			}
			writeJSON(w, Tag{ID: "t-1"})
		case "/tags/t-1/related-tags", "/tags/slug/topic/related-tags":
			if q.Get("omit_empty") != "false" || q.Get("status") != "active" {
				t.Errorf("related-tag options = %v", q)
			}
			writeJSON(w, []RelatedTag{})
		case "/public-search":
			if q.Get("limit_per_type") != "3" || q.Get("page") != "2" {
				t.Errorf("search pagination = %v", q)
			}
			writeJSON(w, SearchResults{})
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	})

	if _, err := client.GetMarket(
		t.Context(),
		"m-1",
		MarketOptions{IncludeTag: boolPtr(true)},
	); err != nil {
		t.Fatalf("GetMarket options: %v", err)
	}
	if _, err := client.GetEvent(t.Context(), "e-1", EventOptions{
		IncludeChat:     boolPtr(true),
		IncludeTemplate: boolPtr(false),
	}); err != nil {
		t.Fatalf("GetEvent options: %v", err)
	}
	if _, err := client.GetSeries(
		t.Context(),
		"s-1",
		SeriesOptions{IncludeChat: boolPtr(true)},
	); err != nil {
		t.Fatalf("GetSeries options: %v", err)
	}
	if _, err := client.GetTag(
		t.Context(),
		"t-1",
		TagOptions{IncludeTemplate: boolPtr(true)},
	); err != nil {
		t.Fatalf("GetTag options: %v", err)
	}
	if _, err := client.GetRelatedTags(t.Context(), "t-1", RelatedTagsOptions{
		OmitEmpty: boolPtr(false),
		Status:    string(RelatedTagsStatusActive),
	}); err != nil {
		t.Fatalf("GetRelatedTags options: %v", err)
	}
	if _, err := client.GetRelatedTagsBySlug(t.Context(), "topic", RelatedTagsOptions{
		OmitEmpty: boolPtr(false),
		Status:    string(RelatedTagsStatusActive),
	}); err != nil {
		t.Fatalf("GetRelatedTagsBySlug options: %v", err)
	}
	if _, err := client.Search(t.Context(), SearchParams{
		Query:        "rain",
		LimitPerType: 3,
		Page:         2,
	}); err != nil {
		t.Fatalf("Search pagination: %v", err)
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
		writeJSON(w, Market{ID: "123", Slug: "will-it-rain"})
	})

	m, err := client.GetMarketBySlug(t.Context(), "will-it-rain")
	if err != nil {
		t.Fatalf("GetMarketBySlug: %v", err)
	}
	if m.Slug != "will-it-rain" {
		t.Errorf("slug = %s", m.Slug)
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
		if q.Get("order") != "volume" {
			t.Errorf("order = %q, want volume", q.Get("order"))
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

func TestClient_GetEvent(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/events/evt-1" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, Event{ID: "evt-1", Title: "Test Event"})
	})

	ev, err := client.GetEvent(t.Context(), "evt-1")
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if ev.Title != "Test Event" {
		t.Errorf("title = %s", ev.Title)
	}
}

func TestClient_GetEventBySlug(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/events/slug/test-event" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, Event{ID: "1", Slug: "test-event"})
	})

	ev, err := client.GetEventBySlug(t.Context(), "test-event")
	if err != nil {
		t.Fatalf("GetEventBySlug: %v", err)
	}
	if ev.Slug != "test-event" {
		t.Errorf("slug = %s", ev.Slug)
	}
}

func TestClient_GetEvents(t *testing.T) {
	featured := true
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/events" {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("featured") != "true" {
			t.Errorf("featured = %q", q.Get("featured"))
		}
		if q.Get("tag_slug") != "politics" {
			t.Errorf("tag_slug = %q", q.Get("tag_slug"))
		}
		writeJSON(w, []Event{{ID: "1"}})
	})

	events, err := client.GetEvents(t.Context(), EventFilterParams{
		Featured: &featured,
		TagSlug:  "politics",
	})
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("got %d events", len(events))
	}
}

func TestClient_GetEvents_AllFilters(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		checks := map[string]string{
			"active":         "true",
			"closed":         "false",
			"negative_risk":  "true",
			"cyom":           "true",
			"include_chat":   "true",
			"related_tags":   "true",
			"recurrence":     "daily",
			"liquidity_min":  "500",
			"volume_max":     "10000",
			"start_date_min": "2025-01-01",
			"end_date_max":   "2025-12-31",
		}
		for k, want := range checks {
			if got := q.Get(k); got != want {
				t.Errorf("%s = %q, want %q", k, got, want)
			}
		}
		writeJSON(w, []Event{})
	})

	_, err := client.GetEvents(t.Context(), EventFilterParams{
		Active:       boolPtr(true),
		Closed:       boolPtr(false),
		NegativeRisk: boolPtr(true),
		CYOM:         boolPtr(true),
		IncludeChat:  boolPtr(true),
		RelatedTags:  boolPtr(true),
		Recurrence:   "daily",
		LiquidityMin: "500",
		VolumeMax:    "10000",
		StartDateMin: "2025-01-01",
		EndDateMax:   "2025-12-31",
	})
	if err != nil {
		t.Fatalf("GetEvents: %v", err)
	}
}

func TestClient_IterEvents(t *testing.T) {
	call := 0
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		call++
		switch call {
		case 1:
			events := make([]Event, 3)
			for i := range events {
				events[i] = Event{ID: string(rune('a' + i))}
			}
			writeJSON(w, events)
		case 2:
			writeJSON(w, []Event{{ID: "d"}})
		default:
			t.Error("iterator did not stop")
		}
	})

	var count int
	for _, err := range client.IterEvents(t.Context(), EventFilterParams{Limit: 3}) {
		if err != nil {
			t.Fatalf("IterEvents: %v", err)
		}
		count++
	}
	if count != 4 {
		t.Errorf("count = %d, want 4", count)
	}
}

func TestClient_Search(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/public-search" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("q") != "rain" {
			t.Errorf("q = %q", r.URL.Query().Get("q"))
		}
		writeJSON(w, SearchResults{
			Events: []Event{{ID: "123", Title: "Will it rain?"}},
			Tags: []SearchTag{
				{ID: "tag-1", Label: "Weather", Slug: "weather", EventCount: 4},
			},
			Pagination: &Pagination{HasMore: true, TotalResults: 2},
		})
	})

	results, err := client.Search(t.Context(), SearchParams{Query: "rain"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results.Events) != 1 || results.Events[0].ID != "123" {
		t.Errorf("events = %+v", results.Events)
	}
	if len(results.Tags) != 1 || results.Tags[0].Slug != "weather" {
		t.Errorf("tags = %+v", results.Tags)
	}
	if results.Pagination == nil || !results.Pagination.HasMore {
		t.Errorf("pagination = %+v", results.Pagination)
	}
}

func TestClient_SearchValidation(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected request")
	})

	_, err := client.Search(t.Context(), SearchParams{})
	if err == nil {
		t.Error("expected error for empty query")
	}

	_, err = client.Search(t.Context(), SearchParams{Query: "test", Sort: "invalid"})
	if err == nil {
		t.Error("expected error for invalid sort")
	}
}

func TestClient_SearchWithSort(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/public-search" {
			t.Errorf("path = %s", r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("q") != "Bitcoin" {
			t.Errorf("q = %q", query.Get("q"))
		}
		if query.Get("sort") != "volume_24hr" {
			t.Errorf("sort = %q", query.Get("sort"))
		}
		if query.Get("ascending") != "false" {
			t.Errorf("ascending = %q", query.Get("ascending"))
		}
		writeJSON(w, SearchResults{Pagination: &Pagination{}})
	})

	asc := false
	_, err := client.Search(t.Context(), SearchParams{
		Query:     "Bitcoin",
		Sort:      SearchSortVolume24h,
		Ascending: &asc,
	})
	if err != nil {
		t.Fatalf("Search with sort: %v", err)
	}
}

func TestSearchSort_IsValid(t *testing.T) {
	tests := []struct {
		sort SearchSort
		ok   bool
	}{
		{SearchSortVolume, true},
		{SearchSortVolume24h, true},
		{SearchSortLiquidity, true},
		{SearchSortCompetitive, true},
		{SearchSortClosedTime, true},
		{SearchSortStartDate, true},
		{SearchSortEndDate, true},
		{SearchSort(""), false},
		{SearchSort("invalid"), false},
	}
	for _, tt := range tests {
		if got := tt.sort.IsValid(); got != tt.ok {
			t.Errorf("SearchSort(%q).IsValid() = %v, want %v", tt.sort, got, tt.ok)
		}
	}
}

// --- Series ---

func TestClient_GetSeries(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/series/s-1" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, Series{ID: "s-1", Title: "NBA Finals"})
	})

	s, err := client.GetSeries(t.Context(), "s-1")
	if err != nil {
		t.Fatalf("GetSeries: %v", err)
	}
	if s.Title != "NBA Finals" {
		t.Errorf("title = %s", s.Title)
	}
}

func TestClient_GetSeriesPage(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/series" {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("limit") != "5" {
			t.Errorf("limit = %q", q.Get("limit"))
		}
		if q.Get("closed") != "true" {
			t.Errorf("closed = %q", q.Get("closed"))
		}
		writeJSON(w, []Series{{ID: "1"}})
	})

	boolPtr := func(v bool) *bool { return &v }
	series, err := client.GetSeriesPage(t.Context(), SeriesFilterParams{
		Limit:  5,
		Closed: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("GetSeriesPage: %v", err)
	}
	if len(series) != 1 {
		t.Errorf("got %d series", len(series))
	}
}

func TestClient_GetSeriesPageClampsOfficialLimit(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("limit"); got != "50" {
			t.Errorf("limit = %q, want 50", got)
		}
		writeJSON(w, []Series{{ID: "1"}})
	})

	_, err := client.GetSeriesPage(t.Context(), SeriesFilterParams{Limit: 100})
	if err != nil {
		t.Fatalf("GetSeriesPage: %v", err)
	}
}

func TestClient_IterSeries(t *testing.T) {
	call := 0
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		call++
		if call == 1 {
			items := make([]Series, 3)
			for i := range items {
				items[i] = Series{ID: string(rune('a' + i))}
			}
			writeJSON(w, items)
		} else {
			writeJSON(w, []Series{{ID: "d"}})
		}
	})

	var ids []string
	for s, err := range client.IterSeries(t.Context(), SeriesFilterParams{Limit: 3}) {
		if err != nil {
			t.Fatalf("IterSeries: %v", err)
		}
		ids = append(ids, s.ID)
	}
	if len(ids) != 4 {
		t.Errorf("ids = %v, want 4", ids)
	}
}

func TestClient_IterSeriesClampsOfficialLimit(t *testing.T) {
	call := 0
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		call++
		if got := r.URL.Query().Get("limit"); got != "50" {
			t.Errorf("limit = %q, want 50", got)
		}
		if call == 1 {
			items := make([]Series, 50)
			for i := range items {
				items[i] = Series{ID: string(rune('a' + i))}
			}
			writeJSON(w, items)
			return
		}
		if got := r.URL.Query().Get("offset"); got != "50" {
			t.Errorf("offset = %q, want 50", got)
		}
		writeJSON(w, []Series{{ID: "tail"}})
	})

	var ids []string
	for s, err := range client.IterSeries(t.Context(), SeriesFilterParams{Limit: 100}) {
		if err != nil {
			t.Fatalf("IterSeries: %v", err)
		}
		ids = append(ids, s.ID)
	}
	if len(ids) != 51 || ids[len(ids)-1] != "tail" {
		t.Fatalf("ids = %v, want 50-page plus tail", ids)
	}
}

func TestClient_ListSeries(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []Series{{ID: "1"}, {ID: "2"}})
	})

	all, err := client.ListSeries(t.Context(), SeriesFilterParams{})
	if err != nil {
		t.Fatalf("ListSeries: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("got %d series", len(all))
	}
}

// --- Tags ---

func TestClient_GetTag(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tags/tag-1" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, Tag{ID: "tag-1", Label: "Politics"})
	})

	tag, err := client.GetTag(t.Context(), "tag-1")
	if err != nil {
		t.Fatalf("GetTag: %v", err)
	}
	if tag.Label != "Politics" {
		t.Errorf("label = %s", tag.Label)
	}
}

func TestClient_GetTagBySlug(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tags/slug/politics" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, Tag{ID: "tag-1", Slug: "politics"})
	})

	tag, err := client.GetTagBySlug(t.Context(), "politics")
	if err != nil {
		t.Fatalf("GetTagBySlug: %v", err)
	}
	if tag.Slug != "politics" {
		t.Errorf("slug = %s", tag.Slug)
	}
}

func TestClient_GetRelatedTags(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tags/tag-1/related-tags" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, []RelatedTag{{ID: "rt-1", TagID: "tag-1", Rank: 1}})
	})

	tags, err := client.GetRelatedTags(t.Context(), "tag-1")
	if err != nil {
		t.Fatalf("GetRelatedTags: %v", err)
	}
	if len(tags) != 1 || tags[0].Rank != 1 {
		t.Errorf("tags = %+v", tags)
	}
}

func TestClient_GetRelatedTagsBySlug(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tags/slug/politics/related-tags" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, []RelatedTag{{ID: "rt-1"}})
	})

	tags, err := client.GetRelatedTagsBySlug(t.Context(), "politics")
	if err != nil {
		t.Fatalf("GetRelatedTagsBySlug: %v", err)
	}
	if len(tags) != 1 {
		t.Errorf("got %d tags", len(tags))
	}
}

func TestClient_GetTagsRelatedToTag(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tags/tag-1/related-tags/tags" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, []Tag{{ID: "t-1", Label: "Elections"}})
	})

	tags, err := client.GetTagsRelatedToTag(t.Context(), "tag-1")
	if err != nil {
		t.Fatalf("GetTagsRelatedToTag: %v", err)
	}
	if len(tags) != 1 || tags[0].Label != "Elections" {
		t.Errorf("tags = %+v", tags)
	}
}

func TestClient_GetTagsRelatedToTagBySlug(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tags/slug/politics/related-tags/tags" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, []Tag{{ID: "t-1"}})
	})

	tags, err := client.GetTagsRelatedToTagBySlug(t.Context(), "politics")
	if err != nil {
		t.Fatalf("GetTagsRelatedToTagBySlug: %v", err)
	}
	if len(tags) != 1 {
		t.Errorf("got %d tags", len(tags))
	}
}

func TestClient_GetRelatedTagResources(t *testing.T) {
	omitEmpty := true
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tags/tag-1/related-tags/tags" {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("locale") != "en" || q.Get("status") != "active" || q.Get("omit_empty") != "true" {
			t.Errorf("query = %v", q)
		}
		writeJSON(w, []Tag{{ID: "market-1"}})
	})

	tags, err := client.GetRelatedTagResources(t.Context(), "tag-1", RelatedTagResourceParams{
		Locale: "en", OmitEmpty: &omitEmpty, Status: "active",
	})
	if err != nil {
		t.Fatalf("GetRelatedTagResources: %v", err)
	}
	if len(tags) != 1 || tags[0].ID != "market-1" {
		t.Errorf("tags = %+v", tags)
	}
}

func TestClient_IterMarketClarifications(t *testing.T) {
	call := 0
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/market-clarifications" {
			t.Errorf("path = %s", r.URL.Path)
		}
		call++
		if got := r.URL.Query().Get("limit"); got != "2" {
			t.Errorf("limit = %q, want 2", got)
		}
		switch call {
		case 1:
			writeJSON(w, []MarketClarification{
				{ID: 1, MarketID: FlexibleID("100")},
				{ID: 2, MarketID: FlexibleID("101")},
			})
		case 2:
			writeJSON(w, []MarketClarification{
				{ID: 3, MarketID: FlexibleID("102")},
				{ID: 4, MarketID: FlexibleID("103")},
			})
		default:
			writeJSON(w, []MarketClarification{})
		}
	})

	var ids []int
	for clarification, err := range client.IterMarketClarifications(t.Context(), MarketClarificationsParams{Limit: 2}) {
		if err != nil {
			t.Fatalf("IterMarketClarifications: %v", err)
		}
		ids = append(ids, clarification.ID)
	}
	if len(ids) != 4 || ids[0] != 1 || ids[3] != 4 {
		t.Fatalf("ids = %v", ids)
	}

	var clarification MarketClarification
	if err := json.Unmarshal([]byte(`{"id":5,"marketId":123}`), &clarification); err != nil {
		t.Fatalf("numeric market id: %v", err)
	}
	if clarification.MarketID != FlexibleID("123") {
		t.Fatalf("market id = %q", clarification.MarketID)
	}
}

func TestClient_GetTags(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tags" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, []Tag{{ID: "t-1"}, {ID: "t-2"}})
	})

	tags, err := client.GetTags(t.Context())
	if err != nil {
		t.Fatalf("GetTags: %v", err)
	}
	if len(tags) != 2 {
		t.Errorf("got %d tags", len(tags))
	}
}

func TestClient_GetTagsPageClampsAndFilters(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("limit"); got != "100" {
			t.Errorf("limit = %q, want 100", got)
		}
		if got := r.URL.Query().Get("offset"); got != "7" {
			t.Errorf("offset = %q, want 7", got)
		}
		checks := map[string]string{
			"ascending":        "true",
			"include_template": "true",
			"is_carousel":      "false",
			"locale":           "en",
			"order":            "label",
		}
		for key, want := range checks {
			if got := r.URL.Query().Get(key); got != want {
				t.Errorf("%s = %q, want %q", key, got, want)
			}
		}
		writeJSON(w, []Tag{{ID: "tag-1"}})
	})

	tags, err := client.GetTagsPage(t.Context(), TagFilterParams{
		Ascending:       boolPtr(true),
		IncludeTemplate: boolPtr(true),
		IsCarousel:      boolPtr(false),
		Locale:          "en",
		Order:           "label",
		Limit:           150,
		Offset:          7,
	})
	if err != nil {
		t.Fatalf("GetTagsPage: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("got %d tags, want 1", len(tags))
	}
}

func TestClient_IterTagsClampsOfficialLimit(t *testing.T) {
	call := 0
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		call++
		if got := r.URL.Query().Get("limit"); got != "100" {
			t.Errorf("call %d limit = %q, want 100", call, got)
		}
		switch call {
		case 1:
			tags := make([]Tag, 100)
			for i := range tags {
				tags[i] = Tag{ID: fmt.Sprintf("tag-%d", i)}
			}
			writeJSON(w, tags)
		case 2:
			if got := r.URL.Query().Get("offset"); got != "100" {
				t.Errorf("second offset = %q, want 100", got)
			}
			writeJSON(w, []Tag{{ID: "tag-100"}})
		default:
			t.Errorf("iterator made unexpected call %d", call)
		}
	})

	var ids []string
	for tag, err := range client.IterTags(t.Context(), TagFilterParams{Limit: 150}) {
		if err != nil {
			t.Fatalf("IterTags: %v", err)
		}
		ids = append(ids, tag.ID)
	}
	if len(ids) != 101 || ids[100] != "tag-100" {
		t.Fatalf("collected %d tags, last %q", len(ids), ids[len(ids)-1])
	}
}

func TestClient_GetEventTags(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/events/evt-1/tags" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, []Tag{{ID: "t-1"}})
	})

	tags, err := client.GetEventTags(t.Context(), "evt-1")
	if err != nil {
		t.Fatalf("GetEventTags: %v", err)
	}
	if len(tags) != 1 {
		t.Errorf("got %d tags", len(tags))
	}
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

// --- Sports & Teams ---

func TestClient_GetSports(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sports" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, []SportsMetadata{{
			ID: 1, Sport: "NBA", Image: "nba.png",
			Resolution: "manual", Ordering: "1",
			Tags: []string{"sports", "basketball"}, Series: "nba",
		}})
	})

	sports, err := client.GetSports(t.Context())
	if err != nil {
		t.Fatalf("GetSports: %v", err)
	}
	if len(sports) != 1 || sports[0].Sport != "NBA" || len(sports[0].Tags) != 2 {
		t.Errorf("sports = %+v", sports)
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

func TestClient_GetTeams(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/teams" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, []Team{{ID: 1, Name: "Lakers", League: "NBA"}})
	})

	teams, err := client.GetTeams(t.Context())
	if err != nil {
		t.Fatalf("GetTeams: %v", err)
	}
	if len(teams) != 1 || teams[0].Name != "Lakers" {
		t.Errorf("teams = %+v", teams)
	}
}

func TestClient_GetTeamsPage(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/teams" {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("league") != "NBA" {
			t.Errorf("league = %q", q.Get("league"))
		}
		if q.Get("limit") != "5" {
			t.Errorf("limit = %q", q.Get("limit"))
		}
		writeJSON(w, []Team{{ID: 1, Name: "Lakers"}})
	})

	teams, err := client.GetTeamsPage(t.Context(), TeamFilterParams{
		League: "NBA",
		Limit:  5,
	})
	if err != nil {
		t.Fatalf("GetTeamsPage: %v", err)
	}
	if len(teams) != 1 {
		t.Errorf("got %d teams", len(teams))
	}
}

func TestClient_IterTeams(t *testing.T) {
	call := 0
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		call++
		if call == 1 {
			items := make([]Team, 2)
			for i := range items {
				items[i] = Team{ID: i + 1}
			}
			writeJSON(w, items)
		} else {
			writeJSON(w, []Team{{ID: 3}})
		}
	})

	var ids []int
	for tm, err := range client.IterTeams(t.Context(), TeamFilterParams{Limit: 2}) {
		if err != nil {
			t.Fatalf("IterTeams: %v", err)
		}
		ids = append(ids, tm.ID)
	}
	if len(ids) != 3 {
		t.Errorf("ids = %v, want 3", ids)
	}
}

func TestClient_ListTeams(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []Team{{ID: 1}, {ID: 2}})
	})

	all, err := client.ListTeams(t.Context(), TeamFilterParams{})
	if err != nil {
		t.Fatalf("ListTeams: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("got %d teams", len(all))
	}
}

// --- Comments ---

func TestClient_GetComments(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/comments" {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("parent_entity_type") != "market" {
			t.Errorf("parent_entity_type = %q", q.Get("parent_entity_type"))
		}
		if q.Get("parent_entity_id") != "cid-1" {
			t.Errorf("parent_entity_id = %q", q.Get("parent_entity_id"))
		}
		writeJSON(w, []Comment{{ID: "c-1", Body: "hello"}})
	})

	comments, err := client.GetComments(t.Context(), CommentFilterParams{ConditionID: "cid-1"})
	if err != nil {
		t.Fatalf("GetComments: %v", err)
	}
	if len(comments) != 1 || comments[0].Body != "hello" {
		t.Errorf("comments = %+v", comments)
	}
}

func TestClient_GetComment(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/comments/c-1" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, []Comment{{ID: "c-1", Body: "test"}})
	})

	comments, err := client.GetComment(t.Context(), "c-1")
	if err != nil {
		t.Fatalf("GetComment: %v", err)
	}
	if len(comments) != 1 {
		t.Errorf("got %d comments", len(comments))
	}
}

func TestClient_GetCommentsByUserAddress(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/comments/user_address/0xabc" {
			t.Errorf("path = %s", r.URL.Path)
		}
		writeJSON(w, []Comment{{ID: "c-1", UserAddress: "0xabc"}})
	})

	comments, err := client.GetCommentsByUserAddress(t.Context(), "0xabc")
	if err != nil {
		t.Fatalf("GetCommentsByUserAddress: %v", err)
	}
	if len(comments) != 1 || comments[0].UserAddress != "0xabc" {
		t.Errorf("comments = %+v", comments)
	}
}

func TestClient_GetCommentsByUserAddressPageFilters(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("limit") != "10" || q.Get("offset") != "4" {
			t.Errorf("pagination = limit %q offset %q", q.Get("limit"), q.Get("offset"))
		}
		if q.Get("ascending") != "false" || q.Get("order") != "created_at" {
			t.Errorf("filters = ascending %q order %q", q.Get("ascending"), q.Get("order"))
		}
		writeJSON(w, []Comment{{ID: "c-1", UserAddress: "0xabc"}})
	})

	comments, err := client.GetCommentsByUserAddressPage(
		t.Context(),
		"0xabc",
		CommentsByUserAddressParams{
			Ascending: boolPtr(false),
			Order:     "created_at",
			Limit:     10,
			Offset:    4,
		},
	)
	if err != nil {
		t.Fatalf("GetCommentsByUserAddressPage: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("got %d comments, want 1", len(comments))
	}
}

func TestClient_IterCommentsByUserAddressClampsOfficialLimit(t *testing.T) {
	call := 0
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		call++
		if got := r.URL.Query().Get("limit"); got != "100" {
			t.Errorf("call %d limit = %q, want 100", call, got)
		}
		switch call {
		case 1:
			comments := make([]Comment, 100)
			for i := range comments {
				comments[i] = Comment{ID: fmt.Sprintf("comment-%d", i)}
			}
			writeJSON(w, comments)
		case 2:
			if got := r.URL.Query().Get("offset"); got != "100" {
				t.Errorf("second offset = %q, want 100", got)
			}
			writeJSON(w, []Comment{{ID: "comment-100"}})
		default:
			t.Errorf("iterator made unexpected call %d", call)
		}
	})

	var ids []string
	for comment, err := range client.IterCommentsByUserAddress(
		t.Context(),
		"0xabc",
		CommentsByUserAddressParams{Limit: 150},
	) {
		if err != nil {
			t.Fatalf("IterCommentsByUserAddress: %v", err)
		}
		ids = append(ids, comment.ID)
	}
	if len(ids) != 101 || ids[100] != "comment-100" {
		t.Fatalf("collected %d comments, last %q", len(ids), ids[len(ids)-1])
	}
}

func TestClient_IterComments(t *testing.T) {
	call := 0
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		call++
		if call == 1 {
			items := make([]Comment, 2)
			for i := range items {
				items[i] = Comment{ID: string(rune('a' + i))}
			}
			writeJSON(w, items)
		} else {
			writeJSON(w, []Comment{{ID: "c"}})
		}
	})

	var ids []string
	for c, err := range client.IterComments(t.Context(), CommentFilterParams{Limit: 2}) {
		if err != nil {
			t.Fatalf("IterComments: %v", err)
		}
		ids = append(ids, c.ID)
	}
	if len(ids) != 3 {
		t.Errorf("ids = %v", ids)
	}
}

// --- Profiles ---

func TestClient_GetPublicProfile(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/public-profile" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("address") != "0xabc" {
			t.Errorf("address = %q", r.URL.Query().Get("address"))
		}
		writeJSON(w, PublicProfile{Address: "0xabc", Name: "Test User"})
	})

	profile, err := client.GetPublicProfile(t.Context(), "0xabc")
	if err != nil {
		t.Fatalf("GetPublicProfile: %v", err)
	}
	if profile.Name != "Test User" {
		t.Errorf("name = %s", profile.Name)
	}
}

// --- Error handling ---

func TestClient_ErrorResponse(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	})

	_, err := client.GetMarket(t.Context(), "missing")
	if err == nil {
		t.Fatal("expected 404 error")
	}
}

func TestClient_ServerError(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	})

	_, err := client.GetEvents(t.Context(), EventFilterParams{})
	if err == nil {
		t.Fatal("expected 500 error")
	}
}

func TestClient_EmptyArrayResponse(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	})

	markets, err := client.GetMarkets(t.Context(), MarketFilterParams{})
	if err != nil {
		t.Fatalf("GetMarkets: %v", err)
	}
	if markets == nil || len(markets) != 0 {
		t.Errorf("expected empty slice, got %v", markets)
	}
}

func TestGammaNewFields(t *testing.T) {
	t.Run("Event.ParentEventID", func(t *testing.T) {
		_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, Event{
				ID:            "event-1",
				ParentEventID: "parent-1",
				Title:         "Sub Event",
			})
		})

		e, err := client.GetEvent(t.Context(), "event-1")
		if err != nil {
			t.Fatalf("GetEvent: %v", err)
		}
		if e.ParentEventID != "parent-1" {
			t.Errorf("ParentEventID = %q, want parent-1", e.ParentEventID)
		}
	})

	t.Run("Market.PositionIDs", func(t *testing.T) {
		_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, Market{
				ID:          "market-1",
				PositionIDs: []string{"pos-1", "pos-2"},
			})
		})

		m, err := client.GetMarket(t.Context(), "market-1")
		if err != nil {
			t.Fatalf("GetMarket: %v", err)
		}
		if len(m.PositionIDs) != 2 || m.PositionIDs[0] != "pos-1" {
			t.Errorf("PositionIDs = %v, want [pos-1 pos-2]", m.PositionIDs)
		}
	})
}
