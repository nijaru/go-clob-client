package gamma

import (
	"errors"
	"net/http"
	"net/url"
	"slices"
	"testing"
)

func assertQuery(t *testing.T, r *http.Request, want url.Values) {
	t.Helper()
	got := r.URL.Query()
	for key, values := range want {
		if !slices.Equal(got[key], values) {
			t.Errorf("%s=%v want %v", key, got[key], values)
		}
	}
}

func TestStableDiscoveryFilters(t *testing.T) {
	t.Run("markets", func(t *testing.T) {
		_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			assertQuery(
				t,
				r,
				url.Values{
					"decimalized":    {"false"},
					"locale":         {"en"},
					"position_ids":   {"p1", "p2"},
					"rfq_enabled":    {"true"},
					"tag_match":      {"all"},
					"order":          {"liquidityNum"},
					"limit":          {"150"},
					"slug":           {"m1", "m2"},
					"clob_token_ids": {"t1", "t2"},
				},
			)
			w.Write([]byte(`{"markets":[]}`))
		})
		_, err := client.GetMarketsPage(
			t.Context(),
			MarketFilterParams{
				Decimalized:  ptr(false),
				Locale:       "en",
				PositionIDs:  []string{"p1", "p2"},
				RfqEnabled:   ptr(true),
				TagMatch:     "all",
				Order:        "liquidity",
				Limit:        150,
				Slugs:        []string{"m1", "m2"},
				ClobTokenIDs: []string{"t1", "t2"},
			},
			"",
		)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("events", func(t *testing.T) {
		_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			assertQuery(
				t,
				r,
				url.Values{
					"ascending":          {"false"},
					"tag_id":             {"1", "2"},
					"series_id":          {"3", "4"},
					"game_id":            {"5", "6"},
					"tag_match":          {"any"},
					"include_best_lines": {"true"},
					"include_children":   {"false"},
					"ended":              {"false"},
					"live":               {"true"},
					"featured_order":     {"false"},
					"event_date":         {"2026-05-01"},
					"event_week":         {"0"},
					"start_time_min":     {"2026-05-01T00:00:00Z"},
					"start_time_max":     {"2026-05-02T00:00:00Z"},
					"parent_event_id":    {"7"},
					"partner_slug":       {"partner"},
					"title_search":       {"rain"},
					"locale":             {"fr"},
					"limit":              {"150"},
				},
			)
			w.Write([]byte(`{"events":[]}`))
		})
		_, err := client.GetEventsPage(
			t.Context(),
			EventFilterParams{
				Ascending:        ptr(false),
				TagIDs:           []string{"1", "2"},
				SeriesIDs:        []string{"3", "4"},
				GameIDs:          []string{"5", "6"},
				TagMatch:         "any",
				IncludeBestLines: ptr(true),
				IncludeChildren:  ptr(false),
				Ended:            ptr(false),
				Live:             ptr(true),
				FeaturedOrder:    ptr(false),
				EventDate:        "2026-05-01",
				EventWeek:        ptr(0),
				StartTimeMin:     "2026-05-01T00:00:00Z",
				StartTimeMax:     "2026-05-02T00:00:00Z",
				ParentEventID:    "7",
				PartnerSlug:      "partner",
				TitleSearch:      "rain",
				Locale:           "fr",
				Limit:            150,
			},
			"",
		)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("series and teams", func(t *testing.T) {
		_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/series":
				assertQuery(
					t,
					r,
					url.Values{
						"exclude_events": {"false"},
						"include_chat":   {"true"},
						"locale":         {"en"},
						"slug":           {"s1", "s2"},
						"categories_ids": {"1", "2"},
					},
				)
			case "/teams":
				assertQuery(
					t,
					r,
					url.Values{
						"provider_id": {"0", "42"},
						"name":        {"A", "B"},
						"league":      {"NBA", "NHL"},
					},
				)
			default:
				t.Errorf("path=%s", r.URL.Path)
			}
			if r.URL.Query().Has("excludeEvents") || r.URL.Query().Has("providerId") {
				t.Errorf("unexpected camel-case params: %v", r.URL.Query())
			}
			w.Write([]byte(`[]`))
		})
		if _, err := client.GetSeriesPage(t.Context(), SeriesFilterParams{ExcludeEvents: ptr(false), IncludeChat: ptr(true), Locale: "en", Slugs: []string{"s1", "s2"}, CategoriesIDs: []string{"1", "2"}}); err != nil {
			t.Fatal(err)
		}
		if _, err := client.GetTeamsPage(t.Context(), TeamFilterParams{ProviderIDs: []int{0, 42}, Names: []string{"A", "B"}, Leagues: []string{"NBA", "NHL"}}); err != nil {
			t.Fatal(err)
		}
	})
}

