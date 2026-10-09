package clob

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestLegacyOrderSigningAndWireContract(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case versionEndpoint:
			_, _ = w.Write([]byte(`{"version":1}`))
		case feeRateEndpoint:
			if r.URL.Query().Get("token_id") != "123" {
				t.Error("wrong fee asset")
			}
			_, _ = w.Write([]byte(`{"base_fee":10}`))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := NewSignerClient(
		Config{
			Host:       server.URL,
			PrivateKey: "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	client.SetTickSize("123", TickSizeHundredth)
	client.SetNegRisk("123", false)
	client.saltGenerator = func() (uint64, error) { return 42, nil }
	fee := uint32(10)
	args := OrderArgs{
		TokenID:    "123",
		Price:      MustDec("0.5"),
		Size:       MustDec("200"),
		Side:       SideBuy,
		Nonce:      7,
		FeeRateBps: &fee,
	}
	order, err := client.CreateOrder(t.Context(), args, nil)
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Version   int
		Signature string
	}
	data, err := os.ReadFile("testdata/order_signatures.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if order.Legacy == nil || order.Legacy.Nonce != "7" || order.Legacy.FeeRateBps != "10" ||
		order.Signature != fixtures[0].Signature {
		t.Fatal("V1 signing disagrees with independent fixture")
	}
	wire, err := json.Marshal(order)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip SignedOrder
	if err := json.Unmarshal(wire, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if roundTrip.Legacy == nil || roundTrip.Order.Timestamp != "" ||
		roundTrip.Order.Metadata != "" ||
		roundTrip.Order.Builder != "" ||
		roundTrip.Signature != order.Signature {
		t.Fatal("V1/V2 fields mixed on wire")
	}
	fee = 11
	if _, err := client.CreateOrder(t.Context(), args, nil); err == nil {
		t.Fatal("accepted a fee override different from the market")
	}
}

func TestSignedOrderDecoderPreservesIntegerSalt(t *testing.T) {
	t.Parallel()
	const largeSalt = "9007199254740993"
	var order SignedOrder
	if err := json.Unmarshal([]byte(`{"salt":9007199254740993,"side":"BUY","signatureType":0}`), &order); err != nil {
		t.Fatal(err)
	}
	if order.Order.Salt != largeSalt {
		t.Fatalf("salt lost precision: %s", order.Order.Salt)
	}
	if err := json.Unmarshal([]byte(`{"salt":1,"side":"BUY","signatureType":0,"taker":"0x0","nonce":"0","feeRateBps":"0","timestamp":"1"}`), &order); err == nil {
		t.Fatal("accepted mixed protocol payload")
	}
}
