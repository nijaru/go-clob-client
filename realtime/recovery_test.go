package realtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestInitialUpgradeFailurePreservesTransportDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	c := testClient(t, Config{
		URL:         "ws" + strings.TrimPrefix(server.URL, "http"),
		Credentials: Credentials{"test-key", "test-secret", "test-pass"},
	})
	_, err := c.Subscribe(t.Context(), Request{Channel: Crypto, Symbols: []string{"btcusd"}})
	failure, ok := errors.AsType[*ConnectError](err)
	if !ok || failure.StatusCode != http.StatusServiceUnavailable || failure.Unwrap() == nil {
		t.Fatalf("upgrade error = %v", err)
	}
	c.Close()
}

func TestAcknowledgementTimeoutRestartsUncertainSocket(t *testing.T) {
	f := startFeed(t, func(p *testPeer, frame serverFrame) {
		if p.id == 1 && frame.Op == "subscribe" {
			return
		}
		p.acknowledge(t, frame, "")
	})
	cfg := testConfig(f)
	cfg.AckTimeout = 75 * time.Millisecond
	c := testClient(t, cfg)
	s := subscribe(t, c, t.Context(), Request{Channel: Crypto, Symbols: []string{"btcusd"}})
	lost := f.next(t, "subscribe")
	replay := f.next(t, "subscribe")
	if lost.peer == replay.peer || replay.frame.Subscriptions[0].Filter["symbol"] != "btcusd" {
		t.Fatal("uncertain subscription was not recovered on a new socket")
	}
	waitClosed(t, lost.peer.done)
	accepted := receive(t, s.Events())
	if accepted.Type != Accepted {
		t.Fatal("acceptance missing after recovery")
	}
	s.Close()
	waitClosed(t, replay.peer.done)
}

func TestUnsubscribeRejectionCannotLeaveOrphanFilters(t *testing.T) {
	f := startFeed(t, func(p *testPeer, frame serverFrame) {
		if frame.Op == "unsubscribe" {
			p.write(t, fmt.Sprintf(`{"op":"error","rid":%q,"code":"bad_filter"}`, frame.RID))
			return
		}
		p.acknowledge(t, frame, "")
	})
	c := testClient(t, testConfig(f))
	keeper := subscribe(t, c, t.Context(), Request{Channel: Crypto, Symbols: []string{"ethusd"}})
	initial := f.next(t, "subscribe")
	receive(t, keeper.Events())
	removed := subscribe(t, c, t.Context(), Request{Channel: Crypto, Symbols: []string{"btcusd"}})
	f.next(t, "subscribe")
	removed.Close()
	f.next(t, "unsubscribe")
	replay := f.next(t, "subscribe")
	if replay.peer == initial.peer || len(replay.frame.Subscriptions) != 1 ||
		replay.frame.Subscriptions[0].Filter["symbol"] != "ethusd" {
		t.Fatal("orphaned filter survived rejected unsubscribe")
	}
	waitClosed(t, initial.peer.done)
	confirmation := receive(t, keeper.Events())
	if confirmation.ConnectionID == 1 {
		t.Fatal("reconnect confirmation not refreshed")
	}
	keeper.Close()
}

func TestApplicationHeartbeatAndStaleRecovery(t *testing.T) {
	var silent atomic.Bool
	f := startFeed(t, func(p *testPeer, frame serverFrame) {
		if frame.Op == "ping" && silent.Load() {
			return
		}
		p.acknowledge(t, frame, "")
	})
	cfg := testConfig(f)
	cfg.PingInterval = 20 * time.Millisecond
	cfg.StaleTimeout = 200 * time.Millisecond
	c := testClient(t, cfg)
	s := subscribe(t, c, t.Context(), Request{Channel: Crypto, Symbols: []string{"btcusd"}})
	initial := f.next(t, "subscribe")
	receive(t, s.Events())
	// Several application pongs keep an otherwise idle socket alive across more
	// than one stale interval. No RFC WebSocket Ping is used for this protocol.
	for range 12 {
		ping := f.next(t, "ping")
		if ping.peer != initial.peer || ping.frame.RID != "" {
			t.Fatal("wrong heartbeat frame or premature reconnect")
		}
	}
	silent.Store(true)
	replay := f.next(t, "subscribe")
	if replay.peer == initial.peer {
		t.Fatal("silent socket was reused")
	}
	waitClosed(t, initial.peer.done)
	confirmation := receive(t, s.Events())
	if confirmation.ConnectionID == uint64(initial.peer.id) {
		t.Fatal("generation did not change")
	}
	s.Close()
}

