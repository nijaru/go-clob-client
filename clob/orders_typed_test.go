package clob

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	json "github.com/go-json-experiment/json"
)

func TestOpenOrderNormalizesRustTimestamps(t *testing.T) {
	t.Parallel()

	var order OpenOrder
	if err := json.Unmarshal(
		[]byte(`{"created_at":1710000000123,"expiration":0}`),
		&order,
	); err != nil {
		t.Fatalf("decode numeric timestamps: %v", err)
	}
	if order.CreatedAt != 1710000000 {
		t.Fatalf("created_at = %d, want 1710000000", order.CreatedAt)
	}
	if order.CreatedAtTime == nil || order.CreatedAtTime.UnixMilli() != 1710000000123 {
		t.Fatalf("created_at_time = %v", order.CreatedAtTime)
	}
	if order.Expiration != "0" || order.ExpirationTime != nil {
		t.Fatalf("expiration = %q, expiration_time = %v", order.Expiration, order.ExpirationTime)
	}

	if err := json.Unmarshal(
		[]byte(`{"created_at":"2026-03-12T18:00:00Z","expiration":"2026-03-12T19:00:00Z"}`),
		&order,
	); err != nil {
		t.Fatalf("decode ISO timestamps: %v", err)
	}
	if want := time.Date(
		2026,
		time.March,
		12,
		18,
		0,
		0,
		0,
		time.UTC,
	); order.CreatedAtTime == nil ||
		!order.CreatedAtTime.Equal(want) {
		t.Fatalf("created_at_time = %v, want %v", order.CreatedAtTime, want)
	}
	if order.ExpirationTime == nil || order.ExpirationTime.Hour() != 19 {
		t.Fatalf("expiration_time = %v", order.ExpirationTime)
	}
}

