package data

import (
	"context"
	"iter"
	"net/url"
	"strconv"
)

func (c *Client) GetLeaderboard(
	ctx context.Context,
	p LeaderboardParams,
) (Page[TraderLeaderboardEntry], error) {
	q := url.Values{}
	if err := category(q, p.Category); err != nil {
		return Page[TraderLeaderboardEntry]{}, err
	}
	if err := leaderboardWindow(q, p.Window); err != nil {
		return Page[TraderLeaderboardEntry]{}, err
	}
	if err := enum(q, "sort_by", p.SortBy, "PNL", "VOLUME"); err != nil {
		return Page[TraderLeaderboardEntry]{}, err
	}
	return getPage[TraderLeaderboardEntry](ctx, c, "/v2/leaderboard", q, p.Page, 1000)
}

func (c *Client) IterLeaderboard(
	ctx context.Context,
	p LeaderboardParams,
) iter.Seq2[TraderLeaderboardEntry, error] {
	return walk(ctx, p.Page, func(page PageParams) (Page[TraderLeaderboardEntry], error) {
		request := p
		request.Page = page
		return c.GetLeaderboard(ctx, request)
	})
}

// GetLeaderboardStanding returns nil, nil when the user has no standing.
// PnLRank and VolumeRank are dense ties and must not be used as offsets.
func (c *Client) GetLeaderboardStanding(
	ctx context.Context,
	p LeaderboardStandingParams,
) (*TraderLeaderboardStanding, error) {
	q := url.Values{}
	if err := requireUser(q, p.User); err != nil {
		return nil, err
	}
	if err := category(q, p.Category); err != nil {
		return nil, err
	}
	if err := leaderboardWindow(q, p.Window); err != nil {
		return nil, err
	}
	return getValue[TraderLeaderboardStanding](ctx, c, "/v2/leaderboard", q, true)
}

func (c *Client) GetBiggestWinners(
	ctx context.Context,
	p BiggestWinnersParams,
) (Page[BiggestWinner], error) {
	q := url.Values{}
	if err := category(q, p.Category); err != nil {
		return Page[BiggestWinner]{}, err
	}
	if err := leaderboardWindow(q, p.Window); err != nil {
		return Page[BiggestWinner]{}, err
	}
	return getPage[BiggestWinner](ctx, c, "/v2/biggest-winners", q, p.Page, 1000)
}

func (c *Client) IterBiggestWinners(
	ctx context.Context,
	p BiggestWinnersParams,
) iter.Seq2[BiggestWinner, error] {
	return walk(ctx, p.Page, func(page PageParams) (Page[BiggestWinner], error) {
		request := p
		request.Page = page
		return c.GetBiggestWinners(ctx, request)
	})
}

func (c *Client) GetBuilderLeaderboard(
	ctx context.Context,
	p BuilderLeaderboardParams,
) (Page[BuilderStanding], error) {
	q := url.Values{}
	if err := leaderboardWindow(q, p.Window); err != nil {
		return Page[BuilderStanding]{}, err
	}
	return getPage[BuilderStanding](ctx, c, "/v2/builders/leaderboard", q, p.Page, 1000)
}

func (c *Client) IterBuilderLeaderboard(
	ctx context.Context,
	p BuilderLeaderboardParams,
) iter.Seq2[BuilderStanding, error] {
	return walk(ctx, p.Page, func(page PageParams) (Page[BuilderStanding], error) {
		request := p
		request.Page = page
		return c.GetBuilderLeaderboard(ctx, request)
	})
}

// GetBuilderVolume returns time buckets, not a paginated rank traversal.
func (c *Client) GetBuilderVolume(
	ctx context.Context,
	p BuilderVolumeParams,
) ([]BuilderVolumePoint, error) {
	q := url.Values{}
	if err := enum(q, "interval", p.Interval, "day", "week", "month", "all"); err != nil {
		return nil, err
	}
	if p.BucketLimit < 0 || p.BucketLimit > 90 {
		return nil, input("bucket_limit", "must be between 1 and 90, or 0 for the server default")
	}
	if p.BucketLimit != 0 {
		q.Set("limit", strconv.Itoa(p.BucketLimit))
	}
	return getList[BuilderVolumePoint](ctx, c, "/v2/builders/volume", q)
}
