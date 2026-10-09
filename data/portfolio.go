package data

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

func (c *Client) GetValue(ctx context.Context, p ValueParams) (*PortfolioValue, error) {
	q := url.Values{}
	if err := requireUser(q, p.User); err != nil {
		return nil, err
	}
	if err := conditionIDs(q, p.ConditionIDs, marketConditions); err != nil {
		return nil, err
	}
	return getValue[PortfolioValue](ctx, c, "/v2/value", q, false)
}

// GetUserStats returns nil, nil when the user has no indexed statistics.
func (c *Client) GetUserStats(ctx context.Context, user string) (*UserStats, error) {
	q := url.Values{}
	if err := requireUser(q, user); err != nil {
		return nil, err
	}
	return getValue[UserStats](ctx, c, "/v2/user-stats", q, true)
}

func (c *Client) GetUserPnL(ctx context.Context, p UserPnLParams) (*UserPnLSeries, error) {
	q := url.Values{}
	if err := requireUser(q, p.User); err != nil {
		return nil, err
	}
	if err := enum(q, "interval", p.Interval, "max", "all", "1m", "1w", "1d", "12h", "6h"); err != nil {
		return nil, err
	}
	if err := enum(q, "fidelity", p.Fidelity, "1d", "18h", "12h", "3h", "1h"); err != nil {
		return nil, err
	}
	return getValue[UserPnLSeries](ctx, c, "/v2/user-pnl", q, false)
}

func (c *Client) GetUserVolume(ctx context.Context, p UserVolumeParams) (*UserVolume, error) {
	q := url.Values{}
	if err := requireUser(q, p.User); err != nil {
		return nil, err
	}
	if err := window(q, p.Window, true); err != nil {
		return nil, err
	}
	return getValue[UserVolume](ctx, c, "/v2/user-volume", q, false)
}

// GetApprovals reads an indexed snapshot, not live on-chain allowance state.
func (c *Client) GetApprovals(ctx context.Context, user string) (*ApprovalsSnapshot, error) {
	q := url.Values{}
	if err := requireUser(q, user); err != nil {
		return nil, err
	}
	return getValue[ApprovalsSnapshot](ctx, c, "/v2/approvals", q, false)
}

// WriteAccountingSnapshot streams the accounting ZIP archive into dst. The
// caller owns dst and any partially written output after an error. The SDK
// closes the HTTP response on every exit and does not buffer the whole archive.
// This download retains its /v1 path in the current unified SDK contract.
func (c *Client) WriteAccountingSnapshot(ctx context.Context, user string, dst io.Writer) error {
	if dst == nil {
		return input("destination", "writer is required")
	}
	q := url.Values{}
	if err := requireUser(q, user); err != nil {
		return err
	}
	return c.http.DoJSON(
		ctx,
		http.MethodGet,
		"/v1/accounting/snapshot",
		q,
		nil,
		polyhttp.AuthNone,
		nil,
		map[string]string{"Accept": "application/zip"},
		dst,
	)
}
