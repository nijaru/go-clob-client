package signing

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

type sendingSigner struct {
	*callbackSigner
	send func(context.Context, TransactionRequest) (common.Hash, error)
}

func (s sendingSigner) SendTransaction(
	ctx context.Context,
	req TransactionRequest,
) (common.Hash, error) {
	return s.send(ctx, req)
}

func sendRequest() TransactionRequest {
	return TransactionRequest{
		ChainID: big.NewInt(137),
		To:      common.HexToAddress("0x1"),
		Value:   big.NewInt(42),
		Data:    []byte{1, 2},
	}
}

func TestVerifiedLocalWalletKeepsItsActualCapabilities(t *testing.T) {
	original := wallet(t, local(t))
	rebound := wallet(t, original)
	if rebound.CanSendTransactions() {
		t.Fatal("binding a verified local signer invented wallet-owned sending")
	}
	if _, err := rebound.SendTransaction(t.Context(), sendRequest()); !errors.Is(
		err,
		ErrTransactionSendingUnsupported,
	) {
		t.Fatalf("unsupported sender was treated as an uncertain broadcast: %v", err)
	}
}

func TestSenderIntentIsolation(t *testing.T) {
	t.Parallel()
	address := local(t).Address()
	source := &callbackSigner{address: address}
	req := sendRequest()
	hash := common.HexToHash("0x123")
	w := wallet(
		t,
		sendingSigner{
			callbackSigner: source,
			send: func(ctx context.Context, got TransactionRequest) (common.Hash, error) {
				if ctx != t.Context() || got.From != address || got.To != req.To ||
					got.ChainID.Cmp(req.ChainID) != 0 ||
					got.Value.Cmp(req.Value) != 0 ||
					string(got.Data) != string(req.Data) {
					t.Fatalf("wrong intent: %+v", got)
				}
				got.ChainID.SetInt64(1)
				got.Value.SetInt64(0)
				got.Data[0] = 9
				return hash, nil
			},
		},
	)
	source.address = common.HexToAddress("0x2")
	if !w.CanSendTransactions() {
		t.Fatal("capability not exposed")
	}
	if got, err := w.SendTransaction(t.Context(), req); err != nil || got != hash {
		t.Fatalf("%s %v", got, err)
	}
	if req.ChainID.Int64() != 137 || req.Value.Int64() != 42 || req.Data[0] != 1 {
		t.Fatal("sender mutated original request")
	}
}

func TestSenderUncertaintyAndCancellation(t *testing.T) {
	t.Parallel()
	backendErr := errors.New("lost wallet acknowledgement")
	for _, tc := range []struct {
		name   string
		hash   common.Hash
		err    error
		cancel bool
		want   error
	}{
		{name: "known error", hash: common.HexToHash("0x123"), err: backendErr, want: backendErr},
		{name: "unknown error", err: backendErr, want: backendErr},
		{name: "invalid success", want: ErrInvalidTransactionHash},
		{name: "known cancellation", hash: common.HexToHash("0x123"), cancel: true, want: context.Canceled},
		{name: "unknown cancellation", cancel: true, want: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			calls := 0
			w := wallet(
				t,
				sendingSigner{
					callbackSigner: &callbackSigner{address: local(t).Address()},
					send: func(got context.Context, _ TransactionRequest) (common.Hash, error) {
						calls++
						if got != ctx {
							t.Fatal("context replaced")
						}
						if tc.cancel {
							cancel()
						}
						return tc.hash, tc.err
					},
				},
			)
			hash, err := w.SendTransaction(ctx, sendRequest())
			var sendErr *TransactionSendError
			if hash != tc.hash || !errors.Is(err, tc.want) || !errors.As(err, &sendErr) ||
				sendErr.Hash != hash {
				t.Fatalf("hash/uncertainty lost: %s %v", hash, err)
			}
			cancel()
			if _, err := w.SendTransaction(ctx, sendRequest()); !errors.Is(err, context.Canceled) ||
				calls != 1 {
				t.Fatal("cancelled request prompted wallet")
			}
		})
	}
}

func TestSenderCapabilityAndValidation(t *testing.T) {
	t.Parallel()
	w := wallet(t, &callbackSigner{address: local(t).Address()})
	w = wallet(t, w) // Config.Signer may itself already be a verified Wallet.
	if w.CanSendTransactions() {
		t.Fatal("invented sending capability")
	}
	if _, err := w.SendTransaction(t.Context(), sendRequest()); !errors.Is(
		err,
		ErrTransactionSendingUnsupported,
	) {
		t.Fatal(err)
	}
	w = wallet(
		t,
		sendingSigner{
			callbackSigner: &callbackSigner{address: local(t).Address()},
			send: func(context.Context, TransactionRequest) (common.Hash, error) {
				t.Fatal("invalid request prompted wallet")
				return common.Hash{}, nil
			},
		},
	)
	for _, mutate := range []func(*TransactionRequest){
		func(r *TransactionRequest) { r.ChainID = nil },
		func(r *TransactionRequest) { r.Value = big.NewInt(-1) },
		func(r *TransactionRequest) { r.Value = new(big.Int).Lsh(big.NewInt(1), 256) },
	} {
		r := sendRequest()
		mutate(&r)
		_, err := w.SendTransaction(t.Context(), r)
		var sendErr *TransactionSendError
		if err == nil || errors.As(err, &sendErr) {
			t.Fatal("invalid input claimed a send attempt")
		}
	}
}
