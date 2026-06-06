package clob

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/quagmt/udecimal"
)

// orderLifecycleServer is a mock CLOB server that tracks order state across
// the full lifecycle: post → get → list → cancel. It validates auth headers
// and request payloads at each step.
type orderLifecycleServer struct {
	mu     sync.Mutex
	orders map[string]openOrderRecord // keyed by order ID
	trades map[string]tradeRecord
	nextID int
}

type openOrderRecord struct {
	ID           string
	Status       string
	Owner        string
	Market       string
	AssetID      string
	Side         string
	OriginalSize string
	SizeMatched  string
	Price        string
	OrderType    string
}

type tradeRecord struct {
	ID         string
	OrderID    string
	Market     string
	AssetID    string
	Side       string
	Size       string
	Price      string
	Status     string
	TraderSide string
}

func newOrderLifecycleServer(t *testing.T) (*httptest.Server, *orderLifecycleServer) {
	t.Helper()
	ols := &orderLifecycleServer{
		orders: make(map[string]openOrderRecord),
		trades: make(map[string]tradeRecord),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case tickSizeEndpoint:
			w.Write([]byte(`{"minimum_tick_size":"0.01"}`))

		case negRiskEndpoint:
			w.Write([]byte(`{"neg_risk":false}`))

		case orderBookEndpoint:
			w.Write([]byte(`{
				"market":"m-1","asset_id":"100","timestamp":"1",
				"bids":[{"price":"0.44","size":"10"}],
				"asks":[{"price":"0.46","size":"10"}],
				"min_order_size":"1","tick_size":"0.01","neg_risk":false,
				"last_trade_price":"0.45","hash":"h"
			}`))

		case postOrderEndpoint:
			// POST /order = post, DELETE /order = cancel.
			if r.Method == http.MethodPost {
				ols.handlePostOrder(t, w, r)
			} else {
				ols.handleCancelOrder(t, w, r)
			}

		case openOrdersEndpoint:
			ols.handleGetOpenOrders(t, w, r)

		case tradesEndpoint:
			ols.handleGetTrades(t, w, r)

		case cancelAllEndpoint:
			ols.handleCancelAll(t, w, r)

		default:
			// Handle /data/order/{id}
			if strings.HasPrefix(r.URL.Path, orderEndpoint) {
				ols.handleGetOrder(t, w, r)
				return
			}
			t.Fatalf("unexpected path: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	return server, ols
}

func (ols *orderLifecycleServer) handlePostOrder(
	t *testing.T,
	w http.ResponseWriter,
	r *http.Request,
) {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read post body: %v", err)
	}

	var req PostOrderRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("decode post order: %v", err)
	}

	if req.Owner == "" {
		t.Error("expected non-empty owner")
	}
	if req.Order.Order.TokenID == "" {
		t.Error("expected non-empty tokenId")
	}

	ols.mu.Lock()
	ols.nextID++
	id := "order-" + string(rune('a'+ols.nextID-1))
	record := openOrderRecord{
		ID:           id,
		Status:       "live",
		Owner:        req.Owner,
		AssetID:      req.Order.Order.TokenID,
		Side:         string(req.Order.Order.Side),
		OriginalSize: req.Order.Order.TakerAmount,
		Price:        req.Order.Order.MakerAmount,
		OrderType:    string(req.OrderType),
	}
	ols.orders[id] = record
	ols.mu.Unlock()

	resp := PostOrderResponse{
		Success: true,
		OrderID: id,
		Status:  "live",
	}
	json.NewEncoder(w).Encode(resp)
}

