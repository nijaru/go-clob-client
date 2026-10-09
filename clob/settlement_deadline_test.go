package clob

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSettlementTimeoutBoundsInflightHTTPRequest(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	client, err := NewAuthenticatedClient(Config{
		Host: server.URL, PrivateKey: "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
		Credentials: &Credentials{Key: "k", Secret: "YQ==", Passphrase: "p"},
	})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = client.WaitForOrderFillSettlement(
		t.Context(),
		PostOrderResponse{TradeIDs: []string{"trade"}},
		OrderSettlementOptions{Timeout: 25 * time.Millisecond},
	)
	if !errors.Is(err, ErrSettlementTimeout) {
		t.Fatalf("wait = %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("settlement deadline did not bound the HTTP request")
	}
}
