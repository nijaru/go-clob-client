package perps

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// fakeServer routes perps endpoints and counts requests per path so pagination
// can be driven deterministically (first call returns more=true, then false).
type fakeServer struct {
	mux    *http.ServeMux
	counts map[string]int
	mu     sync.Mutex
}

func newFakeServer(t *testing.T) (*fakeServer, *httptest.Server) {
	t.Helper()
	fs := &fakeServer{mux: http.NewServeMux(), counts: make(map[string]int)}
	fs.mux.HandleFunc("/v1/info/instruments", fs.jsonHandler(t, []PerpsInstrument{
		{ID: 1, Category: PerpsCategoryCrypto, Symbol: "BTC-USDC", MaxLeverage: 50},
	}))
	fs.mux.HandleFunc("/v1/info/tickers", fs.jsonHandler(t, []PerpsTicker{
		{InstrumentID: 1, Symbol: "BTC-USDC", LastPrice: "60000.0", MidPrice: "60000.5"},
	}))
	fs.mux.HandleFunc("/v1/info/statistics", fs.jsonHandler(t, []PerpsStatistic{
		{InstrumentID: 1, Volume: "123.0", OpenPrice: "59000.0"},
	}))
	fs.mux.HandleFunc("/v1/info/book", fs.jsonHandler(t, PerpsBook{
		InstrumentID: 1,
		Bids:         []PerpsBookLevel{{Price: "59999.0", Quantity: "1.5"}},
		Asks:         []PerpsBookLevel{{Price: "60001.0", Quantity: "2.0"}},
		Timestamp:    1000,
		Sequence:     7,
	}))
	fs.mux.HandleFunc("/v1/info/klines", fs.klinesHandler)
	fs.mux.HandleFunc("/v1/info/trades", fs.tradesHandler)
	fs.mux.HandleFunc("/v1/info/funding", fs.fundingHandler)
	fs.mux.HandleFunc("/v1/info/fees", fs.jsonHandler(t, PerpsFeesInfo{
		FeeSchedule: []PerpsFeeScheduleEntry{
			{Category: PerpsCategoryCrypto, TakerFeeRate: "0.0005", MakerFeeRate: "0.0001"},
		},
	}))
	srv := httptest.NewServer(fs.mux)
	t.Cleanup(srv.Close)
	return fs, srv
}

func (fs *fakeServer) jsonHandler(t *testing.T, v any) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		fs.mu.Lock()
		fs.counts[r.URL.Path]++
		fs.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}
}

func (fs *fakeServer) klinesHandler(w http.ResponseWriter, r *http.Request) {
	fs.mu.Lock()
	fs.counts[r.URL.Path]++
	n := fs.counts[r.URL.Path]
	fs.mu.Unlock()
	more := n == 1
	if more {
		_ = json.NewEncoder(w).Encode(perpsDataResponse[PerpsCandle]{
			Data: []PerpsCandle{{Timestamp: 1000, Open: "1", High: "2", Low: "1", Close: "2", Volume: "10", Trades: 3}},
			More: true,
		})
		return
	}
	_ = json.NewEncoder(w).Encode(perpsDataResponse[PerpsCandle]{
		Data: []PerpsCandle{{Timestamp: 2000, Open: "2", High: "3", Low: "2", Close: "3", Volume: "5", Trades: 1}},
		More: false,
	})
}

func (fs *fakeServer) tradesHandler(w http.ResponseWriter, r *http.Request) {
	fs.mu.Lock()
	fs.counts[r.URL.Path]++
	n := fs.counts[r.URL.Path]
	fs.mu.Unlock()
	if n == 1 {
		// Last trade (t3) shares ts=30 with a trade in page 2.
		_ = json.NewEncoder(w).Encode(perpsDataResponse[PerpsPublicTrade]{
			Data: []PerpsPublicTrade{
				{TradeID: 1, InstrumentID: 1, Side: PerpsSideLong, Price: "10", Quantity: "1", Timestamp: 10},
				{TradeID: 2, InstrumentID: 1, Side: PerpsSideShort, Price: "10", Quantity: "2", Timestamp: 20},
				{TradeID: 3, InstrumentID: 1, Side: PerpsSideLong, Price: "10", Quantity: "3", Timestamp: 30},
			},
			More: true,
		})
		return
	}
	// Raw page 2 includes t3 again (duplicate of page-1 last) plus new trades.
	_ = json.NewEncoder(w).Encode(perpsDataResponse[PerpsPublicTrade]{
		Data: []PerpsPublicTrade{
			{TradeID: 3, InstrumentID: 1, Side: PerpsSideLong, Price: "10", Quantity: "3", Timestamp: 30},
			{TradeID: 4, InstrumentID: 1, Side: PerpsSideShort, Price: "11", Quantity: "4", Timestamp: 30},
			{TradeID: 5, InstrumentID: 1, Side: PerpsSideLong, Price: "12", Quantity: "5", Timestamp: 20},
		},
		More: false,
	})
}

func (fs *fakeServer) fundingHandler(w http.ResponseWriter, r *http.Request) {
	fs.mu.Lock()
	fs.counts[r.URL.Path]++
	n := fs.counts[r.URL.Path]
	fs.mu.Unlock()
	if n == 1 {
		_ = json.NewEncoder(w).Encode(perpsDataResponse[PerpsFundingRate]{
			Data: []PerpsFundingRate{{FundingRate: "0.0001", Timestamp: 1000}},
			More: true,
		})
		return
	}
	_ = json.NewEncoder(w).Encode(perpsDataResponse[PerpsFundingRate]{
		Data: []PerpsFundingRate{{FundingRate: "0.0002", Timestamp: 2000}},
		More: false,
	})
}

