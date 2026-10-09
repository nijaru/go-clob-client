package ws_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/nijaru/go-clob-client/clob/ws"
	"github.com/nijaru/go-clob-client/clob/ws/rtds"
)

type lifecycleClient interface {
	Connect(context.Context) error
	Close() error
	IsConnected() bool
}

func TestCloseDuringWebSocketDial(t *testing.T) {
	t.Parallel()
	for _, factory := range []struct {
		name string
		new  func(string) lifecycleClient
	}{
		{"clob", func(url string) lifecycleClient { return ws.NewClient(url) }},
		{"rtds", func(url string) lifecycleClient { return rtds.NewClient(url, nil) }},
	} {
		t.Run(factory.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			started, release, disconnected := make(
				chan struct{},
			), make(
				chan struct{},
			), make(
				chan struct{},
			)
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					close(started)
					select {
					case <-release:
					case <-ctx.Done():
						return
					}
					conn, err := websocket.Accept(w, r, nil)
					if err != nil {
						return
					}
					defer conn.CloseNow()
					_, _, _ = conn.Read(ctx)
					close(disconnected)
				}),
			)
			defer server.Close()
			client := factory.new("ws" + strings.TrimPrefix(server.URL, "http"))
			defer client.Close()
			result := make(chan error, 1)
			go func() { result <- client.Connect(ctx) }()
			select {
			case <-started:
			case <-ctx.Done():
				t.Fatal("dial did not start")
			}
			if err := client.Close(); err != nil {
				t.Fatal(err)
			}
			close(release)
			select {
			case err := <-result:
				if err == nil {
					t.Error("Connect succeeded after Close")
				}
			case <-ctx.Done():
				t.Fatal("Connect did not complete")
			}
			if client.IsConnected() {
				t.Error("closed client reports connected")
			}
			select {
			case <-disconnected:
			case <-ctx.Done():
				t.Error("late connection leaked after Close")
			}
		})
	}
}
