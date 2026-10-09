package data

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := NewClient(Config{Host: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestServerCursorTraversal(t *testing.T) {
	var calls atomic.Int32
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		q := r.URL.Query()
		if r.URL.Path != "/v2/trades" || q.Get("start") != "1" || q.Get("taker_only") != "false" ||
			q.Get("limit") != "3" ||
			q.Has("offset") {
			t.Errorf("incorrect v2 request: %s", r.URL)
		}
		var reply string
		switch q.Get("cursor") {
		case "resume/opaque+1":
			reply = fmt.Sprintf(
				`{"data":[%s],"pagination":{"has_more":true,"next_cursor":"opaque+2"}}`,
				tradeFixture("a", "0.000001"),
			)
		case "opaque+2":
			reply = `{"data":[],"pagination":{"has_more":true,"next_cursor":"opaque/3"}}`
		case "opaque/3":
			reply = fmt.Sprintf(
				`{"data":[%s],"pagination":{"has_more":false,"next_cursor":null}}`,
				tradeFixture("b", "0.000002"),
			)
		default:
			t.Errorf("unexpected cursor %q", q.Get("cursor"))
			reply = `{}`
		}
		fmt.Fprint(w, reply)
	})
	no := false
	params := TradesParams{
		Window:    TimeWindow{FullHistory: true},
		TakerOnly: &no,
		Page:      PageParams{Limit: 3, Cursor: "resume/opaque+1"},
	}
	var ids []string
	for trade, err := range client.IterTrades(t.Context(), params) {
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, trade.AssetID)
	}
	if strings.Join(ids, ",") != "a,b" || calls.Load() != 3 {
		t.Fatalf("empty/short pages ended traversal: %v, calls=%d", ids, calls.Load())
	}
}

func TestCursorCyclesAndConsumerBreak(t *testing.T) {
	var calls atomic.Int32
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprintf(
			w,
			`{"data":[%s],"pagination":{"has_more":true,"next_cursor":"same"}}`,
			tradeFixture("a", "1"),
		)
	})
	for _, err := range client.IterTrades(t.Context(), TradesParams{}) {
		if err != nil {
			t.Fatal(err)
		}
		break
	}
	if calls.Load() != 1 {
		t.Fatal("iterator prefetched after consumer break")
	}
	calls.Store(0)
	var final error
	for _, err := range client.IterTrades(t.Context(), TradesParams{}) {
		if err != nil {
			final = err
		}
	}
	if !errors.Is(final, ErrCursorCycle) || calls.Load() != 2 {
		t.Fatalf("cycle did not terminate: %v, calls=%d", final, calls.Load())
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	calls.Store(0)
	for _, err := range client.IterTrades(ctx, TradesParams{}) {
		final = err
	}
	if !errors.Is(final, context.Canceled) || calls.Load() != 0 {
		t.Fatal("canceled iteration made a request")
	}
}

