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
		Category:   data.CategoryOverall,
		TimePeriod: data.PeriodWeek,
		SortBy:     data.SortPNL,
		Limit:      5,
	})
	if err != nil {
		log.Fatalf("Leaderboard failed: %v", err)
	}

	for _, e := range entries {
		fmt.Printf(" #%d %s: $%.2f PNL (Vol: $%.2f)\n", e.Rank, e.Username, e.PNL, e.Volume)
	}

	// 2. Fetch user positions (example for a known address or placeholder)
	address := "0x1234567890123456789012345678901234567890"
	fmt.Printf("\nFetching positions for %s...\n", address)
	positions, err := client.GetPositions(ctx, address)
	if err != nil {
		fmt.Printf("Positions lookup failed: %v (expected if address is invalid)\n", err)
	} else {
		fmt.Printf(" Found %d open positions\n", len(positions))
		for _, p := range positions {
			fmt.Printf(" - %s: %.2f tokens @ avg $%.4f\n", p.MarketTitle, p.Size, p.AvgPrice)
		}
	}
}
