package clob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

const (
	comboNativeYes       = "1356938545749799165119972480570561420155507632800475359837393562592731987968"
	comboNativeNo        = "1356938545749799165119972480570561420155507632800475359837393562592731987969"
	comboNativeCondition = "0x0300000000000000000000000000000000000000000000000000000000000000"
)

func comboWSRead(ctx context.Context, conn *websocket.Conn) (map[string]json.RawMessage, error) {
	_, data, err := conn.Read(ctx)
	if err != nil {
		return nil, err
	}
	var value map[string]json.RawMessage
	err = json.Unmarshal(data, &value)
	return value, err
}

func comboWSWrite(ctx context.Context, conn *websocket.Conn, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, data)
}

func comboWSString(value map[string]json.RawMessage, key string) string {
	var s string
	_ = json.Unmarshal(value[key], &s)
	return s
}

func newComboWSServer(t *testing.T, handle func(context.Context, *websocket.Conn)) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.CloseNow()
		auth, err := comboWSRead(r.Context(), conn)
		if err != nil {
			return
		}
		var credentials struct {
			APIKey     string `json:"apiKey"`
			Secret     string `json:"secret"`
			Passphrase string `json:"passphrase"`
		}
		var identity struct {
			Signer        string `json:"signer_address"`
			Maker         string `json:"maker_address"`
			SignatureType int    `json:"signature_type"`
		}
		_ = json.Unmarshal(auth["auth"], &credentials)
		_ = json.Unmarshal(auth["identity"], &identity)
		if comboWSString(auth, "type") != "auth" || credentials.APIKey != "api-key" ||
			credentials.Secret != "c2VjcmV0" ||
			credentials.Passphrase != "pass" ||
			identity.Signer != "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266" ||
			identity.Maker != identity.Signer ||
			identity.SignatureType != 0 {
			t.Errorf("invalid quoter authentication: %s", auth)
			return
		}
		handle(r.Context(), conn)
	}))
	t.Cleanup(server.Close)
	return "ws" + strings.TrimPrefix(server.URL, "http")
}

func comboQuoteRequest() ComboRFQQuoteRequest {
	return ComboRFQQuoteRequest{
		RFQID:         "rfq-1",
		YesPositionID: comboNativeYes,
		NoPositionID:  comboNativeNo,
		Direction:     RFQDirectionBuy,
		Side:          RFQSideYes,
		RequestedSize: ComboRFQRequestedSize{Unit: RFQSizeUnitShares, Value: "1.000001"},
	}
}

