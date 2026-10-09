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
}