func TestAuthenticationTerminalCloseBeforeAcknowledgement(t *testing.T) {
	f := startFeed(t, func(p *testPeer, frame serverFrame) {
		if frame.Op == "auth" {
			p.conn.Close(4001, "authentication expired")
		}
	})
	c := testClient(t, testConfig(f))
	_, err := c.Subscribe(t.Context(), Request{Channel: Crypto, Symbols: []string{"btcusd"}})
	closed, ok := errors.AsType[*ConnectionError](err)
	if !ok || closed.Code != 4001 || closed.Reason != "authentication expired" {
		t.Fatalf("terminal authentication close = %v", err)
	}
	c.Close()
	peer := receive(t, f.peers)
	waitClosed(t, peer.done)
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.observations) != 1 {
		t.Fatal("terminal authentication close retried")
	}
}

func TestTemporaryAuthenticationFailureRetries(t *testing.T) {
	f := startFeed(t, func(p *testPeer, frame serverFrame) {
		if p.id == 1 && frame.Op == "auth" {
			p.write(t, fmt.Sprintf(`{"op":"error","rid":%q,"code":"auth_unavailable"}`, frame.RID))
			return
		}
		p.acknowledge(t, frame, "")
	})
	c := testClient(t, testConfig(f))
	s := subscribe(t, c, t.Context(), Request{Channel: EquityTWAP, Symbols: []string{"eurusd"}})
	first := f.next(t, "auth")
	second := f.next(t, "auth")
	if first.peer == second.peer {
		t.Fatal("temporary auth failure did not restart")
	}
	waitClosed(t, first.peer.done)
	s.Close()
}

func TestSlowConsumerEndsOnlyItsHandle(t *testing.T) {
	f := startFeed(t, nil)
	cfg := testConfig(f)
	cfg.BufferSize = 2
	c := testClient(t, cfg)
	slow := subscribe(t, c, t.Context(), Request{Channel: Crypto, Symbols: []string{"btcusd"}})
	peer := f.next(t, "subscribe").peer
	fast := subscribe(t, c, t.Context(), Request{Channel: Crypto, Symbols: []string{"btcusd"}})
	receive(t, fast.Events()) // Slow still has its acceptance queued.
	for i := range 3 {
		peer.write(
			t,
			strings.Replace(
				priceFixture("price.crypto", "btcusd", false),
				`"seq":2`,
				fmt.Sprintf(`"seq":%d`, i+2),
				1,
			),
		)
		event := receive(t, fast.Events())
		if event.Update == nil {
			t.Fatal("fast consumer lost event")
		}
	}
	waitClosed(t, slow.Done())
	if !errors.Is(slow.Err(), ErrSlowConsumer) {
		t.Fatalf("slow consumer error=%v", slow.Err())
	}
	fast.Close()
	waitClosed(t, peer.done)
}

