package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quagmt/udecimal"
)

// --- Unexported endpoints under test ---

func TestGetMarketPositions(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/market-positions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("market") != "m-1" {
			t.Errorf("market = %q", q.Get("market"))
		}
		if q.Get("status") != "open" {
			t.Errorf("status = %q", q.Get("status"))
		}
		if q.Get("sortBy") != "size" {
			t.Errorf("sortBy = %q", q.Get("sortBy"))
		}
		if q.Get("limit") != "10" {
			t.Errorf("limit = %q", q.Get("limit"))
		}
		json.NewEncoder(w).Encode([]MetaMarketPosition{{
			Token: "0xabc",
			Positions: []MarketPositionDetail{{
				Wallet: "0x123", Size: udecimalMustParse("50"),
			}},
		}})
	})

	items, err := client.GetMarketPositions(t.Context(), MarketPositionParams{
		Market: "m-1",
		Status: MarketPositionStatusOpen,
		SortBy: MarketPositionSortBySize,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("GetMarketPositions: %v", err)
	}
	if len(items) != 1 || items[0].Token != "0xabc" {
		t.Errorf("unexpected: %+v", items)
	}
}

func TestIterMarketPositions(t *testing.T) {
	call := 0
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		call++
		switch call {
		case 1:
			json.NewEncoder(w).Encode([]MetaMarketPosition{
				{Token: "a"}, {Token: "b"},
			})
		default:
			json.NewEncoder(w).Encode([]MetaMarketPosition{})
		}
	})
	defer srv.Close()

	var tokens []string
	for mp, err := range client.IterMarketPositions(t.Context(), MarketPositionParams{
		Market: "m-1", Limit: 2,
	}) {
		if err != nil {
			t.Fatalf("IterMarketPositions: %v", err)
		}
		tokens = append(tokens, mp.Token)
	}
	if len(tokens) != 2 {
		t.Errorf("got %d tokens", len(tokens))
	}
}

func TestGetComboPositions(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/positions/combos" {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("user") != "0x123" {
			t.Errorf("user = %q", q.Get("user"))
		}
		if q.Get("status") != "OPEN" {
			t.Errorf("status = %q", q.Get("status"))
		}
		if q.Get("market_id") != "cid-1" {
			t.Errorf("market_id = %q", q.Get("market_id"))
		}
		json.NewEncoder(w).Encode(struct {
			Combos     []ComboPosition `json:"combos"`
			Pagination comboPagination `json:"pagination"`
		}{
			Combos: []ComboPosition{{
				ConditionID: "cid-1",
				PositionID:  "pos-1",
				Outcome:     ComboPositionOutcomeYes,
				Status:      ComboPositionStatusOpen,
				LegsTotal:   2,
			}},
			Pagination: comboPagination{Limit: 20, HasMore: false},
		})
	})
	defer srv.Close()

	items, err := client.GetComboPositions(t.Context(), ComboPositionParams{
		User:        "0x123",
		Status:      ComboPositionStatusOpen,
		ConditionID: "cid-1",
	})
	if err != nil {
		t.Fatalf("GetComboPositions: %v", err)
	}
	if len(items) != 1 || items[0].Status != ComboPositionStatusOpen {
		t.Errorf("unexpected: %+v", items)
	}
}

func TestIterComboPositions(t *testing.T) {
	call := 0
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		call++
		switch call {
		case 1:
			json.NewEncoder(w).Encode(struct {
				Combos     []ComboPosition `json:"combos"`
				Pagination comboPagination `json:"pagination"`
			}{
				Combos: []ComboPosition{
					{PositionID: "a"}, {PositionID: "b"},
				},
				Pagination: comboPagination{Limit: 2, HasMore: true, NextCursor: "cursor-2"},
			})
		default:
			json.NewEncoder(w).Encode(struct {
				Combos     []ComboPosition `json:"combos"`
				Pagination comboPagination `json:"pagination"`
			}{
				Pagination: comboPagination{Limit: 2, HasMore: false},
			})
		}
	})
	defer srv.Close()

	var ids []string
	for cp, err := range client.IterComboPositions(t.Context(), ComboPositionParams{
		User: "0x123", Limit: 2,
	}) {
		if err != nil {
			t.Fatalf("IterComboPositions: %v", err)
		}
		ids = append(ids, cp.PositionID)
	}
	if len(ids) != 2 {
		t.Errorf("got %d ids", len(ids))
	}
}