func TestComboQuoterFlow(t *testing.T) {
	t.Parallel()
	url := newComboWSServer(t, func(ctx context.Context, conn *websocket.Conn) {
		if comboWSWrite(ctx, conn, map[string]any{"type": "auth", "success": true}) != nil {
			return
		}
		for _, frame := range []string{`{"type":"future"}`, `{"type":"RFQ_REQUEST"}`, `{broken`} {
			_ = conn.Write(ctx, websocket.MessageText, []byte(frame))
		}
		market := map[string]any{
			"rfq_id":           "rfq-1",
			"leg_position_ids": []string{comboNativeYes, comboNativeNo},
			"condition_id":     comboNativeCondition,
			"yes_position_id":  comboNativeYes,
			"no_position_id":   comboNativeNo,
			"direction":        "BUY",
			"side":             "YES",
		}
		market["type"], market["requestor_public_id"], market["requested_size"], market["submission_deadline"] = "RFQ_REQUEST", "requester", map[string]any{
			"unit":     "shares",
			"value_e6": "1000001",
		}, 1700000000000
		if comboWSWrite(ctx, conn, market) != nil {
			return
		}
		quote, err := comboWSRead(ctx, conn)
		if err != nil {
			return
		}
		var order comboQuoterOrder
		_ = json.Unmarshal(quote["signed_order"], &order)
		if comboWSString(quote, "type") != "RFQ_QUOTE" ||
			comboWSString(quote, "price_e6") != "333333" ||
			comboWSString(quote, "size_e6") != "1000001" ||
			order.TokenID != comboNativeNo ||
			order.MakerAmount != "666668" ||
			order.TakerAmount != "1000001" ||
			order.Side != 0 ||
			order.Builder != zeroBytes32 ||
			!strings.HasPrefix(order.Signature, "0x") {
			t.Errorf("bad quote: %+v %+v", quote, order)
		}
		var rawOrder map[string]json.RawMessage
		_ = json.Unmarshal(quote["signed_order"], &rawOrder)
		if rawOrder["expiration"] != nil || string(rawOrder["side"]) != "0" {
			t.Error("quoter order side/expiration wire mismatch")
		}
		_ = comboWSWrite(
			ctx,
			conn,
			map[string]any{"type": "ACK_RFQ_QUOTE", "rfq_id": "rfq-1", "quote_id": "quote-1"},
		)
		market["type"], market["quote_id"], market["signer_address"], market["maker_address"], market["signature_type"], market["fill_size_e6"], market["price_e6"], market["confirm_by"] = "RFQ_CONFIRMATION_REQUEST", "quote-1", order.Signer, order.Maker, 0, "1000001", "333333", 1700000000001
		_ = comboWSWrite(ctx, conn, market)
		confirmation, err := comboWSRead(ctx, conn)
		if err != nil {
			return
		}
		if comboWSString(confirmation, "type") != "RFQ_CONFIRMATION_RESPONSE" ||
			comboWSString(confirmation, "decision") != "DECLINE" {
			t.Errorf("bad confirmation: %s", confirmation)
		}
		_ = comboWSWrite(
			ctx,
			conn,
			map[string]any{
				"type":     "ACK_RFQ_CONFIRMATION_RESPONSE",
				"rfq_id":   "rfq-1",
				"quote_id": "quote-1",
				"decision": "DECLINE",
			},
		)
		cancel, err := comboWSRead(ctx, conn)
		if err != nil {
			return
		}
		if comboWSString(cancel, "type") != "RFQ_QUOTE_CANCEL" ||
			comboWSString(cancel, "signer_address") != order.Signer ||
			comboWSString(cancel, "maker_address") != order.Maker {
			t.Errorf("bad cancel: %s", cancel)
		}
		// An unrelated quote's cancellation ack must not resolve this cancellation.
		_ = comboWSWrite(
			ctx,
			conn,
			map[string]any{"type": "ACK_RFQ_QUOTE_CANCEL", "rfq_id": "rfq-1", "quote_id": "other"},
		)
		_ = comboWSWrite(
			ctx,
			conn,
			map[string]any{
				"type":     "ACK_RFQ_QUOTE_CANCEL",
				"rfq_id":   "rfq-1",
				"quote_id": "quote-1",
			},
		)
		_ = comboWSWrite(
			ctx,
			conn,
			map[string]any{
				"type":    "RFQ_EXECUTION_UPDATE",
				"rfq_id":  "rfq-1",
				"status":  "CONFIRMED",
				"tx_hash": "0x" + strings.Repeat("ab", 32),
			},
		)
		market["type"], market["requester_id"], market["size_e6"], market["executed_at"] = "RFQ_TRADE", "requester", "1000001", 1700000000002
		_ = comboWSWrite(ctx, conn, market)
		_, _, _ = conn.Read(ctx)
	})
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	client := newComboTestClient(t, "https://unused.invalid")
	client.builderAuth = nil // Quoter auth is not builder REST authorization.
	session, err := client.OpenComboRFQSession(ctx, ComboRFQSessionOptions{URL: url})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	event := <-session.Events()
	request, ok := event.(ComboRFQQuoteRequest)
	if !ok || request.RequestedSize.Value != "1.000001" ||
		request.ConditionID != comboNativeCondition[:64] {
		t.Fatalf("bad request: %+v", event)
	}
	reference, err := session.Quote(ctx, request, ComboRFQQuoteResponse{Price: "0.333333"})
	if err != nil {
		t.Fatal(err)
	}
	confirmation, ok := (<-session.Events()).(ComboRFQConfirmationRequest)
	if !ok || confirmation.FillSize != "1.000001" || confirmation.Price != "0.333333" {
		t.Fatalf("bad last look: %+v", confirmation)
	}
	if _, err := session.RespondToConfirmation(ctx, reference, ComboRFQDecline); err != nil {
		t.Fatal(err)
	}
	if ack, err := session.CancelQuote(ctx, reference); err != nil || ack != reference {
		t.Fatalf("cancellation: %+v %v", ack, err)
	}
	if update, ok := (<-session.Events()).(ComboRFQExecutionUpdate); !ok ||
		update.Status != ComboRFQConfirmed {
		t.Fatalf("bad update: %+v", update)
	}
	if trade, ok := (<-session.Events()).(ComboRFQTrade); !ok || trade.Size != "1.000001" {
		t.Fatalf("bad trade: %+v", trade)
	}
}

