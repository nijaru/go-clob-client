package main

import (
	"context"
	"fmt"
	"log"

	"github.com/nijaru/go-clob-client/gamma"
)

func main() {
	client := gamma.New(gamma.Config{})
	ctx := context.Background()

	// 1. Search for a market
	fmt.Println("Searching for 'Bitcoin' markets...")
	markets, err := client.Search(ctx, "Bitcoin")
	if err != nil {
		log.Fatalf("Search failed: %v", err)
	}

	for i, m := range markets {
		if i >= 3 {
			break
		}
		fmt.Printf(" - %s (ConditionID: %s)\n", m.Question, m.ConditionID)
	}

	// 2. List active events
	fmt.Println("\nFetching active events...")
	active := true
	events, err := client.GetEvents(ctx, gamma.EventFilterParams{
		Active: &active,
		Limit:  5,
	})
	if err != nil {
		log.Fatalf("GetEvents failed: %v", err)
	}

	for _, e := range events {
		fmt.Printf(" - %s (%d markets)\n", e.Title, len(e.Markets))
	}

	// 3. Get a specific tag
	fmt.Println("\nFetching 'Crypto' tag...")
	tag, err := client.GetTagBySlug(ctx, "crypto")
	if err != nil {
		fmt.Printf("Tag lookup failed: %v\n", err)
	} else {
		fmt.Printf(" - Found Tag: %s (ID: %s)\n", tag.Label, tag.ID)
	}
}
