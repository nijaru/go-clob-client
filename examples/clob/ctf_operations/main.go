// Low-level Rust-style CTF operations for an EOA. For automatic CTF/V2 market
// routing and combos, see examples/clob/wallet_operations.
// Requires POLYMARKET_PRIVATE_KEY, POLYMARKET_CONDITION_ID (bytes32), and
// POLYMARKET_OPERATION=split|merge|redeem|redeem-neg-risk. Amounts are base units.
package main

import (
	"context"
	"encoding/hex"
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
	condition, err := hex.DecodeString(
		strings.TrimPrefix(os.Getenv("POLYMARKET_CONDITION_ID"), "0x"),
	)
	if err != nil || len(condition) != 32 {
		log.Fatal("POLYMARKET_CONDITION_ID must be bytes32")
	}
	operation := os.Getenv("POLYMARKET_OPERATION")
	if operation == "" {
		log.Fatal("POLYMARKET_OPERATION is required; this example sends a transaction")
	}
	client, err := clob.NewSignerClient(
		clob.Config{ChainID: clob.PolygonChainID, PrivateKey: os.Getenv("POLYMARKET_PRIVATE_KEY")},
	)
	if err != nil {
		log.Fatal(err)
	}
	collateral, err := client.GetCollateralAddress()
	if err != nil {
		log.Fatal(err)
	}
	if override := os.Getenv("POLYMARKET_COLLATERAL_TOKEN"); override != "" {
		if !common.IsHexAddress(override) {
			log.Fatal("invalid collateral override")
		}
		collateral = override
	}
	amount := big.NewInt(1_000_000)
	conditionID := common.BytesToHash(condition)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var receipt *clob.TxReceipt
	switch operation {
	case "split":
		receipt, err = client.SplitPosition(
			ctx,
			clob.SplitBinary(common.HexToAddress(collateral), conditionID, amount),
		)
	case "merge":
		receipt, err = client.MergePositions(
			ctx,
			clob.MergeBinary(common.HexToAddress(collateral), conditionID, amount),
		)
	case "redeem":
		receipt, err = client.RedeemPositions(
			ctx,
			clob.RedeemBinary(common.HexToAddress(collateral), conditionID),
		)
	case "redeem-neg-risk":
		receipt, err = client.RedeemNegRisk(
			ctx,
			clob.RedeemNegRiskRequest{
				ConditionID: conditionID,
				Amounts:     []*big.Int{amount, amount},
			},
		)
	default:
		log.Fatal("unknown operation")
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Confirmed %s in block %d\n", receipt.Hash, receipt.BlockNumber)
}
