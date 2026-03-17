package main

import (
	"context"
	"fmt"
	"log"

	"github.com/nijaru/go-clob-client/data"
)

func main() {
	client := data.New(data.Config{})
	ctx := context.Background()

	// 1. Fetch leaderboard
	fmt.Println("Fetching Overall Weekly PNL Leaderboard...")
	entries, err := client.GetLeaderboard(ctx, data.LeaderboardParams{
		Category:   "OVERALL",
		TimePeriod: "WEEK",
		SortBy:     "PNL",
		Limit:      5,
	})
	if err != nil {
		log.Fatalf("Leaderboard failed: %v", err)
	}

	for _, e := range entries {
		fmt.Printf(" #%d %s: $%s PNL (Vol: $%s)\n", e.Rank, e.Username, e.PNL, e.Volume)
	}

	// 2. Fetch user positions (example for a known address or placeholder)
	address := "0x1234567890123456789012345678901234567890"
	fmt.Printf("\nFetching positions for %s...\n", address)
	positions, err := client.GetPositions(ctx, data.PositionParams{
		User: address,
	})
	if err != nil {
		fmt.Printf("Positions lookup failed: %v (expected if address is invalid)\n", err)
	} else {
		fmt.Printf(" Found %d open positions\n", len(positions))
		for _, p := range positions {
			fmt.Printf(" - %s: %s tokens @ avg $%s\n", p.Title, p.Size, p.AvgPrice)
		}
	}
}