func TestSharedSnapshotRefreshAndIsolation(t *testing.T) {
	f := startFeed(t, nil)
	c := testClient(t, testConfig(f))
	first := subscribe(t, c, t.Context(), Request{Channel: Crypto, Symbols: []string{"btcusd"}})
	peer := f.next(t, "subscribe").peer
	receive(t, first.Events())
	peer.write(t, priceFixture("price.crypto", "btcusd", true))
	snapshot := receive(t, first.Events())
	snapshot.Snapshot.Data[0].Value = "corrupted-by-caller"
	peer.write(t, priceFixture("price.crypto", "btcusd", false))
	receive(t, first.Events())
	join := subscribe(t, c, t.Context(), Request{Channel: Crypto, Symbols: []string{"btcusd"}})
	receive(t, join.Events())
	current := receive(t, join.Events())
	if current.Snapshot == nil || len(current.Snapshot.Data) != 2 ||
		current.Snapshot.Data[0].Value != "123.450000000000000001" ||
		current.Snapshot.Data[1].Value != "123.450000000000000002" {
		t.Fatalf("current history=%+v", current.Snapshot)
	}
	current.Snapshot.Data[1].Value = "mutated"
	join.Close()
	// Older updates cannot roll shared history backwards.
	older := strings.Replace(
		priceFixture("price.crypto", "btcusd", false),
		`"timestamp":123456`,
		`"timestamp":123400`,
		1,
	)
	peer.write(t, older)
	receive(t, first.Events())
	latest := subscribe(
		t,
		c,
		t.Context(),
		Request{Channel: Crypto, Symbols: []string{"btcusd"}, Types: []EventType{SnapshotEvent}},
	)
	receive(t, latest.Events())
	cache := receive(t, latest.Events())
	if cache.Snapshot.Data[1].Value != "123.450000000000000002" {
		t.Fatal("history mutated or regressed")
	}
	// Reconnect invalidates all cached history before replay; a joining listener
	// must not receive an earlier generation's snapshot.
	peer.conn.CloseNow()
	replay := f.next(t, "subscribe")
	receive(t, first.Events())
	receive(t, latest.Events())
	after := subscribe(t, c, t.Context(), Request{Channel: Crypto, Symbols: []string{"btcusd"}})
	confirmation := receive(t, after.Events())
	if confirmation.ConnectionID == cache.ConnectionID {
		t.Fatal("cached confirmation survived reconnect")
	}
	replay.peer.write(t, priceFixture("price.crypto", "btcusd", true))
	fresh := receive(t, after.Events())
	if fresh.ConnectionID != confirmation.ConnectionID || len(fresh.Snapshot.Data) != 1 {
		t.Fatal("stale history survived reconnect")
	}
	first.Close()
	latest.Close()
	after.Close()
}

func TestServerLossReducesNewFilterPlacement(t *testing.T) {
	f := startFeed(t, nil)
	c := testClient(t, testConfig(f))
	symbols := make([]string, 32)
	for i := range symbols {
		symbols[i] = fmt.Sprintf("asset%dusd", i)
	}
	existing := subscribe(t, c, t.Context(), Request{Channel: Crypto, Symbols: symbols})
	peer := f.next(t, "subscribe").peer
	for range len(symbols) {
		receive(t, existing.Events())
	}
	peer.write(t, priceFixture("price.crypto", symbols[0], false))
	loss := receive(t, existing.Events())
	if loss.Dropped == nil || *loss.Dropped != 3 {
		t.Fatal("loss metadata missing")
	}
	// Server loss halves the target for new placements without migrating or
	// interrupting the existing filters. The 33rd filter goes to a new socket.
	additional := subscribe(
		t,
		c,
		t.Context(),
		Request{Channel: Crypto, Symbols: []string{"otherusd"}},
	)
	placement := f.next(t, "subscribe")
	if placement.peer == peer {
		t.Fatal("loss did not reduce new-filter placement capacity")
	}
	existing.Close()
	additional.Close()
	waitClosed(t, peer.done)
	waitClosed(t, placement.peer.done)
}

func TestAcceptanceDeadlineAndSubscriptionLifetime(t *testing.T) {
	f := startFeed(t, func(p *testPeer, frame serverFrame) {
		if frame.Op != "subscribe" {
			p.acknowledge(t, frame, "")
		}
	})
	cfg := testConfig(f)
	cfg.AcceptanceTimeout = 200 * time.Millisecond
	cfg.AckTimeout = time.Second
	c := testClient(t, cfg)
	_, err := c.Subscribe(t.Context(), Request{Channel: Crypto, Symbols: []string{"btcusd"}})
	var timeout *TimeoutError
	if !errors.As(err, &timeout) || timeout.Operation != "subscription acceptance" {
		t.Fatalf("acceptance error=%v", err)
	}
	peer := f.next(t, "subscribe").peer
	waitClosed(t, peer.done)
	// Cancellation after acceptance also terminates the stream, not just the
	// synchronous Subscribe call.
	live := startFeed(t, nil)
	client := testClient(t, testConfig(live))
	ctx, cancel := context.WithCancel(t.Context())
	s := subscribe(t, client, ctx, Request{Channel: Equity, Symbols: []string{"aapl"}})
	wire := live.next(t, "subscribe")
	cancel()
	waitClosed(t, s.Done())
	waitClosed(t, wire.peer.done)
	if !errors.Is(s.Err(), context.Canceled) {
		t.Fatalf("stream lifetime error=%v", s.Err())
	}
}
