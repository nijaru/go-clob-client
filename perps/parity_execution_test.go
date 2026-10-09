package perps

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/coder/websocket"
)

// Contracts are the signed REST operations in websockets/perps/actions/{twaps,chases}.ts
// and the compact active-run records in bindings/perps/{twaps,chases}.ts.
func TestServerManagedExecutions(t *testing.T) {
	var submissions atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("POLYMARKET-PROXY") != fixtureProxy ||
			r.Header.Get("POLYMARKET-SECRET") != "fixture-secret" {
			t.Error("execution credentials missing")
		}
		if r.Method == http.MethodGet {
			switch r.URL.Path {
			case "/v1/account/twaps":
				_, _ = w.Write(
					[]byte(
						`[{"twid":1,"iid":1,"buy":true,"qty":"1.25","fill":"0.25","dur":300000,"ivl":30000,"rnd":false,"slip_bps":0,"min_px":"0","max_px":"0","ro":false,"st":"running","slices":1,"slice_count":10,"sts":1000,"ets":301000,"cts":1000,"avg_px":"100","coid":"` + fixtureClientID + `","oid":42}]`,
					),
				)
			case "/v1/account/chases":
				_, _ = w.Write(
					[]byte(
						`[{"chid":2,"iid":1,"buy":false,"qty":"2","fill":"0","lim":"0","max_dist":"0","max_dist_bps":100,"po":true,"ro":false,"reference_price":"100","reprices":3,"post_only_rejections":2,"cts":1000,"coid":"` + fixtureClientID + `","oid":43}]`,
					),
				)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
			return
		}
		submissions.Add(1)
		var c fixtureCommand
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			t.Error(err)
			return
		}
		switch c.Op.Type {
		case "createTwap":
			var args struct {
				IID      int     `json:"iid"`
				Buy      bool    `json:"buy"`
				Qty      string  `json:"qty"`
				Dur      int64   `json:"dur"`
				Interval *int64  `json:"ivl"`
				Rnd      bool    `json:"rnd"`
				Slip     int     `json:"slip_bps"`
				Min      *string `json:"min_px"`
				Max      *string `json:"max_px"`
				RO       bool    `json:"ro"`
			}
			_ = json.Unmarshal(c.Op.Args, &args)
			if args.Qty != "1.25" || args.Interval != nil || args.Min != nil || args.Max != nil ||
				c.ExpiresAt != 1893456000123 {
				t.Errorf("TWAP args=%+v exp=%d", args, c.ExpiresAt)
			}
			assertOperationSignature(
				t,
				c,
				[]any{"createTwap", []any{1, true, "1.25", int64(300000), false, 0, false}},
			)
			_, _ = w.Write([]byte(`{"status":"ok","twid":1,"ts":1000}`))
		case "controlTwap":
			var args struct {
				ID     int64  `json:"twid"`
				Action string `json:"act"`
			}
			_ = json.Unmarshal(c.Op.Args, &args)
			if r.Method != http.MethodPatch || args.ID != 1 {
				t.Error("invalid TWAP control")
			}
			assertOperationSignature(t, c, []any{"controlTwap", []any{int64(1), args.Action}})
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "cancelTwap":
			if r.Method != http.MethodDelete {
				t.Error("cancelTwap must DELETE")
			}
			assertOperationSignature(t, c, []any{"cancelTwap", []any{int64(1)}})
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "createChase":
			var args struct {
				PO  bool   `json:"po"`
				Bps int    `json:"max_dist_bps"`
				Qty string `json:"qty"`
			}
			_ = json.Unmarshal(c.Op.Args, &args)
			if !args.PO || args.Bps != 100 || args.Qty != "2" {
				t.Errorf("chase args=%+v", args)
			}
			assertOperationSignature(
				t,
				c,
				[]any{"createChase", []any{1, false, "2", 100, true, false}},
			)
			_, _ = w.Write([]byte(`{"status":"ok","chid":2,"ts":1000}`))
		case "cancelChase":
			if r.Method != http.MethodDelete {
				t.Error("cancelChase must DELETE")
			}
			assertOperationSignature(t, c, []any{"cancelChase", []any{int64(2)}})
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			t.Errorf("unexpected op=%s", c.Op.Type)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)
	client := fixtureClient(t, Config{Host: server.URL})
	twap, err := client.CreateTWAP(
		t.Context(),
		TWAPRequest{
			InstrumentID: 1,
			Side:         PerpsOrderBuy,
			Quantity:     "1.2500",
			DurationMs:   300000,
			ExpiresAt:    1893456000123,
		},
	)
	if err != nil || twap.ID != 1 {
		t.Fatalf("TWAP=%+v %v", twap, err)
	}
	active, err := client.GetTWAPs(t.Context())
	if err != nil || len(active) != 1 || active[0].SliceCount != 10 ||
		active[0].FilledQuantity != "0.25" ||
		active[0].OrderID == nil {
		t.Fatalf("active TWAP=%+v %v", active, err)
	}
	for _, action := range []string{"pause", "resume", "cancel"} {
		if err := client.ControlTWAP(t.Context(), 1, action, 0); err != nil {
			t.Fatal(err)
		}
	}
	bps := 100
	chase, err := client.CreateChase(
		t.Context(),
		ChaseRequest{InstrumentID: 1, Side: PerpsOrderSell, Quantity: "2", MaxDistanceBps: &bps},
	)
	if err != nil || chase.ID != 2 {
		t.Fatalf("chase=%+v %v", chase, err)
	}
	chases, err := client.GetChases(t.Context())
	if err != nil || len(chases) != 1 || chases[0].Reprices != 3 ||
		chases[0].PostOnlyRejections != 2 {
		t.Fatalf("active chases=%+v %v", chases, err)
	}
	if err := client.CancelChase(t.Context(), 2, 0); err != nil {
		t.Fatal(err)
	}
	before := submissions.Load()
	interval := int64(31000)
	for _, p := range []TWAPRequest{{InstrumentID: 1, Side: PerpsOrderBuy, Quantity: "1", DurationMs: 300000, IntervalMs: &interval}, {InstrumentID: 1, Side: PerpsOrderBuy, Quantity: "1", DurationMs: 300000, MinPrice: "100", MaxPrice: "99"}, {InstrumentID: 1, Side: PerpsOrderBuy, Quantity: "79228162514264337593543950336", DurationMs: 300000}, {InstrumentID: 1, Side: PerpsOrderBuy, Quantity: "1", DurationMs: 1}} {
		if _, err := client.CreateTWAP(t.Context(), p); err == nil {
			t.Fatalf("accepted invalid TWAP=%+v", p)
		}
	}
	if _, err := client.CreateChase(t.Context(), ChaseRequest{InstrumentID: 1, Side: PerpsOrderBuy, Quantity: "1", MaxDistance: "1", MaxDistanceBps: &bps}); err == nil {
		t.Fatal("ambiguous chase bound accepted")
	}
	if submissions.Load() != before {
		t.Fatal("invalid execution submitted")
	}
}

