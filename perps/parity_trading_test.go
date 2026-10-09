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

const (
	fixturePrivateKey = "0000000000000000000000000000000000000000000000000000000000000001"
	fixtureProxy      = "0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf"
	fixtureClientID   = "0123456789abcdef0123456789abcdef"
)

type fixtureCommand struct {
	ID       int      `json:"id"`
	Request  string   `json:"req"`
	Channels []string `json:"chs"`
	Op       struct {
		Type  string          `json:"type"`
		Args  json.RawMessage `json:"args"`
		Group string          `json:"grp"`
	} `json:"op"`
	Salt      uint64 `json:"salt"`
	Timestamp int64  `json:"ts"`
	Signature string `json:"sig"`
	ExpiresAt int64  `json:"exp"`
	Label     string `json:"label"`
}

func fixtureClient(t *testing.T, config Config) *AuthenticatedClient {
	t.Helper()
	config.ChainID = 31337
	client, err := NewAuthenticated(
		AuthenticatedConfig{
			Config: config,
			Credentials: PerpsCredentials{
				Proxy:      fixtureProxy,
				PrivateKey: fixturePrivateKey,
				Secret:     "fixture-secret",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func fixtureSession(
	t *testing.T,
	execute func(context.Context, *websocket.Conn, fixtureCommand),
	restHost ...string,
) *Session {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		for {
			_, payload, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			var command fixtureCommand
			if err := json.Unmarshal(payload, &command); err != nil {
				t.Error(err)
				return
			}
			if command.Request == "sub" || command.Op.Type == "auth" {
				fixtureReply(t, r.Context(), conn, command.ID, `{"status":"ok"}`)
				continue
			}
			if command.Op.Type == "ping" {
				continue
			}
			execute(r.Context(), conn, command)
		}
	}))
	t.Cleanup(server.Close)
	config := Config{WebSocketHost: "ws" + strings.TrimPrefix(server.URL, "http")}
	if len(restHost) > 0 {
		config.Host = restHost[0]
	}
	client := fixtureClient(t, config)
	session, err := client.OpenSession(t.Context(), SessionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func fixtureReply(t *testing.T, ctx context.Context, conn *websocket.Conn, id int, data string) {
	t.Helper()
	payload, err := json.Marshal(struct {
		ID   int             `json:"id"`
		Data json.RawMessage `json:"data"`
	}{id, json.RawMessage(data)})
	if err != nil {
		t.Error(err)
		return
	}
	if err := conn.Write(ctx, websocket.MessageText, payload); err != nil {
		t.Error(err)
	}
}

// Wire fixtures use the cancellation discriminated union in bindings/perps/orders.ts.
// This protects ordered mixed results, transient-only retries, bounds, and preservation
// of prior results on a transport/protocol failure after a successful first attempt.
func TestCancellationResultsAndRetryBounds(t *testing.T) {
	for _, tc := range []struct {
		name         string
		second       string
		retry        CancelRetry
		wantAttempts int
		wantErr      bool
		wantStatus   string
	}{
		{"mixed_success", `[{"status":"ok","oid":2}]`, CancelRetry{}, 2, false, "ok"},
		{"bounded_attempts", `[{"status":"err","oid":2,"error":"order_in_flight"}]`, CancelRetry{MaxAttempts: 2}, 2, false, "err"},
		{"disabled", ``, CancelRetry{Disable: true}, 1, false, "err"},
		{"elapsed", ``, CancelRetry{MaxElapsed: time.Millisecond}, 1, false, "err"},
		{"retry_protocol_failure", `[]`, CancelRetry{}, 2, true, "err"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var attempts atomic.Int32
			s := fixtureSession(
				t,
				func(ctx context.Context, conn *websocket.Conn, c fixtureCommand) {
					var ids []int
					if err := json.Unmarshal(c.Op.Args, &ids); err != nil {
						t.Error(err)
						return
					}
					if c.Op.Type != "cancelOrders" {
						t.Errorf("op = %q", c.Op.Type)
					}
					assertOperationSignature(t, c, []any{"cancelOrders", ids})
					attempt := attempts.Add(1)
					if attempt == 1 {
						if !reflect.DeepEqual(ids, []int{1, 2}) {
							t.Errorf("IDs = %v", ids)
						}
						fixtureReply(
							t,
							ctx,
							conn,
							c.ID,
							`[{"status":"ok","oid":1},{"status":"err","oid":2,"error":"order_in_flight"}]`,
						)
					} else {
						if !reflect.DeepEqual(ids, []int{2}) {
							t.Errorf("retry IDs = %v", ids)
						}
						fixtureReply(t, ctx, conn, c.ID, tc.second)
					}
				},
			)
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			results, err := s.CancelOrdersWithRetry(
				ctx,
				CancelOrdersRequest{OrderIDs: []int{1, 2}, Retry: tc.retry},
			)
			if (err != nil) != tc.wantErr {
				t.Fatalf("results=%+v error=%v", results, err)
			}
			if len(results) != 2 || results[0].Status != "ok" ||
				results[1].Status != tc.wantStatus ||
				attempts.Load() != int32(tc.wantAttempts) {
				t.Fatalf("results=%+v attempts=%d", results, attempts.Load())
			}
			if tc.wantErr {
				var retryErr *CancelRetryError
				if !errors.As(err, &retryErr) ||
					!reflect.DeepEqual(retryErr.PendingIndexes, []int{1}) ||
					retryErr.Results[0].OrderID != 1 {
					t.Fatalf("retry error = %#v", err)
				}
			}
		})
	}
}

func TestCancellationRequestRejectionIsNotRetried(t *testing.T) {
	var attempts atomic.Int32
	s := fixtureSession(t, func(ctx context.Context, conn *websocket.Conn, c fixtureCommand) {
		attempts.Add(1)
		fixtureReply(t, ctx, conn, c.ID, `[{"status":"err","error":"order_in_flight"}]`)
	})
	_, err := s.CancelOrders(t.Context(), []int{1}, 0)
	var rejected *CommandError
	if !errors.As(err, &rejected) || rejected.Code != "order_in_flight" || attempts.Load() != 1 {
		t.Fatalf("error=%v attempts=%d", err, attempts.Load())
	}
}

func TestCancellationContextStopsBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	var attempts atomic.Int32
	s := fixtureSession(t, func(serverCtx context.Context, conn *websocket.Conn, c fixtureCommand) {
		attempts.Add(1)
		fixtureReply(
			t,
			serverCtx,
			conn,
			c.ID,
			`[{"status":"err","oid":1,"error":"order_in_flight"}]`,
		)
		cancel()
	})
	_, err := s.CancelOrders(ctx, []int{1}, 0)
	if !errors.Is(err, context.Canceled) || attempts.Load() != 1 {
		t.Fatalf("error=%v attempts=%d", err, attempts.Load())
	}
}

func TestBracketWaiterCapturesUpdateBeforeAcknowledgement(t *testing.T) {
	s := fixtureSession(t, func(ctx context.Context, conn *websocket.Conn, c fixtureCommand) {
		if c.Op.Type != "createOrders" || c.Op.Group != "order" {
			t.Errorf("op=%+v", c.Op)
		}
		var args []struct {
			IID     int    `json:"iid"`
			Buy     bool   `json:"buy"`
			Price   string `json:"p"`
			Qty     string `json:"qty"`
			TIF     string `json:"tif"`
			RO      bool   `json:"ro"`
			C       string `json:"c"`
			Trigger *struct {
				Kind        string `json:"tpsl"`
				Market      bool   `json:"market"`
				Price       string `json:"trp"`
				TrailingBps int    `json:"trail_bps"`
				Activation  string `json:"act"`
			} `json:"tr"`
		}
		if err := json.Unmarshal(c.Op.Args, &args); err != nil {
			t.Error(err)
			return
		}
		if len(args) != 3 {
			t.Errorf("args=%+v", args)
			return
		}
		if !validPerpsClientOrderID(args[0].C) || args[1].Buy || !args[1].RO ||
			args[1].Trigger.Kind != "tp" ||
			args[2].Trigger.TrailingBps != 100 {
			t.Errorf("bracket=%+v", args)
		}
		assertOperationSignature(
			t,
			c,
			[]any{
				"createOrders",
				[]any{
					[]any{1, true, "100", "1", "gtc", false, args[0].C},
					[]any{1, false, "1", false, true, []any{true, "120", "tp"}},
					[]any{1, false, "1", false, true, []any{true, "sl", 100}},
				},
				"order",
			},
		)
		payload, _ := json.Marshal(struct {
			Channel   string           `json:"ch"`
			Timestamp int64            `json:"ts"`
			Sequence  int64            `json:"sq"`
			Data      perpsOrderUpdate `json:"data"`
		}{"orders", 10, 1, perpsOrderUpdate{ID: 77, InstrumentID: 1, Buy: true, Price: "100", Quantity: "1", TimeInForce: PerpsTIFGTC, Status: PerpsOrderOpen, RestingQuantity: "1", FilledQuantity: "0", ClientOrderID: args[0].C}})
		_ = conn.Write(ctx, websocket.MessageText, payload)
		fixtureReply(
			t,
			ctx,
			conn,
			c.ID,
			`[{"status":"ok","oid":77},{"status":"ok","oid":78},{"status":"ok","oid":79}]`,
		)
	})
	result, err := s.PlaceOrderWithTPSL(
		t.Context(),
		OrderWithTPSLRequest{
			Order: PerpsOrderRequest{
				InstrumentID: 1,
				Side:         PerpsOrderBuy,
				Price:        "100",
				Quantity:     "1",
				TimeInForce:  PerpsTIFGTC,
			},
			TakeProfit: &TPSLTrigger{TriggerPrice: "120"},
			StopLoss:   &TPSLTrigger{TrailingBps: 100},
		},
	)
	if err != nil || result.Order == nil || result.Order.ID != 77 ||
		len(result.Acknowledgements) != 3 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
}

func TestAdvancedOrderValidationBeforeSubmission(t *testing.T) {
	for _, trigger := range []triggerOrder{
		{kind: "tp", trigger: TPSLTrigger{TrailingBps: 100}},
		{kind: "sl", trigger: TPSLTrigger{TrailingBps: 9}},
		{kind: "sl", trigger: TPSLTrigger{TrailingBps: 100, TriggerPrice: "90"}},
		{kind: "sl", trigger: TPSLTrigger{TriggerPrice: "90", ActivationPrice: "100"}},
		{kind: "sl", trigger: TPSLTrigger{TrailingBps: 100, ActivationPrice: "0"}},
	} {
		if _, _, err := trigger.wire(); err == nil {
			t.Fatalf("accepted malformed trigger %+v", trigger)
		}
	}
	valid := PerpsOrderRequest{
		InstrumentID: 1,
		Side:         PerpsOrderBuy,
		Price:        "10",
		Quantity:     "1",
		TimeInForce:  PerpsTIFGTD,
		GTDExpiry:    1893456000123,
	}
	for _, modify := range []func(*PerpsOrderRequest){func(p *PerpsOrderRequest) { p.GTDExpiry = 1 }, func(p *PerpsOrderRequest) { p.TimeInForce = PerpsTIFGTC }, func(p *PerpsOrderRequest) { p.Price = "" }, func(p *PerpsOrderRequest) { p.GTDExpiry = 18446744073710 }} {
		order := valid
		modify(&order)
		if _, _, err := perpsOrderWire(order); err == nil {
			t.Fatalf("accepted malformed GTD %+v", order)
		}
	}
}
