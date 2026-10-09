package perps

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
)

// Snapshot bodies/statuses are grounded in actions/perps/position-snapshots.ts
// and bindings/perps/position-snapshots.ts at TS 087f9443. Public reads must not
// acquire delegated headers merely because the receiver is authenticated.
func TestPublicAndOwnerPositionSnapshots(t *testing.T) {
	ownerReads := 0
	snapshotReads := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/info/registered":
			if r.URL.Query().Get("address") != fixtureProxy ||
				r.Header.Get("POLYMARKET-SECRET") != "" {
				t.Errorf("registration request=%v", r)
			}
			_, _ = w.Write([]byte(`{"registered":true}`))
		case "/v1/account/credentials":
			ownerReads++
			if r.Header.Get("POLYMARKET-SECRET") != "fixture-secret" {
				t.Error("missing owner read credentials")
			}
			_, _ = w.Write([]byte(`{"address":"` + fixtureProxy + `","keys":[]}`))
		case "/v1/info/position-snapshots":
			snapshotReads++
			var p PositionSnapshotsRequest
			if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
				t.Error(err)
				return
			}
			if p.Address != fixtureProxy || !reflect.DeepEqual(p.ActiveInstrumentIDs, []int{1}) ||
				len(p.HistoryFills) != 1 ||
				p.HistoryFills[0].TradeID != "18446744073709551615" {
				t.Errorf("request=%+v", p)
			}
			if snapshotReads == 1 && r.Header.Get("POLYMARKET-SECRET") != "" {
				t.Error("anonymous snapshots leaked account credentials")
			}
			if snapshotReads == 2 && r.Header.Get("POLYMARKET-SECRET") != "fixture-secret" {
				t.Error("owner snapshots omitted credentials")
			}
			// Owner-only values may be null in anonymous responses. Result order is the selection order.
			_, _ = w.Write(
				[]byte(
					`{"history_as_of_at":1000,"active":[{"instrument_id":1,"status":"ok","snapshot":{"position_cycle_id":"cycle-1","side":"long","started_at":10,"as_of_at":1000,"is_closed":false,"size_after":"1","entry_price":"10","as_of_price":"12","pnl":"2","pnl_percent":null,"leverage":null,"chart":{"candles":[],"markers":[]}}}],"history":[{"instrument_id":1,"trade_id":"18446744073709551615","status":"history_pending"}]}`,
				),
			)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	client := fixtureClient(t, Config{Host: server.URL})
	selection := PositionSnapshotSelection{
		ActiveInstrumentIDs: []int{1},
		HistoryFills: []PositionSnapshotFill{
			{InstrumentID: 1, TradeID: "18446744073709551615", Timestamp: 1000},
		},
	}
	registered, err := client.GetRegistration(t.Context(), fixtureProxy)
	if err != nil || !registered {
		t.Fatalf("registration=%t %v", registered, err)
	}
	public, err := client.GetPositionSnapshots(
		t.Context(),
		PositionSnapshotsRequest{Address: fixtureProxy, PositionSnapshotSelection: selection},
	)
	if err != nil || public.Active[0].Snapshot.Leverage != nil ||
		public.History[0].Status != "history_pending" {
		t.Fatalf("public=%+v error=%v", public, err)
	}
	own, err := client.GetOwnPositionSnapshots(t.Context(), selection)
	if err != nil || len(own.History) != 1 || ownerReads != 1 || snapshotReads != 2 {
		t.Fatalf("own=%+v error=%v reads=%d/%d", own, err, ownerReads, snapshotReads)
	}
	for _, bad := range []PositionSnapshotSelection{
		{},
		{ActiveInstrumentIDs: []int{1, 1}},
		{HistoryFills: []PositionSnapshotFill{{InstrumentID: 1, TradeID: "18446744073709551616", Timestamp: 1}}},
		{HistoryFills: []PositionSnapshotFill{{InstrumentID: 1, TradeID: "+1", Timestamp: 1}}},
		{HistoryFills: []PositionSnapshotFill{{InstrumentID: 1, TradeID: "01", Timestamp: 1}, {InstrumentID: 1, TradeID: "1", Timestamp: 2}}},
	} {
		if _, err := client.GetOwnPositionSnapshots(t.Context(), bad); err == nil {
			t.Fatalf("invalid selection submitted %+v", bad)
		}
	}
	if ownerReads != 1 || snapshotReads != 2 {
		t.Fatal("invalid selections reached server")
	}
}

