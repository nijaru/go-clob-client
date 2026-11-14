package clob

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/time/rate"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

func (c *Client) timestamp(ctx context.Context) (int64, error) {
	if !c.useServerTime {
		return time.Now().Unix(), nil
	}

	var serverTime int64
	if err := c.http.GetJSON(ctx, timeEndpoint, nil, polyhttp.AuthNone, &serverTime); err != nil {
		return 0, err
	}
	return serverTime, nil
}

func (c *Client) getJSON(
	ctx context.Context,
	path string,
	query url.Values,
	auth polyhttp.AuthLevel,
	out any,
) error {
	return c.withRetry(ctx, true, c.rateLimiters.reads, func() error {
		return c.http.GetJSON(ctx, path, query, auth, out)
	})
}

func (c *Client) getGeoblockJSON(
	ctx context.Context,
	path string,
	query url.Values,
	out any,
) error {
	return c.geoblockHTTP.GetJSON(ctx, path, query, polyhttp.AuthNone, out)
}

func (c *Client) postJSON(
	ctx context.Context,
	path string,
	body any,
	auth polyhttp.AuthLevel,
	out any,
) error {
	return c.withRetry(ctx, false, c.rateLimiters.other, func() error {
		return c.http.PostJSON(ctx, path, body, auth, out)
	})
}

// postTradingJSON isolates order-placement admission from reads and cancels.
func (c *Client) postTradingJSON(
	ctx context.Context,
	path string,
	body any,
	auth polyhttp.AuthLevel,
	out any,
) error {
	return c.withRetry(ctx, false, c.rateLimiters.orders, func() error {
		return c.http.PostJSON(ctx, path, body, auth, out)
	})
}

// heartbeatJSON reserves a separate optional budget so routine requests
// cannot delay the liveness signal protecting outstanding orders.
func (c *Client) heartbeatJSON(
	ctx context.Context,
	body any,
	auth polyhttp.AuthLevel,
	out any,
) error {
	return c.withRetry(ctx, false, c.rateLimiters.heartbeats, func() error {
		return c.http.PostJSON(ctx, heartbeatsEndpoint, body, auth, out)
	})
}

// cancelTradingJSON uses its own budget. Never retry an ambiguous cancel
// without reconciling server state first.
func (c *Client) cancelTradingJSON(
	ctx context.Context,
	path string,
	body any,
	auth polyhttp.AuthLevel,
	out any,
) error {
	return c.withRetry(ctx, false, c.rateLimiters.cancels, func() error {
		return c.http.DeleteJSON(ctx, path, body, auth, out)
	})
}

func (c *Client) deleteJSON(
	ctx context.Context,
	path string,
	body any,
	auth polyhttp.AuthLevel,
	out any,
) error {
	return c.withRetry(ctx, false, c.rateLimiters.other, func() error {
		return c.http.DeleteJSON(ctx, path, body, auth, out)
	})
}

func (c *Client) deleteJSONQuery(
	ctx context.Context,
	path string,
	query url.Values,
	body any,
	auth polyhttp.AuthLevel,
	out any,
) error {
	return c.withRetry(ctx, false, c.rateLimiters.other, func() error {
		return c.http.DeleteJSONQuery(ctx, path, query, body, auth, out)
	})
}

func (c *Client) getJSONWithNonce(
	ctx context.Context,
	path string,
	query url.Values,
	auth polyhttp.AuthLevel,
	nonce int64,
	out any,
) error {
	return c.withRetry(ctx, false, c.rateLimiters.other, func() error {
		return c.http.GetJSONWithNonce(ctx, path, query, auth, nonce, out)
	})
}

func (c *Client) postJSONWithNonce(
	ctx context.Context,
	path string,
	body any,
	auth polyhttp.AuthLevel,
	nonce int64,
	out any,
) error {
	return c.withRetry(ctx, false, c.rateLimiters.other, func() error {
		return c.http.PostJSONWithNonce(ctx, path, body, auth, nonce, out)
	})
}

func (c *Client) doJSON(
	ctx context.Context,
	method, path string,
	query url.Values,
	body any,
	auth polyhttp.AuthLevel,
	out any,
	extraHeaders map[string]string,
) error {
	limiter := c.rateLimiters.other
	if method == http.MethodGet {
		limiter = c.rateLimiters.reads
	}
	return c.withRetry(ctx, method == http.MethodGet, limiter, func() error {
		return c.http.DoJSON(ctx, method, path, query, body, auth, nil, extraHeaders, out)
	})
}

func (c *Client) withRetry(
	ctx context.Context,
	retryEnabled bool,
	limiter *rate.Limiter,
	fn func() error,
) error {
	if !retryEnabled {
		if limiter != nil {
			if err := waitRateLimit(ctx, limiter); err != nil {
				return err
			}
		}
		return fn()
	}

	var lastErr error
	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	for i := 0; i <= c.retryMax; i++ {
		if limiter != nil {
			if err := waitRateLimit(ctx, limiter); err != nil {
				return err
			}
		}

		err := fn()
		if err == nil {
			return nil
		}
		lastErr = err

		// Only retry on:
		// 1. HTTP 429 (Rate Limit)
		// 2. HTTP 5xx (Server Error)
		// 3. Connection/transport errors (not context cancellation)
		var apiErr *polyhttp.APIError
		shouldRetry := false
		if errors.As(err, &apiErr) {
			if apiErr.StatusCode == http.StatusTooManyRequests || apiErr.StatusCode >= 500 {
				shouldRetry = true
			}
		} else {
			// Don't retry on context cancellation or deadline — fail fast.
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			shouldRetry = true
		}

		if !shouldRetry || i >= c.retryMax {
			return err
		}

		factor := time.Duration(1 << min(i, 30))
		backoff := min(c.retryBackoff, time.Duration(math.MaxInt64)/factor) * factor
		if timer == nil {
			timer = time.NewTimer(backoff)
		} else {
			timer.Reset(backoff)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			continue
		}
	}
	return lastErr
}

// rate.Wait may reject a reservation before a deadline actually expires.
// Wait for that deadline so callers receive the context's typed error rather
// than a rate-limiter-specific string; the request is never issued.
func waitRateLimit(ctx context.Context, limiter *rate.Limiter) error {
	err := limiter.Wait(ctx)
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if _, ok := ctx.Deadline(); ok {
		<-ctx.Done()
		return ctx.Err()
	}
	return err
}

func cloneTransport(c *polyhttp.Client) *polyhttp.Client {
	copy := *c
	copy.Headers = nil
	return &copy
}
