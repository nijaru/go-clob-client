package data

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDataClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			json.NewEncoder(w).Encode(Health{Data: "OK"})
		case "/positions":
			json.NewEncoder(w).Encode([]Position{
				{Asset: "0x123", Title: "Test Market", Size: "100"},
			})
		case "/trades":
			json.NewEncoder(w).Encode([]DataTrade{
				{Side: "BUY", Size: "50", Price: "0.65"},
			})
		case "/activity":
			json.NewEncoder(w).Encode([]Activity{
				{Type: "TRADE", Size: "10", USDCSize: "6.5"},
			})
		case "/holders":
			json.NewEncoder(w).Encode([]MetaHolder{
				{Token: "0x123", Holders: []Holder{{ProxyWallet: "0xabc", Amount: "100"}}},
			})
		case "/value":
			json.NewEncoder(w).Encode([]Value{
				{User: "0x123", Value: "1000.50"},
			})
		case "/v1/leaderboard":
			json.NewEncoder(w).Encode([]TraderLeaderboardEntry{
				{Rank: 1, Username: "trader1", Volume: "1000", PNL: "500"},
			})
		case "/v1/builders/leaderboard":
			json.NewEncoder(w).Encode([]BuilderLeaderboardEntry{
				{Rank: 1, Builder: "0xabc", Volume: "5000", ActiveUsers: 10},
			})
		case "/v1/builders/volume":
			json.NewEncoder(w).Encode([]BuilderVolumeEntry{
				{Builder: "0xabc", Volume: "1000", ActiveUsers: 5},
			})
		case "/traded":
			json.NewEncoder(w).Encode(Traded{User: "0x123", Traded: 42})
		case "/oi":
			json.NewEncoder(w).Encode([]OpenInterest{
				{Market: "0xabc", Value: "10000"},
			})
		case "/live-volume":
			json.NewEncoder(w).Encode([]LiveVolume{{
				Total:   "50000",
				Markets: []MarketVolume{{Market: "0xabc", Value: "25000"}},
			}})
		case "/closed-positions":
			json.NewEncoder(w).Encode([]ClosedPosition{
				{Asset: "0x123", Title: "Closed", RealizedPNL: "50"},
			})
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
		if len(items) != 1 || items[0].Side != "BUY" {
			t.Errorf("unexpected: %+v", items)
		}
	})

	t.Run("GetActivity", func(t *testing.T) {
		items, err := client.GetActivity(ctx, ActivityParams{User: "0x123"})
		if err != nil {
			t.Fatalf("GetActivity: %v", err)
		}
		if len(items) != 1 || items[0].Type != "TRADE" {
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
		if len(items) != 1 || items[0].Value != "1000.50" {
			t.Errorf("unexpected values: %+v", items)
		}
	})

	t.Run("GetLeaderboard", func(t *testing.T) {
		items, err := client.GetLeaderboard(ctx, LeaderboardParams{Category: "OVERALL"})
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
		if len(items) != 1 || items[0].Volume != "1000" {
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
		if len(items) != 1 || items[0].Value != "10000" {
			t.Errorf("unexpected: %+v", items)
		}
	})

	t.Run("GetLiveVolume", func(t *testing.T) {
		items, err := client.GetLiveVolume(ctx, 12345)
		if err != nil {
			t.Fatalf("GetLiveVolume: %v", err)
		}
		if len(items) != 1 || items[0].Total != "50000" {
			t.Errorf("unexpected live volume: %+v", items)
		}
	})

	t.Run("GetClosedPositions", func(t *testing.T) {
		items, err := client.GetClosedPositions(ctx, ClosedPositionParams{User: "0x123"})
		if err != nil {
			t.Fatalf("GetClosedPositions: %v", err)
		}
		if len(items) != 1 || items[0].RealizedPNL != "50" {
			t.Errorf("unexpected: %+v", items)
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
					json.NewEncoder(w).Encode([]DataTrade{})
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
					json.NewEncoder(w).Encode([]DataTrade{})
					errCh <- nil
				}),
			)
			defer server.Close()

			client := New(Config{Host: server.URL})
			_, err := client.GetTrades(t.Context(), TradeParams{
				User:        "0x123",
				TradeFilter: &TradeFilter{FilterType: FilterTypeCash, FilterAmount: "100"},
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
			for range client.IterLeaderboard(t.Context(), LeaderboardParams{Category: "OVERALL"}) {
			}
			if gotLimit := <-limitCh; gotLimit != "50" {
				t.Fatalf("iterator limit = %q, want %q", gotLimit, "50")
			}
		})

		t.Run("leaderboard direct limit clamp", func(t *testing.T) {
			errCh := make(chan error, 1)
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if got := r.URL.Query().Get("limit"); got != "50" {
						errCh <- fmt.Errorf("limit query = %q, want %q", got, "50")
						return
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode([]TraderLeaderboardEntry{})
					errCh <- nil
				}),
			)
			defer server.Close()

			client := New(Config{Host: server.URL})
			_, err := client.GetLeaderboard(t.Context(), LeaderboardParams{
				Category: "OVERALL",
				Limit:    100,
			})
			if err != nil {
				t.Fatalf("GetLeaderboard: %v", err)
			}
			if err := <-errCh; err != nil {
				t.Fatal(err)
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