func TestInternalTransfersOverlapAndNonprogress(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/v1/account/internal-transfers" {
			t.Errorf("path=%s", r.URL.Path)
		}
		switch requests {
		case 1:
			if r.URL.Query().Get("end_timestamp") != "2000" {
				t.Error("initial end missing")
			}
			_, _ = w.Write(
				[]byte(
					`{"data":[{"transfer_id":42,"type":"internal_transfer","asset":"USDC","amount":"1","direction":"out","counterparty":"` + fixtureRecipient + `","created_timestamp":1000}],"more":true}`,
				),
			)
		case 2:
			if r.URL.Query().Get("end_timestamp") != "1001" {
				t.Error("submillisecond boundary was skipped")
			}
			_, _ = w.Write(
				[]byte(
					`{"data":[{"transfer_id":42,"type":"internal_transfer","asset":"USDC","amount":"1","direction":"out","counterparty":"` + fixtureRecipient + `","created_timestamp":1000},{"transfer_id":43,"type":"internal_transfer","asset":"USDC","amount":"2","direction":"in","counterparty":"` + fixtureRecipient + `","created_timestamp":1000}],"more":true}`,
				),
			)
		default:
			_, _ = w.Write(
				[]byte(
					`{"data":[{"transfer_id":42,"type":"internal_transfer","asset":"USDC","amount":"1","direction":"out","counterparty":"` + fixtureRecipient + `","created_timestamp":1000},{"transfer_id":43,"type":"internal_transfer","asset":"USDC","amount":"2","direction":"in","counterparty":"` + fixtureRecipient + `","created_timestamp":1000}],"more":true}`,
				),
			)
		}
	}))
	t.Cleanup(server.Close)
	client := fixtureClient(t, Config{Host: server.URL})
	var ids []int64
	var lastErr error
	for items, err := range client.IterInternalTransfers(t.Context(), AccountHistoryParams{Start: 1, End: 2000}) {
		if err != nil {
			lastErr = err
			break
		}
		for _, item := range items {
			ids = append(ids, item.ID)
		}
	}
	if !reflect.DeepEqual(ids, []int64{42, 43}) || !errors.Is(lastErr, ErrPaginationNonProgress) ||
		requests != 3 {
		t.Fatalf("IDs=%v error=%v requests=%d", ids, lastErr, requests)
	}
}

func TestPublicTradePageCursorCarriesBoundaryIDs(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch requests {
		case 1:
			_, _ = w.Write(
				[]byte(
					`{"data":[{"trade_id":1,"instrument_id":1,"side":"long","price":"10","quantity":"1","timestamp":100,"hash":"h1"}],"more":true}`,
				),
			)
		case 2:
			if r.URL.Query().Get("end_timestamp") != "100" {
				t.Error("boundary skipped")
			}
			_, _ = w.Write(
				[]byte(
					`{"data":[{"trade_id":1,"instrument_id":1,"side":"long","price":"10","quantity":"1","timestamp":100,"hash":"h1"},{"trade_id":2,"instrument_id":1,"side":"long","price":"11","quantity":"1","timestamp":100,"hash":"h2"}],"more":true}`,
				),
			)
		default:
			_, _ = w.Write(
				[]byte(
					`{"data":[{"trade_id":1,"timestamp":100},{"trade_id":2,"timestamp":100}],"more":true}`,
				),
			)
		}
	}))
	t.Cleanup(server.Close)
	client := New(Config{Host: server.URL})
	first, cursor, err := client.GetTradesPage(
		t.Context(),
		TradesParams{InstrumentID: 1, Start: 1, End: 200},
	)
	if err != nil || len(first) != 1 {
		t.Fatal(err)
	}
	second, cursor, err := client.GetTradesPage(t.Context(), TradesParams{Cursor: cursor})
	if err != nil || len(second) != 1 || second[0].TradeID != 2 {
		t.Fatalf("second=%+v %v", second, err)
	}
	_, _, err = client.GetTradesPage(t.Context(), TradesParams{Cursor: cursor})
	if !errors.Is(err, ErrPaginationNonProgress) {
		t.Fatalf("nonprogress=%v", err)
	}
}

func TestFillsAndFundingRejectRepeatedContinuations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/account/fills":
			_, _ = w.Write([]byte(`{"data":[{"trade_id":12,"fee":"0"}],"more":true}`))
		case "/v1/info/funding":
			_, _ = w.Write([]byte(`{"data":[{"timestamp":100,"funding_rate":"0.01"}],"more":true}`))
		default:
			_, _ = w.Write([]byte(`{"data":[],"more":true}`))
		}
	}))
	t.Cleanup(server.Close)
	client := fixtureClient(t, Config{Host: server.URL})
	if _, err := client.GetFillsPage(t.Context(), AccountHistoryParams{Cursor: "12"}); !errors.Is(
		err,
		ErrPaginationNonProgress,
	) {
		t.Fatalf("fill continuation=%v", err)
	}
	if _, _, err := client.GetFundingHistoryPage(t.Context(), FundingParams{InstrumentID: 1, Start: 1, End: 50}); !errors.Is(
		err,
		ErrPaginationNonProgress,
	) {
		t.Fatalf("funding continuation=%v", err)
	}
	if _, _, err := client.GetCandlesPage(t.Context(), CandlesParams{InstrumentID: 1, Start: 1, End: 200, Interval: PerpsKline1m}); !errors.Is(
		err,
		ErrPaginationNonProgress,
	) {
		t.Fatalf("empty candle continuation=%v", err)
	}
}

