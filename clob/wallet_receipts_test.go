package clob

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestGaslessWalletReceipt(t *testing.T) {
	t.Parallel()
	hash := common.HexToHash("0x123")
	var receiptStatus atomic.Uint64
	receiptStatus.Store(types.ReceiptStatusSuccessful)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rpc" {
			assertRelayerKeyHeaders(t, r)
			switch r.URL.Path {
			case "/v1/account/transactions/params":
				fmt.Fprint(w, `{"nonce":"7"}`)
			case "/submit":
				fmt.Fprint(w, `{"state":"STATE_NEW","transactionID":"receipt"}`)
			case "/v1/account/transactions/receipt":
				fmt.Fprintf(
					w,
					`{"state":"STATE_CONFIRMED","transaction_id":"receipt","transaction_hash":%q}`,
					hash.Hex(),
				)
			default:
				t.Errorf("unexpected endpoint %s", r.URL.Path)
			}
			return
		}
		if r.Header.Get("RELAYER_API_KEY") != "" {
			t.Error("relayer credentials leaked to RPC")
		}
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params []common.Hash   `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		if req.Method != "eth_getTransactionReceipt" || len(req.Params) != 1 ||
			req.Params[0] != hash {
			t.Error("wrong receipt request")
		}
		receipt := &types.Receipt{
			Status:      receiptStatus.Load(),
			TxHash:      hash,
			BlockNumber: big.NewInt(42),
			BlockHash:   common.HexToHash("0xabc"),
			Logs: []*types.Log{
				{Address: common.HexToAddress("0x1"), Topics: []common.Hash{}, Data: []byte{}},
			},
		}
		body, err := json.Marshal(receipt)
		if err != nil {
			t.Error(err)
			return
		}
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":%s}`, req.ID, body)
	}))
	t.Cleanup(server.Close)
	client := newSessionOwner(t, server.URL)
	client.rpcURL = server.URL + "/rpc"
	ctx := testRelayerKeyContext(t)
	handle, err := client.ExecuteWalletTransaction(
		ctx,
		[]TransactionCall{{To: common.HexToAddress("0x1"), Value: big.NewInt(0)}},
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := handle.WaitReceipt(ctx)
	if err != nil || receipt.TxHash != hash || receipt.BlockNumber.Int64() != 42 ||
		len(receipt.Logs) != 1 {
		t.Fatalf("receipt: %+v %v", receipt, err)
	}
	receiptStatus.Store(types.ReceiptStatusFailed)
	if _, err := client.WaitWalletTransactionReceipt(ctx, TransactionOutcome{TransactionHash: hash.Hex()}); !errors.Is(
		err,
		ErrWalletTransactionFailed,
	) {
		t.Fatalf("reverted receipt: %v", err)
	}
	for _, badHash := range []string{"", "0x1", common.Hash{}.Hex(), "0x" + fmt.Sprintf("%064s", "z")} {
		if _, err := client.WaitWalletTransactionReceipt(ctx, TransactionOutcome{TransactionHash: badHash}); err == nil {
			t.Fatal("invalid receipt hash accepted")
		}
	}
}
