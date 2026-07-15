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
//	POLYMARKET_FUNDER (the proxy/Safe/deposit wallet address),
//	POLYMARKET_APPROVAL_SPENDER (the exchange or adapter to approve).
package main

import (
	"context"
	"fmt"
	"log"
	"os"

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

	client, err := clob.NewAuthenticatedClient(clob.Config{
		ChainID:       clob.PolygonChainID,
		PrivateKey:    key,
		SignatureType: clob.SignatureTypePolyProxy, // or PolyGnosisSafe / Poly1271
		FunderAddress: funder,
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

	// Approve the configured exchange or adapter to spend collateral. The
	// spender is deliberately an environment variable because the correct
	// address depends on the chain and trading product.
	collateral := common.HexToAddress("0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174")
	approvalSpender := common.HexToAddress(spender)

	ctx := context.Background()

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
