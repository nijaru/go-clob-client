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
//	POLYMARKET_FUNDER (the proxy/Safe/deposit wallet address).
package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"

	"github.com/nijaru/go-clob-client/clob"
	"github.com/nijaru/go-clob-client/internal/polyrelay"
)

func main() {
	key := os.Getenv("POLYMARKET_PRIVATE_KEY")
	funder := os.Getenv("POLYMARKET_FUNDER")
	if key == "" || funder == "" {
		log.Fatal("POLYMARKET_PRIVATE_KEY and POLYMARKET_FUNDER are required")
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

	// Build a simple call: approve the exchange to spend 1 USDC of collateral.
	// (Calldata shape is illustrative — use your own ABI encoding in practice.)
	collateral := common.HexToAddress("0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174")
	calls := []polyrelay.TransactionCall{
		{To: collateral, Data: common.FromHex("0x..."), Value: big.NewInt(0)},
	}

	ctx := context.Background()

	// IsWalletDeployed checks whether the relayer knows the wallet is on-chain.
	deployed, err := client.IsWalletDeployed(ctx)
	if err != nil {
		log.Printf("IsWalletDeployed: %v", err)
	} else {
		fmt.Printf("Wallet deployed: %v\n", deployed)
	}

	fmt.Println("Submitting approval through the relayer...")
	handle, err := client.PrepareGaslessTransaction(ctx, calls, "approve")
	if err != nil {
		log.Fatalf("PrepareGaslessTransaction: %v", err)
	}
	fmt.Printf("Submitted: transactionID=%s\n", handle.TransactionID)

	// Wait polls the relayer until the transaction is confirmed (or fails).
	outcome, err := handle.Wait(ctx)
	if err != nil {
		log.Fatalf("Wait: %v", err)
	}
	fmt.Printf("Confirmed on-chain: %s\n", outcome.TransactionHash)
}
