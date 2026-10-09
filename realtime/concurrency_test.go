package realtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestCloseCancelsInFlightUpgrade(t *testing.T) {
	entered := make(chan struct{})
	finished := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(finished)
	}))
	t.Cleanup(server.Close)
	c := testClient(t, Config{
		URL:         "ws" + strings.TrimPrefix(server.URL, "http"),
		Credentials: Credentials{"test-key", "test-secret", "test-pass"},
	})
	result := make(chan error, 1)
	go func() {
		_, err := c.Subscribe(t.Context(), Request{Channel: Crypto, Symbols: []string{"btcusd"}})
		result <- err
	}()
	waitClosed(t, entered)
	c.Close()
	if err := receive(t, result); !errors.Is(err, ErrClosed) {
		t.Fatalf("shutdown during upgrade = %v", err)
	}
	waitClosed(t, finished)
}

func TestRemovalAndReplacementDuringSubscribeAcknowledgement(t *testing.T) {
	gate := make(chan struct{})
	blocked := false // Only one server reader, on the retained socket.
	f := startFeed(t, func(p *testPeer, frame serverFrame) {
		if frame.Op == "subscribe" && frame.Subscriptions[0].Filter["symbol"] == "btcusd" &&
			!blocked {
			blocked = true
			select {
			case <-gate:
			case <-p.ctx.Done():
				return
			}
		}
		p.acknowledge(t, frame, "")
	})
	c := testClient(t, testConfig(f))
	keeper := subscribe(t, c, t.Context(), Request{Channel: Crypto, Symbols: []string{"ethusd"}})
	f.next(t, "subscribe")
	oldCtx, cancel := context.WithCancel(t.Context())
	oldResult := make(chan error, 1)
	go func() {
		_, err := c.Subscribe(oldCtx, Request{Channel: Crypto, Symbols: []string{"btcusd"}})
		oldResult <- err
	}()
	old := f.next(t, "subscribe")
	cancel()
	if err := receive(t, oldResult); !errors.Is(err, context.Canceled) {
		t.Fatalf("old acceptance=%v", err)
	}
	replacement := make(chan *Subscription, 1)
	go func() {
		s, err := c.Subscribe(t.Context(), Request{Channel: Crypto, Symbols: []string{"btcusd"}})
		if err != nil {
			t.Error(err)
			return
		}
		replacement <- s
	}()
	close(gate)
	// The late acceptance of the canceled state cannot accept the replacement.
	// Clear the old upstream filter before creating its new incarnation.
	removed := f.next(t, "unsubscribe")
	added := f.next(t, "subscribe")
	if removed.peer != old.peer || added.peer != old.peer ||
		removed.frame.Subscriptions[0].Filter["symbol"] != "btcusd" {
		t.Fatal("late acceptance lost unsubscribe ordering")
	}
	fresh := receive(t, replacement)
	accepted := receive(t, fresh.Events())
	if accepted.Type != Accepted {
		t.Fatal("replacement not accepted")
	}
	fresh.Close()
	keeper.Close()
	c.Close()
	waitClosed(t, old.peer.done)
}

func TestTerminalFailureRacingNewListeners(t *testing.T) {
	gate := make(chan struct{})
	f := startFeed(t, func(p *testPeer, frame serverFrame) {
		if frame.Op == "auth" {
			select {
			case <-gate:
			case <-p.ctx.Done():
				return
			}
			p.write(t, fmt.Sprintf(`{"op":"error","rid":%q,"code":"auth_invalid"}`, frame.RID))
		}
	})
	c := testClient(t, testConfig(f))
	const count = 40
	errorsOut := make(chan error, count+1)
	go func() {
		_, err := c.Subscribe(t.Context(), Request{Channel: Crypto, Symbols: []string{"firstusd"}})
		errorsOut <- err
	}()
	first := f.next(t, "auth")
	start := make(chan struct{})
	var launched sync.WaitGroup
	for i := range count {
		launched.Add(1)
		go func() {
			launched.Done()
			<-start
			_, err := c.Subscribe(
				t.Context(),
				Request{Channel: Crypto, Symbols: []string{fmt.Sprintf("asset%dusd", i)}},
			)
			errorsOut <- err
		}()
	}
	launched.Wait()
	close(start)
	close(gate)
	for range count + 1 {
		err := receive(t, errorsOut)
		var rejected *RejectionError
		if !errors.As(err, &rejected) || rejected.Code != "auth_invalid" {
			t.Fatalf("listener stranded by terminal failure: %v", err)
		}
	}
	c.Close()
	waitClosed(t, first.peer.done)
}
