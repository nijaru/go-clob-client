package rtds_test

import (
	"context"
	"log"
	"time"

	"github.com/nijaru/go-clob-client/clob/ws/rtds"
)

func ExampleClient_Unsubscribe() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	client := rtds.NewClient("", nil)
	defer client.Close()

	btc := rtds.Subscription{Topic: "crypto_prices", Type: "update", Filters: []string{"btcusdt"}}
	eth := rtds.Subscription{Topic: "crypto_prices", Type: "update", Filters: []string{"ethusdt"}}
	// Interests can be registered before connecting. Both symbols share one
	// broad server subscription and are matched locally after each reconnect.
	for _, sub := range []rtds.Subscription{btc, eth} {
		if err := client.Subscribe(ctx, sub); err != nil {
			log.Print(err)
			return
		}
	}
	if err := client.Connect(ctx); err != nil {
		log.Print(err)
		return
	}
	select {
	case message := <-client.Messages():
		price, err := message.AsCryptoPrice()
		if err == nil {
			log.Printf("%s: %s", price.Symbol, price.Value)
		}
	case <-ctx.Done():
	}
	// Release only BTC; ETH continues on the same socket.
	if err := client.Unsubscribe(ctx, btc); err != nil {
		log.Print(err)
	}
	// Convenience unsubscribe methods likewise take the original filters.
	if err := client.UnsubscribeCryptoPrices(ctx, []string{"ethusdt"}); err != nil {
		log.Print(err)
	}
}
