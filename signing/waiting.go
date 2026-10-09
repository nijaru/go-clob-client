package signing

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// TransactionWaiter optionally reconciles an original submission with its mined
// transaction, including fee replacements. It must honor ctx and return any known
// completed transaction and receipt even on error. It must not submit transactions.
// Implementations must support concurrent waits or serialize provider access.
// The provider owns replacement association: an opaque submission hash cannot
// prove the original nonce. Wallet verifies signed call intent and receipt
// identity, not nonce association, canonical-chain inclusion or reorg finality.
type TransactionWaiter interface {
	WaitTransaction(
		context.Context,
		TransactionRequest,
		common.Hash,
	) (*types.Transaction, *types.Receipt, error)
}

var (
	ErrTransactionWaitingUnsupported = errors.New("signing: transaction waiting unsupported")
	ErrInvalidTransactionCompletion  = errors.New("signing: invalid transaction completion")
	ErrTransactionReverted           = errors.New("signing: transaction reverted")
)

// CanWaitTransactions reports the backend's actual completion capability.
func (w *Wallet) CanWaitTransactions() bool {
	_, ok := w.signer.(TransactionWaiter)
	return ok
}

// WaitTransaction pins and isolates the original call just as SendTransaction
// does. Only a signed transaction from the pinned EOA on the requested chain,
// with unchanged To/Value/Data and a matching mined receipt, can complete it.
// A valid receipt survives provider errors, cancellation and reverts. The
// caller's original submission hash is never changed; receipt.TxHash is final.
func (w *Wallet) WaitTransaction(
	ctx context.Context,
	request TransactionRequest,
	original common.Hash,
) (*types.Transaction, *types.Receipt, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	waiter, ok := w.signer.(TransactionWaiter)
	if !ok {
		return nil, nil, ErrTransactionWaitingUnsupported
	}
	intent, err := w.transactionRequest(request)
	if err != nil {
		return nil, nil, err
	}
	if original == (common.Hash{}) {
		return nil, nil, ErrInvalidTransactionHash
	}
	isolated := cloneTransactionRequest(intent)
	tx, receipt, err := waiter.WaitTransaction(ctx, isolated, original)
	err = errors.Join(err, ctx.Err())
	if tx == nil || receipt == nil {
		return nil, nil, errors.Join(err, ErrInvalidTransactionCompletion)
	}
	if tx.ChainId().Cmp(intent.ChainID) != 0 || tx.To() == nil || *tx.To() != intent.To ||
		tx.Value().Cmp(intent.Value) != 0 || !bytes.Equal(tx.Data(), intent.Data) {
		return nil, nil, errors.Join(
			err,
			fmt.Errorf("%w: call intent or chain changed", ErrInvalidTransactionCompletion),
		)
	}
	from, signatureErr := types.Sender(types.LatestSignerForChainID(intent.ChainID), tx)
	if signatureErr != nil || from != w.address {
		return nil, nil, errors.Join(
			err,
			fmt.Errorf("%w: transaction sender", ErrInvalidTransactionCompletion),
		)
	}
	if receipt.TxHash != tx.Hash() || receipt.BlockNumber == nil ||
		receipt.BlockNumber.Sign() < 0 ||
		receipt.BlockHash == (common.Hash{}) ||
		(receipt.Status != types.ReceiptStatusSuccessful && receipt.Status != types.ReceiptStatusFailed) {
		return nil, nil, errors.Join(
			err,
			fmt.Errorf("%w: receipt identity, block or status", ErrInvalidTransactionCompletion),
		)
	}
	if receipt.Status == types.ReceiptStatusFailed {
		err = errors.Join(err, ErrTransactionReverted)
	}
	return tx, receipt, err
}
