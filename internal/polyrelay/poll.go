package polyrelay

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// terminalSuccess is the subset of terminal states that indicate success;
// all other terminal states (per RelayerTransactionState.IsTerminal) are
// failures. IsTerminal stays the single source of truth for which states end
// polling — adding a terminal state means updating IsTerminal, and only a new
// *success* state needs an entry here.
var terminalSuccess = map[RelayerTransactionState]bool{StateConfirmed: true}

// isRetryablePollError reports whether a transient GET error during polling
// should be tolerated (keep polling) rather than aborting the whole wait. Rate
// limits, server errors, and network blips are transient; 4xx auth/validation
// responses and malformed bodies are not.
func isRetryablePollError(err error) bool {
	var apiErr *polyhttp.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusTooManyRequests || apiErr.StatusCode >= 500
	}
	var netErr net.Error
	return errors.As(err, &netErr) // connection resets, dial/temporary failures
}

// sleepCtx pauses for d, returning ctx.Err() if the context is cancelled first.
// It uses a stopped timer rather than time.After to avoid leaking a timer that
// fires after each poll interval.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// PollUntilTerminal polls GET /v1/account/transactions/{id} until the
// transaction reaches a terminal state, returning its outcome. A CONFIRMED
// state with no observed hash falls back to fallbackHash (typically the hash
// from the /submit response); if both are empty the outcome is still returned
// as a success with an empty hash — the transaction settled, so erroring would
// risk a caller double-submitting it.
//
// Transient GET failures (429, 5xx, network blips) are tolerated and polling
// continues; a hard error (4xx auth/validation) is returned immediately.
// FAILED/INVALID surface as ErrTransactionFailed (wrapped with error_msg).
// Exhausting maxPolls surfaces as ErrTransactionTimeout.
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
	var lastTransient error
	for attempt := 0; attempt < maxPolls; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		tx, err := t.GaslessTransaction(ctx, transactionID)
		switch {
		case err == nil:
			if outcome, oerr := terminalOutcome(tx, transactionID, fallbackHash); outcome != nil ||
				oerr != nil {
				return outcome, oerr
			}
		case isRetryablePollError(err):
			// Transient blip: tolerate and keep polling.
			lastTransient = err
		default:
			return nil, err
		}
		// Sleep before the next poll, but skip the trailing sleep after the
		// final attempt (it would only delay the timeout return).
		if attempt < maxPolls-1 {
			if err := sleepCtx(ctx, pollDelay); err != nil {
				return nil, err
			}
		}
	}
	if lastTransient != nil {
		return nil, fmt.Errorf("%w: %s after %d polls (~%v); last poll error: %v",
			ErrTransactionTimeout, transactionID, maxPolls,
			time.Duration(maxPolls)*pollDelay, lastTransient)
	}
	return nil, fmt.Errorf("%w: %s after %d polls (~%v)",
		ErrTransactionTimeout, transactionID, maxPolls, time.Duration(maxPolls)*pollDelay)
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
		// CONFIRMED is a success regardless of whether a hash was observed.
		return &TransactionOutcome{TransactionHash: hash, TransactionID: tx.TransactionID}, nil
	}
	// Terminal but not a success state → failure.
	msg := tx.ErrorMsg
	if msg == "" {
		msg = fmt.Sprintf("transaction %s reached terminal state %s", transactionID, tx.State)
	}
	return nil, fmt.Errorf("%w: %s", ErrTransactionFailed, msg)
}
