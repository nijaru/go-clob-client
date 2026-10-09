// Inspect a protocol-aware split before submitting it. Provide a market
// POLYMARKET_CONDITION_ID or comma-separated POLYMARKET_LEGS (native IDs).
// POLYMARKET_EXECUTE=true is an explicit opt-in to sending the transaction.
// Uses a Deposit Wallet and explicit builder credentials for gasless execution.
package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/nijaru/go-clob-client/clob"
)

func main() {
	builder, err := clob.NewLocalBuilderAuth(
		clob.Credentials{
			Key:        os.Getenv("POLYMARKET_BUILDER_KEY"),
			Secret:     os.Getenv("POLYMARKET_BUILDER_SECRET"),
			Passphrase: os.Getenv("POLYMARKET_BUILDER_PASSPHRASE"),
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	client, err := clob.NewAuthenticatedClient(clob.Config{
		ChainID: clob.PolygonChainID, PrivateKey: os.Getenv("POLYMARKET_PRIVATE_KEY"), SignatureType: clob.SignatureTypePoly1271, FunderAddress: os.Getenv("POLYMARKET_FUNDER"), BuilderAuth: builder, DisableAutoHeartbeat: true,
		Credentials: &clob.Credentials{
			Key:        os.Getenv("POLYMARKET_API_KEY"),
			Secret:     os.Getenv("POLYMARKET_API_SECRET"),
			Passphrase: os.Getenv("POLYMARKET_API_PASSPHRASE"),
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	wallet, err := clob.NewWalletOperations(client, clob.WalletOperationsConfig{})
	if err != nil {
		log.Fatal(err)
	}
	req := clob.WalletSplitRequest{
		ConditionID: os.Getenv("POLYMARKET_CONDITION_ID"),
		Amount:      big.NewInt(1_000_000),
	}
	if raw := os.Getenv("POLYMARKET_LEGS"); raw != "" {
		for _, value := range strings.Split(raw, ",") {
			id, ok := new(big.Int).SetString(strings.TrimSpace(value), 10)
			if !ok {
				log.Fatal("invalid leg ID")
			}
			req.Legs = append(req.Legs, id)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	calls, err := wallet.PrepareSplitPosition(ctx, req)
	if err != nil {
		log.Fatal(err)
	}
	for _, call := range calls {
		fmt.Printf("Call %s value=%s data=0x%x\n", call.To, call.Value, call.Data)
	}
	if os.Getenv("POLYMARKET_EXECUTE") != "true" {
		return
	}
	handle, err := client.ExecuteWalletTransaction(ctx, calls, "Reviewed position split")
	if err != nil {
		log.Fatal(err)
	}
	outcome, err := handle.Wait(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Confirmed %s\n", outcome.TransactionHash)
}
