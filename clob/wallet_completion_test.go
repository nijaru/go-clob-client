package clob

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/nijaru/go-clob-client/signing"
)

type completingWallet struct {
	*signing.LocalSigner
	send func(context.Context, signing.TransactionRequest) (common.Hash, error)
	wait func(context.Context, signing.TransactionRequest, common.Hash) (*types.Transaction, *types.Receipt, error)
}

func (w completingWallet) SendTransaction(
	ctx context.Context,
	req signing.TransactionRequest,
) (common.Hash, error) {
	return w.send(ctx, req)
}

func (w completingWallet) WaitTransaction(
	ctx context.Context,
	req signing.TransactionRequest,
	hash common.Hash,
) (*types.Transaction, *types.Receipt, error) {
	return w.wait(ctx, req, hash)
}

func TestDirectTokenAndCTFReplacementReceipts(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"transfer", "split"} {
		for _, revert := range []bool{false, true} {
			t.Run(
				operation+map[bool]string{false: " success", true: " revert"}[revert],
				func(t *testing.T) {
					local, err := signing.NewLocalSigner(gaslessTestKey)
					if err != nil {
						t.Fatal(err)
					}
					original := common.HexToHash("0x123")
					var submitted signing.TransactionRequest
					var final common.Hash
					backend := completingWallet{
						LocalSigner: local,
						send: func(_ context.Context, req signing.TransactionRequest) (common.Hash, error) {
							submitted = req
							return original, nil
						},
						wait: func(ctx context.Context, req signing.TransactionRequest, hash common.Hash) (*types.Transaction, *types.Receipt, error) {
							if hash != original || req.To != submitted.To ||
								req.Value.Cmp(submitted.Value) != 0 ||
								string(req.Data) != string(submitted.Data) {
								t.Fatal("completion did not receive original intent")
							}
							tx, err := local.SignTransaction(
								ctx,
								req.ChainID,
								types.NewTx(
									&types.DynamicFeeTx{
										ChainID:   req.ChainID,
										To:        &req.To,
										Value:     req.Value,
										Data:      req.Data,
										Gas:       100000,
										GasFeeCap: big.NewInt(20),
										GasTipCap: big.NewInt(2),
									},
								),
							)
							if err != nil {
								return nil, nil, err
							}
							final = tx.Hash()
							status := uint64(1)
							if revert {
								status = 0
							}
							return tx, &types.Receipt{
								TxHash:      final,
								Status:      status,
								BlockHash:   common.HexToHash("0xbeef"),
								BlockNumber: big.NewInt(1),
							}, nil
						},
					}
					client, err := NewSignerClient(
						Config{
							Signer:  backend,
							ChainID: PolygonChainID,
							RPCURL:  "http://127.0.0.1:1",
						},
					)
					if err != nil {
						t.Fatal(err)
					}
					var receipt *TxReceipt
					if operation == "transfer" {
						receipt, err = client.TransferERC20(
							t.Context(),
							ERC20TransferRequest{
								TokenAddress:     common.HexToAddress("0x1"),
								RecipientAddress: common.HexToAddress("0x2"),
								Amount:           big.NewInt(42),
							},
						)
					} else {
						receipt, err = client.SplitPosition(t.Context(), SplitPositionRequest{CollateralToken: common.HexToAddress("0x3"), Partition: []*big.Int{big.NewInt(1), big.NewInt(2)}, Amount: big.NewInt(7)})
					}
					if revert {
						var attempt *WalletTransactionError
						if receipt != nil || !errors.Is(err, ErrWalletTransactionFailed) ||
							!errors.As(err, &attempt) ||
							attempt.Submission.TransactionHash != original.Hex() ||
							attempt.Submission.ConfirmedReceipt == nil ||
							attempt.Submission.ConfirmedReceipt.TxHash != final {
							t.Fatalf("revert/original/final hash lost: %+v %v", receipt, err)
						}
					} else if err != nil || receipt == nil || receipt.Hash != final || final == original {
						t.Fatalf("replacement not completed: %+v %v", receipt, err)
					}
				},
			)
		}
	}
}
