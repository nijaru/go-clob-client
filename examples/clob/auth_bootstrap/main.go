package main

import (
	"context"
	"log"
	"os"

	"github.com/nijaru/go-clob-client/clob"
)

func main() {
	clientRaw, err := clob.New(clob.Config{
		ChainID:    clob.PolygonChainID,
		PrivateKey: os.Getenv("POLYMARKET_PRIVATE_KEY"),
	})
	if err != nil {
		log.Fatal(err)
	}
	client := clientRaw.(*clob.SignerClient)

	creds, err := client.CreateOrDeriveAPIKey(context.Background(), 0)
	if err != nil {
		log.Fatal(err)
	}

	authClient := client.AsAuthenticated(*creds, nil)
	_ = authClient // Use authClient for authenticated requests
	log.Printf("derived API key %s", creds.Key)
}
