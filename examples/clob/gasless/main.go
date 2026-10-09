// Example: submitting on-chain calls through Polymarket's gasless relayer.
//
// Proxy, Safe, and deposit (Poly1271) wallets can only act through the relayer,
// which submits calls as meta-transactions so the caller pays no gas. This
// example builds a single approval call and routes it through the relayer,
// then waits for on-chain confirmation.
//
// Env: POLYMARKET_PRIVATE_KEY (the EOA that controls the wallet),
//
//	POLYMARKET_API_KEY / POLYMARKET_API_SECRET / POLYMARKET_API_PASSPHRASE,
//	POLYMARKET_RELAYER_API_KEY / POLYMARKET_RELAYER_API_KEY_ADDRESS, or
//	POLYMARKET_BUILDER_KEY / POLYMARKET_BUILDER_SECRET / POLYMARKET_BUILDER_PASSPHRASE,
//	POLYMARKET_FUNDER (the proxy/Safe/deposit wallet address),
//	POLYMARKET_APPROVAL_SPENDER (the exchange or adapter to approve).
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/nijaru/go-clob-client/clob"
)

func main() {
	key := os.Getenv("POLYMARKET_PRIVATE_KEY")
	funder := os.Getenv("POLYMARKET_FUNDER")
	spender := os.Getenv("POLYMARKET_APPROVAL_SPENDER")
	if key == "" || funder == "" || spender == "" {
		log.Fatal(
			"POLYMARKET_PRIVATE_KEY, POLYMARKET_FUNDER, and POLYMARKET_APPROVAL_SPENDER are required",
		)
	}

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
		BuilderAuth:   builder,
		ChainID:       clob.PolygonChainID,
		PrivateKey:    key,
		SignatureType: clob.SignatureTypePolyProxy, // or PolyGnosisSafe / Poly1271
		FunderAddress: funder,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Approve the configured exchange or adapter to spend collateral. The
	// spender is deliberately an environment variable because the correct
	// address depends on the chain and trading product.
	collateralAddress, err := client.GetCollateralAddress()
	if err != nil {
		log.Fatal(err)
	}
	collateral := common.HexToAddress(collateralAddress)
	approvalSpender := common.HexToAddress(spender)

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

	// IsWalletDeployed checks whether the relayer knows the wallet is on-chain.
	deployed, err := client.IsWalletDeployed(ctx)
	if err != nil {
		log.Printf("IsWalletDeployed: %v", err)
	} else {
		fmt.Printf("Wallet deployed: %v\n", deployed)
	}

	fmt.Println("Submitting approval through the relayer...")
	handle, err := client.ApproveERC20Gasless(ctx, clob.ERC20ApprovalRequest{
		TokenAddress:   collateral,
		SpenderAddress: approvalSpender,
		Amount:         clob.MaxUint256(),
	}, "approve")
	if err != nil {
		log.Fatalf("ApproveERC20Gasless: %v", err)
	}
	fmt.Printf("Submitted: transactionID=%s\n", handle.TransactionID)

	// Wait polls the relayer until the transaction is confirmed (or fails).
	outcome, err := handle.Wait(ctx)
	if err != nil {
		log.Fatalf("Wait: %v", err)
	}
	fmt.Printf("Confirmed on-chain: %s\n", outcome.TransactionHash)
}
