package perps

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/nijaru/go-clob-client/clob"
)

// CollateralSubmission records an EOA call or an atomic smart-wallet batch.
// ConfirmedReceipt is populated only for EOA prefixes confirmed before the
// next submission. A submission hash/ID alone is not a confirmed receipt.
type CollateralSubmission struct {
	Operation        string
	TransactionID    string
	TransactionHash  string
	ConfirmedReceipt *types.Receipt
	handle           *clob.WalletTransactionHandle
	relay            *clob.GaslessTransactionHandle
}

// CollateralTransaction contains submissions, including any completed EOA
// prefix on error. Treat its fields as read-only. Wait has no cached mutable
// state and may be called concurrently. Cancellation never undoes submission.
type CollateralTransaction struct {
	Submissions []CollateralSubmission
	wallet      *CollateralWallet
}

var ErrCollateralTransactionReverted = errors.New("perps: collateral transaction reverted")

// Wait resolves relayer terminal states and then independently obtains successful
// RPC receipts for every submission. It returns collected receipts even on an
// error, including a reverted receipt. Success means mined execution, not perps
// ledger credit, registry readiness, reorg finality or ERC-20 return-value checks.
// It waits only for recorded submissions, not a missing deposit after a failed
// EOA sequence. No approval/deposit is sent by Wait.
func (t *CollateralTransaction) Wait(ctx context.Context) ([]*types.Receipt, error) {
	if t == nil || t.wallet == nil || len(t.Submissions) == 0 {
		return nil, fmt.Errorf("perps: no collateral submissions to wait for")
	}
	receipts := make([]*types.Receipt, 0, len(t.Submissions))
	for _, submission := range t.Submissions {
		receipt, err := t.waitSubmission(ctx, submission)
		if receipt != nil {
			receipts = append(receipts, receipt)
		}
		if err != nil {
			return receipts, err
		}
	}
	return receipts, nil
}

func (t *CollateralTransaction) waitSubmission(
	ctx context.Context,
	s CollateralSubmission,
) (*types.Receipt, error) {
	hash := s.TransactionHash
	if s.relay != nil || t.wallet.walletType != clob.SignatureTypeEOA {
		var outcome *clob.TransactionOutcome
		var err error
		if s.relay != nil {
			outcome, err = s.relay.Wait(ctx)
		} else {
			outcome, err = s.handle.Wait(ctx)
		}
		if err != nil {
			return nil, err
		}
		hash = outcome.TransactionHash
	}
	bytes, err := hexutil.Decode(hash)
	if err != nil || len(bytes) != common.HashLength ||
		common.BytesToHash(bytes) == (common.Hash{}) {
		return nil, fmt.Errorf("perps: submission has no valid transaction hash")
	}
	ec, err := t.wallet.dial(ctx)
	if err != nil {
		return nil, err
	}
	defer ec.Close()
	for {
		receipt, err := ec.TransactionReceipt(ctx, common.BytesToHash(bytes))
		if err == nil {
			if receipt.TxHash != common.BytesToHash(bytes) || receipt.BlockNumber == nil ||
				receipt.BlockNumber.Sign() < 0 {
				return nil, fmt.Errorf("perps: invalid receipt identity or block")
			}
			if receipt.Status != types.ReceiptStatusSuccessful {
				return receipt, fmt.Errorf("%w: %s", ErrCollateralTransactionReverted, hash)
			}
			return receipt, nil
		}
		if !errors.Is(err, ethereum.NotFound) {
			return nil, err
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
