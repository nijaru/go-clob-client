package clob

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/nijaru/go-clob-client/signing"
)

func TestEOAHandlePinsCompletionIntent(t *testing.T) {
	local, err := signing.NewLocalSigner(gaslessTestKey)
	if err != nil {
		t.Fatal(err)
	}
	calls := []TransactionCall{
		{To: common.HexToAddress("0x1"), Value: big.NewInt(3), Data: []byte{1}},
		{To: common.HexToAddress("0x2"), Value: big.NewInt(4), Data: []byte{2}},
	}
	backend := completingWallet{
		LocalSigner: local,
		send: func(_ context.Context, r signing.TransactionRequest) (common.Hash, error) {
			return common.BigToHash(big.NewInt(int64(r.Data[0]))), nil
		},
		wait: func(ctx context.Context, r signing.TransactionRequest, original common.Hash) (*types.Transaction, *types.Receipt, error) {
			i := original.Big().Int64()
			if i < 1 || i > 2 || r.To != common.BigToAddress(big.NewInt(i)) ||
				r.Value.Int64() != i+2 ||
				len(r.Data) != 1 ||
				int64(r.Data[0]) != i {
				t.Fatalf("completion intent aliased: %+v %s", r, original)
			}
			tx, err := local.SignTransaction(
				ctx,
				r.ChainID,
				types.NewTx(
					&types.DynamicFeeTx{
						ChainID:   r.ChainID,
						To:        &r.To,
						Value:     r.Value,
						Data:      r.Data,
						Gas:       100000,
						GasFeeCap: big.NewInt(20),
						GasTipCap: big.NewInt(2),
					},
				),
			)
			if err != nil {
				return nil, nil, err
			}
			return tx, &types.Receipt{
				TxHash:      tx.Hash(),
				Status:      1,
				BlockHash:   common.HexToHash("0xbeef"),
				BlockNumber: big.NewInt(1),
			}, nil
		},
	}
	client, err := NewSignerClient(
		Config{Signer: backend, ChainID: PolygonChainID, RPCURL: "http://127.0.0.1:1"},
	)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := client.ExecuteEOACalls(t.Context(), calls)
	if err != nil {
		t.Fatal(err)
	}
	for i := range calls {
		calls[i].To = common.HexToAddress("0x999")
		calls[i].Value.SetInt64(999)
		calls[i].Data[0] = 99
	}
	// Reporting fields are not authoritative reconciliation state.
	handle.Submissions[1].TransactionHash = common.HexToHash("0x999").Hex()
	handle.RequestedCalls = 99
	outcome, err := handle.Wait(t.Context())
	if err != nil || outcome == nil || outcome.TransactionHash == handle.TransactionHash {
		t.Fatalf("pinned intent/final outcome lost: %+v %v", outcome, err)
	}
}