func (ols *orderLifecycleServer) handleGetOrder(
	t *testing.T,
	w http.ResponseWriter,
	r *http.Request,
) {
	t.Helper()
	id := strings.TrimPrefix(r.URL.Path, orderEndpoint)

	ols.mu.Lock()
	record, ok := ols.orders[id]
	ols.mu.Unlock()

	if !ok {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(OpenOrder{
		ID:           record.ID,
		Status:       record.Status,
		Owner:        record.Owner,
		AssetID:      record.AssetID,
		Side:         record.Side,
		OriginalSize: record.OriginalSize,
		Price:        record.Price,
		OrderType:    record.OrderType,
	})
}

func (ols *orderLifecycleServer) handleGetOpenOrders(
	t *testing.T,
	w http.ResponseWriter,
	r *http.Request,
) {
	t.Helper()
	ols.mu.Lock()
	defer ols.mu.Unlock()

	var orders []OpenOrder
	for _, record := range ols.orders {
		if record.Status == "live" {
			orders = append(orders, OpenOrder{
				ID:           record.ID,
				Status:       record.Status,
				Owner:        record.Owner,
				AssetID:      record.AssetID,
				Side:         record.Side,
				OriginalSize: record.OriginalSize,
				Price:        record.Price,
				OrderType:    record.OrderType,
			})
		}
	}

	json.NewEncoder(w).Encode(Page[OpenOrder]{
		Data:       orders,
		NextCursor: "-1",
	})
}

func (ols *orderLifecycleServer) handleGetTrades(
	t *testing.T,
	w http.ResponseWriter,
	r *http.Request,
) {
	t.Helper()
	ols.mu.Lock()
	defer ols.mu.Unlock()

	var trades []Trade
	for _, record := range ols.trades {
		trades = append(trades, Trade{
			ID:         record.ID,
			AssetID:    record.AssetID,
			Side:       Side(record.Side),
			Size:       record.Size,
			Price:      record.Price,
			Status:     record.Status,
			TraderSide: record.TraderSide,
		})
	}

	json.NewEncoder(w).Encode(Page[Trade]{
		Data:       trades,
		NextCursor: "-1",
	})
}

func (ols *orderLifecycleServer) handleCancelOrder(
	t *testing.T,
	w http.ResponseWriter,
	r *http.Request,
) {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read cancel body: %v", err)
	}

	var payload OrderPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode cancel: %v", err)
	}

	ols.mu.Lock()
	defer ols.mu.Unlock()

	record, ok := ols.orders[payload.OrderID]
	if !ok {
		json.NewEncoder(w).Encode(CancelOrdersResponse{
			Canceled:    []string{},
			NotCanceled: map[string]string{payload.OrderID: "not found"},
		})
		return
	}

	record.Status = "canceled"
	ols.orders[payload.OrderID] = record

	json.NewEncoder(w).Encode(CancelOrdersResponse{
		Canceled:    []string{payload.OrderID},
		NotCanceled: map[string]string{},
	})
}

func (ols *orderLifecycleServer) handleCancelAll(
	t *testing.T,
	w http.ResponseWriter,
	_ *http.Request,
) {
	t.Helper()
	ols.mu.Lock()
	defer ols.mu.Unlock()

	var canceled []string
	for id, record := range ols.orders {
		if record.Status == "live" {
			record.Status = "canceled"
			ols.orders[id] = record
			canceled = append(canceled, id)
		}
	}

	json.NewEncoder(w).Encode(CancelOrdersResponse{
		Canceled:    canceled,
		NotCanceled: map[string]string{},
	})
}

// --- Integration tests ---

