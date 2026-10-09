package gamma

import (
	"net/http"
	"testing"
)

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
				events[i] = Event{ID: FlexibleID(rune('a' + i))}
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
