// Run a credential-free local protocol demonstration:
//
//	go run ./examples/realtime/stream
//
// For the authenticated live feed, set POLYMARKET_API_KEY,
// POLYMARKET_API_SECRET and POLYMARKET_API_PASSPHRASE, then pass -live.
// Polybolt has no unauthenticated live price endpoint; legacy public prices
// belong to the separate RTDS API.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/nijaru/go-clob-client/realtime"
)

func main() {
	live := flag.Bool("live", false, "connect to the authenticated live Polybolt feed")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	cfg := realtime.Config{}
	if *live {
		cfg.Credentials = realtime.Credentials{
			APIKey:     os.Getenv("POLYMARKET_API_KEY"),
			Secret:     os.Getenv("POLYMARKET_API_SECRET"),
			Passphrase: os.Getenv("POLYMARKET_API_PASSPHRASE"),
		}
	} else {
		server := localFeed()
		defer server.Close()
		cfg.URL = "ws" + strings.TrimPrefix(server.URL, "http")
		cfg.Credentials = realtime.Credentials{APIKey: "demo", Secret: "demo", Passphrase: "demo"}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
	}
	client, err := realtime.NewClient(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()
	sub, err := client.Subscribe(
		ctx,
		realtime.Request{
			Channel:  realtime.Crypto,
			Symbols:  []string{"btcusd"},
			Provider: realtime.Pyth,
		},
	)
	if err != nil {
		log.Fatal(err)
	}
	defer sub.Close()
	for event := range sub.Events() {
		switch event.Type {
		case realtime.Accepted:
			provider := "unreported"
			if event.Confirmation.Provider != nil {
				provider = string(*event.Confirmation.Provider)
			}
			fmt.Printf(
				"accepted %s provider=%s connection=%d\n",
				event.Symbol,
				provider,
				event.ConnectionID,
			)
		case realtime.SnapshotEvent:
			fmt.Printf(
				"history %s source=%s points=%d\n",
				event.Symbol,
				event.Source,
				len(event.Snapshot.Data),
			)
		case realtime.UpdateEvent:
			fmt.Printf(
				"%s %s source=%s at %s\n",
				event.Symbol,
				event.Update.Value,
				event.Source,
				event.Update.Timestamp.Format(time.RFC3339Nano),
			)
		}
	}
	if err := sub.Err(); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}

// A local fixture server demonstrates the actual auth/subscribe/envelope/ping
// protocol without making network calls or asking for private credentials.
func localFeed() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		send := func(raw string) error { return conn.Write(r.Context(), websocket.MessageText, []byte(raw)) }
		for {
			_, raw, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			var frame struct {
				Op  string `json:"op"`
				RID string `json:"rid"`
			}
			if json.Unmarshal(raw, &frame) != nil {
				return
			}
			switch frame.Op {
			case "auth":
				err = send(fmt.Sprintf(`{"op":"authed","rid":%q}`, frame.RID))
			case "subscribe":
				err = send(
					fmt.Sprintf(
						`{"op":"subscribed","channel":"price.crypto","rid":%q,"provider":"pyth"}`,
						frame.RID,
					),
				)
				if err == nil {
					err = send(
						`{"v":1,"channel":"price.crypto","seq":1,"ts":123456,"snapshot":true,"payload":{"symbol":"btcusd","source":"pyth","data":[]}}`,
					)
				}
				if err == nil {
					err = send(
						`{"v":1,"channel":"price.crypto","seq":2,"ts":123457,"payload":{"symbol":"btcusd","source":"pyth","timestamp":123457,"value":123.45,"full_accuracy_value":"123.450000000000000001"}}`,
					)
				}
			case "ping":
				err = send(`{"op":"pong"}`)
			}
			if err != nil {
				return
			}
		}
	}))
}