func TestIterComboActivityUsesOfficialEnvelope(t *testing.T) {
	call := 0
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/activity/combos" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("market_id") != "cid-1" {
			t.Errorf("market_id = %q", r.URL.Query().Get("market_id"))
		}
		call++
		if call == 1 {
			json.NewEncoder(w).Encode(struct {
				Activity   []ComboActivity `json:"activity"`
				Pagination comboPagination `json:"pagination"`
			}{
				Activity: []ComboActivity{{
					ID:          "activity-1",
					Type:        ComboActivityTypeSplit,
					ConditionID: "cid-1",
				}},
				Pagination: comboPagination{Limit: 50, HasMore: true, NextCursor: "cursor-2"},
			})
			return
		}
		json.NewEncoder(w).Encode(struct {
			Activity   []ComboActivity `json:"activity"`
			Pagination comboPagination `json:"pagination"`
		}{
			Activity:   []ComboActivity{{ID: "activity-2", Type: ComboActivityTypeRedeem}},
			Pagination: comboPagination{Limit: 50, HasMore: false},
		})
	})
	defer srv.Close()

	var ids []string
	for item, err := range client.IterComboActivity(t.Context(), ComboActivityParams{
		User:        "0x123",
		ConditionID: "cid-1",
	}) {
		if err != nil {
			t.Fatalf("IterComboActivity: %v", err)
		}
		ids = append(ids, item.ID)
	}
	if len(ids) != 2 || ids[0] != "activity-1" || ids[1] != "activity-2" {
		t.Fatalf("unexpected combo activity ids: %v", ids)
	}
}

func TestDownloadAccountingSnapshot(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/accounting/snapshot" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("user") != "0x123" {
			t.Errorf("user = %q", r.URL.Query().Get("user"))
		}
		w.Write([]byte("\x50\x4b\x03\x04"))
	})
	defer srv.Close()

	data, err := client.DownloadAccountingSnapshot(t.Context(), "0x123")
	if err != nil {
		t.Fatalf("DownloadAccountingSnapshot: %v", err)
	}
	if len(data) != 4 {
		t.Errorf("got %d bytes", len(data))
	}
}

func udecimalMustParse(s string) Decimal {
	return udecimal.MustParse(s)
}

// newTestServer creates an httptest server and data Client wired to it.
func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv, New(Config{Host: srv.URL})
}

func TestDataEndpointBounds(t *testing.T) {
	t.Parallel()

	client := New(Config{})
	tests := []struct {
		name string
		call func() error
	}{
		{"positions offset", func() error {
			_, err := client.GetPositions(t.Context(), PositionParams{Offset: 10_001})
			return err
		}},
		{"closed positions offset", func() error {
			_, err := client.GetClosedPositions(t.Context(), ClosedPositionParams{Offset: 100_001})
			return err
		}},
		{"trades limit", func() error {
			_, err := client.GetTrades(t.Context(), TradeParams{Limit: 10_001})
			return err
		}},
		{"activity limit", func() error {
			_, err := client.GetActivity(t.Context(), ActivityParams{Limit: 501})
			return err
		}},
		{"holders min balance", func() error {
			_, err := client.GetHolders(t.Context(), HoldersParams{MinBalance: 1_000_000})
			return err
		}},
		{"leaderboard offset", func() error {
			_, err := client.GetLeaderboard(t.Context(), LeaderboardParams{Offset: 1_001})
			return err
		}},
		{"builder leaderboard limit", func() error {
			_, err := client.GetBuilderLeaderboard(t.Context(), BuilderLeaderboardParams{Limit: 51})
			return err
		}},
		{"market positions offset", func() error {
			_, err := client.GetMarketPositions(t.Context(), MarketPositionParams{Offset: 10_001})
			return err
		}},
		{"combo positions limit", func() error {
			_, err := client.GetComboPositions(t.Context(), ComboPositionParams{Limit: 501})
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var boundsErr *ParameterBoundsError
			if err := tt.call(); !errors.As(err, &boundsErr) {
				t.Fatalf("error = %v, want ParameterBoundsError", err)
			}
		})
	}
}

func TestDataClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			json.NewEncoder(w).Encode(Health{Data: "OK"})
		case "/positions":
			json.NewEncoder(w).Encode([]Position{
				{Asset: "0x123", Title: "Test Market", Size: udecimalMustParse("100")},
			})
		case "/trades":
			json.NewEncoder(w).Encode([]Trade{
				{Side: SideBuy, Size: udecimalMustParse("50"), Price: udecimalMustParse("0.65")},
			})
		case "/activity":
			json.NewEncoder(w).Encode([]Activity{
				{
					Type:     ActivityTypeTrade,
					Size:     udecimalMustParse("10"),
					USDCSize: udecimalMustParse("6.5"),
				},
			})
		case "/holders":
			json.NewEncoder(w).Encode([]MetaHolder{
				{
					Token:   "0x123",
					Holders: []Holder{{ProxyWallet: "0xabc", Amount: udecimalMustParse("100")}},
				},
			})
		case "/value":
			json.NewEncoder(w).Encode([]Value{
				{User: "0x123", Value: udecimalMustParse("1000.50")},
			})
		case "/v1/leaderboard":
			json.NewEncoder(w).Encode([]TraderLeaderboardEntry{
				{
					Rank:     1,
					Username: "trader1",
					Volume:   udecimalMustParse("1000"),
					PNL:      udecimalMustParse("500"),
				},
			})
		case "/v1/builders/leaderboard":
			json.NewEncoder(w).Encode([]BuilderLeaderboardEntry{
				{Rank: 1, Builder: "0xabc", Volume: udecimalMustParse("5000"), ActiveUsers: 10},
			})
		case "/v1/builders/volume":
			json.NewEncoder(w).Encode([]BuilderVolumeEntry{
				{Builder: "0xabc", Volume: udecimalMustParse("1000"), ActiveUsers: 5},
			})
		case "/traded":
			json.NewEncoder(w).Encode(Traded{User: "0x123", Traded: 42})
		case "/oi":
			json.NewEncoder(w).Encode([]OpenInterest{
				{Market: "0xabc", Value: udecimalMustParse("10000")},
			})
		case "/live-volume":
			json.NewEncoder(w).Encode([]LiveVolume{{
				Total:   udecimalMustParse("50000"),
				Markets: []MarketVolume{{Market: "0xabc", Value: udecimalMustParse("25000")}},
			}})
		case "/closed-positions":
			json.NewEncoder(w).Encode([]ClosedPosition{
				{Asset: "0x123", Title: "Closed", RealizedPNL: udecimalMustParse("50")},
			})
		case "/v1/market-positions":
			json.NewEncoder(w).Encode([]MetaMarketPosition{{
				Token: "0xabc",
				Positions: []MarketPositionDetail{{
					Wallet: "0x123", Size: udecimalMustParse("50"),
				}},
			}})
		case "/v1/positions/combos":
			json.NewEncoder(w).Encode(struct {
				Combos     []ComboPosition `json:"combos"`
				Pagination comboPagination `json:"pagination"`
			}{
				Combos: []ComboPosition{{
					ConditionID: "cid-1",
					PositionID:  "pos-1",
					Status:      ComboPositionStatusOpen,
					LegsTotal:   2,
				}},
				Pagination: comboPagination{Limit: 20, HasMore: false},
			})
		case "/v1/accounting/snapshot":
			w.Write([]byte("\x50\x4b\x03\x04"))
		}
	}))
	defer server.Close()

	client := New(Config{Host: server.URL})
	ctx := t.Context()

	t.Run("GetPositions", func(t *testing.T) {
		items, err := client.GetPositions(ctx, PositionParams{User: "0x123"})
		if err != nil {
			t.Fatalf("GetPositions: %v", err)
		}
		if len(items) != 1 || items[0].Asset != "0x123" {
			t.Errorf("unexpected: %+v", items)
		}
	})

	t.Run("GetHealth", func(t *testing.T) {
		item, err := client.GetHealth(ctx)
		if err != nil {
			t.Fatalf("GetHealth: %v", err)
		}
		if item.Data != "OK" {
			t.Errorf("unexpected health: %+v", item)
		}
	})

	t.Run("GetTrades", func(t *testing.T) {
		items, err := client.GetTrades(ctx, TradeParams{User: "0x123"})
		if err != nil {
			t.Fatalf("GetTrades: %v", err)
		}
		if len(items) != 1 || items[0].Side != SideBuy {
			t.Errorf("unexpected: %+v", items)
		}
	})

	t.Run("GetActivity", func(t *testing.T) {
		items, err := client.GetActivity(ctx, ActivityParams{User: "0x123"})
		if err != nil {
			t.Fatalf("GetActivity: %v", err)
		}
		if len(items) != 1 || items[0].Type != ActivityTypeTrade {
			t.Errorf("unexpected: %+v", items)
		}
	})

	t.Run("GetHolders", func(t *testing.T) {
		items, err := client.GetHolders(ctx, HoldersParams{Markets: []string{"0x123"}})
		if err != nil {
			t.Fatalf("GetHolders: %v", err)
		}
		if len(items) != 1 || items[0].Token != "0x123" {
			t.Errorf("unexpected: %+v", items)
		}
	})

	t.Run("GetValue", func(t *testing.T) {
		items, err := client.GetValue(ctx, "0x123", nil)
		if err != nil {
			t.Fatalf("GetValue: %v", err)
		}
		if len(items) != 1 || items[0].Value.String() != "1000.5" {
			t.Errorf("unexpected values: %+v", items)
		}
	})

	t.Run("GetLeaderboard", func(t *testing.T) {
		items, err := client.GetLeaderboard(
			ctx,
			LeaderboardParams{Category: LeaderboardCategoryOverall},
		)
		if err != nil {
			t.Fatalf("GetLeaderboard: %v", err)
		}
		if len(items) != 1 || items[0].Username != "trader1" {
			t.Errorf("unexpected: %+v", items)
		}
	})

	t.Run("GetBuilderLeaderboard", func(t *testing.T) {
		items, err := client.GetBuilderLeaderboard(ctx, BuilderLeaderboardParams{})
		if err != nil {
			t.Fatalf("GetBuilderLeaderboard: %v", err)
		}
		if len(items) != 1 || items[0].Builder != "0xabc" {
			t.Errorf("unexpected: %+v", items)
		}
	})

	t.Run("GetBuilderVolume", func(t *testing.T) {
		items, err := client.GetBuilderVolume(ctx, BuilderVolumeParams{})
		if err != nil {
			t.Fatalf("GetBuilderVolume: %v", err)
		}
		if len(items) != 1 || items[0].Volume.String() != "1000" {
			t.Errorf("unexpected: %+v", items)
		}
	})

	t.Run("GetTraded", func(t *testing.T) {
		item, err := client.GetTraded(ctx, "0x123")
		if err != nil {
			t.Fatalf("GetTraded: %v", err)
		}
		if item.Traded != 42 {
			t.Errorf("unexpected traded: %+v", item)
		}
	})

	t.Run("GetOpenInterest", func(t *testing.T) {
		items, err := client.GetOpenInterest(ctx, OpenInterestParams{Markets: []string{"0xabc"}})
		if err != nil {
			t.Fatalf("GetOpenInterest: %v", err)
		}
		if len(items) != 1 || items[0].Value.String() != "10000" {
			t.Errorf("unexpected: %+v", items)
		}
	})

	t.Run("GetLiveVolume", func(t *testing.T) {
		items, err := client.GetLiveVolume(ctx, 12345)
		if err != nil {
			t.Fatalf("GetLiveVolume: %v", err)
		}
		if len(items) != 1 || items[0].Total.String() != "50000" {
			t.Errorf("unexpected live volume: %+v", items)
		}
	})

	t.Run("GetClosedPositions", func(t *testing.T) {
		items, err := client.GetClosedPositions(ctx, ClosedPositionParams{User: "0x123"})
		if err != nil {
			t.Fatalf("GetClosedPositions: %v", err)
		}
		if len(items) != 1 || items[0].RealizedPNL.String() != "50" {
			t.Errorf("unexpected: %+v", items)
		}
	})

	t.Run("GetMarketPositions", func(t *testing.T) {
		items, err := client.GetMarketPositions(ctx, MarketPositionParams{
			Market: "0xabc",
		})
		if err != nil {
			t.Fatalf("GetMarketPositions: %v", err)
		}
		if len(items) != 1 || items[0].Token != "0xabc" {
			t.Errorf("unexpected: %+v", items)
		}
	})

	t.Run("GetComboPositions", func(t *testing.T) {
		items, err := client.GetComboPositions(ctx, ComboPositionParams{User: "0x123"})
		if err != nil {
			t.Fatalf("GetComboPositions: %v", err)
		}
		if len(items) != 1 || items[0].Status != ComboPositionStatusOpen {
			t.Errorf("unexpected: %+v", items)
		}
	})

	t.Run("DownloadAccountingSnapshot", func(t *testing.T) {
		data, err := client.DownloadAccountingSnapshot(ctx, "0x123")
		if err != nil {
			t.Fatalf("DownloadAccountingSnapshot: %v", err)
		}
		if len(data) != 4 {
			t.Errorf("unexpected snapshot size: %d", len(data))
		}
	})

	t.Run("QueryEncoding", func(t *testing.T) {
		t.Run("positions market filter", func(t *testing.T) {
			errCh := make(chan error, 1)
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					q := r.URL.Query()
					if got := q.Get("market"); got != "m1,m2" {
						errCh <- fmt.Errorf("market query = %q, want %q", got, "m1,m2")
						return
					}
					if got := q.Get("eventId"); got != "" {
						errCh <- fmt.Errorf("eventId query = %q, want empty", got)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode([]Position{})
					errCh <- nil
				}),
			)
			defer server.Close()

			client := New(Config{Host: server.URL})
			_, err := client.GetPositions(t.Context(), PositionParams{
				User:   "0x123",
				Filter: MarketsFilter("m1", "m2"),
			})
			if err != nil {
				t.Fatalf("GetPositions: %v", err)
			}
			if err := <-errCh; err != nil {
				t.Fatal(err)
			}
		})

		t.Run("trades event filter", func(t *testing.T) {
			errCh := make(chan error, 1)
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					q := r.URL.Query()
					if got := q.Get("eventId"); got != "e1,e2" {
						errCh <- fmt.Errorf("eventId query = %q, want %q", got, "e1,e2")
						return
					}
					if got := q.Get("market"); got != "" {
						errCh <- fmt.Errorf("market query = %q, want empty", got)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode([]Trade{})
					errCh <- nil
				}),
			)
			defer server.Close()

			client := New(Config{Host: server.URL})
			_, err := client.GetTrades(t.Context(), TradeParams{
				User:   "0x123",
				Filter: EventIDsFilter("e1", "e2"),
			})
			if err != nil {
				t.Fatalf("GetTrades: %v", err)
			}
			if err := <-errCh; err != nil {
				t.Fatal(err)
			}
		})

		t.Run("trades filter type and amount", func(t *testing.T) {
			errCh := make(chan error, 1)
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					q := r.URL.Query()
					if got := q.Get("filterType"); got != "CASH" {
						errCh <- fmt.Errorf("filterType query = %q, want %q", got, "CASH")
						return
					}
					if got := q.Get("filterAmount"); got != "100" {
						errCh <- fmt.Errorf("filterAmount query = %q, want %q", got, "100")
						return
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode([]Trade{})
					errCh <- nil
				}),
			)
			defer server.Close()

			client := New(Config{Host: server.URL})
			_, err := client.GetTrades(t.Context(), TradeParams{
				User: "0x123",
				TradeFilter: &TradeFilter{
					FilterType:   FilterTypeCash,
					FilterAmount: udecimalMustParse("100"),
				},
			})
			if err != nil {
				t.Fatalf("GetTrades: %v", err)
			}
			if err := <-errCh; err != nil {
				t.Fatal(err)
			}
		})
	})

	t.Run("LimitClamping", func(t *testing.T) {
		t.Run("closed positions iterator", func(t *testing.T) {
			limitCh := make(chan string, 1)
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					limitCh <- r.URL.Query().Get("limit")
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode([]ClosedPosition{})
				}),
			)
			defer server.Close()

			client := New(Config{Host: server.URL})
			for range client.IterClosedPositions(t.Context(), ClosedPositionParams{User: "0x123"}) {
			}
			if gotLimit := <-limitCh; gotLimit != "50" {
				t.Fatalf("iterator limit = %q, want %q", gotLimit, "50")
			}
		})

		t.Run("leaderboard iterator", func(t *testing.T) {
			limitCh := make(chan string, 1)
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					limitCh <- r.URL.Query().Get("limit")
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode([]TraderLeaderboardEntry{})
				}),
			)
			defer server.Close()

			client := New(Config{Host: server.URL})
			for range client.IterLeaderboard(t.Context(), LeaderboardParams{Category: LeaderboardCategoryOverall}) {
			}
			if gotLimit := <-limitCh; gotLimit != "50" {
				t.Fatalf("iterator limit = %q, want %q", gotLimit, "50")
			}
		})

		t.Run("leaderboard direct limit validation", func(t *testing.T) {
			client := New(Config{})
			_, err := client.GetLeaderboard(t.Context(), LeaderboardParams{
				Category: LeaderboardCategoryOverall,
				Limit:    100,
			})
			var boundsErr *ParameterBoundsError
			if !errors.As(err, &boundsErr) {
				t.Fatalf("expected ParameterBoundsError, got %v", err)
			}
			if boundsErr.Parameter != "leaderboard.limit" {
				t.Fatalf("parameter = %q, want leaderboard.limit", boundsErr.Parameter)
			}
		})
	})

	t.Run("APIError", func(t *testing.T) {
		errServ := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("bad request"))
			}),
		)
		defer errServ.Close()

		badClient := New(Config{Host: errServ.URL})
		_, err := badClient.GetPositions(ctx, PositionParams{User: "0x123"})
		if err == nil {
			t.Fatal("expected error")
		}
		apiErr, ok := err.(*APIError)
		if !ok {
			t.Fatalf("expected APIError, got %T", err)
		}
		if apiErr.StatusCode != http.StatusBadRequest {
			t.Errorf("unexpected status: %d", apiErr.StatusCode)
		}
	})
}

