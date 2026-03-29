package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/nijaru/go-clob-client/clob"
)

func main() {
	key := os.Getenv("POLYMARKET_PRIVATE_KEY")
	if key == "" {
		log.Fatal("POLYMARKET_PRIVATE_KEY is required")
	}

	client, err := clob.NewSignerClient(clob.Config{
		ChainID:    clob.PolygonChainID,
		PrivateKey: key,
	})
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	conditionID := common.HexToHash("0x...")
	collateral := common.HexToAddress("0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174")
	amount := new(big.Int).SetInt64(1_000_000)

	fmt.Println("Splitting 1 USDC into outcome tokens...")
	receipt, err := client.SplitPosition(ctx, clob.SplitBinary(collateral, conditionID, amount))
	if err != nil {
		log.Printf("Split failed: %v", err)
	} else {
		fmt.Printf("Split tx: %s (block %d)\n", receipt.Hash.Hex(), receipt.BlockNumber)
	}

	fmt.Println("Merging outcome tokens back into USDC...")
	receipt, err = client.MergePositions(ctx, clob.MergeBinary(collateral, conditionID, amount))
	if err != nil {
		log.Printf("Merge failed: %v", err)
	} else {
		fmt.Printf("Merge tx: %s (block %d)\n", receipt.Hash.Hex(), receipt.BlockNumber)
	}

	fmt.Println("Redeeming winning tokens...")
	receipt, err = client.RedeemPositions(ctx, clob.RedeemBinary(collateral, conditionID))
	if err != nil {
		log.Printf("Redeem failed: %v", err)
	} else {
		fmt.Printf("Redeem tx: %s (block %d)\n", receipt.Hash.Hex(), receipt.BlockNumber)
	}

	fmt.Println("Redeeming neg risk positions...")
	receipt, err = client.RedeemNegRisk(ctx, clob.RedeemNegRiskRequest{
		ConditionID: conditionID,
		Amounts:     []*big.Int{amount, amount},
	})
	if err != nil {
		log.Printf("NegRisk redeem failed: %v", err)
	} else {
		fmt.Printf("NegRisk redeem tx: %s (block %d)\n", receipt.Hash.Hex(), receipt.BlockNumber)
	}
}