func TestOrderLifecycle_CreatePostGetCancel(t *testing.T) {
	t.Parallel()

	server, ols := newOrderLifecycleServer(t)

	client, err := NewAuthenticatedClient(Config{
		Host:       server.URL,
		PrivateKey: "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		Credentials: &Credentials{
			Key:        "api-key",
			Secret:     "c2VjcmV0",
			Passphrase: "pass",
		},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	client.saltGenerator = func() (uint64, error) { return 42, nil }

	ctx := t.Context()

	// Step 1: Create and post a limit order.
	postResp, err := client.CreateAndPostOrder(ctx, OrderArgs{
		TokenID: "100",
		Price:   udecimal.MustParse("0.45"),
		Size:    udecimal.MustParse("10"),
		Side:    SideBuy,
	}, nil, OrderTypeGTC, false)
	if err != nil {
		t.Fatalf("create and post: %v", err)
	}
	if !postResp.Success || postResp.OrderID == "" {
		t.Fatalf("unexpected post response: %+v", postResp)
	}
	if postResp.Status != "live" {
		t.Fatalf("expected live status, got %q", postResp.Status)
	}
	orderID := postResp.OrderID

	// Step 2: Fetch the order by ID.
	order, err := client.GetOrder(ctx, orderID)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if order.ID != orderID {
		t.Fatalf("order ID mismatch: got %s, want %s", order.ID, orderID)
	}
	if order.Status != "live" {
		t.Fatalf("expected live status, got %q", order.Status)
	}

	// Step 3: List open orders — should include our order.
	openOrders, err := client.GetOpenOrders(ctx, OpenOrderParams{AssetID: "100"})
	if err != nil {
		t.Fatalf("get open orders: %v", err)
	}
	found := false
	for _, oo := range openOrders {
		if oo.ID == orderID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("order %s not found in open orders: %+v", orderID, openOrders)
	}

	// Step 4: Cancel the order.
	cancelResp, err := client.CancelOrder(ctx, orderID)
	if err != nil {
		t.Fatalf("cancel order: %v", err)
	}
	if len(cancelResp.Canceled) != 1 || cancelResp.Canceled[0] != orderID {
		t.Fatalf("unexpected cancel response: %+v", cancelResp)
	}

	// Step 5: Verify order is no longer live.
	ols.mu.Lock()
	record := ols.orders[orderID]
	ols.mu.Unlock()
	if record.Status != "canceled" {
		t.Fatalf("expected canceled status, got %q", record.Status)
	}
}

func TestOrderLifecycle_BatchPostAndCancelAll(t *testing.T) {
	t.Parallel()

	server, ols := newOrderLifecycleServer(t)

	client, err := NewAuthenticatedClient(Config{
		Host:       server.URL,
		PrivateKey: "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		Credentials: &Credentials{
			Key:        "api-key",
			Secret:     "c2VjcmV0",
			Passphrase: "pass",
		},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	client.saltGenerator = func() (uint64, error) { return 99, nil }

	ctx := t.Context()

	// Create two orders.
	resp1, err := client.CreateAndPostOrder(ctx, OrderArgs{
		TokenID: "100", Price: udecimal.MustParse("0.45"),
		Size: udecimal.MustParse("10"), Side: SideBuy,
	}, nil, OrderTypeGTC, false)
	if err != nil {
		t.Fatalf("post order 1: %v", err)
	}

	resp2, err := client.CreateAndPostOrder(ctx, OrderArgs{
		TokenID: "100", Price: udecimal.MustParse("0.44"),
		Size: udecimal.MustParse("5"), Side: SideBuy,
	}, nil, OrderTypeGTC, false)
	if err != nil {
		t.Fatalf("post order 2: %v", err)
	}

	// Verify both are live.
	ols.mu.Lock()
	live := 0
	for _, r := range ols.orders {
		if r.Status == "live" {
			live++
		}
	}
	ols.mu.Unlock()
	if live != 2 {
		t.Fatalf("expected 2 live orders, got %d", live)
	}

	// Cancel all.
	cancelResp, err := client.CancelAll(ctx)
	if err != nil {
		t.Fatalf("cancel all: %v", err)
	}
	if len(cancelResp.Canceled) != 2 {
		t.Fatalf("expected 2 canceled, got %d: %+v", len(cancelResp.Canceled), cancelResp)
	}

	// Verify both canceled.
	ols.mu.Lock()
	for _, id := range []string{resp1.OrderID, resp2.OrderID} {
		if ols.orders[id].Status != "canceled" {
			t.Errorf("order %s not canceled", id)
		}
	}
	ols.mu.Unlock()
}

func TestOrderLifecycle_CancelNonexistentOrder(t *testing.T) {
	t.Parallel()

	server, _ := newOrderLifecycleServer(t)

	client, err := NewAuthenticatedClient(Config{
		Host:       server.URL,
		PrivateKey: "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		Credentials: &Credentials{
			Key: "api-key", Secret: "c2VjcmV0", Passphrase: "pass",
		},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	resp, err := client.CancelOrder(t.Context(), "nonexistent")
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if len(resp.Canceled) != 0 {
		t.Errorf("expected no canceled orders, got %v", resp.Canceled)
	}
	if _, ok := resp.NotCanceled["nonexistent"]; !ok {
		t.Errorf("expected nonexistent in not_canceled")
	}
}

func TestOrderLifecycle_PostOrderAuthHeaders(t *testing.T) {
	t.Parallel()

	var capturedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case tickSizeEndpoint:
			w.Write([]byte(`{"minimum_tick_size":"0.01"}`))
		case negRiskEndpoint:
			w.Write([]byte(`{"neg_risk":false}`))
		case orderBookEndpoint:
			w.Write(
				[]byte(
					`{"market":"m","asset_id":"100","timestamp":"1","bids":[{"price":"0.44","size":"10"}],"asks":[{"price":"0.46","size":"10"}],"min_order_size":"1","tick_size":"0.01","neg_risk":false,"last_trade_price":"0.45","hash":"h"}`,
				),
			)
		case postOrderEndpoint:
			capturedHeaders = r.Header.Clone()
			w.Write([]byte(`{"success":true,"orderID":"o-1","status":"live"}`))
		}
	}))
	t.Cleanup(server.Close)

	client, err := NewAuthenticatedClient(Config{
		Host:       server.URL,
		PrivateKey: "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		Credentials: &Credentials{
			Key: "api-key", Secret: "c2VjcmV0", Passphrase: "pass",
		},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	client.saltGenerator = func() (uint64, error) { return 42, nil }

	_, err = client.CreateAndPostOrder(t.Context(), OrderArgs{
		TokenID: "100", Price: udecimal.MustParse("0.45"),
		Size: udecimal.MustParse("10"), Side: SideBuy,
	}, nil, OrderTypeGTC, false)
	if err != nil {
		t.Fatalf("post: %v", err)
	}

	if capturedHeaders == nil {
		t.Fatal("post handler was not called")
	}
	if got := capturedHeaders.Get("POLY_API_KEY"); got != "api-key" {
		t.Errorf("POLY_API_KEY = %q, want api-key", got)
	}
	if got := capturedHeaders.Get("POLY_SIGNATURE"); got == "" {
		t.Error("expected non-empty POLY_SIGNATURE")
	}
	if got := capturedHeaders.Get("POLY_PASSPHRASE"); got == "" {
		t.Error("expected non-empty POLY_PASSPHRASE")
	}
	if got := capturedHeaders.Get("POLY_TIMESTAMP"); got == "" {
		t.Error("expected non-empty POLY_TIMESTAMP")
	}
}