// --- Iterator pagination tests ---

func TestIterPositions(t *testing.T) {
	call := 0
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		call++
		switch call {
		case 1:
			writeTestJSON(t, w, []Position{{Asset: "a"}, {Asset: "b"}})
		default:
			writeTestJSON(t, w, []Position{})
		}
	})
	defer srv.Close()

	var assets []string
	for pos, iterErr := range client.IterPositions(t.Context(), PositionParams{User: "0x123", Limit: 2}) {
		if iterErr != nil {
			t.Fatalf("IterPositions: %v", iterErr)
		}
		assets = append(assets, pos.Asset)
	}
	if len(assets) != 2 {
		t.Errorf("got %d positions", len(assets))
	}
}

func TestIterTrades(t *testing.T) {
	call := 0
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		call++
		switch call {
		case 1:
			writeTestJSON(t, w, []Trade{{Asset: "a"}, {Asset: "b"}})
		default:
			writeTestJSON(t, w, []Trade{})
		}
	})
	defer srv.Close()

	var count int
	for _, iterErr := range client.IterTrades(t.Context(), TradeParams{User: "0x123", Limit: 2}) {
		if iterErr != nil {
			t.Fatalf("IterTrades: %v", iterErr)
		}
		count++
	}
	if count != 2 {
		t.Errorf("got %d trades", count)
	}
}