func TestComboQuoterCommandErrorsAndReconnect(t *testing.T) {
	t.Parallel()
	var connections atomic.Int32
	url := newComboWSServer(t, func(ctx context.Context, conn *websocket.Conn) {
		connections.Add(1)
		_ = comboWSWrite(ctx, conn, map[string]any{"type": "auth", "success": true})
		_, err := comboWSRead(ctx, conn)
		if err != nil {
			return
		}
		_ = comboWSWrite(
			ctx,
			conn,
			map[string]any{
				"type":         "RFQ_ERROR",
				"request_type": "RFQ_QUOTE_CANCEL",
				"rfq_id":       "r",
				"quote_id":     "q",
				"code":         "FUTURE_CODE",
				"error_id":     "error-1",
				"error":        "rejected",
			},
		)
		_, err = comboWSRead(ctx, conn)
		if err != nil {
			return
		}
		_ = comboWSWrite(
			ctx,
			conn,
			map[string]any{
				"type":         "RFQ_ERROR",
				"request_type": "RFQ_CONFIRMATION_RESPONSE",
				"code":         "INVALID_MESSAGE",
				"error":        "missing correlation",
			},
		)
		_, _, _ = conn.Read(ctx)
	})
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	client := newComboTestClient(t, "https://unused.invalid")
	session, err := client.OpenComboRFQSession(ctx, ComboRFQSessionOptions{URL: url})
	if err != nil {
		t.Fatal(err)
	}
	ref := ComboRFQQuoteReference{RFQID: "r", QuoteID: "q"}
	_, err = session.CancelQuote(ctx, ref)
	var rejection *ComboRFQCommandError
	if !errors.As(err, &rejection) || rejection.Code != "FUTURE_CODE" ||
		rejection.ErrorID != "error-1" ||
		rejection.QuoteID != "q" {
		t.Fatalf("lost command error: %v", err)
	}
	if _, err := session.RespondToConfirmation(ctx, ref, ComboRFQConfirm); err == nil {
		t.Fatal("uncorrelated error did not fail pending command")
	}
	<-session.Done()
	if session.Err() == nil {
		t.Fatal("missing protocol failure")
	}
	// Reopening is explicit and creates a fresh authenticated generation.
	next, err := client.OpenComboRFQSession(ctx, ComboRFQSessionOptions{URL: url})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() { _ = session.Close() })
	}
	wg.Wait()
	select {
	case <-next.Done():
		t.Fatal("old close closed new session")
	default:
	}
	_ = next.Close()
	if connections.Load() != 2 {
		t.Fatalf("connections = %d", connections.Load())
	}
}

