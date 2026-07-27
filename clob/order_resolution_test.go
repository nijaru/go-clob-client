package clob

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const orderResolutionPrivateKey = "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c"

func TestPostOrderResolvesTradeIDs(t *testing.T) {
	var tradeCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case postOrderEndpoint:
			if r.Method != http.MethodPost {
				t.Errorf("post order method = %s, want POST", r.Method)
			}
			_, _ = w.Write([]byte(
				`{"success":true,"status":"MATCHED","orderID":"order-1","tradeIDs":["trade-1"]}`,
			))
		case tradesEndpoint:
			tradeCalls.Add(1)
			if got := r.URL.Query().Get("id"); got != "trade-1" {
				t.Errorf("trade lookup id = %q, want trade-1", got)
			}
			if got := r.URL.Query().Get("next_cursor"); got != "" {
				t.Errorf("trade lookup next_cursor = %q, want empty", got)
			}
			_, _ = w.Write([]byte(
				`{"data":[{"id":"trade-1","status":"CONFIRMED","transaction_hash":"0xhash"}]}`,
			))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newOrderResolutionClient(t, server.URL)
	response, err := client.PostOrder(t.Context(), orderResolutionRequest(false))
	if err != nil {
		t.Fatalf("PostOrder: %v", err)
	}
	if got, want := response.TradeIDs, []string{"trade-1"}; !equalStrings(got, want) {
		t.Fatalf("trade IDs = %#v, want %#v", got, want)
	}
	if got, want := response.TransactionsHashes, []string{"0xhash"}; !equalStrings(got, want) {
		t.Fatalf("transaction hashes = %#v, want %#v", got, want)
	}
	if got := tradeCalls.Load(); got != 1 {
		t.Fatalf("trade lookup calls = %d, want 1", got)
	}
}

func TestPostOrderSkipsTradeResolutionForDeferredExecution(t *testing.T) {
	var tradeCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case postOrderEndpoint:
			_, _ = w.Write([]byte(
				`{"success":true,"status":"MATCHED","trade_ids":["trade-1"]}`,
			))
		case tradesEndpoint:
			tradeCalls.Add(1)
			_, _ = w.Write([]byte(`{"data":[]}`))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newOrderResolutionClient(t, server.URL)
	response, err := client.PostOrder(t.Context(), orderResolutionRequest(true))
	if err != nil {
		t.Fatalf("PostOrder: %v", err)
	}
	if len(response.TransactionsHashes) != 0 {
		t.Fatalf("transaction hashes = %#v, want none", response.TransactionsHashes)
	}
	if got := tradeCalls.Load(); got != 0 {
		t.Fatalf("deferred trade lookup calls = %d, want 0", got)
	}
}

func TestPostOrdersSharesTradeResolution(t *testing.T) {
	var tradeCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case postOrdersEndpoint:
			_, _ = w.Write([]byte(
				`[{"success":true,"tradeIDs":["trade-1"]},{"success":true,"tradeIDs":["trade-2"]}]`,
			))
		case tradesEndpoint:
			tradeCalls.Add(1)
			id := r.URL.Query().Get("id")
			hash := map[string]string{"trade-1": "0xhash-1", "trade-2": "0xhash-2"}[id]
			if hash == "" {
				t.Errorf("unexpected trade lookup id: %q", id)
			}
			_, _ = w.Write([]byte(
				`{"data":[{"id":"` + id + `","status":"CONFIRMED","transaction_hash":"` + hash + `"}]}`,
			))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newOrderResolutionClient(t, server.URL)
	responses, err := client.PostOrders(t.Context(), []PostOrderRequest{
		orderResolutionRequest(false),
		orderResolutionRequest(false),
	})
	if err != nil {
		t.Fatalf("PostOrders: %v", err)
	}
	if len(responses) != 2 {
		t.Fatalf("response count = %d, want 2", len(responses))
	}
	for i, want := range []string{"0xhash-1", "0xhash-2"} {
		if got := responses[i].TransactionsHashes; !equalStrings(got, []string{want}) {
			t.Errorf("response[%d] transaction hashes = %#v, want [%q]", i, got, want)
		}
	}
	if got := tradeCalls.Load(); got != 2 {
		t.Fatalf("trade lookup calls = %d, want 2", got)
	}
}

func TestPostOrderWaitsForConfirmedTrade(t *testing.T) {
	var tradeCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case postOrderEndpoint:
			_, _ = w.Write([]byte(
				`{"success":true,"status":"MATCHED","orderID":"order-1","tradeIDs":["trade-1"]}`,
			))
		case tradesEndpoint:
			if got := r.URL.Query().Get("id"); got != "trade-1" {
				t.Errorf("trade lookup id = %q, want trade-1", got)
			}
			if tradeCalls.Add(1) == 1 {
				_, _ = w.Write([]byte(
					`{"data":[{"id":"trade-1","status":"MINED","transaction_hash":"0xreplaceable"}]}`,
				))
				return
			}
			_, _ = w.Write([]byte(
				`{"data":[{"id":"trade-1","status":"CONFIRMED","transaction_hash":"0xconfirmed"}]}`,
			))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newOrderResolutionClient(t, server.URL)
	response, err := client.PostOrder(t.Context(), orderResolutionRequest(false))
	if err != nil {
		t.Fatalf("PostOrder: %v", err)
	}
	if got, want := response.TransactionsHashes, []string{"0xconfirmed"}; !equalStrings(got, want) {
		t.Fatalf("transaction hashes = %#v, want %#v", got, want)
	}
	if got := tradeCalls.Load(); got != 2 {
		t.Fatalf("trade lookup calls = %d, want 2", got)
	}
}