func TestIterActivity(t *testing.T) {
	call := 0
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		call++
		switch call {
		case 1:
			writeTestJSON(t, w, []Activity{{Type: ActivityTypeTrade}, {Type: ActivityTypeRedeem}})
		default:
			writeTestJSON(t, w, []Activity{})
		}
	})
	defer srv.Close()

	var types []ActivityType
	for a, iterErr := range client.IterActivity(t.Context(), ActivityParams{User: "0x123", Limit: 2}) {
		if iterErr != nil {
			t.Fatalf("IterActivity: %v", iterErr)
		}
		types = append(types, a.Type)
	}
	if len(types) != 2 {
		t.Errorf("got %d activities", len(types))
	}
}

func TestIterBuilderLeaderboard(t *testing.T) {
	call := 0
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		call++
		switch call {
		case 1:
			writeTestJSON(t, w, []BuilderLeaderboardEntry{{Builder: "a"}, {Builder: "b"}})
		default:
			writeTestJSON(t, w, []BuilderLeaderboardEntry{})
		}
	})
	defer srv.Close()

	var builders []string
	for b, iterErr := range client.IterBuilderLeaderboard(t.Context(), BuilderLeaderboardParams{}) {
		if iterErr != nil {
			t.Fatalf("IterBuilderLeaderboard: %v", iterErr)
		}
		builders = append(builders, b.Builder)
	}
	if len(builders) != 2 {
		t.Errorf("got %d builders", len(builders))
	}
}

