package clob

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestWalletRelayerExecutionWithoutCLOBCredentials(t *testing.T) {
	builder, err := NewLocalBuilderAuth(Credentials{Key: "builder", Secret: "c2VjcmV0", Passphrase: "p"})
	if err != nil {
		t.Fatal(err)
	}
	for _, scheme := range []string{"builder", "api-key"} {
		t.Run(scheme, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				for _, name := range []string{"POLY_API_KEY", "POLY_SIGNATURE", "POLY_PASSPHRASE", "POLY_ADDRESS"} {
					if r.Header.Get(name) != "" {
						t.Errorf("wallet request leaked CLOB header %s", name)
					}
				}
				if scheme == "api-key" {
					assertRelayerKeyHeaders(t, r)
				} else if r.Header.Get("POLY_BUILDER_API_KEY") != "builder" || r.Header.Get("RELAYER_API_KEY") != "" {
					t.Error("wrong builder auth identity")
				}
				switch r.URL.Path {
				case "/v1/account/transactions/params":
					fmt.Fprint(w, `{"nonce":"1"}`)
				case "/submit":
					var request RelayerSubmitRequest
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						t.Error(err)
					}
					if request.Type != string(RelayerTransactionSafe) || request.Signature == "" {
						t.Errorf("invalid wallet submission: %+v", request)
					}
					fmt.Fprint(w, `{"state":"STATE_NEW","transactionID":"wallet-only"}`)
				default:
					t.Errorf("unexpected remote request %s", r.URL)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			client, err := NewSignerClient(Config{PrivateKey: gaslessTestKey, SignatureType: SignatureTypePolyGnosisSafe, RelayerHost: server.URL, Host: "http://127.0.0.1:1", BuilderAuth: builder})
			if err != nil {
				t.Fatal(err)
			}
			if requests.Load() != 0 {
				t.Fatal("constructor started remote work")
			}
			ctx := t.Context()
			if scheme == "api-key" {
				ctx = testRelayerKeyContext(t)
			}
			handle, err := client.ExecuteWalletTransaction(ctx, []TransactionCall{{To: common.HexToAddress("0x1"), Value: new(big.Int), Data: []byte{1, 2}}}, "wallet-only")
			if err != nil || handle == nil || handle.TransactionID != "wallet-only" {
				t.Fatalf("wallet required CLOB auth: %+v %v", handle, err)
			}
			if requests.Load() != 2 {
				t.Fatalf("unexpected wallet request count %d", requests.Load())
			}
		})
	}
}
