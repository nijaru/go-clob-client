package perps

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// The fixture leaves acknowledgements under test control so cancellation,
// updates and replies can be interleaved at the actual WebSocket boundary.
type marketCommand struct {
	fixtureCommand
	conn *websocket.Conn
}

type marketFixture struct {
	t        *testing.T
	ctx      context.Context
	commands chan marketCommand
	active   atomic.Int32
	pool     *MarketStream
}

func newMarketFixture(t *testing.T) *marketFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	f := &marketFixture{t: t, ctx: ctx, commands: make(chan marketCommand, 32)}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		f.active.Add(1)
		defer f.active.Add(-1)
		defer conn.CloseNow()
		for {
			_, payload, err := conn.Read(ctx)
			if err != nil {
				return
			}
			var command fixtureCommand
			if err := json.Unmarshal(payload, &command); err != nil {
				t.Error(err)
				return
			}
			if command.Op.Type != "" {
				t.Errorf("unexpected public authentication/command: %+v", command)
			}
			select {
			case f.commands <- marketCommand{command, conn}:
			case <-ctx.Done():
				return
			}
		}
	}))
	f.pool = New(
		Config{WebSocketHost: "ws" + strings.TrimPrefix(server.URL, "http")},
	).NewMarketStream(ctx)
	t.Cleanup(func() { _ = f.pool.Close(); cancel(); server.Close() })
	return f
}

func (f *marketFixture) command(req string, channels ...string) marketCommand {
	f.t.Helper()
	select {
	case cmd := <-f.commands:
		if cmd.Request != req || !reflect.DeepEqual(cmd.Channels, channels) {
			f.t.Fatalf("command = %s %v, want %s %v", cmd.Request, cmd.Channels, req, channels)
		}
		return cmd
	case <-f.ctx.Done():
		f.t.Fatal(f.ctx.Err())
		return marketCommand{}
	}
}

func (f *marketFixture) write(conn *websocket.Conn, payload string) {
	f.t.Helper()
	if err := conn.Write(f.ctx, websocket.MessageText, []byte(payload)); err != nil {
		f.t.Fatal(err)
	}
}

func (f *marketFixture) ack(cmd marketCommand) {
	f.t.Helper()
	f.write(cmd.conn, fmt.Sprintf(`{"id":%d,"data":{"status":"ok"}}`, cmd.ID))
}

type marketResult struct {
	handle *MarketHandle
	err    error
}

func (f *marketFixture) subscribe(
	ctx context.Context,
	specs ...MarketSubscription,
) <-chan marketResult {
	result := make(chan marketResult, 1)
	go func() { h, err := f.pool.Subscribe(ctx, specs); result <- marketResult{h, err} }()
	return result
}

func (f *marketFixture) result(result <-chan marketResult) marketResult {
	f.t.Helper()
	select {
	case r := <-result:
		return r
	case <-f.ctx.Done():
		f.t.Fatal(f.ctx.Err())
		return marketResult{}
	}
}

func (f *marketFixture) handle(result <-chan marketResult) *MarketHandle {
	f.t.Helper()
	r := f.result(result)
	if r.err != nil {
		f.t.Fatal(r.err)
	}
	return r.handle
}

func (f *marketFixture) event(h *MarketHandle) PerpsSessionEvent {
	f.t.Helper()
	select {
	case e, ok := <-h.Events():
		if !ok {
			f.t.Fatal("handle closed early")
		}
		return e
	case <-f.ctx.Done():
		f.t.Fatal(f.ctx.Err())
		return PerpsSessionEvent{}
	}
}