func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	return New(Config{Host: srv.URL})
}

func TestGetInstruments(t *testing.T) {
	_, srv := newFakeServer(t)
	c := newTestClient(t, srv)
	ins, err := c.GetInstruments(t.Context(), InstrumentsParams{})
	if err != nil {
		t.Fatalf("GetInstruments: %v", err)
	}
	if len(ins) != 1 || ins[0].ID != 1 || ins[0].MaxLeverage != 50 {
		t.Fatalf("unexpected instruments: %+v", ins)
	}
}

func TestGetTickerJoinsStatistics(t *testing.T) {
	_, srv := newFakeServer(t)
	c := newTestClient(t, srv)
	tk, err := c.GetTicker(t.Context(), 1)
	if err != nil {
		t.Fatalf("GetTicker: %v", err)
	}
	if tk.OpenPrice != "59000.0" || tk.Volume24h != "123.0" {
		t.Fatalf("ticker not enriched from statistics: %+v", tk)
	}
	if tk.LastPrice != "60000.0" {
		t.Fatalf("ticker base fields missing: %+v", tk)
	}
}

func TestGetBook(t *testing.T) {
	_, srv := newFakeServer(t)
	c := newTestClient(t, srv)
	b, err := c.GetBook(t.Context(), BookParams{InstrumentID: 1, Depth: PerpsBookDepth10})
	if err != nil {
		t.Fatalf("GetBook: %v", err)
	}
	if len(b.Bids) != 1 || b.Bids[0].Price != "59999.0" || b.Sequence != 7 {
		t.Fatalf("unexpected book: %+v", b)
	}
}

func TestGetFees(t *testing.T) {
	_, srv := newFakeServer(t)
	c := newTestClient(t, srv)
	fees, err := c.GetFees(t.Context())
	if err != nil {
		t.Fatalf("GetFees: %v", err)
	}
	if len(fees) != 1 || fees[0].Category != PerpsCategoryCrypto {
		t.Fatalf("unexpected fees: %+v", fees)
	}
}

func TestIterCandlesPagination(t *testing.T) {
	_, srv := newFakeServer(t)
	c := newTestClient(t, srv)

	var pages int
	var total int
	for page, err := range c.IterCandles(t.Context(), CandlesParams{
		InstrumentID: 1,
		Interval:     PerpsKline1m,
		Start:        0,
		End:          9999,
	}) {
		if err != nil {
			t.Fatalf("IterCandles: %v", err)
		}
		pages++
		total += len(page)
	}
	if pages != 2 {
		t.Fatalf("expected 2 pages, got %d", pages)
	}
	if total != 2 {
		t.Fatalf("expected 2 candles total, got %d", total)
	}
}

func TestGetCandlesPageCursor(t *testing.T) {
	_, srv := newFakeServer(t)
	c := newTestClient(t, srv)
	p1, next, err := c.GetCandlesPage(t.Context(), CandlesParams{InstrumentID: 1, Interval: PerpsKline1m, Start: 0, End: 9999})
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(p1) != 1 || next == "" {
		t.Fatalf("page1 should return 1 candle and a cursor, got len=%d next=%q", len(p1), next)
	}
	p2, next2, err := c.GetCandlesPage(t.Context(), CandlesParams{Cursor: next})
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(p2) != 1 || next2 != "" {
		t.Fatalf("page2 should be last (empty cursor), got len=%d next=%q", len(p2), next2)
	}
}

func TestIterTradesDedupes(t *testing.T) {
	_, srv := newFakeServer(t)
	c := newTestClient(t, srv)

	var all []PerpsPublicTrade
	for page, err := range c.IterTrades(t.Context(), TradesParams{InstrumentID: 1, Start: 0, End: 9999}) {
		if err != nil {
			t.Fatalf("IterTrades: %v", err)
		}
		all = append(all, page...)
	}
	// t3 appears in both raw pages but must be yielded exactly once.
	seen := make(map[int64]int)
	for _, tr := range all {
		seen[tr.TradeID]++
	}
	if seen[3] != 1 {
		t.Fatalf("trade 3 should be deduplicated to 1 occurrence, got %d (all=%+v)", seen[3], all)
	}
	if len(all) != 5 {
		t.Fatalf("expected 5 distinct trades, got %d: %+v", len(all), all)
	}
}

func TestIterFundingHistory(t *testing.T) {
	_, srv := newFakeServer(t)
	c := newTestClient(t, srv)
	var total int
	for page, err := range c.IterFundingHistory(t.Context(), FundingParams{InstrumentID: 1, Start: 0, End: 9999}) {
		if err != nil {
			t.Fatalf("IterFundingHistory: %v", err)
		}
		total += len(page)
	}
	if total != 2 {
		t.Fatalf("expected 2 funding samples, got %d", total)
	}
}

func TestCursorRoundTrip(t *testing.T) {
	orig := candlesCursor{Kind: "perpsCandles", InstrumentID: 42, Interval: PerpsKline1d, StartTimestamp: 100, EndTimestamp: 200}
	enc := encodeCursor(orig)
	if enc == "" {
		t.Fatal("encodeCursor returned empty")
	}
	var dec candlesCursor
	if err := decodeCursor(enc, &dec); err != nil {
		t.Fatalf("decodeCursor: %v", err)
	}
	if dec != orig {
		t.Fatalf("round trip mismatch: %+v != %+v", dec, orig)
	}
	if err := decodeCursor("not-base64!!!", &dec); err == nil {
		t.Fatal("expected error for invalid cursor")
	}
}