func TestMalformedEnvelopeAndNullAbsence(t *testing.T) {
	for _, reply := range []string{
		`[]`, `{"data":[],"pagination":{"has_more":true,"next_cursor":null}}`,
		`{"data":[],"pagination":{"has_more":false,"next_cursor":"extra"}}`,
		`{"data":[],"pagination":{"next_cursor":null}}`, `{"data":[],"pagination":{"has_more":false}}`,
		`{"data":null,"pagination":{"has_more":false,"next_cursor":null}}`,
	} {
		t.Run(reply, func(t *testing.T) {
			client := testClient(
				t,
				func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, reply) },
			)
			if _, err := client.GetTrades(t.Context(), TradesParams{}); err == nil {
				t.Fatal("malformed envelope was accepted")
			}
		})
	}
	client := testClient(
		t,
		func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"data":null}`) },
	)
	stats, err := client.GetUserStats(t.Context(), "wallet")
	if err != nil || stats != nil {
		t.Fatalf("no-row result was not preserved: %v %v", stats, err)
	}
	if _, err := client.GetValue(t.Context(), ValueParams{User: "wallet"}); !errors.Is(
		err,
		ErrInvalidResponse,
	) {
		t.Fatalf("nonnullable value accepted null: %v", err)
	}
}

func TestConditionGrammarAndRequestScope(t *testing.T) {
	market := "0x01" + strings.Repeat("ab", 30)
	combo := "0x03" + strings.Repeat("cd", 30)
	var calls atomic.Int32
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch r.URL.Path {
		case "/v2/positions":
			if r.URL.Query().Get("condition_id") != market+"00" {
				t.Errorf("market identifier not normalized: %s", r.URL)
			}
		case "/v2/positions/combos":
			q := r.URL.Query()
			if q.Get("condition_id") != combo || q.Get("updated_after") != "0" ||
				q.Get("status") != "OPEN" {
				t.Errorf("combo filters not normalized: %s", r.URL)
			}
		}
		fmt.Fprint(w, `{"data":[],"pagination":{"has_more":false,"next_cursor":null}}`)
	})
	if _, err := client.GetPositions(t.Context(), PositionsParams{Selectors: Selectors{ConditionIDs: []string{market, market + "00"}}}); err != nil {
		t.Fatal(err)
	}
	epoch := time.Unix(0, 0)
	if _, err := client.GetComboPositions(t.Context(), ComboPositionsParams{User: "wallet", ConditionIDs: []string{combo + "01", combo}, Statuses: []ComboPositionStatus{ComboPositionStatusOpen, ComboPositionStatusOpen}, UpdatedAfter: &epoch}); err != nil {
		t.Fatal(err)
	}
	before := calls.Load()
	invalid := []func() error{
		func() error {
			_, err := client.GetTrades(
				t.Context(),
				TradesParams{
					Selectors: Selectors{ConditionIDs: []string{market}, EventIDs: []int32{1}},
				},
			)
			return err
		},
		func() error {
			_, err := client.GetPositions(
				t.Context(),
				PositionsParams{
					Status:    PositionStatusMergeable,
					Selectors: Selectors{ConditionIDs: []string{market}},
				},
			)
			return err
		},
		func() error {
			_, err := client.GetComboPositions(
				t.Context(),
				ComboPositionsParams{User: "wallet", ConditionIDs: []string{market}},
			)
			return err
		},
		func() error {
			_, err := client.GetComboPositions(
				t.Context(),
				ComboPositionsParams{
					User: "wallet",
					Statuses: []ComboPositionStatus{
						ComboPositionStatusRedeemable,
						ComboPositionStatusOpen,
					},
				},
			)
			return err
		},
		func() error {
			_, err := client.GetTrades(t.Context(), TradesParams{Page: PageParams{Limit: 1001}})
			return err
		},
	}
	for _, request := range invalid {
		if err := request(); err == nil {
			t.Fatal("invalid request accepted")
		}
	}
	if calls.Load() != before {
		t.Fatal("invalid input reached the service")
	}
}

func TestAccountingArchiveStreaming(t *testing.T) {
	archive := []byte{'P', 'K', 3, 4, 0, 255, 1}
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/accounting/snapshot" || r.Header.Get("Accept") != "application/zip" ||
			r.URL.Query().Get("user") != "wallet" {
			t.Errorf("wrong archive request: %s", r.URL)
		}
		w.Write(archive)
	})
	var dst bytes.Buffer
	if err := client.WriteAccountingSnapshot(t.Context(), "wallet", &dst); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dst.Bytes(), archive) {
		t.Fatal("archive was JSON/base64 decoded or changed")
	}
	if err := client.WriteAccountingSnapshot(t.Context(), "wallet", errorWriter{}); !errors.Is(
		err,
		io.ErrClosedPipe,
	) {
		t.Fatalf("lost destination error: %v", err)
	}
}

func tradeFixture(asset, size string) string {
	return fmt.Sprintf(
		`{"proxy_wallet":"0x7c3db723f1d4d8cb9c550095203b686cb11e5c6b","token_id":%q,"condition_id":"0x7ad403c3508f8e3912940fd1a913f227591145ca0614074208e0b962d5fcc422","side":"BUY","size":%q,"price":"0.5","timestamp":1785425912,"transaction_hash":"0x1fc7a1df48dcac15859af515ebd77183c3487591181469a60dbf115b705f005a"}`,
		asset,
		size,
	)
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
