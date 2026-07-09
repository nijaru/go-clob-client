package clob

import (
	"context"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"github.com/nijaru/go-clob-client/internal/polyrelay"
)

const gaslessTestKey = "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c"

func TestRelayerWalletType(t *testing.T) {
	t.Parallel()
	cases := []struct {
		sig  SignatureType
		want polyrelay.RelayerTransactionType
		errs bool
	}{
		{SignatureTypePolyProxy, polyrelay.TransactionTypeProxy, false},
		{SignatureTypePolyGnosisSafe, polyrelay.TransactionTypeSafe, false},
		{SignatureTypePoly1271, polyrelay.TransactionTypeWallet, false},
		{SignatureTypeEOA, "", true},
	}
	for _, c := range cases {
		got, err := c.sig.relayerWalletType()
		if c.errs {
			if err == nil {
				t.Fatalf("%d: expected error, got %s", c.sig, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Fatalf("%d: got %s/%v, want %s", c.sig, got, err, c.want)
		}
	}
}

func newGaslessClient(t *testing.T, sig SignatureType, relayerURL string) *AuthenticatedClient {
	t.Helper()
	cfg := Config{
		ChainID:              PolygonChainID,
		PrivateKey:           gaslessTestKey,
		SignatureType:        sig,
		Credentials:          &Credentials{Key: "k", Secret: "c2VjcmV0", Passphrase: "p"},
		RelayerHost:          relayerURL,
		RPCURL:               "http://127.0.0.1:1", // closed port → estimate falls back fast
		DisableAutoHeartbeat: true,
	}
	if sig != SignatureTypeEOA {
		cfg.FunderAddress = "0x" + repeatHex(20)
	}
	c, err := NewAuthenticatedClient(cfg)
	if err != nil {
		t.Fatalf("NewAuthenticatedClient: %v", err)
	}
	return c
}

func repeatHex(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}

// TestPrepareGaslessTransactionWiring drives the full clob→polyrelay path against
// a mock relayer and asserts the request carries builder auth headers.
func TestPrepareGaslessTransactionWiring(t *testing.T) {
	t.Parallel()
	var sawBuilderHeaders bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("POLY_BUILDER_API_KEY") != "" {
			sawBuilderHeaders = true
		}
		switch r.URL.Path {
		case "/v1/account/transactions/params":
			_ = json.NewEncoder(w).Encode(map[string]string{"address": "0xRelay", "nonce": "3"})
		case "/submit":
			body, _ := io.ReadAll(r.Body)
			var parsed map[string]any
			_ = json.Unmarshal(body, &parsed)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"state":         "STATE_NEW",
				"transactionID": "tx-wired",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	client := newGaslessClient(t, SignatureTypePolyGnosisSafe, srv.URL)
	calls := []polyrelay.TransactionCall{
		{To: common.HexToAddress("0x" + repeatHex(20)), Data: []byte{0x01}, Value: big.NewInt(0)},
	}
	h, err := client.PrepareGaslessTransaction(context.Background(), calls, "merge")
	if err != nil {
		t.Fatalf("PrepareGaslessTransaction: %v", err)
	}
	if h.TransactionID != "tx-wired" {
		t.Fatalf("tx id = %s, want tx-wired", h.TransactionID)
	}
	if !sawBuilderHeaders {
		t.Fatal("relayer request missing POLY_BUILDER_* auth headers")
	}
}

func TestPrepareGaslessTransactionEOAErrors(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(srv.Close)
	client := newGaslessClient(t, SignatureTypeEOA, srv.URL)
	_, err := client.PrepareGaslessTransaction(context.Background(),
		[]polyrelay.TransactionCall{{To: common.Address{}, Value: big.NewInt(0)}}, "")
	if err == nil {
		t.Fatal("expected error for EOA gasless submission")
	}
}

func TestGaslessConfigUnsupportedChain(t *testing.T) {
	t.Parallel()
	c, err := NewAuthenticatedClient(Config{
		ChainID:              1, // Ethereum mainnet — no Polymarket wallet config
		PrivateKey:           gaslessTestKey,
		SignatureType:        SignatureTypePolyProxy,
		FunderAddress:        "0x" + repeatHex(20),
		Credentials:          &Credentials{Key: "k", Secret: "c2VjcmV0", Passphrase: "p"},
		DisableAutoHeartbeat: true,
	})
	if err != nil {
		t.Fatalf("NewAuthenticatedClient: %v", err)
	}
	if _, err := c.PrepareGaslessTransaction(context.Background(),
		[]polyrelay.TransactionCall{{To: common.Address{}, Value: big.NewInt(0)}}, ""); err == nil {
		t.Fatal("expected unsupported-chain error, got nil")
	}
}
