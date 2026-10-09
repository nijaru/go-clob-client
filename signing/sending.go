package signing

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// TransactionRequest is an EOA call in wei, without nonce or gas parameters.
// Wallet supplies its pinned From and gives the sender independent copies of
// ChainID, Value and Data. Contract creation is not supported.
type TransactionRequest struct {
	ChainID *big.Int
	From    common.Address
	To      common.Address
	Value   *big.Int
	Data    []byte
}

// TransactionSender optionally owns transaction authorization, signing, nonce,
// gas and broadcast. It must honor ctx and return any known hash even on error.
// An error without a hash does NOT prove that nothing was broadcast. Implementers
// must serialize nonce/device access as needed. The wallet cannot verify an
// opaque broadcast's signed payload: the sender owns request authorization and
// correct execution, unlike the verified TransactionSigner boundary.
type TransactionSender interface {
	SendTransaction(context.Context, TransactionRequest) (common.Hash, error)
}

var (
	ErrTransactionSendingUnsupported = errors.New("signing: transaction sending unsupported")
	ErrInvalidTransactionHash        = errors.New("signing: sender returned no transaction hash")
)

// TransactionSendError means the sender was invoked and broadcast may have
// occurred. Hash may be zero, in which case receipt reconciliation is impossible
// until the caller obtains the hash from its wallet. Never blindly retry.
type TransactionSendError struct {
	Hash common.Hash
	Err  error
}

func (e *TransactionSendError) Error() string {
	return fmt.Sprintf("signing: uncertain send (%s): %v", e.Hash.Hex(), e.Err)
}

func (e *TransactionSendError) Unwrap() error { return e.Err }

// CanSendTransactions reports whether this wallet owns broadcast. It does not
// perform a prompt or a network request.
func (w *Wallet) CanSendTransactions() bool {
	_, ok := w.signer.(TransactionSender)
	return ok
}

// SendTransaction delegates an isolated request to the sender. Cancellation
// after the callback cannot undo broadcast; both hash and uncertainty survive.
// A successful callback acknowledges submission, not mining or verified intent.
func (w *Wallet) SendTransaction(
	ctx context.Context,
	request TransactionRequest,
) (common.Hash, error) {
	if err := ctx.Err(); err != nil {
		return common.Hash{}, err
	}
	sender, ok := w.signer.(TransactionSender)
	if !ok {
		return common.Hash{}, ErrTransactionSendingUnsupported
	}
	request, err := w.transactionRequest(request)
	if err != nil {
		return common.Hash{}, err
	}
	hash, err := sender.SendTransaction(ctx, request)
	err = errors.Join(err, ctx.Err())
	if hash == (common.Hash{}) && err == nil {
		err = ErrInvalidTransactionHash
	}
	if err != nil {
		return hash, &TransactionSendError{Hash: hash, Err: err}
	}
	return hash, nil
}

func (w *Wallet) transactionRequest(request TransactionRequest) (TransactionRequest, error) {
	if request.ChainID == nil || request.ChainID.Sign() <= 0 || request.Value == nil ||
		request.Value.Sign() < 0 || request.Value.BitLen() > 256 {
		return TransactionRequest{}, fmt.Errorf("signing: invalid transaction request")
	}
	request.From = w.address
	return cloneTransactionRequest(request), nil
}

// cloneTransactionRequest copies an already validated call intent.
func cloneTransactionRequest(request TransactionRequest) TransactionRequest {
	request.ChainID = new(big.Int).Set(request.ChainID)
	request.Value = new(big.Int).Set(request.Value)
	request.Data = append([]byte(nil), request.Data...)
	return request
}