func TestMarketPoolRefcountsFiltersAndInterleavedAcknowledgements(t *testing.T) {
	f := newMarketFixture(t)
	id := 1
	spec := MarketSubscription{Topic: MarketTickers, InstrumentID: &id}
	ctx1, cancel1 := context.WithCancel(f.ctx)
	first := f.subscribe(ctx1, spec, spec)
	cmd := f.command("sub", "tickers::1")
	// Before-ack update, heartbeat, ack and after-ack update in the same batch.
	f.write(
		cmd.conn,
		fmt.Sprintf(
			`[{"ch":"tickers::1","sq":1,"data":{"iid":1,"mark":"10"}},{"id":0,"data":{"status":"ok"}},{"id":%d,"data":[{"status":"ok"}]},{"ch":"tickers::1","sq":2,"data":{"iid":1,"mark":"11"}}]`,
			cmd.ID,
		),
	)
	h1 := f.handle(first)
	if f.event(h1).Market.Ticker.MarkPrice != "10" || f.event(h1).Market.Ticker.MarkPrice != "11" {
		t.Fatal("interleaved updates lost")
	}
	h2 := f.handle(f.subscribe(f.ctx, spec)) // no duplicate wire subscription
	cancel1()
	_ = h1.Close()
	if _, ok := <-h1.Events(); ok {
		t.Fatal("canceled queue still open")
	}
	all := f.subscribe(f.ctx, MarketSubscription{Topic: MarketTickers})
	addAll := f.command("sub", "tickers::all")
	if addAll.conn != cmd.conn {
		t.Fatal("handles did not share a socket")
	}
	f.ack(addAll)
	f.ack(f.command("unsub", "tickers::1"))
	ha := f.handle(all)
	f.write(
		cmd.conn,
		`[{"ch":"tickers::all","sq":1,"data":{"iid":2,"mark":"20"}},{"ch":"tickers::all","sq":2,"data":{"iid":1,"mark":"12"}}]`,
	)
	if f.event(ha).Market.InstrumentID != 2 || f.event(ha).Market.InstrumentID != 1 {
		t.Fatal("all-topic filter lost events")
	}
	if f.event(h2).Market.InstrumentID != 1 {
		t.Fatal("instrument filter received another instrument")
	}
	_ = ha.Close()
	f.ack(f.command("sub", "tickers::1")) // restore iid before removing all
	f.ack(f.command("unsub", "tickers::all"))
	// Closing the last ref closes the socket. Reusing the explicit pool dials anew.
	_ = h2.Close()
	last := f.subscribe(f.ctx, MarketSubscription{Topic: MarketBBO, InstrumentID: &id})
	newCmd := f.command("sub", "bbo::1")
	if newCmd.conn == cmd.conn {
		t.Fatal("idle socket retained")
	}
	f.ack(newCmd)
	_ = f.handle(last).Close()
	_ = f.pool.Close()
	if f.active.Load() != 0 {
		// The local reader is joined; allow the remote handler to observe EOF.
		for f.active.Load() != 0 {
			select {
			case <-f.ctx.Done():
				t.Fatal("remote readers remained")
			case <-time.After(time.Millisecond):
			}
		}
	}
}

func TestMarketPoolCancellationDuringSubscribeResetsUncertainSocket(t *testing.T) {
	f := newMarketFixture(t)
	id := 1
	book := MarketSubscription{Topic: MarketBook, InstrumentID: &id}
	first := f.subscribe(f.ctx, book)
	original := f.command("sub", "book::1")
	f.ack(original)
	h := f.handle(first)
	ctx, cancel := context.WithCancel(f.ctx)
	joining := f.subscribe(ctx, MarketSubscription{Topic: MarketBBO, InstrumentID: &id})
	uncertain := f.command("sub", "bbo::1")
	cancel() // server may have applied this frame; no ack arrives
	if r := f.result(joining); !errors.Is(r.err, context.Canceled) || r.handle != nil {
		t.Fatalf("result=%+v", r)
	}
	resub := f.command("sub", "book::1")
	if resub.conn == uncertain.conn {
		t.Fatal("uncertain subscription kept its socket")
	}
	f.write(
		resub.conn,
		fmt.Sprintf(
			`[{"ch":"book::1","sq":99,"data":{"b":[["10","1"]],"a":[]}},{"id":%d,"data":{"status":"ok"}}]`,
			resub.ID,
		),
	)
	if f.event(h).Resync.Reason != PerpsResyncReconnect {
		t.Fatal("missing reconnect recovery event")
	}
	if e := f.event(h); e.Resync != nil || e.Market.Book.Sequence != 99 {
		t.Fatalf("stale sequences after reconnect: %+v", e)
	}
}

