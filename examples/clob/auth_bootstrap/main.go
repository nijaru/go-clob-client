package main

import (
	"context"
	"log"
	"os"

	"github.com/nijaru/go-clob-client/clob"
)

func main() {
	client, err := clob.NewSignerClient(clob.Config{
		ChainID:    clob.PolygonChainID,
		PrivateKey: os.Getenv("POLYMARKET_PRIVATE_KEY"),
	})
	if err != nil {
		log.Fatal(err)
	}

	creds, err := client.CreateOrDeriveAPIKey(context.Background(), 0)
	if err != nil {
		log.Fatal(err)
	}

	authClient := client.AsAuthenticated(*creds, nil)
	_ = authClient // Use authClient for authenticated requests
	log.Printf("derived API key %s", creds.Key)
}
