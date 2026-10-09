// Inspect a protocol-aware split before submitting it. Provide a market
// POLYMARKET_CONDITION_ID or comma-separated POLYMARKET_LEGS (native IDs).
// POLYMARKET_EXECUTE=true is an explicit opt-in to sending the transaction.
// Uses a Deposit Wallet and explicit builder or Relayer API-key credentials.
// Deployment is checked/created only after the execute opt-in.
package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/nijaru/go-clob-client/clob"
)

func main() {
	var builder clob.BuilderAuth
	var err error
	if os.Getenv("POLYMARKET_RELAYER_API_KEY") == "" {
		builder, err = clob.NewLocalBuilderAuth(clob.Credentials{
			Key: os.Getenv(
				"POLYMARKET_BUILDER_KEY",
			), Secret: os.Getenv("POLYMARKET_BUILDER_SECRET"),
			Passphrase: os.Getenv("POLYMARKET_BUILDER_PASSPHRASE"),
		})
		if err != nil {
			log.Fatal(err)
		}
	}
	client, err := clob.NewSignerClient(clob.Config{
		ChainID: clob.PolygonChainID, PrivateKey: os.Getenv("POLYMARKET_PRIVATE_KEY"), SignatureType: clob.SignatureTypePoly1271, FunderAddress: os.Getenv("POLYMARKET_FUNDER"), BuilderAuth: builder,
	})
	if err != nil {
		log.Fatal(err)
	}
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
	if key := os.Getenv("POLYMARKET_RELAYER_API_KEY"); key != "" {
		address := os.Getenv("POLYMARKET_RELAYER_API_KEY_ADDRESS")
		if !common.IsHexAddress(address) {
			log.Fatal("invalid relayer API-key address")
		}
		ctx, err = clob.WithRelayerAuth(ctx, clob.RelayerAuthConfig{
			APIKey: &clob.RelayerAPIKey{Key: key, Address: common.HexToAddress(address)},
		})
		if err != nil {
			log.Fatal(err)
		}
	}
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
	readiness, err := client.EnsureWalletReady(ctx, "Deploy Deposit Wallet")
	if err != nil {
		log.Fatalf("wallet readiness: %v (progress: %+v)", err, readiness)
	}
	handle, err := client.ExecuteWalletTransaction(ctx, calls, "Reviewed position split")
	if err != nil {
		log.Fatal(err)
	}
	receipt, err := handle.WaitReceipt(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf(
		"Confirmed %s in block %s (CLOB indexing may lag)\n",
		receipt.TxHash,
		receipt.BlockNumber,
	)
}
