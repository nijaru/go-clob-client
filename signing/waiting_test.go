package signing

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type waitingSigner struct {
	*callbackSigner
	wait func(context.Context, TransactionRequest, common.Hash) (*types.Transaction, *types.Receipt, error)
}

func (s waitingSigner) WaitTransaction(
	ctx context.Context,
	req TransactionRequest,
	hash common.Hash,
) (*types.Transaction, *types.Receipt, error) {
	return s.wait(ctx, req, hash)
}

func TestWaiterPinnedIdentityAndReceiptProof(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"success", "unsigned", "nil transaction", "nil receipt", "negative block", "missing block hash", "backend error", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			local := local(t)
			source := &callbackSigner{address: local.Address()}
			req := sendRequest()
			req.From = common.HexToAddress("0xbad")
			original := common.HexToHash("0x123")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			backendErr := errors.New("wallet connection lost")
			calls := 0
			w := wallet(
				t,
				waitingSigner{
					callbackSigner: source,
					wait: func(got context.Context, r TransactionRequest, hash common.Hash) (*types.Transaction, *types.Receipt, error) {
						calls++
						if got != ctx || r.From != local.Address() || hash != original {
							t.Fatal("context, identity or original hash lost")
						}
						tx := types.NewTx(
							&types.DynamicFeeTx{
								ChainID:   big.NewInt(137),
								To:        &r.To,
								Value:     r.Value,
								Data:      r.Data,
								Gas:       100000,
								GasFeeCap: big.NewInt(20),
								GasTipCap: big.NewInt(2),
							},
						)
						if mode != "unsigned" {
							var err error
							tx, err = local.SignTransaction(ctx, big.NewInt(137), tx)
							if err != nil {
								t.Fatal(err)
							}
						}
						receipt := &types.Receipt{
							TxHash:      tx.Hash(),
							BlockNumber: big.NewInt(1),
							BlockHash:   common.HexToHash("0xbeef"),
							Status:      1,
						}
						switch mode {
						case "nil transaction":
							tx = nil
						case "nil receipt":
							receipt = nil
						case "negative block":
							receipt.BlockNumber = big.NewInt(-1)
						case "missing block hash":
							receipt.BlockHash = common.Hash{}
						case "cancel":
							cancel()
						}
						r.ChainID.SetInt64(1)
						r.Value.SetInt64(0)
						r.Data[0] = 99
						if mode == "backend error" {
							return tx, receipt, backendErr
						}
						return tx, receipt, nil
					},
				},
			)
			source.address = common.HexToAddress("0xbad") // constructor pins identity
			rebound := wallet(t, w)
			if rebound != w || !rebound.CanWaitTransactions() {
				t.Fatal("completion capability lost on rebind")
			}
			tx, receipt, err := rebound.WaitTransaction(ctx, req, original)
			switch mode {
			case "success":
				if err != nil || tx == nil || receipt == nil || receipt.TxHash != tx.Hash() ||
					receipt.TxHash == original {
					t.Fatalf("completion lost: %v %v", receipt, err)
				}
			case "backend error", "cancel":
				want := backendErr
				if mode == "cancel" {
					want = context.Canceled
				}
				if receipt == nil || tx == nil || !errors.Is(err, want) {
					t.Fatalf("known completion/error lost: %v %v", receipt, err)
				}
			default:
				if tx != nil || receipt != nil || !errors.Is(err, ErrInvalidTransactionCompletion) {
					t.Fatalf("invalid proof trusted: %v %v", receipt, err)
				}
			}
			if req.ChainID.Int64() != 137 || req.Value.Int64() != 42 || req.Data[0] != 1 {
				t.Fatal("completion request aliases caller")
			}
			cancel()
			if _, _, err := rebound.WaitTransaction(ctx, req, original); !errors.Is(
				err,
				context.Canceled,
			) ||
				calls != 1 {
				t.Fatal("pre-cancelled completion prompted provider")
			}
		})
	}
}

func TestWaiterUnsupportedRebind(t *testing.T) {
	w := wallet(t, local(t))
	w = wallet(t, w)
	if w.CanWaitTransactions() {
		t.Fatal("invented completion capability")
	}
	if _, _, err := w.WaitTransaction(t.Context(), sendRequest(), common.HexToHash("0x1")); !errors.Is(
		err,
		ErrTransactionWaitingUnsupported,
	) {
		t.Fatal(err)
	}
}
