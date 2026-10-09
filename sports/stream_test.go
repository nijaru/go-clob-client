package sports

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func server(t *testing.T, handler func(context.Context, *websocket.Conn)) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("POLY_API_KEY") != "" {
			t.Error("unexpected authentication")
		}
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		handler(ctx, conn)
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

func dial(t *testing.T, ctx context.Context, config Config) *Stream {
	t.Helper()
	s, err := Dial(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value, ok := <-ch:
		if !ok {
			t.Fatal("channel closed before expected value")
		}
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("timed out")
		var zero T
		return zero
	}
}

func joined(t *testing.T, s *Stream) {
	t.Helper()
	select {
	case <-s.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("lifecycle did not join")
	}
}

func send(
	t *testing.T,
	ctx context.Context,
	conn *websocket.Conn,
	kind websocket.MessageType,
	data []byte,
) bool {
	t.Helper()
	if err := conn.Write(ctx, kind, data); err != nil {
		t.Error(err)
		return false
	}
	return true
}

func TestWireContractAndMalformedFrames(t *testing.T) {
	wire := fixture(t, "python-game")
	checked := make(chan struct{})
	url := server(t, func(ctx context.Context, conn *websocket.Conn) {
		frames := make(chan string, 1)
		readerDone := make(chan struct{})
		go func() {
			defer close(readerDone)
			_, data, err := conn.Read(ctx)
			if err == nil {
				frames <- string(data)
			}
		}()
		defer func() { conn.CloseNow(); <-readerDone }()
		select {
		case frame := <-frames:
			t.Errorf("unsolicited client frame: %s", frame)
			return
		case <-time.After(50 * time.Millisecond):
		}
		if !send(t, ctx, conn, websocket.MessageText, []byte("ping")) {
			return
		}
		select {
		case frame := <-frames:
			if frame != "pong" {
				t.Errorf("heartbeat reply = %q", frame)
			}
		case <-ctx.Done():
			t.Error("no pong")
			return
		}
		for _, bad := range []string{"not json", `{"event":"future_event","payload":"new"}`} {
			if !send(t, ctx, conn, websocket.MessageText, []byte(bad)) {
				return
			}
		}
		// Python handles UTF-8 binary messages as well as text.
		if !send(t, ctx, conn, websocket.MessageBinary, wire) {
			return
		}
		close(checked)
		_, _, _ = conn.Read(ctx)
	})
	s := dial(t, t.Context(), Config{URL: url})
	event := receive(t, s.Events())
	if event.Score != "98-102" || string(event.Raw) != string(wire) {
		t.Fatalf("wrong event: %+v", event)
	}
	select {
	case <-checked:
	case <-time.After(3 * time.Second):
		t.Fatal("server contract check not completed")
	}
	if s.Stats().MalformedMessages != 2 {
		t.Fatalf("stats = %+v", s.Stats())
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	joined(t, s)
	if _, ok := <-s.Events(); ok {
		t.Fatal("events not closed")
	}
}

func TestCancellationAndConcurrentCloseJoin(t *testing.T) {
	peerClosed := make(chan struct{})
	url := server(t, func(ctx context.Context, conn *websocket.Conn) {
		_, _, _ = conn.Read(ctx)
		close(peerClosed)
	})
	ctx, cancel := context.WithCancel(t.Context())
	s := dial(t, ctx, Config{URL: url})
	cancel()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if err := s.Close(); !errors.Is(err, context.Canceled) {
				t.Errorf("Close lost context cancellation: %v", err)
			}
		})
	}
	wg.Wait()
	joined(t, s)
	select {
	case <-peerClosed:
	case <-time.After(3 * time.Second):
		t.Fatal("peer socket not closed")
	}
}

func TestOverflowTerminatesWithoutBlockingReader(t *testing.T) {
	wire := fixture(t, "ts-minimal")
	start := make(chan struct{})
	url := server(t, func(ctx context.Context, conn *websocket.Conn) {
		<-start
		for range 2 {
			if !send(t, ctx, conn, websocket.MessageText, wire) {
				return
			}
		}
		_, _, _ = conn.Read(ctx)
	})
	s := dial(t, t.Context(), Config{URL: url, BufferSize: 1})
	close(start)
	joined(t, s)
	if !errors.Is(s.Err(), ErrSlowConsumer) {
		t.Fatalf("overflow error = %v", s.Err())
	}
	if receive(t, s.Events()).GameID != 123 {
		t.Fatal("queued event lost")
	}
	if _, ok := <-s.Events(); ok {
		t.Fatal("extra event after termination")
	}
	if s.Stats().Reconnects != 0 {
		t.Fatal("overflow reconnected")
	}
}

