package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/nijaru/go-clob-client/clob"
	"github.com/nijaru/go-clob-client/clob/ws"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt for graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	client := ws.NewClient("")
	fmt.Printf("Connecting to %s...\n", ws.ChannelMarket)
	if err := client.Connect(ctx); err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer client.Close()

	// Subscribe to a popular market (e.g. Trump 2024 Election Win token)
	// Asset ID for "Yes" token of the 2024 Election Win market (example ID)
	assetID := "20593414902008800045145829672023910384812242371994326577488052671542151608248"
	if err := client.SubscribeMarket(ctx, []string{assetID}); err != nil {
		log.Fatalf("failed to subscribe: %v", err)
	}

	fmt.Println("Subscribed! Waiting for events...")

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Shutting down...")
			return
		case err := <-client.Errors():
			log.Printf("WS error: %v", err)
		case event := <-client.Events():
			switch e := event.(type) {
			case *ws.BookEvent:
				fmt.Printf("\n[BOOK] %s (Timestamp: %s)\n", e.AssetID, e.Timestamp)
				fmt.Printf(" Bids: %d levels, Best Bid: %s\n", len(e.Bids), safeBest(e.Bids))
				fmt.Printf(" Asks: %d levels, Best Ask: %s\n", len(e.Asks), safeBest(e.Asks))
			case *ws.PriceChangeEvent:
				fmt.Printf("[PRICE] %s: %d changes\n", e.AssetID, len(e.Changes))
				for _, c := range e.Changes {
					fmt.Printf("  %s %s @ %s\n", c.Side, c.Size, c.Price)
				}
			case *ws.LastTradePriceEvent:
				fmt.Printf("[TRADE] %s: %s @ %s (%s)\n", e.AssetID, e.Size, e.Price, e.Side)
			case *ws.TickSizeChangeEvent:
				fmt.Printf("[TICK] %s: %s -> %s\n", e.AssetID, e.OldTickSize, e.NewTickSize)
			}
		}
	}
}

func safeBest(levels []clob.OrderSummary) string {
	if len(levels) == 0 {
		return "none"
	}
	// Best bid is last in Bids array, Best ask is last in Asks array per SDK docs
	return levels[len(levels)-1].Price
}
