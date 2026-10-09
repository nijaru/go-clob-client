package gamma

import (
	"net/http"
	"testing"
)

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
