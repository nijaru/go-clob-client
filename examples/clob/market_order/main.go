package main

import (
	"context"
	"log"
	"os"

	"github.com/nijaru/go-clob-client/clob"
	"github.com/quagmt/udecimal"
)

func main() {
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

	response, err := client.CreateAndPostMarketOrder(context.Background(), clob.MarketOrderArgs{
		TokenID: os.Getenv("POLYMARKET_TOKEN_ID"),
		Amount:  udecimal.MustParse("10"),
		Side:    clob.SideBuy,
	}, nil, clob.OrderTypeFOK)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("market order response: %+v", response)
}
