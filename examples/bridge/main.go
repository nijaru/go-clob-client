package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/nijaru/go-clob-client/bridge"
)

func main() {
	client := bridge.New(bridge.Config{})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Get supported assets
	fmt.Println("Fetching supported assets...")
	resp, err := client.GetSupportedAssets(ctx)
	if err != nil {
		log.Fatalf("failed to get supported assets: %v", err)
	}
	fmt.Printf("Found %d supported assets\n", len(resp.SupportedAssets))
	if len(resp.SupportedAssets) > 0 {
		a := resp.SupportedAssets[0]
		fmt.Printf("Example asset: %s (%s) on %s\n", a.Token.Name, a.Token.Symbol, a.ChainName)
	}

	// 2. Create deposit address (mock address)
	address := "0x1234567890123456789012345678901234567890"
	fmt.Printf("\nGenerating deposit addresses for %s...\n", address)
	addrs, err := client.CreateDepositAddress(ctx, common.HexToAddress(address))
	if err != nil {
		fmt.Printf(
			"failed to generate deposit addresses: %v (expected if not on whitelist/live)\n",
			err,
		)
	} else {
		fmt.Printf("- EVM: %s\n", addrs.Address.EVM)
		fmt.Printf("- SVM: %s\n", addrs.Address.SVM)
		fmt.Printf("- BTC: %s\n", addrs.Address.BTC)
	}
}