func TestIterError(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	})
	defer srv.Close()

	for _, err := range client.IterPositions(t.Context(), PositionParams{User: "0x123"}) {
		if err == nil {
			t.Fatal("expected error")
		}
		return
	}
}

// --- Query param coverage ---

func TestPositionsQueryParams(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		checks := map[string]string{
			"user":          "0x123",
			"sizeThreshold": "10",
			"redeemable":    "true",
			"mergeable":     "true",
			"sortBy":        "CURRENT",
			"sortDirection": "DESC",
			"title":         "test",
			"limit":         "5",
			"offset":        "10",
		}
		for k, want := range checks {
			if got := q.Get(k); got != want {
				t.Errorf("%s = %q, want %q", k, got, want)
			}
		}
		writeTestJSON(t, w, []Position{})
	})
	defer srv.Close()

	_, err := client.GetPositions(t.Context(), PositionParams{
		User:          "0x123",
		SizeThreshold: "10",
		Redeemable:    boolPtr(true),
		Mergeable:     boolPtr(true),
		SortBy:        PositionSortCurrent,
		SortDirection: SortDesc,
		Title:         "test",
		Limit:         5,
		Offset:        10,
	})
	if err != nil {
		t.Fatalf("GetPositions: %v", err)
	}
}

func TestActivityQueryIncludesAccountActivities(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("type"); got != "DEPOSIT,TAKER_REBATE" {
			t.Errorf("type = %q", got)
		}
		if got := r.URL.Query().Get("excludeDepositsWithdrawals"); got != "false" {
			t.Errorf("excludeDepositsWithdrawals = %q, want false", got)
		}
		writeTestJSON(t, w, []Activity{})
	})
	defer srv.Close()

	_, err := client.GetActivity(t.Context(), ActivityParams{
		User:          "0x123",
		ActivityTypes: []ActivityType{ActivityTypeDeposit, ActivityTypeTakerRebate},
	})
	if err != nil {
		t.Fatalf("GetActivity: %v", err)
	}
}

