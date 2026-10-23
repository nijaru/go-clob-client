package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nijaru/go-clob-client/data"
)

func main() {
	client, err := data.NewClient(data.Config{})
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user := os.Getenv("POLYMARKET_USER")
	if user == "" {
		user = "0x1234567890123456789012345678901234567890"
	}

	page, err := client.GetPositions(
		ctx,
		data.PositionsParams{User: user, Page: data.PageParams{Limit: 5}},
	)
	if err != nil {
		log.Fatalf("get positions: %v", err)
	}

	fmt.Printf("Fetched %d positions for %s (more: %t)\n", len(page.Items), user, page.HasMore)
	for _, pos := range page.Items {
		title := pos.AssetID
		if pos.Title != nil {
			title = *pos.Title
		}
		fmt.Printf("%s: %s shares @ avg %s\n", title, pos.CurrentSize, pos.AvgPrice)
	}
	if len(page.Items) == 0 {
		return
	}

	// Token references add the issuing module, on-chain outcome index and the
	// market's order-book index, which can differ from the outcome slot.
	references, err := client.GetTokenReferences(ctx, data.TokenReferenceSelectors{
		AssetIDs: []string{page.Items[0].AssetID},
	})
	if err != nil {
		log.Fatalf("get token references: %v", err)
	}
	for _, reference := range references {
		fmt.Printf(
			"%s: module=%s outcome=%d\n",
			reference.AssetID,
			reference.Module,
			reference.OutcomeIndex,
		)
	}
}