func TestComboQuoterTimeoutAndConcurrentClose(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"auth timeout", "ack timeout", "close", "owner cancellation", "duplicate"} {
		t.Run(mode, func(t *testing.T) {
			received := make(chan struct{}, 1)
			url := newComboWSServer(t, func(ctx context.Context, conn *websocket.Conn) {
				if mode == "auth timeout" {
					_, _, _ = conn.Read(ctx)
					return
				}
				_ = comboWSWrite(ctx, conn, map[string]any{"type": "auth", "success": true})
				if _, err := comboWSRead(ctx, conn); err != nil {
					return
				}
				received <- struct{}{}
				// Malformed/unknown acks do not settle commands.
				_ = conn.Write(
					ctx,
					websocket.MessageText,
					[]byte(`{"type":"ACK_RFQ_QUOTE_CANCEL","rfq_id":"r"}`),
				)
				_, _, _ = conn.Read(ctx)
			})
			owner, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			client := newComboTestClient(t, "https://unused.invalid")
			options := ComboRFQSessionOptions{
				URL:         url,
				AckTimeout:  100 * time.Millisecond,
				AuthTimeout: 100 * time.Millisecond,
			}
			if mode != "auth timeout" && mode != "ack timeout" {
				options.AckTimeout = 2 * time.Second
			}
			session, err := client.OpenComboRFQSession(owner, options)
			if mode == "auth timeout" {
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("auth timeout: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			result := make(chan error, 1)
			ref := ComboRFQQuoteReference{RFQID: "r", QuoteID: "q"}
			go func() { _, err := session.CancelQuote(owner, ref); result <- err }()
			<-received
			switch mode {
			case "duplicate":
				if _, err := session.CancelQuote(owner, ref); !errors.Is(
					err,
					ErrComboRFQCommandPending,
				) {
					t.Fatalf("duplicate command: %v", err)
				}
				_ = session.Close()
			case "close":
				var wg sync.WaitGroup
				for range 16 {
					wg.Go(func() { _ = session.Close() })
				}
				wg.Wait()
			case "owner cancellation":
				cancel()
			}
			if err := <-result; err == nil {
				t.Fatal("pending command succeeded without an ack")
			}
			<-session.Done()
			_ = session.Close()
			if _, err := session.CancelQuote(t.Context(), ref); !errors.Is(
				err,
				ErrComboRFQSessionClosed,
			) {
				t.Fatalf("closed command: %v", err)
			}
		})
	}
}

func TestComboQuoterExactQuoteAmounts(t *testing.T) {
	t.Parallel()
	client := newComboTestClient(t, "https://unused.invalid")
	for _, tc := range []struct {
		direction    RFQDirection
		source       ComboQuoteSource
		token        string
		side         uint8
		maker, taker string
	}{
		{RFQDirectionBuy, ComboQuoteCollateral, comboNativeNo, 0, "666668", "1000001"},
		{RFQDirectionBuy, ComboQuoteInventory, comboNativeYes, 1, "1000001", "333333"},
		{RFQDirectionSell, ComboQuoteCollateral, comboNativeYes, 0, "333334", "1000001"},
		{RFQDirectionSell, ComboQuoteInventory, comboNativeNo, 1, "1000001", "666667"},
	} {
		request := comboQuoteRequest()
		request.Direction = tc.direction
		quote, err := client.buildComboQuoterQuote(
			t.Context(),
			request,
			ComboRFQQuoteResponse{Price: "0.333333", Source: tc.source},
		)
		if err != nil {
			t.Fatal(err)
		}
		order := quote.SignedOrder
		if order.TokenID != tc.token || order.Side != tc.side || order.MakerAmount != tc.maker ||
			order.TakerAmount != tc.taker {
			t.Fatalf("%s/%s: %+v", tc.direction, tc.source, order)
		}
	}
	request := comboQuoteRequest()
	request.RequestedSize = ComboRFQRequestedSize{Unit: RFQSizeUnitNotional, Value: "1"}
	quote, err := client.buildComboQuoterQuote(
		t.Context(),
		request,
		ComboRFQQuoteResponse{Price: "0.333333"},
	)
	if err != nil || quote.SizeE6 != "3000003" {
		t.Fatalf("notional size: %+v %v", quote, err)
	}
	for _, response := range []ComboRFQQuoteResponse{{Price: "0"}, {Price: "1"}, {Price: "0.0000001"}, {Price: "0.5", Size: "0"}, {Price: "0.5", Source: "future"}, {Price: "0.000001", Source: ComboQuoteInventory, Size: "0.000001"}} {
		if _, err := client.buildComboQuoterQuote(t.Context(), request, response); err == nil {
			t.Fatalf("accepted invalid quote %+v", response)
		}
	}
}

// Independent eth-account 0.14.0 EIP-712 fixtures, not outputs of the Go signer.
// Deposit fixture signs TypedDataSign and appends domain/contents/type bytes.
func TestComboSigningIndependentFixtures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		deposit   bool
		signature string
	}{
		{false, "0xe30ea4135e5722d8dbea4b1326307c45d8c21a4af68b141ca569aba15d8f321d3f63f0afe56f5fb632efe8dd22ef2bcede3ed0694740fba80a7a748e0c043b7b1b"},
		{true, "0xd25daaaec3ae84a8692dbc68421619cd7ffd0ad0176d078de2aa861763d557e213580f2c702f87dd888df52fb1c69c76210126f7398bcf92090e5058b1588cb51b466c63910185bbd55e8679264200c4e0abdcbb0c6264eb3d41d13326022e095bcbfe427928910a4ceaeda15cd490592ac8f7d2f0950fd5d8fa7fcef7bb6e77eb4f726465722875696e743235362073616c742c61646472657373206d616b65722c61646472657373207369676e65722c75696e7432353620746f6b656e49642c75696e74323536206d616b6572416d6f756e742c75696e743235362074616b6572416d6f756e742c75696e743820736964652c75696e7438207369676e6174757265547970652c75696e743235362074696d657374616d702c62797465733332206d657461646174612c62797465733332206275696c6465722900ba"},
	} {
		client := newComboTestClient(t, "https://unused.invalid")
		address := client.Address()
		if tc.deposit {
			client.signatureType = SignatureTypePoly1271
			address = "0x1111111111111111111111111111111111111111"
		}
		order := comboSignedOrderWire{
			Salt:          "1",
			Maker:         address,
			Signer:        address,
			TokenID:       comboNativeYes,
			MakerAmount:   "666667",
			TakerAmount:   "1000001",
			Side:          SideBuy,
			SignatureType: client.signatureType,
			Timestamp:     "1700000000",
			Builder:       zeroBytes32,
			Metadata:      zeroBytes32,
		}
		if err := client.signComboOrder(t.Context(), &order, "0xe3333700cA9d93003F00f0F71f8515005F6c00Aa"); err != nil {
			t.Fatal(err)
		}
		if order.Signature != tc.signature {
			t.Fatalf("deposit=%v: signature %s", tc.deposit, order.Signature)
		}
	}
}

