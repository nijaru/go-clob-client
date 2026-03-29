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
		w.Write([]byte(`{
			"supportedAssets": [
				{
					"chainId": "1",
					"chainName": "Ethereum",
					"token": {"name": "USD Coin", "symbol": "USDC", "address": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", "decimals": 6},
					"minCheckoutUsd": "45.0"
				}
			]
		}`))
	}))
	defer server.Close()

	client := New(Config{Host: server.URL})
	resp, err := client.GetSupportedAssets(context.Background())
	if err != nil {
		t.Fatalf("failed to get supported assets: %v", err)
	}

	if len(resp.SupportedAssets) != 1 {
		t.Errorf("expected 1 asset, got %d", len(resp.SupportedAssets))
	}
	if resp.SupportedAssets[0].ChainID != "1" {
		t.Errorf("expected chainId 1, got %s", resp.SupportedAssets[0].ChainID)
	}
	if resp.SupportedAssets[0].Token.Symbol != "USDC" {
		t.Errorf("expected symbol USDC, got %s", resp.SupportedAssets[0].Token.Symbol)
	}
}

func TestCreateDepositAddress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"address": {
				"evm": "0x23566f8b2E82aDfCf01846E54899d110e97AC053",
				"svm": "CrvTBvzryYxBHbWu2TiQpcqD5M7Le7iBKzVmEj3f36Jb",
				"btc": "bc1q8eau83qffxcj8ht4hsjdza3lha9r3egfqysj3g"
			},
			"note": "Only certain chains and tokens are supported."
		}`))
	}))
	defer server.Close()

	client := New(Config{Host: server.URL})
	resp, err := client.CreateDepositAddress(context.Background(), "0x123")
	if err != nil {
		t.Fatalf("failed to create deposit address: %v", err)
	}

	if resp.Address.EVM != "0x23566f8b2E82aDfCf01846E54899d110e97AC053" {
		t.Errorf("unexpected EVM address: %s", resp.Address.EVM)
	}
	if resp.Address.SVM == "" {
		t.Error("expected non-empty SVM address")
	}
	if resp.Address.BTC == "" {
		t.Error("expected non-empty BTC address")
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
		w.Write([]byte(`{
			"transactions": [
				{
					"fromChainId": "1",
					"fromTokenAddress": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
					"fromAmountBaseUnit": "13566635",
					"toChainId": "137",
					"toTokenAddress": "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174",
					"status": "COMPLETED",
					"txHash": "0xabc123"
				}
			]
		}`))
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
	if status.Transactions[0].Status != DepositStatusCompleted {
		t.Errorf("expected COMPLETED, got %s", status.Transactions[0].Status)
	}
	if status.Transactions[0].TxHash == nil || *status.Transactions[0].TxHash != "0xabc123" {
		t.Errorf("unexpected tx hash: %v", status.Transactions[0].TxHash)
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
		w.Write([]byte(`{
			"address": {
				"evm": "0x23566f8b2E82aDfCf01846E54899d110e97AC053",
				"svm": "CrvTBvzryYxBHbWu2TiQpcqD5M7Le7iBKzVmEj3f36Jb",
				"btc": "bc1q8eau83qffxcj8ht4hsjdza3lha9r3egfqysj3g"
			},
			"note": "Send funds to these addresses to bridge to your destination chain and token."
		}`))
	}))
	defer server.Close()

	client := New(Config{Host: server.URL})
	resp, err := client.Withdraw(context.Background(), WithdrawRequest{
		Address:        "0x56687bf447db6ffa42ffe2204a05edaa20f55839",
		ToChainID:      "1",
		ToTokenAddress: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		RecipientAddr:  "0x0000000000000000000000000000000000000000",
	})
	if err != nil {
		t.Fatalf("failed to withdraw: %v", err)
	}

	if resp.Address.EVM != "0x23566f8b2E82aDfCf01846E54899d110e97AC053" {
		t.Errorf("unexpected EVM address: %s", resp.Address.EVM)
	}
	if resp.Note == "" {
		t.Error("expected non-empty note")
	}
}
