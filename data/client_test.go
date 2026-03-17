package data

import (
	"net/http"
	"net/http/httptest"
	"testing"

	json "github.com/go-json-experiment/json"
)

func TestClient_GetPositions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/positions" {
			t.Errorf("expected path /positions, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("user") != "0x123" {
			t.Errorf("expected user 0x123, got %s", r.URL.Query().Get("user"))
		}
		positions := []Position{
			{AssetAddress: "0xabc", MarketTitle: "Will it rain?", Size: 100},
		}
		data, _ := json.Marshal(positions)
		w.Write(data)
	}))
	defer server.Close()

	client := New(Config{Host: server.URL})
	positions, err := client.GetPositions(t.Context(), "0x123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(positions) != 1 || positions[0].AssetAddress != "0xabc" {
		t.Errorf("unexpected positions data: %+v", positions)
	}
}

func TestClient_GetLeaderboard(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/leaderboard" {
			t.Errorf("expected path /v1/leaderboard, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("category") != "POLITICS" {
			t.Errorf("expected category POLITICS, got %s", r.URL.Query().Get("category"))
		}
		entries := []LeaderboardEntry{
			{ProxyWallet: "0x123", Username: "trader1", Rank: 1},
		}
		data, _ := json.Marshal(entries)
		w.Write(data)
	}))
	defer server.Close()

	client := New(Config{Host: server.URL})
	entries, err := client.GetLeaderboard(t.Context(), LeaderboardParams{
		Category: CategoryPolitics,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 1 || entries[0].Username != "trader1" {
		t.Errorf("unexpected leaderboard data: %+v", entries)
	}
}
