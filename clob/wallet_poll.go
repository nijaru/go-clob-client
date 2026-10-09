package clob

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// Registry 404s may mean indexing lag; other 4xx errors are always fatal.
func retryWalletError(err error, registry bool) bool {
	var apiErr *polyhttp.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 429 || apiErr.StatusCode >= 500 ||
			(registry && apiErr.StatusCode == 404)
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

func walletSleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