func TestWaitForComboFill(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		status  ComboRFQStatus
		tx      string
		failure bool
		missing bool
	}{
		{ComboRFQFilled, "0x" + strings.Repeat("ab", 32), false, false},
		{ComboRFQConfirmed, "0x" + strings.Repeat("ab", 32), false, false},
		{ComboRFQFailed, "", true, false},
		{ComboRFQExpired, "", true, false},
		{ComboRFQCanceled, "", true, false},
		{ComboRFQFilled, "", false, true},
		{ComboRFQConfirmed, "", false, true},
		{ComboRFQFilled, "not-a-hash", false, true},
	} {
		t.Run(string(tc.status)+tc.tx, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != builderRFQRequestsEndpoint+"/rfq-1" {
						t.Errorf("unexpected status path %s", r.URL.Path)
					}
					status := tc.status
					if calls.Add(1) == 1 {
						status = ComboRFQMined
					}
					_ = json.NewEncoder(w).
						Encode(map[string]any{"rfq_id": "rfq-1", "status": status, "tx_hash": tc.tx, "error": map[string]string{"code": "MAKER_DECLINED", "message": "declined"}})
				}),
			)
			defer server.Close()
			client := newComboTestClient(t, server.URL)
			result, err := client.WaitForComboFill(
				t.Context(),
				WaitForComboFillParams{RFQID: "rfq-1", PollInterval: time.Millisecond},
			)
			if tc.missing {
				var responseErr *ComboRFQResponseError
				if !errors.As(err, &responseErr) {
					t.Fatalf("premature settlement: %+v %v", result, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if tc.failure {
					if result.Status != tc.status || result.Error == nil {
						t.Fatalf("lost terminal failure: %+v", result)
					}
				} else if result.Status != ComboRFQFilled || result.TxHash != tc.tx {
					t.Fatalf("bad fill: %+v", result)
				}
			}
			if calls.Load() != 2 {
				t.Fatalf("did not poll through MINED: %d", calls.Load())
			}
		})
	}
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"rfq_id":"rfq-1","status":"EXECUTING"}`) },
		),
	)
	defer server.Close()
	client := newComboTestClient(t, server.URL)
	if _, err := client.WaitForComboFill(t.Context(), WaitForComboFillParams{RFQID: "rfq-1", Timeout: 20 * time.Millisecond, PollInterval: time.Second}); !errors.Is(
		err,
		context.DeadlineExceeded,
	) {
		t.Fatalf("fill timeout: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := client.WaitForComboFill(ctx, WaitForComboFillParams{RFQID: "rfq-1"}); !errors.Is(
		err,
		context.Canceled,
	) {
		t.Fatalf("fill cancellation: %v", err)
	}
}
