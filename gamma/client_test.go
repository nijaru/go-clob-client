package gamma

import (
	"net/http"
	"net/http/httptest"
	"testing"

	json "github.com/go-json-experiment/json"
)

func TestClient_GetMarket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/markets/123" {
			t.Errorf("expected path /markets/123, got %s", r.URL.Path)
		}
		market := Market{
			ID:       "123",
			Question: "Will it rain?",
		}
		data, _ := json.Marshal(market)
		w.Write(data)
	}))
	defer server.Close()

	client := New(Config{Host: server.URL})
	market, err := client.GetMarket(t.Context(), "123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if market.ID != "123" || market.Question != "Will it rain?" {
		t.Errorf("unexpected market data: %+v", market)
	}
}

func TestClient_GetMarketBySlug(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/markets/slug/will-it-rain" {
			t.Errorf("expected path /markets/slug/will-it-rain, got %s", r.URL.Path)
		}
		if raw := r.URL.RawQuery; raw != "" {
			t.Errorf("expected no query string, got %q", raw)
		}

		market := Market{
			ID:       "123",
			Question: "Will it rain?",
			Slug:     "will-it-rain",
		}
		data, _ := json.Marshal(market)
		w.Write(data)
	}))
	defer server.Close()

	client := New(Config{Host: server.URL})
	market, err := client.GetMarketBySlug(t.Context(), "will-it-rain")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if market.ID != "123" || market.Slug != "will-it-rain" {
		t.Errorf("unexpected market data: %+v", market)
	}
}

func TestClient_Search(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/public-search" {
			t.Errorf("expected path /public-search, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("query") != "rain" {
			t.Errorf("expected query rain, got %s", r.URL.Query().Get("query"))
		}
		results := SearchResults{
			Events: []Event{
				{ID: "123", Title: "Will it rain?"},
			},
			Tags: []SearchTag{
				{ID: "tag-1", Label: "Weather", Slug: "weather", EventCount: 4},
			},
			Pagination: &Pagination{HasMore: true, TotalResults: 2},
		}
		data, _ := json.Marshal(results)
		w.Write(data)
	}))
	defer server.Close()

	client := New(Config{Host: server.URL})
	results, err := client.Search(t.Context(), "rain")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results.Events) != 1 || results.Events[0].ID != "123" {
		t.Errorf("unexpected search events: %+v", results.Events)
	}
	if len(results.Tags) != 1 || results.Tags[0].Slug != "weather" {
		t.Errorf("unexpected search tags: %+v", results.Tags)
	}
	if results.Pagination == nil || !results.Pagination.HasMore ||
		results.Pagination.TotalResults != 2 {
		t.Errorf("unexpected pagination: %+v", results.Pagination)
	}
}

func TestClient_GetSports(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sports" {
			t.Errorf("expected path /sports, got %s", r.URL.Path)
		}
		data, _ := json.Marshal([]SportsMetadata{
			{
				ID:         1,
				Sport:      "NBA",
				Image:      "nba.png",
				Resolution: "manual",
				Ordering:   "1",
				Tags:       []string{"sports", "basketball"},
				Series:     "nba",
			},
		})
		w.Write(data)
	}))
	defer server.Close()

	client := New(Config{Host: server.URL})
	sports, err := client.GetSports(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sports) != 1 || sports[0].Sport != "NBA" || len(sports[0].Tags) != 2 {
		t.Errorf("unexpected sports metadata: %+v", sports)
	}
}

func TestClient_GetMarketTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sports/market-types" {
			t.Errorf("expected path /sports/market-types, got %s", r.URL.Path)
		}
		data, _ := json.Marshal(SportsMarketTypesResponse{
			MarketTypes: []string{"moneyline", "spread"},
		})
		w.Write(data)
	}))
	defer server.Close()

	client := New(Config{Host: server.URL})
	resp, err := client.GetMarketTypes(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.MarketTypes) != 2 || resp.MarketTypes[0] != "moneyline" {
		t.Errorf("unexpected market types: %+v", resp)
	}
}
