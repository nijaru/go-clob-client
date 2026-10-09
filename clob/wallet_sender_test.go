package clob

import (
	"context"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/nijaru/go-clob-client/signing"
)

// No MessageSigner, TransactionSigner, private key or CLOB credentials.
type externalSendingWallet struct {
	address common.Address
	send    func(context.Context, signing.TransactionRequest) (common.Hash, error)
}

func (w *externalSendingWallet) Address() common.Address { return w.address }
func (w *externalSendingWallet) SignTypedData(context.Context, apitypes.TypedData) ([]byte, error) {
	return nil, errors.New("unexpected typed-data prompt")
}

func (w *externalSendingWallet) SendTransaction(
	ctx context.Context,
	req signing.TransactionRequest,
) (common.Hash, error) {
	return w.send(ctx, req)
}

func externalSenderClient(
	t *testing.T,
	fixture *walletRPCFixture,
	send func(context.Context, signing.TransactionRequest) (common.Hash, error),
) *SignerClient {
	t.Helper()
	fixture.receiptOnly = true
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fixture.serve(t, w, r) }),
	)
	t.Cleanup(server.Close)
	client, err := NewSignerClient(
		Config{
			ChainID: PolygonChainID,
			RPCURL:  server.URL,
			Signer:  &externalSendingWallet{address: common.HexToAddress("0x123"), send: send},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestExternalSenderConsumerIntent(t *testing.T) {
	t.Parallel()
	transfer := ERC20TransferRequest{
		TokenAddress:     common.HexToAddress("0x1"),
		RecipientAddress: common.HexToAddress("0x2"),
		Amount:           big.NewInt(42),
	}
	split := SplitPositionRequest{
		CollateralToken: common.HexToAddress("0x3"),
		Partition:       []*big.Int{big.NewInt(1), big.NewInt(2)},
		Amount:          big.NewInt(7),
	}
	transferData, err := packERC20Transfer(transfer)
	if err != nil {
		t.Fatal(err)
	}
	splitData, err := packSplitPosition(split)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		data []byte
		to   common.Address
		run  func(context.Context, *SignerClient) (*TxReceipt, error)
	}{
		{name: "transfer", data: transferData, to: transfer.TokenAddress, run: func(ctx context.Context, c *SignerClient) (*TxReceipt, error) { return c.TransferERC20(ctx, transfer) }},
		{name: "CTF split", data: splitData, to: common.HexToAddress("0x4D97DCd97eC945f40cF65F87097ACe5EA0476045"), run: func(ctx context.Context, c *SignerClient) (*TxReceipt, error) { return c.SplitPosition(ctx, split) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hash := common.HexToHash("0xabc")
			calls := 0
			client := externalSenderClient(
				t,
				&walletRPCFixture{},
				func(ctx context.Context, req signing.TransactionRequest) (common.Hash, error) {
					calls++
					if ctx != t.Context() || req.From != common.HexToAddress("0x123") ||
						req.ChainID.Int64() != PolygonChainID ||
						req.To != tc.to ||
						req.Value.Sign() != 0 ||
						string(req.Data) != string(tc.data) {
						t.Fatalf("wrong consumer intent: %+v", req)
					}
					return hash, nil
				},
			)
			receipt, err := tc.run(t.Context(), client)
			if err != nil || receipt == nil || receipt.Hash != hash || calls != 1 {
				t.Fatalf("%+v %v, calls %d", receipt, err, calls)
			}
		})
	}
}

func TestExternalSenderBatchReconciliation(t *testing.T) {
	t.Parallel()
	backendErr := errors.New("wallet acknowledgement lost")
	for _, tc := range []struct {
		name    string
		failAt  int
		unknown bool
		cancel  bool
		waitErr error
	}{
		{name: "known last uncertainty resolves", failAt: 2},
		{name: "unknown first", failAt: 1, unknown: true, waitErr: ErrWalletTransactionUnknownHash},
		{name: "unknown last cannot claim completion", failAt: 2, unknown: true, waitErr: ErrWalletTransactionUnknownHash},
		{name: "cancel after known first send", failAt: 1, cancel: true, waitErr: ErrWalletTransactionIncomplete},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			calls := reconciliationCalls(2)
			sends := 0
			client := externalSenderClient(
				t,
				&walletRPCFixture{},
				func(got context.Context, req signing.TransactionRequest) (common.Hash, error) {
					sends++
					if got != ctx || req.Data[0] != byte(sends-1) {
						t.Fatal("wrong context or batch intent")
					}
					// Caller mutation while awaiting the first receipt must not alter later intent.
					if sends == 1 {
						calls[1].Data[0] = 99
						calls[1].Value.SetInt64(99)
					}
					if req.Value.Sign() != 0 {
						t.Fatal("batch intent aliased")
					}
					hash := common.BigToHash(big.NewInt(int64(sends)))
					if sends == tc.failAt {
						if tc.cancel {
							cancel()
						}
						if tc.unknown {
							hash = common.Hash{}
						}
						return hash, backendErr
					}
					return hash, nil
				},
			)
			handle, err := client.ExecuteEOACalls(ctx, calls)
			var sendErr *WalletTransactionError
			if handle == nil || len(handle.Submissions) != tc.failAt ||
				handle.RequestedCalls != 2 ||
				!errors.As(err, &sendErr) ||
				!errors.Is(err, backendErr) {
				t.Fatalf("attempt lost: %+v %v", handle, err)
			}
			last := handle.Submissions[tc.failAt-1]
			if !last.BroadcastUncertain || (last.TransactionHash == "") != tc.unknown {
				t.Fatalf("wrong uncertainty: %+v", last)
			}
			if tc.cancel && !errors.Is(err, context.Canceled) {
				t.Fatal("cancellation lost")
			}
			receipts, err := handle.WaitReceipts(t.Context())
			wantReceipts := tc.failAt
			if tc.unknown {
				wantReceipts--
			}
			if len(receipts) != wantReceipts || !errors.Is(err, tc.waitErr) {
				t.Fatalf("false completion: %d %v", len(receipts), err)
			}
			if sends != tc.failAt {
				t.Fatal("reconciliation resent calls")
			}
		})
	}
}

