package bridge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetSupportedAssets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != supportedAssetsEndpoint {
			t.Errorf("expected path %s, got %s", supportedAssetsEndpoint, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(
			[]byte(
				`[{"chainId":"1","chainName":"Ethereum","tokenAddress":"0x...","tokenSymbol":"USDC"}]`,
			),
		)
	}))
	defer server.Close()

	client := New(Config{Host: server.URL})
	assets, err := client.GetSupportedAssets(context.Background())
	if err != nil {
		t.Fatalf("failed to get supported assets: %v", err)
	}

	if len(assets) != 1 {
		t.Errorf("expected 1 asset, got %d", len(assets))
	}
	if assets[0].ChainID != "1" {
		t.Errorf("expected chainId 1, got %s", assets[0].ChainID)
	}
}

func TestCreateDepositAddress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"addresses":[{"network":"ethereum","address":"0xabc"}]}`))
	}))
	defer server.Close()

	client := New(Config{Host: server.URL})
	addrs, err := client.CreateDepositAddress(context.Background(), "0x123")
	if err != nil {
		t.Fatalf("failed to create deposit address: %v", err)
	}

	if len(addrs.Addresses) != 1 {
		t.Errorf("expected 1 address, got %d", len(addrs.Addresses))
	}
	if addrs.Addresses[0].Network != "ethereum" {
		t.Errorf("expected network ethereum, got %s", addrs.Addresses[0].Network)
	}
}

func TestGetStatus(t *testing.T) {
	address := "0x123"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := statusEndpoint + "/" + address
		if r.URL.Path != expectedPath {
			t.Errorf("expected path %s, got %s", expectedPath, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"transactions":[{"id":"tx1","status":"COMPLETED"}]}`))
	}))
	defer server.Close()

	client := New(Config{Host: server.URL})
	status, err := client.GetStatus(context.Background(), address)
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}

	if len(status.Transactions) != 1 {
		t.Errorf("expected 1 transaction, got %d", len(status.Transactions))
	}
	if status.Transactions[0].ID != "tx1" {
		t.Errorf("expected transaction id tx1, got %s", status.Transactions[0].ID)
	}
}

func TestWithdraw(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != withdrawEndpoint {
			t.Errorf("expected path %s, got %s", withdrawEndpoint, r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"transactionId":"tx_withdraw","status":"PENDING"}`))
	}))
	defer server.Close()

	client := New(Config{Host: server.URL})
	resp, err := client.Withdraw(context.Background(), WithdrawRequest{
		ToAddress: "0xabc",
		Amount:    "100",
		FromToken: "USDC",
		ToToken:   "USDC",
		ToChain:   "1",
	})
	if err != nil {
		t.Fatalf("failed to withdraw: %v", err)
	}

	if resp.TransactionID != "tx_withdraw" {
		t.Errorf("expected transaction id tx_withdraw, got %s", resp.TransactionID)
	}
}
