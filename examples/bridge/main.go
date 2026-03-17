package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nijaru/go-clob-client/bridge"
)

func main() {
	client := bridge.New(bridge.Config{})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Get supported assets
	fmt.Println("Fetching supported assets...")
	assets, err := client.GetSupportedAssets(ctx)
	if err != nil {
		log.Fatalf("failed to get supported assets: %v", err)
	}
	fmt.Printf("Found %d supported assets\n", len(assets))
	if len(assets) > 0 {
		fmt.Printf("Example asset: %s on %s\n", assets[0].TokenSymbol, assets[0].ChainName)
	}

	// 2. Create deposit address (mock address)
	address := "0x1234567890123456789012345678901234567890"
	fmt.Printf("\nGenerating deposit addresses for %s...\n", address)
	addrs, err := client.CreateDepositAddress(ctx, address)
	if err != nil {
		fmt.Printf(
			"failed to generate deposit addresses: %v (expected if not on whitelist/live)\n",
			err,
		)
	} else {
		for _, a := range addrs.Addresses {
			fmt.Printf("- %s: %s\n", a.Network, a.Address)
		}
	}
}
