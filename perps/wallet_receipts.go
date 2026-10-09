package perps

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/nijaru/go-clob-client/clob"
)

// CollateralSubmission records an EOA send attempt or an atomic smart-wallet
// batch. BroadcastUncertain retains the locally computed hash after an RPC send
// error; it does not imply rejection. ConfirmedReceipt records a receipt observed
// during execution, including a revert. Treat these fields as read-only.
type CollateralSubmission struct {
	Operation          string
	TransactionID      string
	TransactionHash    string
	BroadcastUncertain bool
	ConfirmedReceipt   *types.Receipt
}

// CollateralTransaction retains every attempted submission on error. RequestedCalls
// counts the intended calls, not just the submitted prefix. Wait has no mutable
// cached state and supports concurrent callers. Cancellation cannot undo sends.
type CollateralTransaction struct {
	Submissions    []CollateralSubmission
	RequestedCalls int
	wallet         *CollateralWallet
	handle         *clob.WalletTransactionHandle
	relay          *clob.GaslessTransactionHandle
}

var (
	ErrCollateralTransactionReverted = errors.New("perps: collateral transaction reverted")
	// ErrCollateralTransactionIncomplete means an unsubmitted tail remains.
	// Wait never sends those missing calls or retries an uncertain broadcast.
	ErrCollateralTransactionIncomplete = clob.ErrWalletTransactionIncomplete
)

// Wait reconciles every recorded submission, including uncertain EOA hashes,
// through independent RPC receipts. An unsubmitted tail returns
// ErrCollateralTransactionIncomplete, never success. Collected receipts are
// returned on error, including a revert. Success means mined execution, not
// perps ledger credit, registry readiness, reorg finality or ERC-20 return-value
// checks. Wait sends nothing and obeys its context.
func (t *CollateralTransaction) Wait(ctx context.Context) ([]*types.Receipt, error) {
	if t == nil || t.wallet == nil || len(t.Submissions) == 0 {
		return nil, fmt.Errorf("perps: no collateral submissions to wait for")
	}
	// Receipt reconciliation must validate the configured RPC chain too, not
	// just the chain seen before submission. CLOB owns receipt integrity checks.
	ec, err := t.wallet.dial(ctx)
	if err != nil {
		return nil, err
	}
	ec.Close()
	if t.handle != nil {
		receipts, err := t.handle.WaitReceipts(ctx)
		return receipts, collateralReceiptError(err)
	}
	if t.relay == nil {
		return nil, fmt.Errorf("perps: no collateral execution handle")
	}
	outcome, err := t.relay.Wait(ctx)
	if err != nil {
		return nil, err
	}
	receipt, err := t.wallet.transactions.WaitWalletTransactionReceipt(ctx, *outcome)
	if receipt == nil {
		return nil, collateralReceiptError(err)
	}
	return []*types.Receipt{receipt}, collateralReceiptError(err)
}

func collateralReceiptError(err error) error {
	if errors.Is(err, clob.ErrWalletTransactionFailed) {
		return fmt.Errorf("%w: %w", ErrCollateralTransactionReverted, err)
	}
	return err
}
