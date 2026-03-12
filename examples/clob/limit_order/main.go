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

	response, err := client.CreateAndPostOrder(context.Background(), clob.OrderArgs{
		TokenID: os.Getenv("POLYMARKET_TOKEN_ID"),
		Price:   0.45,
		Size:    5,
		Side:    clob.SideBuy,
	}, nil, clob.OrderTypeGTC, false, false)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("order response: %s", response)
}
