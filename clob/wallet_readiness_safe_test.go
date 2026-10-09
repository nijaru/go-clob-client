package clob

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestSafeWalletDeploymentAndReadiness(t *testing.T) {
	t.Parallel()
	// Signature fixture from py-builder-relayer-client 4968df2,
	// tests/builder/test_create.py (public Anvil key, zero-payment Polygon create).
	const key = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	const signature = "0xe3e791c24134b7bebe93b4771bd07c7fe7bbe115eeb0bf629ac3b7a435e7ac8d05f979729d873f7d0e16205becf48ee450aa382bc28c65eedcd6454e81d81f921b"
	owner := common.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	wallet, err := DeriveSafeWallet(owner, PolygonChainID)
	if err != nil {
		t.Fatal(err)
	}
	var submitted atomic.Int32
	var confirmed atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/deployed":
			if r.Header.Get("RELAYER_API_KEY") == "" {
				assertPublicWalletHeaders(t, r)
				if r.URL.Query().Has("type") {
					t.Error("public Safe probe must omit type")
				}
			} else {
				assertRelayerKeyHeaders(t, r)
				if r.URL.Query().Get("type") != "SAFE" {
					t.Error("configured Safe probe must retain its type")
				}
			}
			if r.Method != "GET" || r.URL.Query().Get("address") != wallet.Hex() {
				t.Errorf("unexpected Safe probe: %s", r.URL)
			}
			fmt.Fprintf(w, `{"deployed":%t}`, confirmed.Load())
		case "/submit":
			assertRelayerKeyHeaders(t, r)
			if r.Method != "POST" || submitted.Add(1) != 1 {
				t.Error("unexpected Safe submission")
			}
			var body map[string]json.RawMessage
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				return
			}
			wc, _ := getWalletConfig(PolygonChainID)
			for name, want := range map[string]string{
				"type": "SAFE-CREATE", "from": owner.Hex(), "to": common.HexToAddress(wc.SafeFactory).Hex(),
				"proxyWallet": wallet.Hex(), "data": "0x", "signature": signature, "metadata": "deploy-Safe",
			} {
				var got string
				if err := json.Unmarshal(body[name], &got); err != nil || got != want {
					t.Errorf("%s: %s %v, want %s", name, got, err, want)
				}
			}
			var params map[string]string
			if err := json.Unmarshal(body["signatureParams"], &params); err != nil {
				t.Error(err)
			}
			zero := (common.Address{}).Hex()
			if len(params) != 3 || params["paymentToken"] != zero ||
				params["paymentReceiver"] != zero ||
				params["payment"] != "0" ||
				len(body) != 8 {
				t.Errorf("unexpected signed payment fields: %v", body)
			}
			fmt.Fprint(w, `{"state":"STATE_NEW","transactionID":"safe-create"}`)
		case "/v1/account/transactions/safe-create":
			assertRelayerKeyHeaders(t, r)
			confirmed.Store(true)
			fmt.Fprint(
				w,
				`{"state":"STATE_CONFIRMED","transaction_id":"safe-create","transaction_hash":"0x0000000000000000000000000000000000000000000000000000000000000001"}`,
			)
		default:
			t.Errorf(
				"unexpected request (readiness must not imply receipt verification): %s",
				r.URL,
			)
			http.Error(w, "unexpected", 404)
		}
	}))
	t.Cleanup(server.Close)
	client, err := NewAuthenticatedClient(Config{
		PrivateKey: key, ChainID: PolygonChainID, SignatureType: SignatureTypePolyGnosisSafe,
		Credentials: &Credentials{Key: "clob-only", Secret: "c2VjcmV0", Passphrase: "p"},
		RelayerHost: server.URL, RPCURL: server.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if submitted.Load() != 0 || client.WalletAddress() != wallet {
		t.Fatal("constructor mutated or selected wrong identity")
	}
	// CLOB credentials alone cannot submit the deployment.
	if _, err := client.DeploySafeWallet(t.Context(), ""); err == nil || submitted.Load() != 0 {
		t.Fatalf("missing relayer auth accepted: %v", err)
	}
	result, err := client.EnsureWalletReady(testRelayerKeyContext(t), "deploy-Safe")
	if err != nil || !result.Deployed || result.Deployment == nil || result.Transaction == nil ||
		result.Transaction.TransactionID != "safe-create" {
		t.Fatalf("Safe readiness: %+v %v", result, err)
	}
	if submitted.Load() != 1 || client.WalletAddress() != wallet {
		t.Fatal("deployment changed wallet or duplicated write")
	}
	again, err := client.EnsureWalletReady(testRelayerKeyContext(t), "no write")
	if err != nil || !again.Deployed || again.Deployment != nil {
		t.Fatalf("already ready: %+v %v", again, err)
	}
	if _, err := client.DeploySafeWallet(testRelayerKeyContext(t), ""); !errors.Is(
		err,
		ErrWalletAlreadyDeployed,
	) {
		t.Fatalf("already deployed: %v", err)
	}
	for _, sig := range []SignatureType{SignatureTypeEOA, SignatureTypePolyProxy, SignatureTypePoly1271} {
		client.signatureType = sig
		if _, err := client.DeploySafeWallet(testRelayerKeyContext(t), ""); !errors.Is(
			err,
			ErrSafeWalletDeploymentIdentity,
		) {
			t.Fatalf("wrong wallet type %d: %v", sig, err)
		}
	}
	if submitted.Load() != 1 {
		t.Fatal("rejected deployment submitted writes")
	}
}
