package data

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// getReadJSON retries only the Data API's indexed JSON reads. Accounting
// downloads and other services deliberately do not use this policy. The budget
// belongs to one request (one page), never to an entire cursor traversal.
func (c *Client) getReadJSON(ctx context.Context, path string, q url.Values, out any) error {
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := c.http.GetJSON(ctx, path, q, polyhttp.AuthNone, out)
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTooManyRequests ||
			attempt >= 2 {
			return err
		}
		delay := 1.0
		if apiErr.RetryAfterSeconds != nil {
			delay = *apiErr.RetryAfterSeconds
		}
		if delay > 5 {
			return err
		}
		// GetJSON has consumed and closed the response before the wait. Use the
		// caller's context for both requests and waits, without a background task.
		timer := time.NewTimer(time.Duration(delay * float64(time.Second)))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
