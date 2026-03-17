package clob_test

import (
	"context"
	"log"

	"github.com/nijaru/go-clob-client/clob"
	"github.com/quagmt/udecimal"
)

func ExampleNew_readOnly() {
	clientRaw, err := clob.New(clob.Config{})
	if err != nil {
		log.Fatal(err)
	}
	client := clientRaw.(*clob.Client)

	_ = client
}

func ExampleSignerClient_CreateOrDeriveAPIKey() {
	clientRaw, err := clob.New(clob.Config{
		ChainID:    clob.PolygonChainID,
		PrivateKey: "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
	})
	if err != nil {
		log.Fatal(err)
	}
	client := clientRaw.(*clob.SignerClient)

	if false {
		creds, err := client.CreateOrDeriveAPIKey(context.Background(), 0)
		if err != nil {
			log.Fatal(err)
		}
		authClient := client.AsAuthenticated(*creds, nil)
		_ = authClient
	}
}

func ExampleNewLocalBuilderAuth() {
	clientRaw, err := clob.New(clob.Config{
		ChainID:    clob.PolygonChainID,
		PrivateKey: "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		Credentials: &clob.Credentials{
			Key:        "api-key",
			Secret:     "api-secret",
			Passphrase: "api-passphrase",
		},
		BuilderAuth: clob.NewLocalBuilderAuth(clob.Credentials{
			Key:        "builder-key",
			Secret:     "builder-secret",
			Passphrase: "builder-passphrase",
		}),
	})
	if err != nil {
		log.Fatal(err)
	}
	client := clientRaw.(*clob.AuthenticatedClient)

	_ = client
}

func ExampleAuthenticatedClient_CreateAndPostOrder() {
	clientRaw, err := clob.New(clob.Config{
		ChainID:    clob.PolygonChainID,
		PrivateKey: "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		Credentials: &clob.Credentials{
			Key:        "api-key",
			Secret:     "api-secret",
			Passphrase: "api-passphrase",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	client := clientRaw.(*clob.AuthenticatedClient)

	if false {
		_, err := client.CreateAndPostOrder(context.Background(), clob.OrderArgs{
			TokenID: "token-id",
			Price:   udecimal.MustParse("0.45"),
			Size:    udecimal.MustParse("5"),
			Side:    clob.SideBuy,
		}, nil, clob.OrderTypeGTC, false, false)
		if err != nil {
			log.Fatal(err)
		}
	}
}
