package gamma

import (
	"net/http"
	"testing"

	json "github.com/go-json-experiment/json"
)

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