func TestExternalSenderDirectErrorsRetainAttempt(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"known send error", "unknown send error", "zero hash success", "receipt cancellation", "invalid receipt", "revert", "pre cancelled"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			fixture := &walletRPCFixture{}
			if name == "receipt cancellation" {
				fixture.cancel = cancel
			}
			if name == "invalid receipt" {
				fixture.receiptChange = func(r *types.Receipt) { r.TxHash = common.HexToHash("0x999") }
			}
			if name == "revert" {
				fixture.receiptChange = func(r *types.Receipt) { r.Status = 0 }
			}
			sends := 0
			hash := common.HexToHash("0xabc")
			client := externalSenderClient(
				t,
				fixture,
				func(context.Context, signing.TransactionRequest) (common.Hash, error) {
					sends++
					switch name {
					case "known send error":
						return hash, errors.New("lost ack")
					case "unknown send error":
						return common.Hash{}, errors.New("lost ack")
					case "zero hash success":
						return common.Hash{}, nil
					default:
						return hash, nil
					}
				},
			)
			if name == "pre cancelled" {
				cancel()
			}
			receipt, err := client.TransferERC20(
				ctx,
				ERC20TransferRequest{
					TokenAddress:     common.HexToAddress("0x1"),
					RecipientAddress: common.HexToAddress("0x2"),
					Amount:           big.NewInt(1),
				},
			)
			var sendErr *WalletTransactionError
			if receipt != nil || err == nil {
				t.Fatalf("false success: %+v %v", receipt, err)
			}
			if name == "pre cancelled" {
				if sends != 0 || errors.As(err, &sendErr) || !errors.Is(err, context.Canceled) {
					t.Fatal("cancelled before send claimed attempt")
				}
				return
			}
			if !errors.As(err, &sendErr) {
				t.Fatalf("attempt uninspectable: %v", err)
			}
			unknown := name == "unknown send error" || name == "zero hash success"
			if (sendErr.Submission.TransactionHash == "") != unknown {
				t.Fatal("hash lost or invented")
			}
			if name == "revert" &&
				(sendErr.Submission.ConfirmedReceipt == nil || !errors.Is(err, ErrWalletTransactionFailed)) {
				t.Fatal("revert receipt lost")
			}
		})
	}
}