func TestTypedAuthenticatedResponses(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case openOrdersEndpoint:
			cursor := r.URL.Query().Get("next_cursor")
			switch cursor {
			case initialCursor:
				data, _ := json.Marshal(Page[OpenOrder]{
					Limit:      1,
					Count:      1,
					NextCursor: "cursor-2",
					Data: []OpenOrder{{
						ID:              "order-1",
						Status:          "LIVE",
						Owner:           "api-key",
						MakerAddress:    "0xmaker",
						Market:          "market-1",
						AssetID:         "asset-1",
						Side:            "BUY",
						OriginalSize:    "10",
						SizeMatched:     "1",
						Price:           "0.45",
						AssociateTrades: []string{"trade-1"},
						Outcome:         "YES",
						CreatedAt:       1710000000,
						Expiration:      "0",
						OrderType:       "GTC",
					}},
				})
				w.Write(data)
			case "cursor-2":
				data, _ := json.Marshal(Page[OpenOrder]{
					Limit:      1,
					Count:      1,
					NextCursor: endCursor,
					Data: []OpenOrder{{
						ID:              "order-2",
						Status:          "LIVE",
						Owner:           "api-key",
						MakerAddress:    "0xmaker",
						Market:          "market-2",
						AssetID:         "asset-2",
						Side:            "SELL",
						OriginalSize:    "8",
						SizeMatched:     "0",
						Price:           "0.55",
						AssociateTrades: nil,
						Outcome:         "NO",
						CreatedAt:       1710000002,
						Expiration:      "0",
						OrderType:       "GTC",
					}},
				})
				w.Write(data)
			default:
				t.Fatalf("unexpected open orders cursor: %q", cursor)
			}
		case orderEndpoint + "order-1":
			data, _ := json.Marshal(OpenOrder{
				ID:           "order-1",
				Status:       "LIVE",
				Owner:        "api-key",
				MakerAddress: "0xmaker",
				Market:       "market-1",
				AssetID:      "asset-1",
				Side:         "BUY",
				OriginalSize: "10",
				SizeMatched:  "1",
				Price:        "0.45",
				Outcome:      "YES",
				CreatedAt:    1710000000,
				Expiration:   "0",
				OrderType:    "GTC",
			})
			w.Write(data)
		case tradesEndpoint:
			cursor := r.URL.Query().Get("next_cursor")
			switch cursor {
			case initialCursor:
				data, _ := json.Marshal(Page[Trade]{
					Limit:      1,
					Count:      1,
					NextCursor: "cursor-2",
					Data: []Trade{{
						ID:           "trade-1",
						TakerOrderID: "order-2",
						Market:       "market-1",
						AssetID:      "asset-1",
						Side:         SideBuy,
						Size:         "10",
						FeeRateBps:   "0",
						Price:        "0.45",
						Status:       "MATCHED",
						MatchTime:    "1710000000",
						LastUpdate:   "1710000001",
						Outcome:      "YES",
						BucketIndex:  1,
						Owner:        "api-key",
						MakerAddress: "0xmaker",
						MakerOrders: []MakerOrder{{
							OrderID:       "order-1",
							Owner:         "api-key",
							MakerAddress:  "0xmaker",
							MatchedAmount: "10",
							Price:         "0.45",
							FeeRateBps:    "0",
							AssetID:       "asset-1",
							Outcome:       "YES",
							Side:          SideBuy,
						}},
						TransactionHash: "0xhash",
						TraderSide:      "TAKER",
					}},
				})
				w.Write(data)
			case "cursor-2":
				data, _ := json.Marshal(Page[Trade]{
					Limit:      1,
					Count:      1,
					NextCursor: endCursor,
					Data: []Trade{{
						ID:              "trade-2",
						TakerOrderID:    "order-3",
						Market:          "market-2",
						AssetID:         "asset-2",
						Side:            SideSell,
						Size:            "8",
						FeeRateBps:      "0",
						Price:           "0.55",
						Status:          "MATCHED",
						MatchTime:       "1710000002",
						LastUpdate:      "1710000003",
						Outcome:         "NO",
						BucketIndex:     2,
						Owner:           "api-key",
						MakerAddress:    "0xmaker",
						TransactionHash: "0xhash2",
						TraderSide:      "MAKER",
					}},
				})
				w.Write(data)
			default:
				t.Fatalf("unexpected trades cursor: %q", cursor)
			}
		case postOrderEndpoint:
			if r.Method == http.MethodPost {
				data, _ := json.Marshal(PostOrderResponse{
					Success:            true,
					OrderID:            "order-1",
					TransactionsHashes: []string{"0xhash"},
					Status:             "LIVE",
					TakingAmount:       "10",
					MakingAmount:       "4.5",
				})
				w.Write(data)
				return
			}

			_, _ = w.Write(
				[]byte(`{"canceled":["order-1"],"not_canceled":{"order-2":"already canceled"}}`),
			)
		case cancelOrdersEndpoint, cancelAllEndpoint:
			_, _ = w.Write(
				[]byte(`{"canceled":["order-1"],"not_canceled":{"order-2":"already canceled"}}`),
			)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewAuthenticatedClient(Config{
		Host:       server.URL,
		PrivateKey: "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		Credentials: &Credentials{
			Key:        "api-key",
			Secret:     "c2VjcmV0",
			Passphrase: "pass",
		},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	openOrdersPage, err := client.GetOpenOrdersPage(t.Context(), OpenOrderParams{}, "")
	if err != nil {
		t.Fatalf("get open orders page: %v", err)
	}
	if len(openOrdersPage.Data) != 1 || openOrdersPage.Data[0].ID != "order-1" {
		t.Fatalf("unexpected open orders page: %#v", openOrdersPage)
	}

	openOrders, err := client.GetOpenOrders(t.Context(), OpenOrderParams{})
	if err != nil {
		t.Fatalf("get open orders: %v", err)
	}
	if len(openOrders) != 2 || openOrders[1].ID != "order-2" {
		t.Fatalf("unexpected open orders: %#v", openOrders)
	}

	order, err := client.GetOrder(t.Context(), "order-1")
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if order.ID != "order-1" {
		t.Fatalf("unexpected order: %#v", order)
	}

	tradesPage, err := client.GetTradesPage(t.Context(), TradeParams{}, "")
	if err != nil {
		t.Fatalf("get trades page: %v", err)
	}
	if len(tradesPage.Data) != 1 || tradesPage.Data[0].ID != "trade-1" {
		t.Fatalf("unexpected trades page: %#v", tradesPage)
	}

	trades, err := client.GetTrades(t.Context(), TradeParams{})
	if err != nil {
		t.Fatalf("get trades: %v", err)
	}
	if len(trades) != 2 || trades[1].ID != "trade-2" {
		t.Fatalf("unexpected trades: %#v", trades)
	}

	postResponse, err := client.PostOrder(t.Context(), PostOrderRequest{
		Order: SignedOrder{
			Order: Order{
				Salt:          "42",
				Maker:         "0x0000000000000000000000000000000000000001",
				Signer:        "0x0000000000000000000000000000000000000001",
				TokenID:       "100",
				MakerAmount:   "1000000",
				TakerAmount:   "2000000",
				Side:          SideBuy,
				SignatureType: SignatureTypeEOA,
			},
			Expiration: "0",
			Signature:  "0xsig",
		},
		Owner:     "api-key",
		OrderType: OrderTypeGTC,
	})
	if err != nil {
		t.Fatalf("post order: %v", err)
	}
	if !postResponse.Success || postResponse.OrderID != "order-1" {
		t.Fatalf("unexpected post order response: %#v", postResponse)
	}

	cancelResponse, err := client.CancelOrder(t.Context(), "order-1")
	if err != nil {
		t.Fatalf("cancel order: %v", err)
	}
	if len(cancelResponse.Canceled) != 1 || cancelResponse.NotCanceled["order-2"] == "" {
		t.Fatalf("unexpected cancel response: %#v", cancelResponse)
	}
}

func TestPostOrdersBatch(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != postOrdersEndpoint {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		resp := []PostOrderResponse{
			{
				Success: true,
				OrderID: "order-batch-1",
			},
		}
		data, _ := json.Marshal(resp)
		w.Write(data)
	}))
	defer server.Close()

	client, err := NewAuthenticatedClient(Config{
		Host:       server.URL,
		PrivateKey: "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		Credentials: &Credentials{
			Key:        "api-key",
			Secret:     "c2VjcmV0",
			Passphrase: "pass",
		},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	ctx := t.Context()

	// 1. Success case
	reqs := []PostOrderRequest{
		{
			Order: SignedOrder{
				Order: Order{
					Salt: "42",
				},
			},
			Owner:     "api-key",
			OrderType: OrderTypeGTC,
		},
	}
	resps, err := client.PostOrders(ctx, reqs)
	if err != nil {
		t.Fatalf("PostOrders: %v", err)
	}
	if len(resps) != 1 || resps[0].OrderID != "order-batch-1" {
		t.Errorf("unexpected responses: %+v", resps)
	}

	// 2. Exceed batch limit
	oversizedReqs := make([]PostOrderRequest, PostOrdersBatchLimit+1)
	_, err = client.PostOrders(ctx, oversizedReqs)
	if err == nil {
		t.Error("expected error for oversized batch request")
	}
}
