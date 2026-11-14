package clob

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const walletOrderMaker = "0x1111111111111111111111111111111111111111"
const walletOrderSigner = "0x2222222222222222222222222222222222222222"
const walletOrderHash = "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

func TestWalletOpenOrdersPreserveWireAndFollowEmptyPages(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != walletOpenOrdersEndpoint {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		if r.URL.Query().Get("market") != "condition" ||
			r.URL.Query().Get("asset_id") != "123" ||
			r.URL.Query().Get("session_keys") != walletOrderSigner {
			t.Errorf("unexpected wallet filters: %s", r.URL.RawQuery)
		}
		if requests.Add(1) == 1 {
			if _, hasCursor := r.URL.Query()["cursor"]; hasCursor {
				t.Error("first page unexpectedly sent a cursor")
			}
			_, _ = io.WriteString(w, "{\"data\":[],\"next_cursor\":\"more\"}")
			return
		}
		if r.URL.Query().Get("cursor") != "more" {
			t.Errorf("continuation cursor: %s", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `{"data":[{"id":"order-1","asset_id":123,"market":"condition",
			"signer_address":"0x2222222222222222222222222222222222222222",
			"maker_address":"0x1111111111111111111111111111111111111111",
			"owner":"","side":"BUY","price":0.45,"original_size":10.25,
			"size_matched":1.25,"outcome":"Yes","order_type":"GTC","status":"LIVE",
			"created_at":1700000000,"expiration":"0"}],"next_cursor":""}`)
	}))
	defer server.Close()

	client := newSessionOwner(t, server.URL)
	params := WalletOpenOrdersParams{
		Market: "condition", AssetID: "123", SignerAddresses: []string{walletOrderSigner},
	}
	var orders []WalletOpenOrder
	for order, err := range client.IterWalletOpenOrders(t.Context(), params) {
		if err != nil {
			t.Fatalf("iterate wallet orders: %v", err)
		}
		orders = append(orders, order)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("wallet order requests = %d, want 2", got)
	}
	if len(orders) != 1 {
		t.Fatalf("wallet orders = %+v, want one order", orders)
	}
	order := orders[0]
	if order.AssetID != "123" || order.Price != "0.45" ||
		order.OriginalSize != "10.25" || order.SizeMatched != "1.25" ||
		order.Owner != nil || order.SignerAddress != walletOrderSigner ||
		order.MakerAddress != walletOrderMaker ||
		!order.CreatedAt.Equal(time.Unix(1700000000, 0).UTC()) ||
		order.ExpiresAt != nil {
		t.Fatalf("wallet order lost financial/identity/timestamp fields: %+v", order)
	}
}

func TestWalletOrdersOwnerCancellationAndPartialFailures(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var bodies []string
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path != cancelWalletOrdersEndpoint && r.URL.Path != walletOpenOrdersEndpoint {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request: %v", err)
		}
		bodies = append(bodies, string(raw))
		methods = append(methods, r.Method)
		_, _ = io.WriteString(w, `{"canceled":[],"not_canceled":{"missing":"not found"}}`)
	}))
	defer server.Close()

	client := newSessionOwner(t, server.URL)
	out, err := client.CancelWalletOrders(t.Context(), []string{walletOrderHash})
	if err != nil {
		t.Fatalf("cancel selected: %v", err)
	}
	if out.NotCanceled["missing"] != "not found" {
		t.Fatalf("lost partial failures: %+v", out)
	}
	if _, err := client.CancelAllWalletOrders(t.Context()); err != nil {
		t.Fatalf("cancel all: %v", err)
	}
	mu.Lock()
	if len(bodies) != 2 || methods[0] != http.MethodDelete || methods[1] != http.MethodDelete {
		t.Fatalf("unexpected requests: %v / %v", methods, bodies)
	}
	var selected, all map[string]any
	if err := json.Unmarshal([]byte(bodies[0]), &selected); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(bodies[1]), &all); err != nil {
		t.Fatal(err)
	}
	if got := selected["order_ids"]; got == nil || selected["all"] != nil {
		t.Fatalf("bad selected cancellation body: %#v", selected)
	}
	if all["all"] != true || all["order_ids"] != nil {
		t.Fatalf("bad cancel-all body: %#v", all)
	}
	mu.Unlock()

	// Owner and a session signer share a Deposit Wallet. Only the owner
	// may execute a wallet-wide cancellation.
	client.funderAddress = walletOrderMaker
	if _, err := client.CancelAllWalletOrders(t.Context()); !errors.Is(err, ErrWalletOrdersOwnerRequired) {
		t.Fatalf("session cancellation error = %v", err)
	}
	page, err := client.GetWalletOpenOrdersPage(t.Context(), WalletOpenOrdersParams{SignerAddresses: []string{walletOrderSigner}})
	if err == nil || page != nil {
		// This fixture has no listing handler; the point is that a session
		// signer passes the local permission check and reaches the server.
		t.Fatalf("session signer should reach server listing: page=%+v err=%v", page, err)
	}
	mu.Lock()
	if len(bodies) != 3 || methods[2] != http.MethodGet {
		t.Fatalf("session signer should read but not cancel: %v", methods)
	}
	mu.Unlock()
}

func TestWalletOrderValidationAndCursorCycle(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = io.WriteString(w, `{"data":[],"next_cursor":"repeat"}`)
	}))
	defer server.Close()

	client := newSessionOwner(t, server.URL)
	for _, ids := range [][]string{
		nil, {}, {walletOrderHash[:65]}, {"0x" + strings.Repeat("z", 64)},
	} {
		if _, err := client.CancelWalletOrders(t.Context(), ids); err == nil {
			t.Fatalf("accepted invalid wallet order hashes: %v", ids)
		}
	}
	if _, err := client.CancelWalletOrders(t.Context(), make([]string, 1001)); err == nil {
		t.Fatal("accepted 1001 order hashes")
	}
	for _, signers := range [][]string{{}, {"not-an-address"}, make([]string, 51)} {
		if _, err := client.GetWalletOpenOrdersPage(t.Context(), WalletOpenOrdersParams{SignerAddresses: signers}); err == nil {
			t.Fatalf("accepted invalid signer filter of length %d", len(signers))
		}
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("invalid requests reached network: %d", got)
	}

	count := 0
	var lastErr error
	for _, err := range client.IterWalletOpenOrders(t.Context(), WalletOpenOrdersParams{}) {
		if err != nil {
			lastErr = err
			break
		}
		count++
	}
	if count != 0 || !errors.Is(lastErr, ErrPaginationStalled) || requests.Load() != 2 {
		t.Fatalf("cursor cycle: items=%d err=%v requests=%d", count, lastErr, requests.Load())
	}

	client.signatureType = SignatureTypeEOA
	if _, err := client.GetWalletOpenOrdersPage(t.Context(), WalletOpenOrdersParams{}); !errors.Is(err, ErrWalletOrdersDepositWalletRequired) {
		t.Fatalf("non-deposit wallet was accepted: %v", err)
	}
}

func TestWalletOrderPageRejectsMissingCursor(t *testing.T) {
	t.Parallel()
	var page WalletOpenOrdersPage
	if err := json.Unmarshal([]byte(`{"data":[]}`), &page); err == nil {
		t.Fatal("wallet page silently treated missing cursor as completion")
	}
	if err := json.Unmarshal([]byte(`{"next_cursor":""}`), &page); err == nil {
		t.Fatal("wallet page silently treated missing data as empty page")
	}
}