func TestPostOrderRecognizesPrefixedFailedTrade(t *testing.T) {
	var tradeCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case postOrderEndpoint:
			_, _ = w.Write([]byte(
				`{"success":true,"status":"MATCHED","orderID":"order-1","tradeIDs":["trade-1"]}`,
			))
		case tradesEndpoint:
			tradeCalls.Add(1)
			_, _ = w.Write([]byte(
				`{"data":[{"id":"trade-1","status":"TRADE_STATUS_FAILED","transaction_hash":""}]}`,
			))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newOrderResolutionClient(t, server.URL)
	response, err := client.PostOrder(t.Context(), orderResolutionRequest(false))
	if err != nil {
		t.Fatalf("PostOrder: %v", err)
	}
	if len(response.TransactionsHashes) != 0 {
		t.Fatalf("transaction hashes = %#v, want none", response.TransactionsHashes)
	}
	if got := tradeCalls.Load(); got != 1 {
		t.Fatalf("trade lookup calls = %d, want 1", got)
	}
}

func TestWaitForOrderFillSettlement(t *testing.T) {
	var tradeCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != tradesEndpoint {
			t.Errorf("unexpected path: %s", r.URL.Path)
			return
		}
		if tradeCalls.Add(1) == 1 {
			_, _ = w.Write([]byte(
				`{"data":[{"id":"trade-1","status":"MINED","transaction_hash":"0xreplaceable"}]}`,
			))
			return
		}
		_, _ = w.Write([]byte(
			`{"data":[{"id":"trade-1","status":"CONFIRMED","transaction_hash":"0xconfirmed"}]}`,
		))
	}))
	defer server.Close()

	client := newOrderResolutionClient(t, server.URL)
	hashes, err := client.WaitForOrderFillSettlement(
		t.Context(),
		PostOrderResponse{OrderID: "order-1", TradeIDs: []string{"trade-1"}},
		OrderSettlementOptions{Timeout: time.Second, PollInterval: time.Millisecond},
	)
	if err != nil {
		t.Fatalf("WaitForOrderFillSettlement: %v", err)
	}
	if got, want := hashes, []string{"0xconfirmed"}; !equalStrings(got, want) {
		t.Fatalf("transaction hashes = %#v, want %#v", got, want)
	}
	if got := tradeCalls.Load(); got != 2 {
		t.Fatalf("trade lookup calls = %d, want 2", got)
	}
}

func TestWaitForOrderFillSettlementReportsAllFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(
			`{"data":[{"id":"trade-1","status":"TRADE_STATUS_FAILED","transaction_hash":""}]}`,
		))
	}))
	defer server.Close()

	client := newOrderResolutionClient(t, server.URL)
	_, err := client.WaitForOrderFillSettlement(
		t.Context(),
		PostOrderResponse{OrderID: "order-1", TradeIDs: []string{"trade-1"}},
		OrderSettlementOptions{Timeout: time.Second, PollInterval: time.Millisecond},
	)
	if err == nil || !errors.Is(err, ErrSettlementFailed) {
		t.Fatalf("error = %v, want ErrSettlementFailed", err)
	}
}

func TestWaitForOrderFillSettlementTimesOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(
			`{"data":[{"id":"trade-1","status":"MINED","transaction_hash":"0xpending"}]}`,
		))
	}))
	defer server.Close()

	client := newOrderResolutionClient(t, server.URL)
	_, err := client.WaitForOrderFillSettlement(
		t.Context(),
		PostOrderResponse{TradeIDs: []string{"trade-1"}},
		OrderSettlementOptions{Timeout: time.Millisecond, PollInterval: time.Millisecond},
	)
	if err == nil || !errors.Is(err, ErrSettlementTimeout) {
		t.Fatalf("error = %v, want ErrSettlementTimeout", err)
	}
}

func newOrderResolutionClient(t *testing.T, host string) *AuthenticatedClient {
	t.Helper()
	client, err := NewAuthenticatedClient(Config{
		Host:                 host,
		PrivateKey:           orderResolutionPrivateKey,
		DisableAutoHeartbeat: true,
		Credentials: &Credentials{
			Key:        "api-key",
			Secret:     "c2VjcmV0",
			Passphrase: "pass",
		},
	})
	if err != nil {
		t.Fatalf("NewAuthenticatedClient: %v", err)
	}
	return client
}

func orderResolutionRequest(deferExec bool) PostOrderRequest {
	return PostOrderRequest{
		Order:     SignedOrder{Order: Order{Salt: "1"}},
		DeferExec: deferExec,
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
