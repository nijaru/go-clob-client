package main

import (
	"context"
	"fmt"
	"log"

	"github.com/nijaru/go-clob-client/clob"
)

func main() {
	clientRaw, err := clob.New(clob.Config{})
	if err != nil {
		log.Fatal(err)
	}
	client := clientRaw.(*clob.Client)

	serverTime, err := client.GetServerTime(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("server time: %d\n", serverTime)
}
