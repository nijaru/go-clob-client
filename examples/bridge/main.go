package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/nijaru/go-clob-client/bridge"
)

func main() {
	depositWallet := flag.String(
		"deposit-wallet",
		"",
		"explicitly create routing addresses for this Polymarket wallet (remote write)",
	)
	flag.Parse()
	client, err := bridge.NewClient(bridge.Config{})
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	assets, err := client.GetSupportedAssets(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Supported assets: %d\n", len(assets.SupportedAssets))
	for _, asset := range assets.SupportedAssets {
		fmt.Printf(
			"%s on %s (%d): minimum USD %s\n",
			asset.Token.Symbol,
			asset.ChainName,
			asset.ChainID,
			asset.MinCheckoutUSD,
		)
	}
	// Running this example without a flag performs only a public GET.
	if *depositWallet == "" {
		return
	}
	if !common.IsHexAddress(*depositWallet) {
		log.Fatal("deposit-wallet must be an EVM address")
	}
	addresses, err := client.CreateDepositAddress(ctx, common.HexToAddress(*depositWallet))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf(
		"Routing addresses: EVM %s / SVM %s / BTC %s\n",
		addresses.Address.EVM,
		addresses.Address.SVM,
		addresses.Address.BTC,
	)
}
