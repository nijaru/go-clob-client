// The default example uses an in-process, read-only sports WebSocket fixture.
// Pass -url wss://sports-api.polymarket.com/ws to explicitly use the public feed.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/nijaru/go-clob-client/sports"
)

func main() {
	url := flag.String("url", "", "sports WebSocket URL; empty starts a local fixture")
	timeout := flag.Duration("timeout", 10*time.Second, "maximum wait for one game result")
	flag.Parse()
	if err := run(*url, *timeout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(url string, timeout time.Duration) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if url == "" {
		server := localFixture()
		defer server.Close()
		url = "ws" + strings.TrimPrefix(server.URL, "http")
	}
	stream, err := sports.Dial(ctx, sports.Config{URL: url})
	if err != nil {
		return err
	}
	defer stream.Close()
	for event := range stream.Events() {
		// Print the unchanged wire payload, including any unknown metadata.
		fmt.Println(string(event.Raw))
		return nil
	}
	return stream.Err()
}

func localFixture() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if err := conn.Write(ctx, websocket.MessageText, []byte("ping")); err != nil {
			return
		}
		_, response, err := conn.Read(ctx)
		if err != nil || string(response) != "pong" {
			return
		}
		// TS 087f9443 packages/client/src/websockets/sports.test.ts fixture.
		game := `{"ended":false,"gameId":123,"leagueAbbreviation":"NBA","live":true,"score":"0-0","status":"inprogress"}`
		if err := conn.Write(ctx, websocket.MessageText, []byte(game)); err != nil {
			return
		}
		_, _, _ = conn.Read(ctx)
	}))
}
