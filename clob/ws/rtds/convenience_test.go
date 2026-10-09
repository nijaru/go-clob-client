package rtds

import (
	"context"
	"testing"
)

func TestConvenienceReleasesPreserveOtherInterests(t *testing.T) {
	creds := &Credentials{Key: "test-key"}
	for _, tc := range []struct {
		name        string
		subscribe   func(*Client, context.Context) error
		unsubscribe func(*Client, context.Context) error
		other       Subscription
	}{
		{"binance", func(c *Client, ctx context.Context) error { return c.SubscribeCryptoPrices(ctx, []string{"btc"}) }, func(c *Client, ctx context.Context) error { return c.UnsubscribeCryptoPrices(ctx, []string{"btc"}) }, Subscription{Topic: "crypto_prices", Type: "update", Filters: []string{"eth"}}},
		{"chainlink", func(c *Client, ctx context.Context) error { return c.SubscribeChainlinkPrices(ctx, "btc") }, func(c *Client, ctx context.Context) error { return c.UnsubscribeChainlinkPrices(ctx, "btc") }, Subscription{Topic: "crypto_prices_chainlink", Type: "*", Filters: map[string]string{"symbol": "eth"}}},
		{"twap", func(c *Client, ctx context.Context) error { return c.SubscribeChainlinkTWAP30Seconds(ctx) }, func(c *Client, ctx context.Context) error { return c.UnsubscribeChainlinkTWAP30Seconds(ctx) }, Subscription{Topic: "crypto_prices_twap_thirty", Type: "update", Filters: map[string]string{"symbol": "eth"}}},
		{"comments", func(c *Client, ctx context.Context) error { return c.SubscribeComments(ctx, CommentCreated, nil) }, func(c *Client, ctx context.Context) error { return c.UnsubscribeComments(ctx, CommentCreated, nil) }, Subscription{Topic: "comments", Type: string(CommentCreated), Filters: map[string]string{"symbol": "eth"}, CLOBAuth: creds}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := NewClient("", nil).WithCredentials(creds)
			defer client.Close()
			if err := tc.subscribe(client, t.Context()); err != nil {
				t.Fatal(err)
			}
			if err := client.Subscribe(t.Context(), tc.other); err != nil {
				t.Fatal(err)
			}
			if err := tc.unsubscribe(client, t.Context()); err != nil {
				t.Fatal(err)
			}
			if client.SubscriptionCount() != 1 {
				t.Fatal("convenience release removed another owner")
			}
			msg := &RtdsMessage{
				Topic:   tc.other.Topic,
				Type:    tc.other.Type,
				Payload: []byte(`{"symbol":"eth"}`),
			}
			if msg.Type == "*" {
				msg.Type = "update"
			}
			client.dispatch(t.Context(), msg)
			select {
			case <-client.Messages():
			default:
				t.Fatal("remaining interest stopped matching")
			}
			if err := client.Close(); err != nil {
				t.Fatal(err)
			}
			if client.SubscriptionCount() != 0 {
				t.Fatal("Close retained registrations")
			}
		})
	}
}