func TestOrderLifecycle_GetTradesEmpty(t *testing.T) {
	t.Parallel()

	server, _ := newOrderLifecycleServer(t)

	client, err := NewAuthenticatedClient(Config{
		Host:       server.URL,
		PrivateKey: "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		Credentials: &Credentials{
			Key: "api-key", Secret: "c2VjcmV0", Passphrase: "pass",
		},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	trades, err := client.GetTrades(t.Context(), TradeParams{})
	if err != nil {
		t.Fatalf("get trades: %v", err)
	}
	if len(trades) != 0 {
		t.Errorf("expected 0 trades, got %d", len(trades))
	}
}

func TestOrderLifecycle_IterOpenOrdersEmpty(t *testing.T) {
	t.Parallel()

	server, _ := newOrderLifecycleServer(t)

	client, err := NewAuthenticatedClient(Config{
		Host:       server.URL,
		PrivateKey: "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		Credentials: &Credentials{
			Key: "api-key", Secret: "c2VjcmV0", Passphrase: "pass",
		},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	var count int
	for _, iterErr := range client.IterOpenOrders(t.Context(), OpenOrderParams{}) {
		if iterErr != nil {
			t.Fatalf("iter: %v", iterErr)
		}
		count++
	}
	if count != 0 {
		t.Errorf("expected 0 open orders, got %d", count)
	}
}

func TestOrderLifecycle_ServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case tickSizeEndpoint:
			w.Write([]byte(`{"minimum_tick_size":"0.01"}`))
		case negRiskEndpoint:
			w.Write([]byte(`{"neg_risk":false}`))
		case orderBookEndpoint:
			w.Write(
				[]byte(
					`{"market":"m","asset_id":"100","timestamp":"1","bids":[],"asks":[],"min_order_size":"1","tick_size":"0.01","neg_risk":false,"last_trade_price":"0.45","hash":"h"}`,
				),
			)
		default:
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"internal"}`))
		}
	}))
	t.Cleanup(server.Close)

	client, err := NewAuthenticatedClient(Config{
		Host:       server.URL,
		PrivateKey: "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		Credentials: &Credentials{
			Key: "api-key", Secret: "c2VjcmV0", Passphrase: "pass",
		},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	client.saltGenerator = func() (uint64, error) { return 42, nil }

	_, err = client.CreateAndPostOrder(t.Context(), OrderArgs{
		TokenID: "100", Price: udecimal.MustParse("0.45"),
		Size: udecimal.MustParse("10"), Side: SideBuy,
	}, nil, OrderTypeGTC, false)
	if err == nil {
		t.Fatal("expected error from server 500")
	}
}
