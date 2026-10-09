package perps

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/nijaru/go-clob-client/clob"
)

// PrepareCollateralApproval sets exactly the requested ERC-20 allowance for
// the perps deposit contract. Zero revokes; no unlimited approval is implicit.
func (c *OwnerClient) PrepareCollateralApproval(amount *big.Int) (TransactionCall, error) {
	if c.token == (common.Address{}) || c.deposit == (common.Address{}) {
		return TransactionCall{}, fmt.Errorf(
			"perps: collateral token and deposit contract required",
		)
	}
	if amount == nil || amount.Sign() < 0 || amount.BitLen() > 256 {
		return TransactionCall{}, fmt.Errorf("perps: allowance must be non-negative uint256")
	}
	data := append([]byte(nil), crypto.Keccak256([]byte("approve(address,uint256)"))[:4]...)
	data = append(data, common.LeftPadBytes(c.deposit.Bytes(), 32)...)
	data = append(data, common.LeftPadBytes(amount.Bytes(), 32)...)
	return TransactionCall{To: c.token, Data: data, Value: new(big.Int)}, nil
}

// ApproveCollateral explicitly submits an exact allowance (zero revokes).
// It does not wait, deploy, or deposit. Tokens requiring zero-first approval
// must be revoked and confirmed explicitly before setting a new allowance.
func (w *CollateralWallet) ApproveCollateral(
	ctx context.Context,
	amount *big.Int,
	metadata string,
) (*CollateralTransaction, error) {
	call, err := w.PrepareCollateralApproval(amount)
	if err != nil {
		return nil, err
	}
	return w.execute(ctx, []TransactionCall{call}, []string{"approve"}, metadata)
}

// Deposit submits collateral already approved by the wallet, crediting the
// owner signer, not the smart-wallet address. It never approves implicitly.
func (w *CollateralWallet) Deposit(
	ctx context.Context,
	amount *big.Int,
	metadata string,
) (*CollateralTransaction, error) {
	call, err := w.PrepareDeposit(amount)
	if err != nil {
		return nil, err
	}
	return w.execute(ctx, []TransactionCall{call}, []string{"deposit"}, metadata)
}

// ApproveAndDeposit explicitly authorizes both effects. Smart wallets submit
// one atomic batch. EOAs confirm approval before sending deposit; failure can
// leave an allowance without a deposit. The returned transaction is retained
// alongside errors for reconciliation. Never blindly retry an uncertain send.
func (w *CollateralWallet) ApproveAndDeposit(
	ctx context.Context,
	amount *big.Int,
	metadata string,
) (*CollateralTransaction, error) {
	deposit, err := w.PrepareDeposit(amount)
	if err != nil {
		return nil, err
	}
	approval, err := w.PrepareCollateralApproval(amount)
	if err != nil {
		return nil, err
	}
	return w.execute(
		ctx,
		[]TransactionCall{approval, deposit},
		[]string{"approve", "deposit"},
		metadata,
	)
}

// CollateralSubmissionError identifies the failed step. Its submission may
// have reached the RPC/relayer even without a returned hash. Earlier confirmed
// EOA submissions remain effective; inspect the accompanying transaction.
type CollateralSubmissionError struct {
	Operation string
	Err       error
}

func (e *CollateralSubmissionError) Error() string {
	return fmt.Sprintf("perps: %s submission uncertain: %v", e.Operation, e.Err)
}
func (e *CollateralSubmissionError) Unwrap() error { return e.Err }

func (w *CollateralWallet) execute(
	ctx context.Context,
	calls []TransactionCall,
	operations []string,
	metadata string,
) (*CollateralTransaction, error) {
	state, err := w.Readiness(ctx)
	if err != nil {
		return nil, err
	}
	if !state.OnChain {
		return nil, fmt.Errorf("perps: wallet has no on-chain code; deploy explicitly")
	}
	result := &CollateralTransaction{wallet: w}
	converted := make([]clob.TransactionCall, len(calls))
	for i, call := range calls {
		converted[i] = clob.TransactionCall{To: call.To, Data: call.Data, Value: call.Value}
	}
	if w.walletType != clob.SignatureTypeEOA {
		h, err := w.transactions.ExecuteWalletTransaction(ctx, converted, metadata)
		if err != nil {
			return result, &CollateralSubmissionError{Operation: "batch", Err: err}
		}
		result.Submissions = []CollateralSubmission{
			{
				Operation:       "batch",
				TransactionID:   h.TransactionID,
				TransactionHash: h.TransactionHash,
				handle:          h,
			},
		}
		return result, nil
	}
	for i, call := range converted {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		h, err := w.transactions.ExecuteWalletTransaction(
			ctx,
			[]clob.TransactionCall{call},
			metadata,
		)
		if err != nil {
			return result, &CollateralSubmissionError{Operation: operations[i], Err: err}
		}
		result.Submissions = append(
			result.Submissions,
			CollateralSubmission{
				Operation:       operations[i],
				TransactionID:   h.TransactionID,
				TransactionHash: h.TransactionHash,
				handle:          h,
			},
		)
		if i+1 < len(converted) {
			receipt, err := result.waitSubmission(ctx, result.Submissions[i])
			if err != nil {
				return result, fmt.Errorf("perps: confirm %s: %w", operations[i], err)
			}
			result.Submissions[i].ConfirmedReceipt = receipt
		}
	}
	return result, nil
}