func TestActivityQueryParams(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		checks := map[string]string{
			"user":          "0x123",
			"type":          "TRADE,REDEEM",
			"start":         "1000",
			"end":           "2000",
			"sortBy":        "TIMESTAMP",
			"sortDirection": "ASC",
			"side":          "BUY",
		}
		for k, want := range checks {
			if got := q.Get(k); got != want {
				t.Errorf("%s = %q, want %q", k, got, want)
			}
		}
		writeTestJSON(t, w, []Activity{})
	})
	defer srv.Close()

	_, err := client.GetActivity(t.Context(), ActivityParams{
		User:          "0x123",
		ActivityTypes: []ActivityType{ActivityTypeTrade, ActivityTypeRedeem},
		Start:         1000,
		End:           2000,
		SortBy:        ActivitySortTimestamp,
		SortDirection: SortAsc,
		Side:          SideBuy,
	})
	if err != nil {
		t.Fatalf("GetActivity: %v", err)
	}
}

func TestLeaderboardQueryParams(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		checks := map[string]string{
			"category":   "OVERALL",
			"timePeriod": "WEEK",
			"orderBy":    "PNL",
			"user":       "0x123",
			"userName":   "trader1",
		}
		for k, want := range checks {
			if got := q.Get(k); got != want {
				t.Errorf("%s = %q, want %q", k, got, want)
			}
		}
		writeTestJSON(t, w, []TraderLeaderboardEntry{})
	})
	defer srv.Close()

	_, err := client.GetLeaderboard(t.Context(), LeaderboardParams{
		Category:   LeaderboardCategoryOverall,
		TimePeriod: TimePeriodWeek,
		SortBy:     LeaderboardOrderByPNL,
		User:       "0x123",
		UserName:   "trader1",
	})
	if err != nil {
		t.Fatalf("GetLeaderboard: %v", err)
	}
}

