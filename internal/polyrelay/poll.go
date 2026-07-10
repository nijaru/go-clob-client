package polyrelay

import (
	"context"
	"fmt"
	"time"
)

// terminalSuccess is the subset of terminal states that indicate success;
// all other terminal states (per RelayerTransactionState.IsTerminal) are
// failures. IsTerminal stays the single source of truth for which states end
// polling — adding a terminal state means updating IsTerminal, and only a new
// *success* state needs an entry here.
var terminalSuccess = map[RelayerTransactionState]bool{StateConfirmed: true}

// PollUntilTerminal polls GET /v1/account/transactions/{id} until the transaction
// reaches a terminal state, returning its outcome. A CONFIRMED state with no
// hash falls back to fallbackHash (typically the hash from the /submit response);
// if both are empty the transaction settled without a hash we can observe.
//
// FAILED/INVALID surface as ErrTransactionFailed (wrapped with error_msg).
// Exhausting maxPolls surfaces as ErrTransactionTimeout.
//
// pollDelay between polls respects ctx cancellation.
func PollUntilTerminal(
	ctx context.Context,
	t *Transport,
	transactionID string,
	fallbackHash string,
	maxPolls int,
	pollDelay time.Duration,
) (*TransactionOutcome, error) {
	if maxPolls < 1 {
		maxPolls = 1
	}
	for range maxPolls {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		tx, err := t.GaslessTransaction(ctx, transactionID)
		if err != nil {
			return nil, err
		}
		if outcome, err := terminalOutcome(tx, transactionID, fallbackHash); err != nil ||
			outcome != nil {
			return outcome, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollDelay):
		}
	}
	return nil, fmt.Errorf("%w: %s after %d polls (%ds)",
		ErrTransactionTimeout, transactionID, maxPolls, maxPolls*int(pollDelay/time.Second))
}

// terminalOutcome returns the outcome if the transaction has settled, the
// failure error if it failed, or (nil, nil) to keep polling.
func terminalOutcome(
	tx GaslessTransaction,
	transactionID, fallbackHash string,
) (*TransactionOutcome, error) {
	if !tx.State.IsTerminal() {
		return nil, nil // keep polling
	}
	if terminalSuccess[tx.State] {
		hash := tx.TransactionHash
		if hash == "" {
			hash = fallbackHash
		}
		if hash == "" {
			return nil, fmt.Errorf("%w: %s", ErrNoTransactionHash, transactionID)
		}
		return &TransactionOutcome{TransactionHash: hash, TransactionID: tx.TransactionID}, nil
	}
	// Terminal but not a success state → failure.
	msg := tx.ErrorMsg
	if msg == "" {
		msg = fmt.Sprintf("transaction %s reached terminal state %s", transactionID, tx.State)
	}
	return nil, fmt.Errorf("%w: %s", ErrTransactionFailed, msg)
}
