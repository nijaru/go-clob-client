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

func TestClient_GetSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Errorf("expected path /search, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("query") != "rain" {
			t.Errorf("expected query rain, got %s", r.URL.Query().Get("query"))
		}
		markets := []Market{
			{ID: "123", Question: "Will it rain?"},
		}
		data, _ := json.Marshal(markets)
		w.Write(data)
	}))
	defer server.Close()

	client := New(Config{Host: server.URL})
	markets, err := client.GetSearch(t.Context(), "rain")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(markets) != 1 || markets[0].ID != "123" {
		t.Errorf("unexpected search results: %+v", markets)
	}
}
