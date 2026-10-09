package clob

import (
	"context"
	"fmt"
	"time"
)

// WaitForComboFillParams controls settlement polling. Zero durations use the
// stable SDK defaults (30 seconds and one second). The context may end sooner.
type WaitForComboFillParams struct {
	RFQID        string
	Timeout      time.Duration
	PollInterval time.Duration
}

// WaitForComboFillResult reports settlement or a terminal business failure.
// Only FILLED has a transaction hash; CONFIRMED is normalized to FILLED.
type WaitForComboFillResult struct {
	RFQID  string
	Status ComboRFQStatus
	TxHash string
	Error  *BuilderRfqError
}

// ComboRFQResponseError indicates an inconsistent gateway response.
type ComboRFQResponseError struct {
	RFQID  string
	Status ComboRFQStatus
}

func (e *ComboRFQResponseError) Error() string {
	return fmt.Sprintf(
		"combo RFQ %s reached %s without a valid transaction hash",
		e.RFQID,
		e.Status,
	)
}

// WaitForComboFill waits for settlement, not merely execution handoff. Terminal
// failure is a result, not an error. A timeout does not mean the trade failed:
// callers can resume with GetComboRFQStatus or another wait.
func (c *AuthenticatedClient) WaitForComboFill(
	ctx context.Context,
	params WaitForComboFillParams,
) (*WaitForComboFillResult, error) {
	if params.RFQID == "" || params.Timeout < 0 || params.PollInterval < 0 {
		return nil, fmt.Errorf("combo fill: RFQID is required and durations must not be negative")
	}
	if params.Timeout == 0 {
		params.Timeout = 30 * time.Second
	}
	if params.PollInterval == 0 {
		params.PollInterval = time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, params.Timeout)
	defer cancel()
	for {
		status, err := c.GetComboRFQStatus(ctx, params.RFQID)
		if err != nil {
			return nil, err
		}
		switch status.Status {
		case ComboRFQFilled, ComboRFQConfirmed:
			if !comboTransactionHashValid(status.TxHash) {
				return nil, &ComboRFQResponseError{RFQID: status.RFQID, Status: status.Status}
			}
			return &WaitForComboFillResult{
				RFQID:  status.RFQID,
				Status: ComboRFQFilled,
				TxHash: status.TxHash,
			}, nil
		case ComboRFQFailed, ComboRFQExpired, ComboRFQCanceled:
			return &WaitForComboFillResult{
				RFQID:  status.RFQID,
				Status: status.Status,
				Error:  status.Error,
			}, nil
		}
		timer := time.NewTimer(params.PollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
