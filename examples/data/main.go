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
	client := data.New(data.Config{})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user := os.Getenv("POLYMARKET_USER")
	if user == "" {
		user = "0x1234567890123456789012345678901234567890"
	}

	positions, err := client.GetPositions(ctx, data.PositionParams{User: user, Limit: 5})
	if err != nil {
		log.Fatalf("get positions: %v", err)
	}

	fmt.Printf("Fetched %d positions for %s\n", len(positions), user)
	for _, pos := range positions {
		fmt.Printf("%s: %s shares @ avg %s\n", pos.Title, pos.Size, pos.AvgPrice)
	}
}