func TestReconnectAndNoReplayRequest(t *testing.T) {
	first, second := fixture(t, "ts-minimal"), fixture(t, "python-game")
	var accepts atomic.Int64
	url := server(t, func(ctx context.Context, conn *websocket.Conn) {
		if accepts.Add(1) == 1 {
			if send(t, ctx, conn, websocket.MessageText, first) {
				_ = conn.Close(websocket.StatusServiceRestart, "fixture restart")
			}
			return
		}
		if !send(t, ctx, conn, websocket.MessageText, []byte("ping")) {
			return
		}
		_, data, err := conn.Read(ctx)
		if err != nil || string(data) != "pong" {
			t.Errorf("replacement sent unexpected frame: %q, %v", data, err)
			return
		}
		if send(t, ctx, conn, websocket.MessageText, second) {
			_, _, _ = conn.Read(ctx)
		}
	})
	s := dial(
		t,
		t.Context(),
		Config{URL: url, ReconnectMin: time.Millisecond, ReconnectMax: 5 * time.Millisecond},
	)
	if receive(t, s.Events()).Score != "0-0" || receive(t, s.Events()).Score != "98-102" {
		t.Fatal("events lost or reordered across reconnect")
	}
	stats := s.Stats()
	if stats.Reconnects != 1 ||
		websocket.CloseStatus(stats.LastConnectionError) != websocket.StatusServiceRestart {
		t.Fatalf("recovery diagnostics = %+v", stats)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestHeartbeatStalenessIgnoresGameTraffic(t *testing.T) {
	wire := fixture(t, "ts-minimal")
	var accepts atomic.Int64
	url := server(t, func(ctx context.Context, conn *websocket.Conn) {
		if accepts.Add(1) == 1 {
			// Real game traffic does not refresh the sports ping watchdog.
			for {
				if err := conn.Write(ctx, websocket.MessageText, wire); err != nil {
					return
				}
				select {
				case <-time.After(10 * time.Millisecond):
				case <-ctx.Done():
					return
				}
			}
		}
		if !send(t, ctx, conn, websocket.MessageText, []byte("ping")) {
			return
		}
		_, _, _ = conn.Read(ctx)
		if send(t, ctx, conn, websocket.MessageText, wire) {
			_, _, _ = conn.Read(ctx)
		}
	})
	s := dial(t, t.Context(), Config{
		URL: url, StaleTimeout: 100 * time.Millisecond,
		ReconnectMin: time.Millisecond, ReconnectMax: 5 * time.Millisecond,
	})
	deadline := time.After(3 * time.Second)
	for s.Stats().Reconnects == 0 {
		select {
		case <-s.Events():
		case <-deadline:
			t.Fatal("busy stale socket not replaced")
		}
	}
	if !errors.Is(s.Stats().LastConnectionError, ErrStale) {
		t.Fatalf("stale diagnostics = %+v", s.Stats())
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestServerPingsKeepIdleStreamAlive(t *testing.T) {
	wire := fixture(t, "ts-minimal")
	var accepts atomic.Int64
	url := server(t, func(ctx context.Context, conn *websocket.Conn) {
		accepts.Add(1)
		for range 5 {
			if !send(t, ctx, conn, websocket.MessageText, []byte("ping")) {
				return
			}
			_, data, err := conn.Read(ctx)
			if err != nil || string(data) != "pong" {
				t.Errorf("heartbeat = %q, %v", data, err)
				return
			}
			time.Sleep(30 * time.Millisecond)
		}
		if send(t, ctx, conn, websocket.MessageText, wire) {
			_, _, _ = conn.Read(ctx)
		}
	})
	s := dial(t, t.Context(), Config{URL: url, StaleTimeout: 100 * time.Millisecond})
	_ = receive(t, s.Events())
	if accepts.Load() != 1 || s.Stats().Reconnects != 0 {
		t.Fatal("healthy idle stream reconnected")
	}
}

func TestFailedUpgradeAndCanceledInitialDial(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fixture denied", http.StatusForbidden)
	}))
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	_, err := Dial(t.Context(), Config{URL: url})
	var upgrade *ConnectError
	if !errors.As(err, &upgrade) || upgrade.StatusCode != http.StatusForbidden {
		t.Fatalf("upgrade error = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Dial(ctx, Config{URL: url}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled dial = %v", err)
	}
}

func TestCloseJoinsReplacementUpgrade(t *testing.T) {
	var accepts atomic.Int64
	blocked := make(chan struct{})
	peerDone := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if accepts.Add(1) == 1 {
			conn, err := websocket.Accept(w, r, nil)
			if err != nil {
				t.Error(err)
				return
			}
			defer conn.CloseNow()
			_ = conn.Close(websocket.StatusServiceRestart, "restart")
			return
		}
		close(blocked)
		<-r.Context().Done()
		close(peerDone)
	}))
	t.Cleanup(srv.Close)
	s := dial(t, t.Context(), Config{
		URL:          "ws" + strings.TrimPrefix(srv.URL, "http"),
		ReconnectMin: time.Millisecond, ReconnectMax: 5 * time.Millisecond,
	})
	select {
	case <-blocked:
	case <-time.After(3 * time.Second):
		t.Fatal("replacement upgrade not started")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	joined(t, s)
	select {
	case <-peerDone:
	case <-time.After(3 * time.Second):
		t.Fatal("replacement request not canceled")
	}
}
