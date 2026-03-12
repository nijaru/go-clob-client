package main

import (
	"context"
	"log"
	"os"

	"github.com/nijaru/go-clob-client/clob"
)

func main() {
	client, err := clob.New(clob.Config{
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

	response, err := client.CreateAndPostMarketOrder(context.Background(), clob.MarketOrderArgs{
		TokenID:   os.Getenv("POLYMARKET_TOKEN_ID"),
		Amount:    25,
		Side:      clob.SideBuy,
		OrderType: clob.OrderTypeFOK,
	}, nil, clob.OrderTypeFOK, false)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("market order response: %+v", response)
}
