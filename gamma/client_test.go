package gamma

import (
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
