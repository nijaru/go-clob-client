package perps

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// ErrPerpsAutoCancelDailyLimit indicates that the account cannot arm another
// auto-cancel schedule until its daily trigger counter resets.
var ErrPerpsAutoCancelDailyLimit = errors.New("perps: auto-cancel daily trigger limit reached")

const (
	perpsAutoCancelMinimumDelay = 5 * time.Second
	autoCancelDailyLimitCode    = "auto_cancel_daily_limit_reached"
)

// ArmAutoCancel arms the one-shot schedule that cancels all open orders at
// cancelAt, expressed as Unix milliseconds. The deadline must be at least five
// seconds in the future. Arming again replaces the prior schedule.
func (s *Session) ArmAutoCancel(
	ctx context.Context,
	cancelAt int64,
	expiresAt int64,
) error {
	if cancelAt < time.Now().Add(perpsAutoCancelMinimumDelay).UnixMilli() {
		return fmt.Errorf("perps: cancel time must be at least 5 seconds in the future")
	}
	return s.updateAutoCancel(ctx, cancelAt, expiresAt)
}

// DisarmAutoCancel clears the current auto-cancel schedule without triggering
// it. It remains allowed after the daily trigger limit is reached.
func (s *Session) DisarmAutoCancel(ctx context.Context, expiresAt int64) error {
	return s.updateAutoCancel(ctx, 0, expiresAt)
}

// GetAutoCancelStatus returns the schedule and daily trigger counters for the
// session account.
func (s *Session) GetAutoCancelStatus(
	ctx context.Context,
) (*PerpsAutoCancelStatus, error) {
	return s.client.GetAutoCancelStatus(ctx)
}

func (s *Session) updateAutoCancel(
	ctx context.Context,
	deadline int64,
	expiresAt int64,
) error {
	if expiresAt < 0 {
		return fmt.Errorf("perps: expiration must not be negative")
	}
	op := []any{"autoCancel", []any{deadline}}
	body, err := makePerpsSignedCommand(
		ctx,
		s.signer,
		s.chainID,
		op,
		map[string]any{"type": "autoCancel", "args": map[string]any{"time": deadline}},
		expiresAt,
	)
	if err != nil {
		return err
	}
	var response PerpsAutoCancelResponse
	if err := s.client.patchAuthenticatedJSON(
		ctx,
		"/v1/trade/auto-cancel",
		body,
		&response,
	); err != nil {
		if isAutoCancelDailyLimitError(err) {
			return fmt.Errorf("%w: %w", ErrPerpsAutoCancelDailyLimit, err)
		}
		return err
	}
	if response.Status != "ok" {
		if response.Error == "" {
			response.Error = "auto-cancel update rejected"
		}
		return fmt.Errorf("perps: %s", response.Error)
	}
	return nil
}

func isAutoCancelDailyLimitError(err error) bool {
	var apiErr *polyhttp.APIError
	return errors.As(err, &apiErr) && strings.Contains(apiErr.Message, autoCancelDailyLimitCode)
}
