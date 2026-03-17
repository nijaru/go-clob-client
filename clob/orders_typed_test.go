package clob

import (
	"net/http"
	"net/http/httptest"
	"testing"

	json "github.com/go-json-experiment/json"
)

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

	clientRaw, err := New(Config{
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
	client := clientRaw.(*AuthenticatedClient)

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
			Salt:          "42",
			Maker:         "0x0000000000000000000000000000000000000001",
			Signer:        "0x0000000000000000000000000000000000000001",
			Taker:         zeroAddress,
			TokenID:       "100",
			MakerAmount:   "1000000",
			TakerAmount:   "2000000",
			Expiration:    "0",
			Nonce:         "0",
			FeeRateBps:    "0",
			Side:          SideBuy,
			SignatureType: SignatureTypeEOA,
			Signature:     "0xsig",
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