func TestBuilderEarningsUseServerSnapshotCursor(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/account/builder-earnings-summary":
			if r.URL.Query().Get("as_of_sequence") != "7" {
				t.Error("summary cutoff missing")
			}
			_, _ = w.Write(
				[]byte(
					`{"start_timestamp":100,"end_timestamp":200,"as_of_sequence":7,"data":[{"fee_asset":"USDC","fill_count":1,"notional":"100","builder_fee":"0.01"}],"trader_count":1,"active_approval_count":2}`,
				),
			)
		case "/v1/account/builder-earnings":
			requests++
			if requests == 1 {
				if r.URL.Query().Get("start_timestamp") != "100" ||
					r.URL.Query().Get("as_of_sequence") != "7" {
					t.Error("history snapshot missing")
				}
			} else if len(r.URL.Query()) != 1 || r.URL.Query().Get("cursor") != "snapshot-page" {
				t.Error("continuation did not use cursor alone")
			}
			_, _ = w.Write(
				[]byte(
					`{"start_timestamp":100,"end_timestamp":200,"as_of_sequence":7,"data":[{"earning_id":"receipt:opaque/01", "trade_id":1,"order_id":2,"instrument_id":1,"trader":"` + fixtureProxy + `","buy":true,"price":"100","quantity":"1","side":"taker","timestamp":100,"sequence":7,"notional":"100","fee_asset":"USDC","fee":"0.02","builder_fee":"0.01","total_fee":"0.03","fee_rate":"0.0001"}],"more":true,"cursor":"snapshot-page"}`,
				),
			)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	client := fixtureClient(t, Config{Host: server.URL})
	start, end, sequence := int64(100), int64(200), int64(7)
	p := BuilderReportingParams{Start: &start, End: &end, AsOfSequence: &sequence}
	summary, err := client.GetBuilderEarningsSummary(t.Context(), p)
	if err != nil || summary.AsOfSequence != 7 || summary.Data[0].BuilderFee != "0.01" {
		t.Fatalf("summary=%+v %v", summary, err)
	}
	var lastErr error
	count := 0
	for items, err := range client.IterBuilderEarnings(t.Context(), p) {
		if err != nil {
			lastErr = err
			break
		}
		if len(items) != 1 || items[0].EarningID != "receipt:opaque/01" {
			t.Fatalf("opaque earning identity lost: %+v", items)
		}
		count += len(items)
	}
	if count != 1 || requests != 2 || !errors.Is(lastErr, ErrPaginationNonProgress) {
		t.Fatalf("count=%d requests=%d error=%v", count, requests, lastErr)
	}
}

func TestSettlementADLAndExactBuilderFeeReads(t *testing.T) {
	var instrument PerpsInstrument
	if err := json.Unmarshal([]byte(`{"instrument_id":1,"symbol":"BTC-USDC","display_symbol":"Bitcoin","close_only":true,"settlement":{"sequence":9007199254740991,"timestamp":1000,"price":"100","insurance_debit":"2"}}`), &instrument); err != nil {
		t.Fatal(err)
	}
	if instrument.DisplaySymbol != "Bitcoin" || !instrument.CloseOnly ||
		instrument.Settlement.Sequence != 9007199254740991 {
		t.Fatalf("instrument=%+v", instrument)
	}
	var notification PerpsNotification
	if err := json.Unmarshal([]byte(`{"id":"adl-1","type":"position_deleveraged","instrument_id":1,"side":"short","size_closed":"1","price":"100","pnl":"5","margin_type":"cross"}`), &notification); err != nil {
		t.Fatal(err)
	}
	if notification.PositionDeleveraged == nil || notification.PositionDeleveraged.Price != "100" {
		t.Fatalf("ADL=%+v", notification)
	}
	for _, tc := range []struct{ fee, builder, want string }{{"-0.002", "0.0005", "-0.0015"}, {"-0.0002", "0.0005", "0.0003"}, {"999999999999999999999999.1234567890123456789012345678", "0.0000000000000000000000000002", "999999999999999999999999.123456789012345678901234568"}} {
		t.Run(tc.want, func(t *testing.T) {
			var fill PerpsAccountFill
			wire := `{"fee":` + strconv.Quote(
				tc.fee,
			) + `,"builder_fee":` + strconv.Quote(
				tc.builder,
			) + `,"settlement":true}`
			if err := json.Unmarshal([]byte(wire), &fill); err != nil || fill.TotalFee != tc.want ||
				!fill.Settlement {
				t.Fatalf("fill=%+v %v", fill, err)
			}
		})
	}
}
