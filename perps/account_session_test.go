package perps

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

const testPerpsProxy = "0x1111111111111111111111111111111111111111"

func testPerpsCredentials() PerpsCredentials {
	return PerpsCredentials{Proxy: testPerpsProxy, Secret: "perps-secret"}
}

func TestAuthenticatedAccountReads(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/account/balances", func(w http.ResponseWriter, r *http.Request) {
		assertPerpsAuth(t, r)
		_ = json.NewEncoder(w).
			Encode([]PerpsBalance{{Asset: "USDC", Balance: "12.5", Value: "12.5"}})
	})
	mux.HandleFunc("/v1/account/portfolio", func(w http.ResponseWriter, r *http.Request) {
		assertPerpsAuth(t, r)
		_ = json.NewEncoder(w).Encode(PerpsPortfolio{Withdrawable: "10", Timestamp: 1000})
	})
	mux.HandleFunc("/v1/account/stats", func(w http.ResponseWriter, r *http.Request) {
		assertPerpsAuth(t, r)
		_ = json.NewEncoder(w).Encode(PerpsAccountStats{Volume7d: "100"})
	})
	mux.HandleFunc("/v1/account/config", func(w http.ResponseWriter, r *http.Request) {
		assertPerpsAuth(t, r)
		if got := r.URL.Query().Get("instrument_id"); got != "7" {
			t.Errorf("instrument_id = %q, want 7", got)
		}
		_ = json.NewEncoder(w).
			Encode([]PerpsAccountConfig{{InstrumentID: 7, Leverage: 3, Cross: true}})
	})
	mux.HandleFunc("/v1/account/open-orders", func(w http.ResponseWriter, r *http.Request) {
		assertPerpsAuth(t, r)
		_ = json.NewEncoder(w).
			Encode([]PerpsOrder{{ID: 1, InstrumentID: 7, Status: PerpsOrderOpen}})
	})
	mux.HandleFunc("/v1/account/orders", func(w http.ResponseWriter, r *http.Request) {
		assertPerpsAuth(t, r)
		query := r.URL.Query()
		if query.Get("start_timestamp") != "100" || query.Get("end_timestamp") != "200" {
			t.Errorf(
				"order timestamps = %q/%q, want 100/200",
				query.Get("start_timestamp"),
				query.Get("end_timestamp"),
			)
		}
		_ = json.NewEncoder(w).
			Encode([]PerpsOrder{{ID: 2, InstrumentID: 7, Status: PerpsOrderFilled}})
	})
	mux.HandleFunc("/v1/account/fills", func(w http.ResponseWriter, r *http.Request) {
		assertPerpsAuth(t, r)
		_ = json.NewEncoder(w).Encode(PerpsPage[PerpsAccountFill]{
			Data: []PerpsAccountFill{{TradeID: 3, InstrumentID: 7, Price: "4"}},
		})
	})
	mux.HandleFunc("/v1/account/funding", func(w http.ResponseWriter, r *http.Request) {
		assertPerpsAuth(t, r)
		_ = json.NewEncoder(w).Encode(PerpsPage[PerpsAccountFundingPayment]{
			Data: []PerpsAccountFundingPayment{{InstrumentID: 7, Funding: "0.1"}},
		})
	})
	mux.HandleFunc("/v1/account/deposits", func(w http.ResponseWriter, r *http.Request) {
		assertPerpsAuth(t, r)
		_ = json.NewEncoder(w).Encode(PerpsPage[PerpsDeposit]{
			Data: []PerpsDeposit{{Hash: "0xdeposit", Amount: "100"}},
		})
	})
	mux.HandleFunc("/v1/account/withdrawals", func(w http.ResponseWriter, r *http.Request) {
		assertPerpsAuth(t, r)
		_ = json.NewEncoder(w).Encode(PerpsPage[PerpsWithdrawal]{
			Data: []PerpsWithdrawal{{WithdrawalID: 4, Amount: "90"}},
		})
	})
	mux.HandleFunc("/v1/account/equity", func(w http.ResponseWriter, r *http.Request) {
		assertPerpsAuth(t, r)
		_, _ = w.Write([]byte(`{"data":[[1000,"5"]],"more":false}`))
	})
	mux.HandleFunc("/v1/account/pnl", func(w http.ResponseWriter, r *http.Request) {
		assertPerpsAuth(t, r)
		_, _ = w.Write([]byte(`{"data":[[1000,"-1"]],"more":false}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client, err := NewAuthenticated(AuthenticatedConfig{
		Config:      Config{Host: server.URL},
		Credentials: testPerpsCredentials(),
	})
	if err != nil {
		t.Fatalf("NewAuthenticated: %v", err)
	}

	balances, err := client.GetBalances(t.Context())
	if err != nil || len(balances) != 1 || balances[0].Balance != "12.5" {
		t.Fatalf("GetBalances = %+v, %v", balances, err)
	}
	portfolio, err := client.GetPortfolio(t.Context())
	if err != nil || portfolio.Timestamp != 1000 {
		t.Fatalf("GetPortfolio = %+v, %v", portfolio, err)
	}
	stats, err := client.GetAccountStats(t.Context())
	if err != nil || stats.Volume7d != "100" {
		t.Fatalf("GetAccountStats = %+v, %v", stats, err)
	}
	instrumentID := 7
	configs, err := client.GetAccountConfig(
		t.Context(),
		AccountConfigParams{InstrumentID: &instrumentID},
	)
	if err != nil || len(configs) != 1 || configs[0].Leverage != 3 {
		t.Fatalf("GetAccountConfig = %+v, %v", configs, err)
	}
	openOrders, err := client.GetOpenOrders(t.Context(), OpenOrdersParams{})
	if err != nil || len(openOrders) != 1 || openOrders[0].Status != PerpsOrderOpen {
		t.Fatalf("GetOpenOrders = %+v, %v", openOrders, err)
	}
	orders, err := client.GetOrders(t.Context(), AccountOrdersParams{Start: 100, End: 200})
	if err != nil || len(orders) != 1 || orders[0].Status != PerpsOrderFilled {
		t.Fatalf("GetOrders = %+v, %v", orders, err)
	}
	fills, err := client.GetFillsPage(t.Context(), AccountHistoryParams{})
	if err != nil || len(fills.Data) != 1 || fills.Data[0].TradeID != 3 {
		t.Fatalf("GetFillsPage = %+v, %v", fills, err)
	}
	funding, err := client.GetFundingPaymentsPage(t.Context(), AccountHistoryParams{})
	if err != nil || len(funding.Data) != 1 || funding.Data[0].Funding != "0.1" {
		t.Fatalf("GetFundingPaymentsPage = %+v, %v", funding, err)
	}
	deposits, err := client.GetDepositsPage(t.Context(), AccountHistoryParams{})
	if err != nil || len(deposits.Data) != 1 || deposits.Data[0].Amount != "100" {
		t.Fatalf("GetDepositsPage = %+v, %v", deposits, err)
	}
	withdrawals, err := client.GetWithdrawalsPage(t.Context(), AccountHistoryParams{})
	if err != nil || len(withdrawals.Data) != 1 || withdrawals.Data[0].WithdrawalID != 4 {
		t.Fatalf("GetWithdrawalsPage = %+v, %v", withdrawals, err)
	}
	equity, err := client.GetEquityHistoryPage(
		t.Context(),
		AccountIntervalHistoryParams{Interval: PerpsPnl1h},
	)
	if err != nil || len(equity.Data) != 1 || equity.Data[0].Equity != "5" {
		t.Fatalf("GetEquityHistoryPage = %+v, %v", equity, err)
	}
	pnl, err := client.GetPnlHistoryPage(
		t.Context(),
		AccountIntervalHistoryParams{Interval: PerpsPnl1h},
	)
	if err != nil || len(pnl.Data) != 1 || pnl.Data[0].PnL != "-1" {
		t.Fatalf("GetPnlHistoryPage = %+v, %v", pnl, err)
	}
}

func TestFillsPageCarriesSortAndNextCursor(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/account/fills", func(w http.ResponseWriter, r *http.Request) {
		assertPerpsAuth(t, r)
		query := r.URL.Query()
		if query.Get("sort") != "asc" || query.Get("cursor") != "12" {
			t.Errorf("fills query = %v, want sort=asc cursor=12", query)
		}
		_ = json.NewEncoder(w).Encode(PerpsPage[PerpsAccountFill]{
			Data: []PerpsAccountFill{{TradeID: 34}},
			More: true,
		})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client, err := NewAuthenticated(AuthenticatedConfig{
		Config:      Config{Host: server.URL},
		Credentials: testPerpsCredentials(),
	})
	if err != nil {
		t.Fatalf("NewAuthenticated: %v", err)
	}
	page, err := client.GetFillsPage(t.Context(), AccountHistoryParams{
		Sort:   PerpsSortAscending,
		Cursor: "12",
	})
	if err != nil {
		t.Fatalf("GetFillsPage: %v", err)
	}
	if !page.More || page.NextCursor != "34" {
		t.Fatalf("page = %+v, want more with cursor 34", page)
	}

	_, err = client.GetFillsPage(t.Context(), AccountHistoryParams{Sort: "middle"})
	if err == nil || !strings.Contains(err.Error(), "invalid sort direction") {
		t.Fatalf("invalid sort error = %v", err)
	}
}

func TestOpenSessionAuthenticatesSubscribesAndEmitsEvents(t *testing.T) {
	frames := make(chan map[string]any, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		for i := 0; i < 2; i++ {
			_, payload, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			var frame map[string]any
			if json.Unmarshal(payload, &frame) == nil {
				frames <- frame
			}
			id, _ := frame["id"].(float64)
			ack := map[string]any{"id": int(id), "data": map[string]any{"status": "ok"}}
			if i == 1 {
				ack["data"] = []map[string]any{{"status": "ok"}}
			}
			response, _ := json.Marshal(ack)
			if err := conn.Write(r.Context(), websocket.MessageText, response); err != nil {
				return
			}
		}
		event, _ := json.Marshal(map[string]any{
			"ch":   "balances",
			"ts":   1234,
			"sq":   9,
			"data": map[string]any{"asset": "USDC", "balance": "4", "value": "4"},
		})
		_ = conn.Write(r.Context(), websocket.MessageText, event)
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	client, err := NewAuthenticated(AuthenticatedConfig{
		Config:      Config{WebSocketHost: "ws" + strings.TrimPrefix(server.URL, "http")},
		Credentials: testPerpsCredentials(),
	})
	if err != nil {
		t.Fatalf("NewAuthenticated: %v", err)
	}
	session, err := client.OpenSession(t.Context(), SessionConfig{})
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	framesCtx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	for i := 0; i < 2; i++ {
		select {
		case frame := <-frames:
			if frame["id"] == nil {
				t.Fatalf("handshake frame has no id: %+v", frame)
			}
			if i == 1 {
				channels, ok := frame["chs"].([]any)
				if !ok || len(channels) != len(defaultSessionChannels) {
					t.Fatalf(
						"subscription channels = %#v, want %d channels",
						frame["chs"],
						len(defaultSessionChannels),
					)
				}
			}
		case <-framesCtx.Done():
			t.Fatal("timed out waiting for handshake frames")
		}
	}
	select {
	case event := <-session.Events():
		if event.Channel != "balances" || event.Sequence != 9 ||
			string(event.Data) != `{"asset":"USDC","balance":"4","value":"4"}` {
			t.Fatalf("event = %+v, want balances payload", event)
		}
	case <-framesCtx.Done():
		t.Fatal("timed out waiting for session event")
	}
}

func TestSessionHandlesBatchedFramesAndOrderWaiters(t *testing.T) {
	session := &Session{
		ctx:          context.Background(),
		events:       make(chan PerpsSessionEvent, 2),
		errors:       make(chan error, 1),
		orderWaiters: make(map[int][]chan orderWaitResponse),
	}
	session.handlePayload([]byte(`[
		{"ch":"orders","ts":1,"sq":2,"data":{"oid":77,"status":"open"}},
		{"ch":"balances","ts":3,"sq":4,"data":{"asset":"USDC"}}
	]`))

	update, err := session.waitForOrderUpdate(context.Background(), 77)
	if err != nil || update.ID != 77 {
		t.Fatalf("waitForOrderUpdate = %+v, %v", update, err)
	}
	events := []PerpsSessionEvent{<-session.events, <-session.events}
	if events[0].Channel != "orders" || events[1].Channel != "balances" {
		t.Fatalf("batched events = %+v", events)
	}
}

func TestSessionHeartbeatSendsApplicationPing(t *testing.T) {
	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		_, payload, err := conn.Read(r.Context())
		if err == nil {
			received <- append([]byte(nil), payload...)
		}
	}))
	t.Cleanup(server.Close)

	conn, _, err := websocket.Dial(
		t.Context(),
		"ws"+strings.TrimPrefix(server.URL, "http"),
		nil,
	)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	sessionCtx, cancel := context.WithCancel(t.Context())
	session := &Session{conn: conn, ctx: sessionCtx, cancel: cancel}
	session.lastMessage.Store(time.Now().UnixNano())
	go session.heartbeatLoopWith(5*time.Millisecond, time.Second)
	t.Cleanup(func() {
		cancel()
		_ = conn.Close(websocket.StatusNormalClosure, "")
	})

	select {
	case payload := <-received:
		if string(payload) != string(perpsHeartbeatPayload) {
			t.Fatalf("heartbeat payload = %s, want %s", payload, perpsHeartbeatPayload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for heartbeat")
	}
}

func TestSessionReconnectsAndResubscribes(t *testing.T) {
	connections := atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		connection := connections.Add(1)
		for i := 0; i < 2; i++ {
			_, payload, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			var frame map[string]any
			if json.Unmarshal(payload, &frame) != nil {
				return
			}
			response, _ := json.Marshal(map[string]any{
				"id":   frame["id"],
				"data": map[string]any{"status": "ok"},
			})
			if err := conn.Write(r.Context(), websocket.MessageText, response); err != nil {
				return
			}
		}
		if connection == 1 {
			return
		}
		event, _ := json.Marshal(map[string]any{
			"ch":   "balances",
			"ts":   10,
			"sq":   11,
			"data": map[string]any{"asset": "USDC"},
		})
		_ = conn.Write(r.Context(), websocket.MessageText, event)
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	client, err := NewAuthenticated(AuthenticatedConfig{
		Config:      Config{WebSocketHost: "ws" + strings.TrimPrefix(server.URL, "http")},
		Credentials: testPerpsCredentials(),
	})
	if err != nil {
		t.Fatalf("NewAuthenticated: %v", err)
	}
	session, err := client.OpenSession(t.Context(), SessionConfig{})
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	select {
	case event, ok := <-session.Events():
		if !ok || event.Type != "resync" || event.Resync == nil ||
			event.Resync.Reason != PerpsResyncReconnect {
			t.Fatalf("reconnect resync event = %+v, open = %v", event, ok)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for reconnect resync event")
	}
	select {
	case event, ok := <-session.Events():
		if !ok || event.Channel != "balances" || event.Sequence != 11 {
			t.Fatalf("reconnected event = %+v, open = %v", event, ok)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for reconnected event")
	}
	if got := connections.Load(); got < 2 {
		t.Fatalf("connections = %d, want reconnect", got)
	}
}

func assertPerpsAuth(t *testing.T, r *http.Request) {
	t.Helper()
	if got := r.Header.Get("POLYMARKET-PROXY"); got != testPerpsProxy {
		t.Errorf("POLYMARKET-PROXY = %q, want %q", got, testPerpsProxy)
	}
	if got := r.Header.Get("POLYMARKET-SECRET"); got != "perps-secret" {
		t.Errorf("POLYMARKET-SECRET = %q, want perps-secret", got)
	}
}
