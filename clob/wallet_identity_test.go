package clob

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCurrentDepositWalletFactoryContract(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, result  string
		beacon, fails bool
	}{
		{"beacon", `"0x0000000000000000000000007a18edfe055488a3128f01f563e5b479d92ffc3a"`, true, false},
		{"legacy-zero", `"0x0000000000000000000000000000000000000000000000000000000000000000"`, false, false},
		{"legacy-empty", `"0x"`, false, false},
		{"legacy-revert", `{"code":-32000,"message":"execution reverted"}`, false, false},
		{"rpc-error", `{"code":-32000,"message":"upstream unavailable"}`, false, true},
		{"malformed", `"0x12"`, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var request struct {
						ID json.RawMessage `json:"id"`
					}
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						t.Error(err)
						return
					}
					field := "result"
					if test.result[0] == '{' {
						field = "error"
					}
					fmt.Fprintf(
						w,
						`{"jsonrpc":"2.0","id":%s,%q:%s}`,
						request.ID,
						field,
						test.result,
					)
				}),
			)
			t.Cleanup(server.Close)
			client := newSessionOwner(t, server.URL)
			client.rpcURL = server.URL
			actual, err := client.DeriveCurrentDepositWallet(t.Context())
			if test.fails {
				if err == nil {
					t.Fatal("non-revert error must not select UUPS")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want, _ := DeriveUUPSDepositWallet(client.signer.Address(), 137)
			if test.beacon {
				want, _ = DeriveBeaconDepositWallet(client.signer.Address(), 137)
			}
			if actual != want {
				t.Fatalf("factory-selected wallet: %s, want %s", actual, want)
			}
		})
	}
}
