package realtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// A removed filter can still emit until its unsubscribe is acknowledged. Those
// frames must not become the replacement's events or replay history.
func TestReplacementIgnoresEventsDuringUnsubscribe(t *testing.T) {
	gate := make(chan struct{})
	f := startFeed(t, func(p *testPeer, frame serverFrame) {
		if frame.Op == "unsubscribe" {
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
	receive(t, keeper.Events())
	old := subscribe(t, c, t.Context(), Request{Channel: Crypto, Symbols: []string{"btcusd"}})
	f.next(t, "subscribe")
	old.Close()
	removal := f.next(t, "unsubscribe")
	registered := make(chan struct{})
	// AfterFunc asks for Done while registering the stream's lifetime callback.
	ctx := &registrationContext{Context: t.Context(), registered: registered}
	result := make(chan *Subscription, 1)
	go func() {
		s, err := c.Subscribe(ctx, Request{Channel: Crypto, Symbols: []string{"btcusd"}})
		if err != nil {
			t.Error(err)
			return
		}
		result <- s
	}()
	waitClosed(t, registered)
	removal.peer.write(t, priceFixture("price.crypto", "btcusd", true))
	removal.peer.write(t, priceFixture("price.crypto", "btcusd", false))
	// The keeper proves the preceding frames have been consumed, without sleeps.
	removal.peer.write(t, priceFixture("price.crypto", "ethusd", false))
	receive(t, keeper.Events())
	close(gate)
	fresh := receive(t, result)
	if event := receive(t, fresh.Events()); event.Type != Accepted {
		t.Fatalf("unaccepted replacement received old event: %+v", event)
	}
	join := subscribe(t, c, t.Context(), Request{Channel: Crypto, Symbols: []string{"btcusd"}})
	if event := receive(t, join.Events()); event.Type != Accepted {
		t.Fatalf("joining replacement received old event: %+v", event)
	}
	select {
	case event := <-join.Events():
		t.Fatalf("old incarnation was cached: %+v", event)
	default:
	}
}

type registrationContext struct {
	context.Context
	registered chan struct{}
	once       sync.Once
}

func (c *registrationContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.registered) })
	return c.Context.Done()
}

// Cancellation becomes observable while the shared key is already ready,
// before the asynchronously scheduled lifetime callback can acquire client.mu.
func TestCanceledContextCannotWinCachedAcceptance(t *testing.T) {
	f := startFeed(t, nil)
	c := testClient(t, testConfig(f))
	request := Request{Channel: Crypto, Symbols: []string{"btcusd"}}
	subscribe(t, c, t.Context(), request)
	for range 32 {
		ctx, cancel := context.WithCancel(t.Context())
		joining := &cancelOnRegistrationContext{Context: ctx, cancel: cancel}
		s, err := c.Subscribe(joining, request)
		if s != nil {
			s.Close()
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled cached acceptance returned handle=%v error=%v", s != nil, err)
		}
	}
}

type cancelOnRegistrationContext struct {
	context.Context
	cancel context.CancelFunc
}

func (c *cancelOnRegistrationContext) Done() <-chan struct{} {
	c.cancel()
	return c.Context.Done()
}

func TestEventsDuringSubscribeAcknowledgement(t *testing.T) {
	f := startFeed(t, func(p *testPeer, frame serverFrame) {
		if frame.Op == "subscribe" {
			p.write(t, priceFixture("price.crypto", "btcusd", true))
		}
		p.acknowledge(t, frame, "")
	})
	c := testClient(t, testConfig(f))
	s := subscribe(t, c, t.Context(), Request{Channel: Crypto, Symbols: []string{"btcusd"}})
	if event := receive(t, s.Events()); event.Type != SnapshotEvent {
		t.Fatalf("sent filter lost its in-flight snapshot: %+v", event)
	}
	if event := receive(t, s.Events()); event.Type != Accepted {
		t.Fatalf("acceptance missing: %+v", event)
	}
}

func TestFilteredSubscriptionIgnoresUnselectedEvents(t *testing.T) {
	f := startFeed(t, nil)
	cfg := testConfig(f)
	cfg.BufferSize = 1
	c := testClient(t, cfg)
	request := Request{Channel: Equity, Symbols: []string{"aapl"}}
	observer := subscribe(t, c, t.Context(), request)
	peer := f.next(t, "subscribe").peer
	receive(t, observer.Events())
	request.Types = []EventType{SnapshotEvent}
	filtered := subscribe(t, c, t.Context(), request)
	receive(t, filtered.Events())
	for i := range 8 {
		peer.write(
			t,
			strings.Replace(
				priceFixture("price.equity", "aapl", false),
				`"seq":2`,
				fmt.Sprintf(`"seq":%d`, i+2),
				1,
			),
		)
		receive(t, observer.Events())
	}
	if err := filtered.Err(); err != nil {
		t.Fatalf("unselected updates terminated filtered subscription: %v", err)
	}
	peer.write(t, priceFixture("price.equity", "aapl", true))
	if event := receive(t, filtered.Events()); event.Type != SnapshotEvent {
		t.Fatalf("selected snapshot not delivered: %+v", event)
	}
}

func TestPayloadValidationIsScopedToEventKind(t *testing.T) {
	for _, row := range []struct {
		name, channel string
		snapshot      bool
		extra         string
	}{
		{"snapshot ignores update fields", "price.crypto", true, `"timestamp":false,"value":{},"received_at":"future","is_carried_forward":[],`},
		{"spot ignores TWAP window", "price.equity", false, `"window_seconds":"future",`},
		{"update ignores snapshot data", "price.crypto", false, `"data":{},`},
		{"TWAP ignores spot receipt metadata", "price.crypto.twap", false, `"received_at":"future","is_carried_forward":[],`},
	} {
		t.Run(row.name, func(t *testing.T) {
			raw := priceFixture(row.channel, "btcusd", row.snapshot)
			// Remove the fixture's optional fields before adding unrelated ones.
			raw = strings.ReplaceAll(
				raw,
				`"received_at":123457,"is_carried_forward":false`,
				`"ignored":true`,
			)
			if row.channel == "price.equity" {
				raw = strings.ReplaceAll(raw, `"window_seconds":60,`, "")
			}
			raw = strings.Replace(raw, `"payload":{`, `"payload":{`+row.extra, 1)
			if _, ok := parseEvent([]byte(raw)); !ok {
				t.Fatal("unrelated extension field invalidated a valid payload")
			}
		})
	}
}