func TestMarketPoolRejectedUnsubscribeAndSubscribeCleanUp(t *testing.T) {
	f := newMarketFixture(t)
	id := 1
	initial := f.subscribe(f.ctx, MarketSubscription{Topic: MarketBook, InstrumentID: &id})
	cmd := f.command("sub", "book::1")
	f.ack(cmd)
	book := f.handle(initial)
	joining := f.subscribe(f.ctx, MarketSubscription{Topic: MarketBBO, InstrumentID: &id})
	f.ack(f.command("sub", "bbo::1"))
	bbo := f.handle(joining)
	_ = bbo.Close()
	unsub := f.command("unsub", "bbo::1")
	f.write(
		unsub.conn,
		fmt.Sprintf(`{"id":%d,"data":{"status":"err","error":"rejected"}}`, unsub.ID),
	)
	resub := f.command("sub", "book::1")
	if resub.conn == unsub.conn {
		t.Fatal("rejected unsubscribe leaked a filter")
	}
	f.ack(resub)
	if f.event(book).Resync == nil {
		t.Fatal("missing resync")
	}
	bad := f.subscribe(f.ctx, MarketSubscription{Topic: MarketBBO, InstrumentID: &id})
	rejected := f.command("sub", "bbo::1")
	f.write(rejected.conn, fmt.Sprintf(`{"id":%d,"data":[]}`, rejected.ID))
	if r := f.result(bad); r.err == nil || r.handle != nil {
		t.Fatalf("invalid acknowledgement accepted: %+v", r)
	}
	clean := f.command("sub", "book::1")
	if clean.conn == rejected.conn {
		t.Fatal("uncertain rejected subscription kept socket")
	}
	f.ack(clean)
}

func TestMarketPoolSlowHandleDoesNotBlockPeerOrAcknowledgements(t *testing.T) {
	f := newMarketFixture(t)
	id := 1
	result := f.subscribe(f.ctx, MarketSubscription{Topic: MarketBBO, InstrumentID: &id})
	cmd := f.command("sub", "bbo::1")
	f.ack(cmd)
	slow := f.handle(result)
	peer := f.handle(f.subscribe(f.ctx, MarketSubscription{Topic: MarketBBO, InstrumentID: &id}))
	// Reading the peer for every write makes the overflow specific to slow.
	for i := 1; i <= 129; i++ {
		f.write(cmd.conn, fmt.Sprintf(`{"ch":"bbo::1","sq":%d,"data":{"iid":1,"bp":"10"}}`, i))
		if e := f.event(peer); e.Market == nil {
			t.Fatal("peer lost update")
		}
	}
	select {
	case err := <-slow.Errors():
		if !errors.Is(err, ErrPerpsSlowConsumer) {
			t.Fatal(err)
		}
	case <-f.ctx.Done():
		t.Fatal(f.ctx.Err())
	}
	_ = slow.Close()
	// A new topic still acknowledges on the same socket after the overflow.
	joining := f.subscribe(f.ctx, MarketSubscription{Topic: MarketBook, InstrumentID: &id})
	add := f.command("sub", "book::1")
	if add.conn != cmd.conn {
		t.Fatal("slow handle disconnected its healthy peer")
	}
	f.ack(add)
	_ = f.handle(joining).Close()
	f.ack(f.command("unsub", "book::1"))
	f.write(cmd.conn, `{"ch":"bbo::1","sq":130,"data":{"iid":1,"bp":"11"}}`)
	if f.event(peer).Market.BBO.BidPrice != "11" {
		t.Fatal("peer stopped after slow handle closed")
	}
}

