package sports

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestRetriesFailedReplacementUpgrade(t *testing.T) {
	wire := fixture(t, "python-game")
	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request := requests.Add(1)
		if request == 2 {
			http.Error(w, "temporary outage", http.StatusServiceUnavailable)
			return
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		if request == 1 {
			_ = conn.Close(websocket.StatusNormalClosure, "fixture disconnect")
			return
		}
		ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
		defer cancel()
		if send(t, ctx, conn, websocket.MessageText, wire) {
			_, _, _ = conn.Read(ctx)
		}
	}))
	t.Cleanup(srv.Close)
	s := dial(t, t.Context(), Config{
		URL:          "ws" + strings.TrimPrefix(srv.URL, "http"),
		ReconnectMin: time.Millisecond, ReconnectMax: 5 * time.Millisecond,
	})
	if receive(t, s.Events()).Score != "98-102" {
		t.Fatal("event not delivered after failed upgrade")
	}
	stats := s.Stats()
	var upgrade *ConnectError
	if stats.Reconnects != 1 || !errors.As(stats.LastConnectionError, &upgrade) ||
		upgrade.StatusCode != 503 {
		t.Fatalf("lost replacement upgrade details: %+v", stats)
	}
}

func TestMessageLimitClosesOversizeSocketAndRecovers(t *testing.T) {
	wire := fixture(t, "ts-minimal")
	var accepts atomic.Int64
	oversizeClosed := make(chan websocket.StatusCode, 1)
	url := server(t, func(ctx context.Context, conn *websocket.Conn) {
		if accepts.Add(1) == 1 {
			if !send(t, ctx, conn, websocket.MessageText, []byte(strings.Repeat("x", 1024))) {
				return
			}
			_, _, err := conn.Read(ctx)
			oversizeClosed <- websocket.CloseStatus(err)
			return
		}
		if send(t, ctx, conn, websocket.MessageText, wire) {
			_, _, _ = conn.Read(ctx)
		}
	})
	s := dial(t, t.Context(), Config{
		URL: url, MaxMessageBytes: 512,
		ReconnectMin: time.Millisecond, ReconnectMax: 5 * time.Millisecond,
	})
	if receive(t, oversizeClosed) != websocket.StatusMessageTooBig {
		t.Fatal("oversize frame did not trigger protocol size limit")
	}
	if receive(t, s.Events()).GameID != 123 {
		t.Fatal("replacement event missing")
	}
	if s.Stats().Reconnects != 1 || s.Stats().MalformedMessages != 0 {
		t.Fatalf("size limit not applied before parsing: %+v", s.Stats())
	}
}

func TestCloseCancelsReconnectDelay(t *testing.T) {
	url := server(t, func(ctx context.Context, conn *websocket.Conn) {
		_ = conn.Close(websocket.StatusServiceRestart, "fixture restart")
	})
	s := dial(t, t.Context(), Config{URL: url, ReconnectMin: time.Hour, ReconnectMax: time.Hour})
	deadline := time.After(3 * time.Second)
	for s.Stats().LastConnectionError == nil {
		select {
		case <-time.After(time.Millisecond):
		case <-deadline:
			t.Fatal("disconnect not observed")
		}
	}
	closed := make(chan error, 1)
	go func() { closed <- s.Close() }()
	if err := receive(t, closed); err != nil {
		t.Fatal(err)
	}
	joined(t, s)
}
