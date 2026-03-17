package data

import (
	"context"
	jsonv1 "encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDataClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case positionsEndpoint:
			jsonv1.NewEncoder(w).Encode([]Position{
				{Asset: "0x123", Title: "Test Market", Size: "100"},
			})
		case leaderboardEndpoint:
			jsonv1.NewEncoder(w).Encode([]TraderLeaderboardEntry{
				{Rank: 1, Username: "trader1", Volume: "1000", PNL: "500"},
			})
		}
	}))
	defer server.Close()

	client := New(Config{Host: server.URL})
	ctx := context.Background()

	t.Run("GetPositions", func(t *testing.T) {
		positions, err := client.GetPositions(ctx, PositionParams{User: "0x123"})
		if err != nil {
			t.Fatalf("GetPositions failed: %v", err)
		}
		if len(positions) != 1 || positions[0].Asset != "0x123" {
			t.Errorf("unexpected positions: %+v", positions)
		}
	})

	t.Run("GetLeaderboard", func(t *testing.T) {
		entries, err := client.GetLeaderboard(ctx, LeaderboardParams{Category: "POLITICS"})
		if err != nil {
			t.Fatalf("GetLeaderboard failed: %v", err)
		}
		if len(entries) != 1 || entries[0].Username != "trader1" {
			t.Errorf("unexpected entries: %+v", entries)
		}
	})
}
