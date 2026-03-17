package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/nijaru/go-clob-client/clob"
)

func main() {
	// Initialize client with Private Key for signing and Credentials for auth
	client, err := clob.NewAuthenticatedClient(clob.Config{
		ChainID:    clob.PolygonChainID,
		PrivateKey: os.Getenv("POLYMARKET_PRIVATE_KEY"),
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

	ctx := context.Background()
	conditionID := "0x..." // Replace with a real market condition ID
	amount := "100.0"      // 100 USDC.e

	// 1. Split: USDC.e -> (YES + NO)
	fmt.Printf("Splitting %s USDC.e into outcome tokens...\n", amount)
	err = client.SplitTokens(ctx, clob.SplitArgs{
		ConditionID: conditionID,
		Amount:      amount,
	})
	if err != nil {
		log.Printf("Split failed (expecting failure if not approved/funded): %v", err)
	} else {
		fmt.Println("Split successful!")
	}

	// 2. Merge: (YES + NO) -> USDC.e
	fmt.Printf("Merging outcome tokens back into USDC.e...\n")
	err = client.MergeTokens(ctx, clob.MergeArgs{
		ConditionID: conditionID,
		Amount:      amount,
	})
	if err != nil {
		log.Printf("Merge failed: %v", err)
	} else {
		fmt.Println("Merge successful!")
	}

	// 3. Redeem: Winning Tokens -> USDC.e (after resolution)
	fmt.Printf("Redeeming winning tokens...\n")
	err = client.RedeemTokens(ctx, clob.RedeemArgs{
		ConditionID: conditionID,
	})
	if err != nil {
		log.Printf("Redeem failed: %v", err)
	} else {
		fmt.Println("Redeem successful!")
	}
}
