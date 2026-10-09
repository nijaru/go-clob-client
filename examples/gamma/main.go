package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nijaru/go-clob-client/gamma"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	client := gamma.New(gamma.Config{})

	search, err := client.GetSearchPage(ctx, gamma.SearchParams{Query: "Bitcoin", LimitPerType: 3})
	if err != nil {
		return err
	}
	for _, event := range search.Results.Events {
		fmt.Printf("Search: %s (%s)\n", event.Title, event.ID)
	}
	fmt.Printf("Search has more: %t; boundary reached: %t\n", search.HasMore, search.LimitReached)

	// Keyset event discovery defaults to open events. Keep the exact filters when
	// following NextCursor; the SDK rejects continuations for another query.
	// Limit can change between keyset pages without invalidating the cursor.
	filters := gamma.EventFilterParams{Limit: 3, Order: "volume", IncludeBestLines: boolptr(true)}
	page, err := client.GetEventsPage(ctx, filters, "")
	if err != nil {
		return err
	}
	for _, event := range page.Items {
		fmt.Printf("Event: %s (%d markets)\n", event.Title, len(event.Markets))
	}
	if page.HasMore {
		next, err := client.GetEventsPage(ctx, filters, page.NextCursor)
		if err != nil {
			return err
		}
		fmt.Printf("Next event page: %d events\n", len(next.Items))
	}

	// Asset IDs and prices remain in their wire order. Decimal values retain
	// their exact text, and Rat is available when exact arithmetic is needed.
	for market, err := range client.IterMarketsKeyset(ctx, gamma.MarketFilterParams{Order: "volume", Limit: 3}, "") {
		if err != nil {
			return err
		}
		fmt.Printf(
			"Market: %s; volume: %s; tokens: %v; V2 positions: %v\n",
			market.Question,
			market.Volume,
			market.CLOBTokenIDs,
			market.PositionIDs,
		)
		break
	}

	if len(page.Items) > 0 {
		comments, err := client.GetCommentsPage(ctx, gamma.CommentFilterParams{
			ParentEntityType: gamma.ParentEntityTypeEvent,
			ParentEntityID:   string(page.Items[0].ID),
			GetPositions:     boolptr(true), // Position-enriched reads use capped offset pages.
			Limit:            3,
		}, "")
		if err != nil {
			return err
		}
		fmt.Printf(
			"Comment rows (including replies): %d; more: %t; boundary reached: %t\n",
			len(comments.Items),
			comments.HasMore,
			comments.LimitReached,
		)
		// A full offset page at the cap does not prove completeness. Item
		// iterators/ListComments return ErrPaginationLimit with accessible rows;
		// page iterators instead stop with LimitReached on the final page.
	}

	sports, err := client.GetSports(ctx)
	if err != nil {
		return err
	}
	if len(sports) > 0 {
		fmt.Printf(
			"Sport: %s; optional name: %q; tags: %v\n",
			sports[0].Sport,
			sports[0].Name,
			sports[0].Tags,
		)
	}
	return nil
}

func boolptr(value bool) *bool { return &value }
