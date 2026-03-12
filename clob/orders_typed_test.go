package clob

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTypedAuthenticatedResponses(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case openOrdersEndpoint:
			_ = json.NewEncoder(w).Encode([]OpenOrder{{
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
			}})
		case orderEndpoint + "order-1":
			_ = json.NewEncoder(w).Encode(OpenOrder{
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
		case tradesEndpoint:
			_ = json.NewEncoder(w).Encode([]Trade{{
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
			}})
		case postOrderEndpoint:
			if r.Method == http.MethodPost {
				_ = json.NewEncoder(w).Encode(PostOrderResponse{
					Success:            true,
					OrderID:            "order-1",
					TransactionsHashes: []string{"0xhash"},
					Status:             "LIVE",
					TakingAmount:       "10",
					MakingAmount:       "4.5",
				})
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

	client, err := New(Config{
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

	openOrders, err := client.GetOpenOrders(context.Background(), OpenOrderParams{})
	if err != nil {
		t.Fatalf("get open orders: %v", err)
	}
	if len(openOrders) != 1 || openOrders[0].ID != "order-1" {
		t.Fatalf("unexpected open orders: %#v", openOrders)
	}

	order, err := client.GetOrder(context.Background(), "order-1")
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if order.ID != "order-1" {
		t.Fatalf("unexpected order: %#v", order)
	}

	trades, err := client.GetTrades(context.Background(), TradeParams{})
	if err != nil {
		t.Fatalf("get trades: %v", err)
	}
	if len(trades) != 1 || trades[0].ID != "trade-1" {
		t.Fatalf("unexpected trades: %#v", trades)
	}

	postResponse, err := client.PostOrder(context.Background(), PostOrderRequest{
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

	cancelResponse, err := client.CancelOrder(context.Background(), "order-1")
	if err != nil {
		t.Fatalf("cancel order: %v", err)
	}
	if len(cancelResponse.Canceled) != 1 || cancelResponse.NotCanceled["order-2"] == "" {
		t.Fatalf("unexpected cancel response: %#v", cancelResponse)
	}
}
