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
		w.Write([]byte(`[{"network":"ethereum","address":"0xabc"}]`))
	}))
	defer server.Close()

	client := New(Config{Host: server.URL})
	addrs, err := client.CreateDepositAddress(context.Background(), "0x123")
	if err != nil {
		t.Fatalf("failed to create deposit address: %v", err)
	}

	if len(addrs) != 1 {
		t.Errorf("expected 1 address, got %d", len(addrs))
	}
	if addrs[0].Network != "ethereum" {
		t.Errorf("expected network ethereum, got %s", addrs[0].Network)
	}
}
