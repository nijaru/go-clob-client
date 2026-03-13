package clob

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTypedMarketPricingSurfaces(t *testing.T) {
	t.Parallel()

	geoblockServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != geoblockEndpoint {
				t.Fatalf("unexpected geoblock path: %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
			"blocked": false,
			"ip": "127.0.0.1",
			"country": "US",
			"region": "CA"
		}`))
		}),
	)
	defer geoblockServer.Close()

	clobServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.Method + " " + r.URL.Path {
		case http.MethodGet + " " + midpointEndpoint:
			if got := r.URL.Query().Get("token_id"); got != "123" {
				t.Fatalf("unexpected midpoint token id: %s", got)
			}
			_, _ = w.Write([]byte(`{"mid":"0.50"}`))
		case http.MethodPost + " " + midpointsEndpoint:
			var books []BookParams
			if err := json.NewDecoder(r.Body).Decode(&books); err != nil {
				t.Fatalf("decode midpoints request: %v", err)
			}
			if len(books) != 2 || books[0].TokenID != "123" || books[1].TokenID != "456" {
				t.Fatalf("unexpected midpoints request: %+v", books)
			}
			_, _ = w.Write([]byte(`{"123":"0.50","456":"0.61"}`))
		case http.MethodGet + " " + priceEndpoint:
			if got := r.URL.Query().Get("token_id"); got != "123" {
				t.Fatalf("unexpected price token id: %s", got)
			}
			if got := r.URL.Query().Get("side"); got != string(SideBuy) {
				t.Fatalf("unexpected price side: %s", got)
			}
			_, _ = w.Write([]byte(`{"price":"0.41"}`))
		case http.MethodPost + " " + pricesEndpoint:
			var books []BookParams
			if err := json.NewDecoder(r.Body).Decode(&books); err != nil {
				t.Fatalf("decode prices request: %v", err)
			}
			if len(books) != 1 || books[0].TokenID != "123" {
				t.Fatalf("unexpected prices request: %+v", books)
			}
			_, _ = w.Write([]byte(`{"123":{"BUY":"0.41","SELL":"0.59"}}`))
		case http.MethodGet + " " + pricesEndpoint:
			if r.URL.RawQuery != "" {
				t.Fatalf("unexpected all-prices query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"123":{"BUY":"0.41","SELL":"0.59"},"456":{"BUY":"0.22"}}`))
		case http.MethodGet + " " + spreadEndpoint:
			if got := r.URL.Query().Get("token_id"); got != "123" {
				t.Fatalf("unexpected spread token id: %s", got)
			}
			_, _ = w.Write([]byte(`{"spread":"0.18"}`))
		case http.MethodPost + " " + spreadsEndpoint:
			var books []BookParams
			if err := json.NewDecoder(r.Body).Decode(&books); err != nil {
				t.Fatalf("decode spreads request: %v", err)
			}
			if len(books) != 1 || books[0].TokenID != "123" {
				t.Fatalf("unexpected spreads request: %+v", books)
			}
			_, _ = w.Write([]byte(`{"123":"0.18"}`))
		case http.MethodGet + " " + lastTradePriceEndpoint:
			if got := r.URL.Query().Get("token_id"); got != "123" {
				t.Fatalf("unexpected last trade token id: %s", got)
			}
			_, _ = w.Write([]byte(`{"price":"0.55","side":"BUY"}`))
		case http.MethodPost + " " + lastTradesPricesEndpoint:
			var books []BookParams
			if err := json.NewDecoder(r.Body).Decode(&books); err != nil {
				t.Fatalf("decode last trades request: %v", err)
			}
			if len(books) != 1 || books[0].TokenID != "123" {
				t.Fatalf("unexpected last trades request: %+v", books)
			}
			_, _ = w.Write([]byte(`[{"token_id":"123","price":"0.55","side":"BUY"}]`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer clobServer.Close()

	client, err := New(Config{
		Host:         clobServer.URL,
		GeoblockHost: geoblockServer.URL,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	geoblock, err := client.CheckGeoblock(context.Background())
	if err != nil {
		t.Fatalf("check geoblock: %v", err)
	}
	if geoblock.Blocked || geoblock.Country != "US" {
		t.Fatalf("unexpected geoblock response: %+v", geoblock)
	}

	midpoint, err := client.GetMidpoint(context.Background(), "123")
	if err != nil {
		t.Fatalf("get midpoint: %v", err)
	}
	if midpoint.Mid != "0.50" {
		t.Fatalf("unexpected midpoint: %+v", midpoint)
	}

	midpoints, err := client.GetMidpoints(context.Background(), []BookParams{
		{TokenID: "123"},
		{TokenID: "456"},
	})
	if err != nil {
		t.Fatalf("get midpoints: %v", err)
	}
	if midpoints["456"] != "0.61" {
		t.Fatalf("unexpected midpoints: %+v", midpoints)
	}

	price, err := client.GetPrice(context.Background(), "123", string(SideBuy))
	if err != nil {
		t.Fatalf("get price: %v", err)
	}
	if price.Price != "0.41" {
		t.Fatalf("unexpected price: %+v", price)
	}

	prices, err := client.GetPrices(context.Background(), []BookParams{{TokenID: "123"}})
	if err != nil {
		t.Fatalf("get prices: %v", err)
	}
	if prices["123"][SideSell] != "0.59" {
		t.Fatalf("unexpected prices: %+v", prices)
	}

	allPrices, err := client.GetAllPrices(context.Background())
	if err != nil {
		t.Fatalf("get all prices: %v", err)
	}
	if allPrices["456"][SideBuy] != "0.22" {
		t.Fatalf("unexpected all prices: %+v", allPrices)
	}

	spread, err := client.GetSpread(context.Background(), "123")
	if err != nil {
		t.Fatalf("get spread: %v", err)
	}
	if spread.Spread != "0.18" {
		t.Fatalf("unexpected spread: %+v", spread)
	}

	spreads, err := client.GetSpreads(context.Background(), []BookParams{{TokenID: "123"}})
	if err != nil {
		t.Fatalf("get spreads: %v", err)
	}
	if spreads["123"] != "0.18" {
		t.Fatalf("unexpected spreads: %+v", spreads)
	}

	lastTrade, err := client.GetLastTradePrice(context.Background(), "123")
	if err != nil {
		t.Fatalf("get last trade price: %v", err)
	}
	if lastTrade.Price != "0.55" || lastTrade.Side != SideBuy {
		t.Fatalf("unexpected last trade price: %+v", lastTrade)
	}

	lastTrades, err := client.GetLastTradesPrices(
		context.Background(),
		[]BookParams{{TokenID: "123"}},
	)
	if err != nil {
		t.Fatalf("get last trades prices: %v", err)
	}
	if len(lastTrades) != 1 || lastTrades[0].TokenID != "123" || lastTrades[0].Side != SideBuy {
		t.Fatalf("unexpected last trades prices: %+v", lastTrades)
	}
}
