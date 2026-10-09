// Discover an owner's deposit wallet using public, read-only requests.
// POLYMARKET_OWNER is required; no keys or credentials are needed.
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
	owner := os.Getenv("POLYMARKET_OWNER")
	if !common.IsHexAddress(owner) {
		log.Fatal("POLYMARKET_OWNER must be an Ethereum address")
	}
	// Construction is offline. Discovery is a separate, explicit read.
	client, err := clob.NewClient(clob.Config{ChainID: clob.PolygonChainID})
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	wallet, err := client.DiscoverDepositWallet(ctx, common.HexToAddress(owner))
	if err != nil {
		log.Fatal(err)
	}
	deployed, err := client.IsWalletDeployedAt(ctx, wallet, clob.SignatureTypePoly1271)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Deposit wallet: %s; relayer deployment view: %t\n", wallet, deployed)
	// Discovery neither configures nor deploys a wallet. After reviewing this
	// identity, explicitly pass it as FunderAddress to a Poly1271 signer client.
	// Deployment visibility does not prove chain receipts or trading readiness.
}
