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
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestEOAWalletTransactionSequence(t *testing.T) {
	t.Parallel()
	var sent, confirmations atomic.Int32
	transactions := make(chan *types.Transaction, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		var result any
		switch req.Method {
		case "eth_getTransactionCount":
			result = "0x0"
		case "eth_maxPriorityFeePerGas":
			result = "0x1"
		case "eth_getBlockByNumber":
			result = &types.Header{
				Number:     big.NewInt(1),
				Difficulty: big.NewInt(0),
				GasLimit:   1000000,
				BaseFee:    big.NewInt(1),
			}
		case "eth_estimateGas":
			result = "0x5208"
		case "eth_sendRawTransaction":
			var wire string
			if err := json.Unmarshal(req.Params[0], &wire); err != nil {
				t.Error(err)
				return
			}
			raw, err := hexutil.Decode(wire)
			if err != nil {
				t.Error(err)
				return
			}
			var tx types.Transaction
			if err := tx.UnmarshalBinary(raw); err != nil {
				t.Error(err)
				return
			}
			if sent.Add(1) == 2 && confirmations.Load() == 0 {
				t.Error("second EOA call broadcast before first confirmation")
			}
			transactions <- &tx
			result = tx.Hash().Hex()
		case "eth_getTransactionReceipt":
			var hash common.Hash
			if err := json.Unmarshal(req.Params[0], &hash); err != nil {
				t.Error(err)
				return
			}
			confirmations.Add(1)
			result = &types.Receipt{
				Status:      types.ReceiptStatusSuccessful,
				TxHash:      hash,
				BlockNumber: big.NewInt(1),
				Logs:        []*types.Log{},
			}
		default:
			t.Errorf("unexpected RPC method %s", req.Method)
			return
		}
		body, err := json.Marshal(result)
		if err != nil {
			t.Error(err)
			return
		}
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":%s}`, req.ID, body)
	}))
	t.Cleanup(server.Close)
	client := newGaslessClient(t, SignatureTypeEOA, "http://127.0.0.1:1")
	client.rpcURL = server.URL
	calls := []TransactionCall{
		{To: common.HexToAddress("0x1"), Data: []byte{1}, Value: big.NewInt(3)},
		{To: common.HexToAddress("0x2"), Data: []byte{2}, Value: big.NewInt(4)},
	}
	handle, err := client.ExecuteWalletTransaction(t.Context(), calls, "")
	if err != nil {
		t.Fatal(err)
	}
	if sent.Load() != 2 || confirmations.Load() != 1 {
		t.Fatal("submission should confirm only the preceding call")
	}
	for _, call := range calls {
		tx := <-transactions
		if tx.To() == nil || *tx.To() != call.To || tx.Value().Cmp(call.Value) != 0 ||
			string(tx.Data()) != string(call.Data) {
			t.Fatal("broadcast changed EVM call")
		}
		owner, err := types.Sender(types.LatestSignerForChainID(big.NewInt(137)), tx)
		if err != nil || owner != client.signer.Address() {
			t.Fatal("wrong EOA signature")
		}
	}
	outcome, err := handle.Wait(t.Context())
	if err != nil || outcome.TransactionHash != handle.TransactionHash ||
		confirmations.Load() != 2 {
		t.Fatalf("explicit wait: %+v %v", outcome, err)
	}
}

func TestDirectCTFRejectsSmartWalletAndInvalidPartition(t *testing.T) {
	t.Parallel()
	client := newGaslessClient(t, SignatureTypePolyGnosisSafe, "http://127.0.0.1:1")
	_, err := client.SplitPosition(
		t.Context(),
		SplitBinary(common.HexToAddress("0x1"), common.HexToHash("0x1"), big.NewInt(1)),
	)
	if !errors.Is(err, ErrTokenOperationRequiresEOA) {
		t.Fatalf("smart wallet direct CTF path must reject before RPC: %v", err)
	}
	for _, req := range []SplitPositionRequest{
		{Amount: nil, Partition: BinaryPartition()},
		{Amount: big.NewInt(-1), Partition: BinaryPartition()},
		{Amount: big.NewInt(1), Partition: []*big.Int{big.NewInt(1), big.NewInt(3)}},
		{Amount: big.NewInt(1), Partition: []*big.Int{nil, big.NewInt(2)}},
	} {
		if _, err := packSplitPosition(req); err == nil {
			t.Fatal("invalid CTF partition/amount accepted")
		}
	}
}

func TestGaslessDoesNotUseCLOBCredentialsAsBuilderKeys(t *testing.T) {
	t.Parallel()
	client := newGaslessClient(t, SignatureTypePolyGnosisSafe, "http://127.0.0.1:1")
	client.builderAuth = nil
	_, err := client.PrepareGaslessTransaction(
		t.Context(),
		[]TransactionCall{{To: common.HexToAddress("0x1"), Value: big.NewInt(0)}},
		"",
	)
	if err == nil {
		t.Fatal("CLOB credentials must not be relabeled as builder credentials")
	}
}
