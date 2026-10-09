package clob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

type walletRPCFixture struct {
	mu              sync.Mutex
	receiptOnly     bool
	sent            []common.Hash
	failPreparation int
	failSend        int
	receiptChange   func(*types.Receipt)
	cancel          context.CancelFunc
}

func (f *walletRPCFixture) serve(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	var req struct {
		ID     json.RawMessage
		Method string
		Params []json.RawMessage
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		t.Error(err)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.receiptOnly && req.Method != "eth_getTransactionReceipt" {
		t.Errorf("external wallet path made SDK RPC call %s", req.Method)
		return
	}
	var result any
	rpcError := false
	switch req.Method {
	case "eth_getTransactionCount":
		rpcError = f.failPreparation == len(f.sent)+1
		result = hexutil.EncodeUint64(uint64(len(f.sent)))
	case "eth_maxPriorityFeePerGas":
		result = "0x1"
	case "eth_getBlockByNumber":
		result = &types.Header{
			Number:     big.NewInt(42),
			Difficulty: new(big.Int),
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
		f.sent = append(f.sent, tx.Hash())
		result = tx.Hash()
		// The node knows this signed transaction even though its response is an error.
		rpcError = f.failSend == len(f.sent)
	case "eth_getTransactionReceipt":
		var hash common.Hash
		if err := json.Unmarshal(req.Params[0], &hash); err != nil {
			t.Error(err)
			return
		}
		if f.cancel != nil {
			f.cancel()
			result = nil
			break
		}
		receipt := &types.Receipt{
			TxHash:      hash,
			Status:      1,
			BlockNumber: big.NewInt(42),
			BlockHash:   common.HexToHash("0xabc"),
			Logs:        []*types.Log{},
		}
		if f.receiptChange != nil {
			f.receiptChange(receipt)
		}
		result = receipt
	default:
		t.Errorf("unexpected RPC method %s", req.Method)
		return
	}
	if rpcError {
		fmt.Fprintf(
			w,
			`{"jsonrpc":"2.0","id":%s,"error":{"code":-32000,"message":"lost acknowledgement"}}`,
			req.ID,
		)
		return
	}
	body, err := json.Marshal(result)
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":%s}`, req.ID, body)
}

func walletFixtureClient(t *testing.T, fixture *walletRPCFixture) *AuthenticatedClient {
	t.Helper()
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fixture.serve(t, w, r) }),
	)
	t.Cleanup(server.Close)
	client := newGaslessClient(t, SignatureTypeEOA, "http://127.0.0.1:1")
	client.rpcURL = server.URL
	return client
}

func reconciliationCalls(n int) []TransactionCall {
	calls := make([]TransactionCall, n)
	for i := range calls {
		calls[i] = TransactionCall{
			To:    common.HexToAddress("0x1"),
			Data:  []byte{byte(i)},
			Value: new(big.Int),
		}
	}
	return calls
}

// This assertion also compiles against the original implementation.
func TestWalletRetainsHandleOnConfirmationCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	fixture := &walletRPCFixture{cancel: cancel}
	client := walletFixtureClient(t, fixture)
	handle, err := client.ExecuteWalletTransaction(ctx, reconciliationCalls(2), "")
	if !errors.Is(err, context.Canceled) || handle == nil || handle.TransactionHash == "" {
		t.Fatalf("known broadcast lost on cancellation: %+v %v", handle, err)
	}
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	if len(fixture.sent) != 1 || handle.TransactionHash != fixture.sent[0].Hex() {
		t.Fatal("sequence advanced or lost known hash")
	}
}

