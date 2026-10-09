package polyrelay

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

func TestDepositNonceCorrectionRejectsNonStaleOrUnrelatedNonce(t *testing.T) {
	t.Parallel()
	for _, message := range []string{
		"batch nonce 5 does not match on-chain nonce 4",
		"batch nonce 5 does not match on-chain nonce 5",
		"batch nonce 3 does not match on-chain nonce 6",
	} {
		payload := &SubmitRequest{
			Type: string(TransactionTypeWallet), Nonce: "5",
			DepositWallet: &DepositWalletParams{
				Deadline: "1700000000",
				Calls:    []DepositCall{{Target: addrRepeat(0x20).Hex(), Value: "0", Data: "0x01"}},
			},
		}
		corrected, err := CorrectDepositNonce(
			t.Context(),
			testGaslessConfig(TransactionTypeWallet),
			mustKey(t),
			payload,
			&polyhttp.APIError{StatusCode: 400, Message: message},
		)
		if err != nil || corrected != nil {
			t.Fatalf("unexpected correction for %q: %+v %v", message, corrected, err)
		}
	}
}

func TestDepositNonceCorrectionPreservesBatch(t *testing.T) {
	t.Parallel()
	for _, session := range []bool{false, true} {
		t.Run(fmt.Sprint(session), func(t *testing.T) {
			var paramsCalls, submitCalls atomic.Int32
			var original SubmitRequest
			cfg := testGaslessConfig(TransactionTypeWallet)
			cfg.SessionSigner = session
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == executeParamsPath {
						paramsCalls.Add(1)
						fmt.Fprint(w, `{"nonce":"1"}`)
						return
					}
					var body SubmitRequest
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
						return
					}
					if submitCalls.Add(1) == 1 {
						original = body
						http.Error(
							w,
							`{"error":"batch nonce 1 does not match on-chain nonce 18446744073709551616"}`,
							400,
						)
						return
					}
					if body.Nonce != "18446744073709551616" ||
						body.Signature == original.Signature ||
						!reflect.DeepEqual(body.DepositWallet, original.DepositWallet) ||
						body.Metadata != original.Metadata {
						t.Error("nonce correction did not preserve original batch")
					}
					fmt.Fprint(w, `{"state":"STATE_NEW","transactionID":"corrected"}`)
				}),
			)
			t.Cleanup(server.Close)
			calls := []TransactionCall{
				{To: addrRepeat(0x20), Data: []byte{1}, Value: big.NewInt(0)},
			}
			h, err := PrepareGasless(
				t.Context(),
				testTransport(t, server),
				cfg,
				mustKey(t),
				calls,
				"original metadata",
			)
			if err != nil || h.TransactionID != "corrected" || paramsCalls.Load() != 1 ||
				submitCalls.Load() != 2 {
				t.Fatalf("stale params correction: %+v %v", h, err)
			}
		})
	}
}
