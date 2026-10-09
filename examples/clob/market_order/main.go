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

	// Do not pay more than $0.50 per share, even if the book moves.
	maxPrice := udecimal.MustParse("0.50")
	response, err := client.CreateAndPostMarketOrder(context.Background(), clob.MarketOrderArgs{
		TokenID:  os.Getenv("POLYMARKET_TOKEN_ID"),
		Amount:   udecimal.MustParse("10"),
		Side:     clob.SideBuy,
		MaxPrice: &maxPrice,
	}, nil, clob.OrderTypeFOK)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("market order response: %+v", response)
}
