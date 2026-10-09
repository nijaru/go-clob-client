package gamma

import (
	"net/http"
	"testing"
)

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
				items[i] = Series{ID: FlexibleID(rune('a' + i))}
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
		ids = append(ids, string(s.ID))
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
				items[i] = Series{ID: FlexibleID(rune('a' + i))}
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
		ids = append(ids, string(s.ID))
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
