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
	})
	if err != nil {
		log.Fatal(err)
	}

	creds, err := client.CreateOrDeriveAPIKey(context.Background(), 0)
	if err != nil {
		log.Fatal(err)
	}

	client.SetCredentials(*creds)
	log.Printf("derived API key %s", creds.Key)
}