func TestComboPositionQueryParams(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		checks := map[string]string{
			"user":              "0x123",
			"status":            "OPEN",
			"market_id":         "cid-1",
			"combo_position_id": "pos-1",
			"limit":             "5",
		}
		for k, want := range checks {
			if got := q.Get(k); got != want {
				t.Errorf("%s = %q, want %q", k, got, want)
			}
		}
		writeTestJSON(t, w, struct {
			Combos     []ComboPosition `json:"combos"`
			Pagination comboPagination `json:"pagination"`
		}{Pagination: comboPagination{Limit: 5, HasMore: false}})
	})
	defer srv.Close()

	_, err := client.GetComboPositions(t.Context(), ComboPositionParams{
		User:        "0x123",
		Status:      ComboPositionStatusOpen,
		ConditionID: "cid-1",
		PositionID:  "pos-1",
		Limit:       5,
	})
	if err != nil {
		t.Fatalf("GetComboPositions: %v", err)
	}
}

func TestMarketPositionQueryParams(t *testing.T) {
	srv, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		checks := map[string]string{
			"market":        "m-1",
			"user":          "0x123",
			"status":        "open",
			"sortBy":        "cashPnl",
			"sortDirection": "ASC",
			"limit":         "10",
			"offset":        "5",
		}
		for k, want := range checks {
			if got := q.Get(k); got != want {
				t.Errorf("%s = %q, want %q", k, got, want)
			}
		}
		writeTestJSON(t, w, []MetaMarketPosition{})
	})
	defer srv.Close()

	_, err := client.GetMarketPositions(t.Context(), MarketPositionParams{
		Market:        "m-1",
		User:          "0x123",
		Status:        MarketPositionStatusOpen,
		SortBy:        MarketPositionSortByCashPnl,
		SortDirection: SortAsc,
		Limit:         10,
		Offset:        5,
	})
	if err != nil {
		t.Fatalf("GetMarketPositions: %v", err)
	}
}

// writeTestJSON is a test helper that writes JSON to w with content-type header.
func writeTestJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