// Python ed8d04ca normalizes every comma-separated market sort token,
// including whitespace, without changing event sorts or unknown columns.
func TestMarketSortTokens(t *testing.T) {
	for _, order := range []string{"volume, liquidity, id", "volumeNum,liquidityNum,id"} {
		for _, keyset := range []bool{false, true} {
			t.Run(
				order+"/"+map[bool]string{false: "offset", true: "keyset"}[keyset],
				func(t *testing.T) {
					_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
						assertQuery(t, r, url.Values{"order": {"volumeNum,liquidityNum,id"}})
						if keyset {
							w.Write([]byte(`{"markets":[]}`))
						} else {
							w.Write([]byte(`[]`))
						}
					})
					p := MarketFilterParams{Order: order}
					var err error
					if keyset {
						_, err = client.GetMarketsPage(t.Context(), p, "")
					} else {
						_, err = client.GetMarkets(t.Context(), p)
					}
					if err != nil {
						t.Fatal(err)
					}
				},
			)
		}
	}
}

func TestDiscoveryLookupOptions(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/markets/slug/rain":
			assertQuery(t, r, url.Values{"include_tag": {"false"}, "locale": {"en"}})
		case "/events/slug/rain":
			assertQuery(
				t,
				r,
				url.Values{
					"include_best_lines": {"true"},
					"include_chat":       {"false"},
					"include_template":   {"true"},
					"locale":             {"fr"},
				},
			)
		case "/series/123", "/tags/123":
			assertQuery(t, r, url.Values{"locale": {"en"}})
		case "/comments/123":
			assertQuery(t, r, url.Values{"get_positions": {"false"}})
			w.Write([]byte(`[]`))
			return
		case "/tags/slug/weather/related-tags":
			assertQuery(t, r, url.Values{"status": {"all"}, "omit_empty": {"false"}})
			w.Write([]byte(`[{"id":1,"tagID":2,"relatedTagID":"3"}]`))
			return
		default:
			t.Errorf("path=%s", r.URL.Path)
		}
		w.Write([]byte(`{"id":"123"}`))
	})
	if _, err := client.GetMarketByURL(t.Context(), "https://polymarket.com/event/weather/rain", MarketOptions{IncludeTag: ptr(false), Locale: "en"}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetEventByURL(t.Context(), "https://www.polymarket.com/event/rain?x=1", EventOptions{IncludeBestLines: ptr(true), IncludeChat: ptr(false), IncludeTemplate: ptr(true), Locale: "fr"}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetSeries(t.Context(), "123", SeriesOptions{Locale: "en"}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetTag(t.Context(), "123", TagOptions{Locale: "en"}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetComment(t.Context(), "123", CommentOptions{GetPositions: ptr(false)}); err != nil {
		t.Fatal(err)
	}
	tags, err := client.GetRelatedTagsBySlug(
		t.Context(),
		"weather",
		RelatedTagsOptions{Status: "all", OmitEmpty: ptr(false)},
	)
	if err != nil || len(tags) != 1 || tags[0].ID != "1" || tags[0].TagID != "2" ||
		tags[0].RelatedTagID != "3" {
		t.Fatalf("tags=%+v err=%v", tags, err)
	}
}

func TestPolymarketURLValidation(t *testing.T) {
	_, client := newTestServer(
		t,
		func(w http.ResponseWriter, r *http.Request) { t.Error("request for invalid URL") },
	)
	for _, raw := range []string{"http://polymarket.com/event/rain", "https://evil.test/event/rain", "https://polymarket.com/event/rain/child/extra", "https://polymarket.com/event/", "not a url"} {
		if _, err := client.GetMarketByURL(t.Context(), raw); err == nil {
			t.Fatalf("accepted market URL %q", raw)
		}
	}
	if _, err := client.GetEventByURL(t.Context(), "https://polymarket.com/event/rain/market"); err == nil {
		t.Fatal("event accepted nested market URL")
	}
}

func TestAPIErrorDetails(t *testing.T) {
	for _, status := range []int{404, 429, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				w.Write([]byte(`{"error":"upstream details"}`))
			})
			_, err := client.GetMarket(t.Context(), "1")
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != status ||
				string(apiErr.Body) != `{"error":"upstream details"}` {
				t.Fatalf("API details: %+v %v", apiErr, err)
			}
		})
	}
}
