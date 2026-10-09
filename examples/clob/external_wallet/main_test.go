package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/nijaru/go-clob-client/signing"
)

func TestBackendKeepsHashOnErrorResponse(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusOK, http.StatusBadGateway} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			hash := common.HexToHash("0x123")
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var req map[string]string
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						t.Error(err)
					}
					if r.Method != http.MethodPost || r.URL.Path != "/send-transaction" ||
						r.Header.Get("Authorization") != "Bearer test" ||
						req["chainId"] != "0x89" ||
						req["value"] != "0x2a" ||
						req["data"] != "0x0102" ||
						req["from"] != common.HexToAddress("0x1").Hex() ||
						req["to"] != common.HexToAddress("0x2").Hex() {
						t.Errorf("wrong backend intent: %+v", req)
					}
					w.WriteHeader(status)
					fmt.Fprintf(w, `{"hash":%q}`, hash.Hex())
				}),
			)
			defer server.Close()
			wallet, err := signing.NewWallet(
				&remoteWallet{
					address: common.HexToAddress("0x1"),
					url:     server.URL,
					token:   "test",
					http:    server.Client(),
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			got, err := wallet.SendTransaction(
				t.Context(),
				signing.TransactionRequest{
					ChainID: big.NewInt(137),
					To:      common.HexToAddress("0x2"),
					Value:   big.NewInt(42),
					Data:    []byte{1, 2},
				},
			)
			if got != hash || (err != nil) != (status != http.StatusOK) {
				t.Fatalf("backend hash lost: %s %v", got, err)
			}
			if err != nil {
				var sendErr *signing.TransactionSendError
				if !errors.As(err, &sendErr) || sendErr.Hash != hash {
					t.Fatal("uncertainty not inspectable")
				}
			}
		})
	}
}
