package gamma

import (
	"net/http"
	"testing"
)

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
