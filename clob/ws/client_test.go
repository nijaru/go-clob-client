package ws

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	json "github.com/go-json-experiment/json"

	"github.com/coder/websocket"
	"github.com/nijaru/go-clob-client/clob"
	"github.com/nijaru/go-clob-client/internal/polyauth"
)

func TestHandleMessageBookEvent(t *testing.T) {
	t.Parallel()

	c := NewClient("")

	data, _ := json.Marshal([]BookEvent{{
		BaseEvent: BaseEvent{EventType: EventTypeBook},
		Market:    "market-1",
		AssetID:   "asset-1",
		Bids:      []clob.OrderSummary{{Price: "0.45", Size: "10"}},
		Asks:      []clob.OrderSummary{{Price: "0.55", Size: "12"}},
		Timestamp: "1710000000",
		Hash:      "hash-1",
	}})

	c.handleMessage(t.Context(), data)

	select {
	case ev := <-c.Events():
		book, ok := ev.(*BookEvent)
		if !ok {
			t.Fatalf("expected *BookEvent, got %T", ev)
		}
		if book.AssetID != "asset-1" {
			t.Errorf("asset id = %q, want %q", book.AssetID, "asset-1")
		}
		if len(book.Bids) != 1 || book.Bids[0].Price != "0.45" {
			t.Errorf("unexpected bids: %+v", book.Bids)
		}
		if len(book.Asks) != 1 || book.Asks[0].Price != "0.55" {
			t.Errorf("unexpected asks: %+v", book.Asks)
		}
		if book.Market != "market-1" || book.Hash != "hash-1" {
			t.Errorf("metadata = %q/%q", book.Market, book.Hash)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestHandleMessageBookEventSingleObjectCompatibility(t *testing.T) {
	t.Parallel()

	c := NewClient("")
	data, _ := json.Marshal(BookEvent{
		BaseEvent: BaseEvent{EventType: EventTypeBook},
		AssetID:   "asset-legacy",
		Bids:      []clob.OrderSummary{{Price: "0.45", Size: "10"}},
		Asks:      []clob.OrderSummary{{Price: "0.55", Size: "12"}},
		Timestamp: "1710000000",
	})
	c.handleMessage(t.Context(), data)
	select {
	case ev := <-c.Events():
		book, ok := ev.(*BookEvent)
		if !ok || book.AssetID != "asset-legacy" {
			t.Fatalf("event = %#v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestHandleMessagePriceChangeEvent(t *testing.T) {
	t.Parallel()

	c := NewClient("")

	data, _ := json.Marshal(PriceChangeEvent{
		BaseEvent: BaseEvent{EventType: EventTypePriceChange},
		Market:    "market-2",
		Timestamp: "1710000000123",
		PriceChanges: []PriceChange{{
			AssetID: "asset-2",
			Price:   "0.42",
			Size:    "5.5",
			Side:    clob.SideBuy,
			Hash:    "hash-2",
			BestBid: "0.41",
			BestAsk: "0.43",
		}},
	})

	c.handleMessage(t.Context(), data)

	select {
	case ev := <-c.Events():
		pe, ok := ev.(*PriceChangeEvent)
		if !ok {
			t.Fatalf("expected *PriceChangeEvent, got %T", ev)
		}
		if len(pe.PriceChanges) != 1 || pe.PriceChanges[0].Price != "0.42" {
			t.Errorf("price changes = %+v", pe.PriceChanges)
		}
		if pe.PriceChanges[0].Side != clob.SideBuy || pe.PriceChanges[0].Hash != "hash-2" {
			t.Errorf("change metadata = %+v", pe.PriceChanges[0])
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestHandleMessageLiveWireBatchFixtures(t *testing.T) {
	t.Parallel()

	c := NewClient("")
	c.handleMessage(
		t.Context(),
		[]byte(
			`[{"event_type":"book","market":"m1","asset_id":"a1","bids":[{"price":"0.4","size":"2"}],"asks":[{"price":"0.6","size":"3"}],"timestamp":"1710000000000"},{"event_type":"book","market":"m1","asset_id":"a2","bids":[],"asks":[],"timestamp":"1710000000000"}]`,
		),
	)
	c.handleMessage(
		t.Context(),
		[]byte(
			`{"event_type":"price_change","market":"m1","price_changes":[{"asset_id":"a1","price":"0.41","size":"1","side":"BUY","hash":"h1","best_bid":"0.41","best_ask":"0.6"}],"timestamp":"1710000000123"}`,
		),
	)

	for i := 0; i < 3; i++ {
		select {
		case <-c.Events():
		case err := <-c.Errors():
			t.Fatalf("unexpected fixture decode error: %v", err)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for fixture event")
		}
	}
}

func TestHandleMessageMixedBatchDispatchesPerEvent(t *testing.T) {
	t.Parallel()

	c := NewClient("")
	c.handleMessage(t.Context(), []byte(`[{
		"event_type":"book",
		"market":"m1",
		"asset_id":"a1",
		"bids":[],"asks":[],"timestamp":"1"
	},{
		"event_type":"price_change",
		"market":"m1",
		"price_changes":[{"asset_id":"a1","price":"0.41","size":"1","side":"BUY"}],
		"timestamp":"2"
	}]`))

	var got []Event
	for range 2 {
		select {
		case event := <-c.Events():
			got = append(got, event)
		case err := <-c.Errors():
			t.Fatalf("unexpected mixed batch error: %v", err)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for mixed batch event")
		}
	}
	if _, ok := got[0].(*BookEvent); !ok {
		t.Fatalf("first event = %T, want *BookEvent", got[0])
	}
	if _, ok := got[1].(*PriceChangeEvent); !ok {
		t.Fatalf("second event = %T, want *PriceChangeEvent", got[1])
	}
}

func TestHandleMessageFullUserEventFields(t *testing.T) {
	t.Parallel()

	c := NewClient("")
	c.handleMessage(t.Context(), []byte(`{
		"event_type":"order","id":"ord-1","owner":"owner-1","market":"m1","asset_id":"a1",
		"side":"BUY","order_owner":"order-owner","original_size":"10","size_matched":"2",
		"associate_trades":["trade-1"],"outcome":"Yes","type":"UPDATE","status":"LIVE",
		"maker_address":"maker-1","timestamp":"3"
	}`))
	c.handleMessage(t.Context(), []byte(`{
		"event_type":"trade","type":"TRADE","id":"trade-1","taker_order_id":"ord-1",
		"market":"m1","asset_id":"a1","side":"BUY","size":"2","fee_rate_bps":"5",
		"price":"0.5","status":"TRADE_STATUS_MATCHED_NOT_BROADCASTED","owner":"owner-1",
		"trade_owner":"trade-owner","transaction_hash":"0xhash","trader_side":"TAKER",
		"maker_orders":[{"order_id":"maker-order","owner":"maker-owner","matched_amount":"2","price":"0.5","asset_id":"a1","side":"SELL"}],
		"timestamp":"4"
	}`))

	select {
	case event := <-c.Events():
		order, ok := event.(*OrderEvent)
		if !ok || order.OrderID != "ord-1" || order.SizeMatched != "2" ||
			order.OrderEventType != UserOrderEventTypeUpdate {
			t.Fatalf("unexpected order event: %#v", event)
		}
	case err := <-c.Errors():
		t.Fatalf("order decode: %v", err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for order event")
	}

	select {
	case event := <-c.Events():
		trade, ok := event.(*TradeEvent)
		if !ok || trade.TradeID != "trade-1" || trade.Status != TradeStatusMatchedNotBroadcasted ||
			len(trade.MakerOrders) != 1 {
			t.Fatalf("unexpected trade event: %#v", event)
		}
	case err := <-c.Errors():
		t.Fatalf("trade decode: %v", err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for trade event")
	}
}

func TestMarketEventAssetIDAlias(t *testing.T) {
	var event NewMarketEvent
	if err := json.Unmarshal([]byte(`{"event_type":"new_market","asset_ids":["a1"]}`), &event); err != nil {
		t.Fatalf("unmarshal new market event: %v", err)
	}
	if len(event.AssetIDs) != 1 || event.AssetIDs[0] != "a1" {
		t.Fatalf("asset ids = %v", event.AssetIDs)
	}
}

func TestHandleMessageAllEventTypes(t *testing.T) {
	t.Parallel()

	events := []struct {
		name  string
		event any
	}{
		{"tick_size_change", TickSizeChangeEvent{
			BaseEvent:   BaseEvent{EventType: EventTypeTickSizeChange},
			AssetID:     "a1",
			Market:      "m1",
			OldTickSize: "0.01",
			NewTickSize: "0.001",
			Timestamp:   "1",
		}},
		{"last_trade_price", LastTradePriceEvent{
			BaseEvent: BaseEvent{EventType: EventTypeLastTradePrice},
			AssetID:   "a1",
			Market:    "m1",
			Price:     "0.50",
			Size:      "10",
			Side:      clob.SideSell,
			Timestamp: "2",
		}},
		{"order", OrderEvent{
			BaseEvent: BaseEvent{EventType: EventTypeOrder},
			OrderID:   "ord-1",
			AssetID:   "a1",
			Market:    "m1",
			Price:     "0.60",
			Size:      "100",
			Side:      clob.SideBuy,
			Status:    OrderStatusOpen,
			Timestamp: "3",
		}},
		{"trade", TradeEvent{
			BaseEvent: BaseEvent{EventType: EventTypeTrade},
			TradeID:   "trd-1",
			AssetID:   "a1",
			Market:    "m1",
			Price:     "0.55",
			Size:      "25",
			Side:      clob.SideSell,
			Status:    TradeStatusMatched,
			Timestamp: "4",
		}},
	}

	for _, tt := range events {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient("")
			data, _ := json.Marshal(tt.event)
			c.handleMessage(t.Context(), data)

			select {
			case <-c.Events():
				// Successfully decoded
			case err := <-c.Errors():
				t.Fatalf("unexpected error: %v", err)
			case <-time.After(time.Second):
				t.Fatal("timed out")
			}
		})
	}
}

func TestHandleMessageUnknownEventType(t *testing.T) {
	t.Parallel()

	c := NewClient("")
	c.handleMessage(t.Context(), []byte(`{"event_type":"future_event","data":"x"}`))

	select {
	case err := <-c.Errors():
		if !strings.Contains(err.Error(), "unknown event type") {
			t.Errorf("expected unknown event type error, got: %v", err)
		}
	case <-c.Events():
		t.Fatal("should not receive event for unknown type")
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for error")
	}
}

func TestHandleMessageMalformedEventJSON(t *testing.T) {
	t.Parallel()

	c := NewClient("")
	// Valid event_type but invalid inner fields
	c.handleMessage(t.Context(), []byte(`{"event_type":"book","bids":"not-an-array"}`))

	select {
	case err := <-c.Errors():
		if !strings.Contains(err.Error(), "decode event book") {
			t.Errorf("expected decode error, got: %v", err)
		}
	case <-c.Events():
		t.Fatal("should not receive event for malformed data")
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for error")
	}
}

func TestHandleMessageNonJSON(t *testing.T) {
	t.Parallel()

	c := NewClient("")
	// Text like "PONG" should not produce errors or events
	c.handleMessage(t.Context(), []byte("PONG"))

	select {
	case err := <-c.Errors():
		t.Fatalf("unexpected error: %v", err)
	case <-c.Events():
		t.Fatal("unexpected event from non-JSON")
	case <-time.After(50 * time.Millisecond):
		// Expected: no error, no event
	}
}

func TestConnectAndClose(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Logf("accept: %v", err)
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")

		// Keep connection open until client closes
		for {
			_, _, err := c.Read(r.Context())
			if err != nil {
				return
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(wsURL)

	if err := client.Connect(t.Context()); err != nil {
		t.Fatalf("connect: %v", err)
	}

	// Close should not block and should not leak goroutines
	if err := client.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Second close is safe
	client.Close()
}

func TestCloseWithoutConnect(t *testing.T) {
	t.Parallel()

	client := NewClient("ws://localhost:0")
	if err := client.Close(); err != nil {
		t.Fatalf("close without connect: %v", err)
	}
}

func TestSendJSONNotConnected(t *testing.T) {
	t.Parallel()

	client := NewClient("")
	err := client.SubscribeOrderBook(t.Context(), []string{"asset-1"})
	if err == nil {
		t.Fatal("expected error when not connected")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHeartbeatPingPong(t *testing.T) {
	t.Parallel()

	pingReceived := make(chan struct{}, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Logf("accept: %v", err)
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")

		for {
			_, data, err := c.Read(r.Context())
			if err != nil {
				return
			}
			if string(data) == "PING" {
				select {
				case pingReceived <- struct{}{}:
				default:
				}
				_ = c.Write(r.Context(), websocket.MessageText, []byte("PONG"))
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(wsURL)
	client.Connect(t.Context())
	defer client.Close()

	select {
	case <-pingReceived:
		// Good — heartbeat reached the server
	case <-time.After(2 * client.heartbeatInterval):
		t.Fatal("timed out waiting for PING")
	}
}

func TestHeartbeatTimeoutTriggersReconnect(t *testing.T) {
	t.Parallel()

	reconnected := make(chan struct{}, 1)
	var connCount int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connCount++
		current := connCount

		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Logf("accept: %v", err)
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")

		for {
			_, data, err := c.Read(r.Context())
			if err != nil {
				return
			}

			if string(data) != "PING" {
				continue
			}

			if current == 1 {
				// Intentionally ignore the first connection's heartbeat so the client
				// declares it stale and reconnects.
				continue
			}

			select {
			case reconnected <- struct{}{}:
			default:
			}
			_ = c.Write(r.Context(), websocket.MessageText, []byte("PONG"))
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(wsURL)
	client.heartbeatInterval = 25 * time.Millisecond
	client.heartbeatTimeout = 25 * time.Millisecond

	if err := client.Connect(t.Context()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	select {
	case <-reconnected:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for reconnect, connCount=%d", connCount)
	}

	if connCount < 2 {
		t.Fatalf("expected reconnect after heartbeat timeout, connCount=%d", connCount)
	}
}

func TestUserSubscriptionsRefcountAndScopedUnsubscribe(t *testing.T) {
	t.Parallel()

	server, messages := newSubscriptionTestServer(t)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewAuthenticatedClient(wsURL, clob.Credentials{
		Key:        "key",
		Secret:     "secret",
		Passphrase: "pass",
	})
	if err := client.Connect(t.Context()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	if err := client.SubscribeOrders(t.Context(), []string{"market-1"}); err != nil {
		t.Fatalf("subscribe orders: %v", err)
	}
	msg := mustReceiveSubscriptionMessage(t, messages)
	if got := msg["type"]; got != "user" {
		t.Fatalf("type = %v, want user", got)
	}
	if got := messageStrings(msg["markets"]); len(got) != 1 || got[0] != "market-1" {
		t.Fatalf("markets = %#v, want [market-1]", got)
	}
	auth, ok := msg["auth"].(map[string]any)
	if !ok {
		t.Fatalf("auth = %#v, want object", msg["auth"])
	}
	timestampText, ok := auth["timestamp"].(string)
	if !ok {
		t.Fatalf("auth timestamp = %#v, want string", auth["timestamp"])
	}
	timestamp, err := strconv.ParseInt(timestampText, 10, 64)
	if err != nil {
		t.Fatalf("parse auth timestamp %q: %v", timestampText, err)
	}
	expectedSignature, err := polyauth.HMACSignature("secret", timestamp, "GET", "/ws/user", nil)
	if err != nil {
		t.Fatalf("derive expected signature: %v", err)
	}
	if got := auth["signature"]; got != expectedSignature {
		t.Fatalf("auth signature = %v, want %v", got, expectedSignature)
	}

	if err := client.SubscribeTrades(t.Context(), []string{"market-1"}); err != nil {
		t.Fatalf("subscribe trades: %v", err)
	}
	assertNoSubscriptionMessage(t, messages)
	if got := client.SubscriptionCount(); got != 2 {
		t.Fatalf("subscription count = %d, want 2", got)
	}

	if err := client.UnsubscribeOrders(t.Context(), []string{"market-1"}); err != nil {
		t.Fatalf("unsubscribe orders: %v", err)
	}
	assertNoSubscriptionMessage(t, messages)
	if got := client.SubscriptionCount(); got != 1 {
		t.Fatalf("subscription count = %d, want 1", got)
	}

	if err := client.UnsubscribeTrades(t.Context(), []string{"market-1"}); err != nil {
		t.Fatalf("unsubscribe trades: %v", err)
	}
	msg = mustReceiveSubscriptionMessage(t, messages)
	if got := msg["operation"]; got != "unsubscribe" {
		t.Fatalf("operation = %v, want unsubscribe", got)
	}
	if got := messageStrings(msg["markets"]); len(got) != 1 || got[0] != "market-1" {
		t.Fatalf("markets = %#v, want [market-1]", got)
	}
	if got := client.SubscriptionCount(); got != 0 {
		t.Fatalf("subscription count = %d, want 0", got)
	}
}

func TestWithCredentialsDecodesSecret(t *testing.T) {
	t.Parallel()

	client := NewClient("").WithCredentials(clob.Credentials{
		Key:        "key",
		Secret:     "secret",
		Passphrase: "pass",
	})
	auth, err := client.deriveWSAuth(t.Context())
	if err != nil {
		t.Fatalf("derive WS auth: %v", err)
	}
	timestamp, err := strconv.ParseInt(auth.Timestamp, 10, 64)
	if err != nil {
		t.Fatalf("parse auth timestamp %q: %v", auth.Timestamp, err)
	}
	expectedSignature, err := polyauth.HMACSignature("secret", timestamp, "GET", "/ws/user", nil)
	if err != nil {
		t.Fatalf("derive expected signature: %v", err)
	}
	if auth.Signature != expectedSignature {
		t.Fatalf("auth signature = %q, want %q", auth.Signature, expectedSignature)
	}
}

func TestInvalidWebSocketSecretReturnsError(t *testing.T) {
	t.Parallel()

	client := NewAuthenticatedClient("", clob.Credentials{
		Key:        "key",
		Secret:     "%%%",
		Passphrase: "pass",
	})
	if _, err := client.deriveWSAuth(t.Context()); err == nil ||
		!strings.Contains(err.Error(), "invalid API secret") {
		t.Fatalf("derive WS auth error = %v, want invalid API secret error", err)
	}
}

func TestMarketSubscriptionsRefcountAndCustomFeatureFlag(t *testing.T) {
	t.Parallel()

	server, messages := newSubscriptionTestServer(t)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	client := NewClient(wsURL)
	if err := client.Connect(t.Context()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()

	if err := client.SubscribeOrderBook(t.Context(), []string{"asset-1"}); err != nil {
		t.Fatalf("subscribe order book: %v", err)
	}
	msg := mustReceiveSubscriptionMessage(t, messages)
	if got := msg["type"]; got != "market" {
		t.Fatalf("type = %v, want market", got)
	}
	if got := messageStrings(msg["assets_ids"]); len(got) != 1 || got[0] != "asset-1" {
		t.Fatalf("assets_ids = %#v, want [asset-1]", got)
	}
	if got := msg["initial_dump"]; got != true {
		t.Fatalf("initial_dump = %v, want true", got)
	}
	if got := msg["custom_feature_enabled"]; got != true {
		t.Fatalf("custom_feature_enabled = %v, want true", got)
	}

	if err := client.SubscribePrices(t.Context(), []string{"asset-1"}); err != nil {
		t.Fatalf("subscribe prices: %v", err)
	}
	assertNoSubscriptionMessage(t, messages)

	if err := client.SubscribeBestBidAsk(t.Context(), []string{"asset-1"}); err != nil {
		t.Fatalf("subscribe best bid ask: %v", err)
	}
	msg = mustReceiveSubscriptionMessage(t, messages)
	if got := msg["custom_feature_enabled"]; got != true {
		t.Fatalf("custom_feature_enabled = %v, want true", got)
	}
	if got := messageStrings(msg["assets_ids"]); len(got) != 1 || got[0] != "asset-1" {
		t.Fatalf("assets_ids = %#v, want [asset-1]", got)
	}

	if err := client.UnsubscribeOrderBook(t.Context(), []string{"asset-1"}); err != nil {
		t.Fatalf("unsubscribe order book: %v", err)
	}
	assertNoSubscriptionMessage(t, messages)

	if err := client.UnsubscribePrices(t.Context(), []string{"asset-1"}); err != nil {
		t.Fatalf("unsubscribe prices: %v", err)
	}
	assertNoSubscriptionMessage(t, messages)

	if err := client.UnsubscribeOrderBook(t.Context(), []string{"asset-1"}); err != nil {
		t.Fatalf("unsubscribe remaining market ref: %v", err)
	}
	msg = mustReceiveSubscriptionMessage(t, messages)
	if got := msg["operation"]; got != "unsubscribe" {
		t.Fatalf("operation = %v, want unsubscribe", got)
	}
	if got := messageStrings(msg["assets_ids"]); len(got) != 1 || got[0] != "asset-1" {
		t.Fatalf("assets_ids = %#v, want [asset-1]", got)
	}
}

func newSubscriptionTestServer(
	t *testing.T,
) (*httptest.Server, <-chan map[string]any) {
	t.Helper()

	messages := make(chan map[string]any, 16)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Logf("accept: %v", err)
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")

		for {
			_, data, err := c.Read(r.Context())
			if err != nil {
				return
			}
			if string(data) == "PING" {
				_ = c.Write(r.Context(), websocket.MessageText, []byte("PONG"))
				continue
			}

			var msg map[string]any
			if err := json.Unmarshal(data, &msg); err != nil {
				t.Logf("decode ws message: %v", err)
				continue
			}
			select {
			case messages <- msg:
			default:
				t.Log("dropping ws test message")
			}
		}
	}))

	return server, messages
}

func mustReceiveSubscriptionMessage(t *testing.T, messages <-chan map[string]any) map[string]any {
	t.Helper()

	select {
	case msg := <-messages:
		return msg
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for websocket message")
		return nil
	}
}

func assertNoSubscriptionMessage(t *testing.T, messages <-chan map[string]any) {
	t.Helper()

	select {
	case msg := <-messages:
		t.Fatalf("unexpected websocket message: %#v", msg)
	case <-time.After(100 * time.Millisecond):
	}
}

func messageStrings(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}

	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
