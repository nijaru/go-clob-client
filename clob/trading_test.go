package clob

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateOrderBuildsSignedLimitOrder(t *testing.T) {
	t.Parallel()

	server := newTradingTestServer(t, nil)
	defer server.Close()

	client, err := New(Config{
		Host:       server.URL,
		PrivateKey: "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	client.saltGenerator = func() uint64 { return 42 }

	order, err := client.CreateOrder(context.Background(), OrderArgs{
		TokenID: "100",
		Price:   0.45,
		Size:    10,
		Side:    SideBuy,
	}, nil)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	if order.Salt != "42" {
		t.Fatalf("unexpected salt: %s", order.Salt)
	}
	if order.MakerAmount != "4500000" {
		t.Fatalf("unexpected maker amount: %s", order.MakerAmount)
	}
	if order.TakerAmount != "10000000" {
		t.Fatalf("unexpected taker amount: %s", order.TakerAmount)
	}
	if order.Signature == "" {
		t.Fatal("expected non-empty signature")
	}
}

func TestCreateAndPostOrderSendsExpectedPayload(t *testing.T) {
	t.Parallel()

	var requestBody []byte
	server := newTradingTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != postOrderEndpoint {
			t.Fatalf("unexpected post path: %s", r.URL.Path)
		}
		if got := r.Header.Get("POLY_API_KEY"); got != "api-key" {
			t.Fatalf("unexpected api key header: %s", got)
		}
		if got := r.Header.Get("POLY_SIGNATURE"); got == "" {
			t.Fatal("expected poly signature header")
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		requestBody = body

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	})
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
	client.saltGenerator = func() uint64 { return 42 }

	_, err = client.CreateAndPostOrder(context.Background(), OrderArgs{
		TokenID: "100",
		Price:   0.45,
		Size:    10,
		Side:    SideBuy,
	}, nil, OrderTypeGTC, false, false)
	if err != nil {
		t.Fatalf("create and post order: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(requestBody, &payload); err != nil {
		t.Fatalf("decode post payload: %v", err)
	}

	if got := payload["owner"]; got != "api-key" {
		t.Fatalf("unexpected owner: %#v", got)
	}
	if got := payload["orderType"]; got != string(OrderTypeGTC) {
		t.Fatalf("unexpected orderType: %#v", got)
	}

	orderPayload, ok := payload["order"].(map[string]any)
	if !ok {
		t.Fatalf("expected order object, got %#v", payload["order"])
	}
	if got := orderPayload["salt"]; got != float64(42) {
		t.Fatalf("unexpected salt: %#v", got)
	}
	if got := orderPayload["side"]; got != string(SideBuy) {
		t.Fatalf("unexpected side: %#v", got)
	}
	if got := orderPayload["signatureType"]; got != float64(SignatureTypeEOA) {
		t.Fatalf("unexpected signatureType: %#v", got)
	}
}

func TestCreateMarketOrderDerivesPriceFromBook(t *testing.T) {
	t.Parallel()

	server := newTradingTestServer(t, nil)
	defer server.Close()

	client, err := New(Config{
		Host:       server.URL,
		PrivateKey: "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	client.saltGenerator = func() uint64 { return 42 }

	order, err := client.CreateMarketOrder(context.Background(), MarketOrderArgs{
		TokenID:   "100",
		Amount:    2,
		Side:      SideBuy,
		OrderType: OrderTypeFOK,
	}, nil)
	if err != nil {
		t.Fatalf("create market order: %v", err)
	}

	if order.MakerAmount != "2000000" {
		t.Fatalf("unexpected maker amount: %s", order.MakerAmount)
	}
	if order.TakerAmount == "" {
		t.Fatal("expected taker amount")
	}
}

func newTradingTestServer(t *testing.T, postHandler http.HandlerFunc) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case tickSizeEndpoint:
			_, _ = w.Write([]byte(`{"minimum_tick_size":"0.01"}`))
		case feeRateEndpoint:
			_, _ = w.Write([]byte(`{"base_fee":0}`))
		case negRiskEndpoint:
			_, _ = w.Write([]byte(`{"neg_risk":false}`))
		case orderBookEndpoint:
			_, _ = w.Write(
				[]byte(
					`{"market":"m","asset_id":"100","timestamp":"1","bids":[{"price":"0.44","size":"10"}],"asks":[{"price":"0.46","size":"10"}],"min_order_size":"1","tick_size":"0.01","neg_risk":false,"last_trade_price":"0.45","hash":"h"}`,
				),
			)
		case postOrderEndpoint:
			if postHandler == nil {
				t.Fatalf("unexpected POST /order")
			}
			postHandler(w, r)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
}
