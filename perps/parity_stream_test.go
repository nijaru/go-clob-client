package perps

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// Public channel names and compact payloads come from subscriptions/perps.ts
// and websockets/perps/subscription.ts. This exercises real socket handshakes,
// automatic resubscription, typed decoding, and context-owned shutdown.
func TestPublicMarketStreamReconnectsAndDecodes(t *testing.T) {
	var connections atomic.Int32
	subscriptions := make(chan []string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		attempt := connections.Add(1)
		_, payload, err := conn.Read(r.Context())
		if err != nil {
			return
		}
		var command fixtureCommand
		if err := json.Unmarshal(payload, &command); err != nil {
			t.Error(err)
			return
		}
		if command.Request != "sub" || command.Op.Type != "" {
			t.Errorf("public stream authenticated: %+v", command)
		}
		subscriptions <- command.Channels
		fixtureReply(t, r.Context(), conn, command.ID, `{"status":"ok"}`)
		if attempt == 1 {
			payload := `[
    {"ch":"trades::1","ts":1000,"sq":1,"data":[{"tid":1,"iid":1,"side":"long","settlement":true,"p":"100","qty":"1","ts":1000,"hash":"0xtrade"}]},
    {"ch":"bbo::1","ts":1000,"sq":1,"data":{"iid":1,"bp":"99","bq":"1","ap":"101","aq":"2"}},
    {"ch":"book::1","ts":1000,"sq":1,"data":{"b":[["99","1"]],"a":[["101","2"]]}},
    {"ch":"tickers::all","ts":1000,"sq":1,"data":{"iid":1,"idx":"100","mark":"100","last":"100","mid":"100","oi":"1000","fr":"0.01","nxf":2000}},
    {"ch":"statistics::all","ts":1000,"sq":1,"data":{"iid":1,"vol":"10","open":"99","klines":[]}},
    {"ch":"klines::1::1m","ts":1000,"sq":1,"data":[[1000,"99","101","98","100","10",2]]}
   ]`
			if err := conn.Write(r.Context(), websocket.MessageText, []byte(payload)); err != nil {
				t.Error(err)
			}
			return
		}
		_ = conn.Write(
			r.Context(),
			websocket.MessageText,
			[]byte(`{"ch":"book::1","ts":2000,"sq":10,"data":{"b":[["100","1"]],"a":[]}}`),
		)
		for {
			if _, _, err := conn.Read(r.Context()); err != nil {
				return
			}
		}
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	id := 1
	specs := []MarketSubscription{
		{Topic: MarketTrades, InstrumentID: &id},
		{Topic: MarketBBO, InstrumentID: &id},
		{Topic: MarketBook, InstrumentID: &id},
		{Topic: MarketTickers, InstrumentID: &id},
		{Topic: MarketTickers},
		{Topic: MarketStatistics},
		{Topic: MarketCandles, InstrumentID: &id, Interval: PerpsKline1m},
	}
	pool := New(
		Config{WebSocketHost: "ws" + strings.TrimPrefix(server.URL, "http")},
	).NewMarketStream(ctx)
	t.Cleanup(func() { _ = pool.Close() })
	stream, err := pool.Subscribe(ctx, specs)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stream.Close() })
	first := <-subscriptions
	second := []string(nil)
	topics := map[MarketTopic]bool{}
	resync := false
	reconnectedBook := false
	for !reconnectedBook {
		select {
		case event, ok := <-stream.Events():
			if !ok {
				t.Fatal("stream closed early")
			}
			if event.Resync != nil {
				if event.Resync.Reason == PerpsResyncReconnect {
					resync = true
				}
				continue
			}
			market := event.Market
			if market == nil || market.InstrumentID != 1 {
				t.Fatalf("market event=%+v", event)
			}
			topics[market.Topic] = true
			switch market.Topic {
			case MarketTrades:
				if len(market.Trades) != 1 || !market.Trades[0].Settlement {
					t.Fatal("settlement trade lost")
				}
			case MarketBBO:
				if market.BBO.AskQuantity != "2" {
					t.Fatal("BBO tuple lost")
				}
			case MarketBook:
				if market.Book.Bids[0].Price == "100" {
					reconnectedBook = true
				}
			case MarketTickers:
				if market.Ticker.NextFunding != 2000 {
					t.Fatal("ticker funding deadline lost")
				}
			case MarketStatistics:
				if market.Statistic.OpenPrice != "99" {
					t.Fatal("statistics open price lost")
				}
			case MarketCandles:
				if market.Interval != PerpsKline1m || market.Candles[0].Trades != 2 {
					t.Fatal("candle tuple lost")
				}
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	select {
	case second = <-subscriptions:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if !resync || len(topics) != 6 || len(first) != 6 || !reflect.DeepEqual(first, second) {
		t.Fatalf("topics=%v resync=%t subscriptions=%v/%v", topics, resync, first, second)
	}
	cancel()
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSlowConsumerClosesInsteadOfDroppingUpdates(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	s := &Session{
		ctx:    ctx,
		cancel: cancel,
		events: make(chan PerpsSessionEvent, 1),
		errors: make(chan error, 1),
	}
	s.emitEvent(PerpsSessionEvent{Channel: "balances"})
	s.emitEvent(PerpsSessionEvent{Channel: "orders"})
	if !errors.Is(<-s.errors, ErrPerpsSlowConsumer) || ctx.Err() == nil {
		t.Fatal("overflow silently dropped an update")
	}
}

func TestLiveFillFeesAndSparseBuilderReceipts(t *testing.T) {
	event := PerpsSessionEvent{
		Channel: "fills",
		Data: json.RawMessage(
			`[{"tid":1,"oid":2,"iid":1,"side":"long","settlement":true,"p":"100","qty":"1","taker":false,"fee":"-0.002","builder_fee":"0.0005","fea":"USDC","psz":"0","pep":"0","pnl":"0","liq":false,"ts":1000,"coid":"` + fixtureClientID + `"}]`,
		),
	}
	fills, err := event.AsFills()
	if err != nil || len(fills) != 1 || fills[0].TotalFee != "-0.0015" ||
		fills[0].ClientOrderID != fixtureClientID ||
		!fills[0].Settlement {
		t.Fatalf("fills=%+v error=%v", fills, err)
	}
	s := &Session{
		ctx:    t.Context(),
		events: make(chan PerpsSessionEvent, 2),
		errors: make(chan error, 1),
	}
	s.handlePayload([]byte(`{"ch":"builderFills","ts":1,"sq":7,"data":[]}`))
	s.handlePayload([]byte(`{"ch":"builderFills","ts":2,"sq":10,"data":[]}`))
	first, second := <-s.events, <-s.events
	if first.Resync != nil || second.Resync != nil {
		t.Fatal("sparse engine sequences were treated as builder receipt gaps")
	}
}
