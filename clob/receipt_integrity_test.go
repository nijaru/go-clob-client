package clob

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Exercise the real JSON-RPC decoder and shared waiter, not a mocked receipt reader.
func TestWalletReceiptIntegrity(t *testing.T) {
	t.Parallel()
	hash := common.HexToHash("0x123")
	for _, tc := range []struct {
		name     string
		change   func(*types.Receipt)
		invalid  bool
		reverted bool
	}{
		{name: "success"},
		{name: "revert", change: func(r *types.Receipt) { r.Status = 0 }, reverted: true},
		{name: "wrong hash", change: func(r *types.Receipt) { r.TxHash = common.HexToHash("0x456") }, invalid: true},
		{name: "missing block number", change: func(r *types.Receipt) { r.BlockNumber = nil }, invalid: true},
		{name: "missing block hash", change: func(r *types.Receipt) { r.BlockHash = common.Hash{} }, invalid: true},
		{name: "unknown status", change: func(r *types.Receipt) { r.Status = 2 }, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var req struct {
						ID     json.RawMessage
						Method string
						Params []common.Hash
					}
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						t.Error(err)
						return
					}
					if req.Method != "eth_getTransactionReceipt" || len(req.Params) != 1 ||
						req.Params[0] != hash {
						t.Error("wrong receipt request")
						return
					}
					receipt := &types.Receipt{
						TxHash:      hash,
						Status:      1,
						BlockNumber: big.NewInt(42),
						BlockHash:   common.HexToHash("0xabc"),
						Logs:        []*types.Log{},
					}
					if tc.change != nil {
						tc.change(receipt)
					}
					body, err := json.Marshal(receipt)
					if err != nil {
						t.Error(err)
						return
					}
					fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":%s}`, req.ID, body)
				}),
			)
			t.Cleanup(server.Close)
			client := newGaslessClient(t, SignatureTypeEOA, "http://127.0.0.1:1")
			client.rpcURL = server.URL
			receipt, err := client.WaitWalletTransactionReceipt(
				t.Context(),
				TransactionOutcome{TransactionHash: hash.Hex()},
			)
			if tc.invalid {
				if err == nil || receipt != nil {
					t.Fatalf("invalid receipt trusted: %+v %v", receipt, err)
				}
			} else if tc.reverted {
				if !errors.Is(err, ErrWalletTransactionFailed) || receipt == nil || receipt.Status != 0 || receipt.TxHash != hash {
					t.Fatalf("reverted receipt lost: %+v %v", receipt, err)
				}
			} else if err != nil || receipt == nil || receipt.TxHash != hash {
				t.Fatalf("valid receipt: %+v %v", receipt, err)
			}
		})
	}
}