func TestExecutionRejectionAndUnknownOutcomeAreNotRetried(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		if attempts.Load() == 1 {
			_, _ = w.Write([]byte(`{"status":"err","error":"insufficient_margin"}`))
		} else {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":"unknown outcome"}`))
		}
	}))
	t.Cleanup(server.Close)
	client := fixtureClient(t, Config{Host: server.URL})
	_, err := client.CreateChase(
		t.Context(),
		ChaseRequest{InstrumentID: 1, Side: PerpsOrderBuy, Quantity: "1"},
	)
	var rejected *CommandError
	if !errors.As(err, &rejected) || rejected.Code != "insufficient_margin" {
		t.Fatalf("rejection=%v", err)
	}
	_, err = client.CreateTWAP(
		t.Context(),
		TWAPRequest{InstrumentID: 1, Side: PerpsOrderBuy, Quantity: "1", DurationMs: 300000},
	)
	if err == nil || attempts.Load() != 2 {
		t.Fatalf("error=%v attempts=%d", err, attempts.Load())
	}
}

func TestBatchLeverageKeepsItemRejections(t *testing.T) {
	s := fixtureSession(t, func(ctx context.Context, conn *websocket.Conn, c fixtureCommand) {
		assertOperationSignature(
			t,
			c,
			[]any{"updateLeverages", []any{[]any{1, 5, true}, []any{2, 10, false}}},
		)
		fixtureReply(
			t,
			ctx,
			conn,
			c.ID,
			`[{"status":"ok","instrument_id":1,"leverage":5,"cross":true},{"status":"err","instrument_id":2,"error":"risk_limit"}]`,
		)
	})
	out, err := s.UpdateLeverages(t.Context(), []LeverageUpdate{{1, 5, true}, {2, 10, false}})
	if err != nil || len(out) != 2 || out[0].Leverage != 5 || out[1].Error != "risk_limit" {
		t.Fatalf("results=%+v error=%v", out, err)
	}
	if _, err := s.UpdateLeverages(t.Context(), []LeverageUpdate{{1, 5, true}, {1, 10, false}}); err == nil {
		t.Fatal("duplicate leverage update submitted")
	}
}

func TestPositionTPSLPartialAndFullExitDirection(t *testing.T) {
	rest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/account/portfolio" {
			t.Error(r.URL.Path)
		}
		_, _ = w.Write(
			[]byte(
				`{"positions":[{"instrument_id":1,"symbol":"BTC-USDC","size":"-2","entry_price":"100","leverage":5,"cross":true,"initial_margin":"40","maintenance_margin":"10","position_value":"200","liquidation_price":"140","unrealized_pnl":"0","return_on_equity":"0","cumulative_funding":"0"}],"margin":{"total_account_value":"1000","total_initial_margin":"40","total_maintenance_margin":"10","total_position_value":"200"},"withdrawable":"960","in_liquidation":false,"timestamp":1000}`,
			),
		)
	}))
	t.Cleanup(rest.Close)
	s := fixtureSession(t, func(ctx context.Context, conn *websocket.Conn, c fixtureCommand) {
		if c.Op.Group != "position" {
			t.Errorf("group=%s", c.Op.Group)
		}
		assertOperationSignature(
			t,
			c,
			[]any{
				"createOrders",
				[]any{
					[]any{1, true, "0.25", false, true, []any{true, "90", "tp"}},
					[]any{1, true, "0", false, true, []any{true, "sl", 100, "110"}},
				},
				"position",
			},
		)
		fixtureReply(t, ctx, conn, c.ID, `[{"status":"ok","oid":1},{"status":"ok","oid":2}]`)
	}, rest.URL)
	out, err := s.PostPositionTPSL(
		t.Context(),
		PositionTPSLRequest{
			InstrumentID: 1,
			TakeProfit:   &TPSLTrigger{TriggerPrice: "90", Quantity: "0.25"},
			StopLoss:     &TPSLTrigger{TrailingBps: 100, ActivationPrice: "110"},
		},
	)
	if err != nil || len(out) != 2 || out[1].OrderID != 2 {
		t.Fatalf("exits=%+v %v", out, err)
	}
	if _, err := s.PostPositionTPSL(t.Context(), PositionTPSLRequest{InstrumentID: 1, TakeProfit: &TPSLTrigger{TriggerPrice: "90", Quantity: "0.00000000000000000000000000001"}}); err == nil {
		t.Fatal("unrepresentable partial exit submitted")
	}
}