func TestWalletEOAPartialReconciliation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		fixture     *walletRPCFixture
		calls       int
		submissions int
		confirmed   int
		uncertain   bool
		waitErr     error
	}{
		{name: "later preparation fails", fixture: &walletRPCFixture{failPreparation: 2}, calls: 3, submissions: 1, confirmed: 1, waitErr: ErrWalletTransactionIncomplete},
		{name: "later send uncertain", fixture: &walletRPCFixture{failSend: 2}, calls: 3, submissions: 2, confirmed: 1, uncertain: true, waitErr: ErrWalletTransactionIncomplete},
		{name: "first send uncertain", fixture: &walletRPCFixture{failSend: 1}, calls: 2, submissions: 1, uncertain: true, waitErr: ErrWalletTransactionIncomplete},
		{name: "last send uncertainty resolves", fixture: &walletRPCFixture{failSend: 2}, calls: 2, submissions: 2, confirmed: 1, uncertain: true},
		{name: "reverted prefix", fixture: &walletRPCFixture{receiptChange: func(r *types.Receipt) { r.Status = 0 }}, calls: 2, submissions: 1, confirmed: 1, waitErr: ErrWalletTransactionFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := walletFixtureClient(t, tc.fixture)
			handle, err := client.ExecuteWalletTransaction(
				t.Context(),
				reconciliationCalls(tc.calls),
				"",
			)
			if err == nil || handle == nil {
				t.Fatalf("partial execution not retained: %+v %v", handle, err)
			}
			if len(handle.Submissions) != tc.submissions || handle.RequestedCalls != tc.calls {
				t.Fatalf("wrong partial batch: %+v", handle)
			}
			for i, s := range handle.Submissions {
				if s.TransactionHash != tc.fixture.sent[i].Hex() ||
					s.BroadcastUncertain != (tc.uncertain && i == tc.submissions-1) {
					t.Fatalf("wrong submission %d: %+v", i, s)
				}
				if (s.ConfirmedReceipt != nil) != (i < tc.confirmed) {
					t.Fatalf("wrong confirmation %d: %+v", i, s)
				}
			}
			if handle.TransactionHash != tc.fixture.sent[tc.submissions-1].Hex() {
				t.Fatal("last hash compatibility lost")
			}
			// Reconciliation is context-owned and safe to run concurrently, without sends.
			var waits sync.WaitGroup
			for range 2 {
				waits.Go(func() {
					receipts, err := handle.WaitReceipts(t.Context())
					if !errors.Is(err, tc.waitErr) || len(receipts) != tc.submissions {
						t.Errorf("reconcile: %d receipts, %v", len(receipts), err)
					}
					for i, receipt := range receipts {
						if receipt.TxHash.Hex() != handle.Submissions[i].TransactionHash {
							t.Error("receipt identity changed")
						}
					}
				})
			}
			waits.Wait()
			receipt, err := handle.WaitReceipt(t.Context())
			if !errors.Is(err, tc.waitErr) || receipt == nil {
				t.Fatalf("last receipt lost on error: %+v %v", receipt, err)
			}
			outcome, err := handle.Wait(t.Context())
			if !errors.Is(err, tc.waitErr) || (outcome != nil) != (tc.waitErr == nil) {
				t.Fatalf("partial batch reported success: %+v %v", outcome, err)
			}
			tc.fixture.mu.Lock()
			defer tc.fixture.mu.Unlock()
			if len(tc.fixture.sent) != tc.submissions {
				t.Fatal("wait retransmitted calls")
			}
		})
	}
}

func TestWalletInvalidReceiptStopsSequence(t *testing.T) {
	t.Parallel()
	for _, change := range []func(*types.Receipt){
		func(r *types.Receipt) { r.TxHash = common.HexToHash("0x999") },
		func(r *types.Receipt) { r.BlockNumber = nil },
		func(r *types.Receipt) { r.Status = 2 },
	} {
		fixture := &walletRPCFixture{receiptChange: change}
		client := walletFixtureClient(t, fixture)
		handle, err := client.ExecuteWalletTransaction(t.Context(), reconciliationCalls(2), "")
		if err == nil || handle == nil || len(handle.Submissions) != 1 ||
			handle.Submissions[0].ConfirmedReceipt != nil {
			t.Fatalf("invalid confirmation advanced sequence: %+v %v", handle, err)
		}
		if len(fixture.sent) != 1 {
			t.Fatal("second call sent after invalid receipt")
		}
	}
}

func TestWalletNoSubmissionCannotWaitSuccessfully(t *testing.T) {
	t.Parallel()
	fixture := &walletRPCFixture{failPreparation: 1}
	client := walletFixtureClient(t, fixture)
	handle, err := client.ExecuteWalletTransaction(t.Context(), reconciliationCalls(2), "")
	if err == nil || handle != nil {
		t.Fatalf("pre-send failure claimed submission: %+v %v", handle, err)
	}
	if outcome, err := handle.Wait(t.Context()); outcome != nil || err == nil {
		t.Fatal("empty handle claimed success")
	}
	if receipts, err := (&WalletTransactionHandle{signer: client.SignerClient}).WaitReceipts(t.Context()); len(
		receipts,
	) != 0 ||
		err == nil {
		t.Fatal("unknown submission claimed success")
	}
}