func TestMarketPoolCancellationKeepsSharedInflightCoverage(t *testing.T) {
	f := newMarketFixture(t)
	id := 1
	bookSpec := MarketSubscription{Topic: MarketBook, InstrumentID: &id}
	ctx, cancel := context.WithCancel(f.ctx)
	first := f.subscribe(ctx, bookSpec)
	cmd := f.command("sub", "book::1")
	f.ack(cmd)
	one := f.handle(first)
	two := f.handle(f.subscribe(f.ctx, bookSpec))
	joining := f.subscribe(f.ctx, MarketSubscription{Topic: MarketBBO, InstrumentID: &id})
	pending := f.command("sub", "bbo::1")
	cancel()
	_ = one.Close() // this ref removal does not invalidate the pending BBO change
	f.ack(pending)
	bbo := f.handle(joining)
	f.write(cmd.conn, `{"ch":"book::1","sq":1,"data":{"b":[],"a":[]}}`)
	if e := f.event(two); e.Resync != nil || e.Market.Book == nil {
		t.Fatalf("shared ref cancellation disrupted surviving handle: %+v", e)
	}
	_ = bbo.Close()
	unsub := f.command("unsub", "bbo::1")
	// A new identical subscription must not be considered ready using wire
	// coverage that is already being removed. Ack removal, then re-add it.
	rejoining := f.subscribe(f.ctx, MarketSubscription{Topic: MarketBBO, InstrumentID: &id})
	f.ack(unsub)
	readded := f.command("sub", "bbo::1")
	if readded.conn != cmd.conn {
		t.Fatal("unrelated subscription forced reconnect")
	}
	select {
	case premature := <-rejoining:
		t.Fatalf("Subscribe returned before replacement acknowledgement: %+v", premature)
	case <-time.After(20 * time.Millisecond):
	}
	f.ack(readded)
	newBBO := f.handle(rejoining)
	f.write(cmd.conn, `{"ch":"bbo::1","sq":1,"data":{"iid":1,"bp":"10"}}`)
	if f.event(newBBO).Market.BBO.BidPrice != "10" {
		t.Fatal("re-added subscription did not receive updates")
	}
}

// Keep the actual WebSocket handshake/reader, but stall the network write
// boundary deterministically instead of depending on OS socket-buffer sizes.
type stalledMarketConn struct {
	net.Conn
	entered   chan struct{}
	closed    chan struct{}
	enterOnce sync.Once
	closeOnce sync.Once
}

func (c *stalledMarketConn) Write(p []byte) (int, error) {
	if bytes.HasPrefix(p, []byte("GET ")) {
		return c.Conn.Write(p)
	}
	c.enterOnce.Do(func() { close(c.entered) })
	<-c.closed
	return 0, net.ErrClosed
}

func (c *stalledMarketConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return c.Conn.Close()
}

func TestMarketPoolCancellationInterruptsStalledWrite(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		_, _, _ = conn.Read(ctx)
	}))
	defer server.Close()
	entered := make(chan struct{})
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
			if err != nil {
				return nil, err
			}
			return &stalledMarketConn{
				Conn:    conn,
				entered: entered,
				closed:  make(chan struct{}),
			}, nil
		},
	}
	defer transport.CloseIdleConnections()
	pool := New(
		Config{
			WebSocketHost: "ws" + strings.TrimPrefix(server.URL, "http"),
			HTTPClient:    &http.Client{Transport: transport},
		},
	).NewMarketStream(ctx)
	defer pool.Close()
	handleCtx, stopHandle := context.WithCancel(ctx)
	defer stopHandle()
	f := &marketFixture{t: t, ctx: ctx, pool: pool}
	id := 1
	result := f.subscribe(handleCtx, MarketSubscription{Topic: MarketBook, InstrumentID: &id})
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	stopHandle()
	if r := f.result(result); !errors.Is(r.err, context.Canceled) || r.handle != nil {
		t.Fatalf("stalled write blocked cancellation: %+v", r)
	}
	// Cancellation must join the write/read workers without waiting for the
	// 15-second operation deadline, and repeated Close remains safe.
	_ = pool.Close()
	_ = pool.Close()
}

func TestMarketPoolCloseInterruptsPendingHandshake(t *testing.T) {
	f := newMarketFixture(t)
	id := 1
	joining := f.subscribe(f.ctx, MarketSubscription{Topic: MarketBook, InstrumentID: &id})
	f.command("sub", "book::1")
	if err := f.pool.Close(); err != nil {
		t.Fatal(err)
	}
	if r := f.result(joining); r.err == nil || r.handle != nil {
		t.Fatalf("pending Subscribe survived Close: %+v", r)
	}
	if r := f.result(f.subscribe(f.ctx, MarketSubscription{Topic: MarketBook, InstrumentID: &id})); !errors.Is(
		r.err,
		errMarketStreamClosed,
	) {
		t.Fatal(r.err)
	}
}
