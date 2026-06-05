package data

import (
	"context"
	"iter"
	"net/url"
)

const (
	healthEndpoint             = "/"
	positionsEndpoint          = "/positions"
	closedPositionsEndpoint    = "/closed-positions"
	valueEndpoint              = "/value"
	tradesEndpoint             = "/trades"
	activityEndpoint           = "/activity"
	holdersEndpoint            = "/holders"
	tradedEndpoint             = "/traded"
	oiEndpoint                 = "/oi"
	liveVolumeEndpoint         = "/live-volume"
	leaderboardEndpoint        = "/v1/leaderboard"
	builderLeaderboardEndpoint = "/v1/builders/leaderboard"
	builderVolumeEndpoint      = "/v1/builders/volume"
)

func (c *Client) GetHealth(ctx context.Context) (*Health, error) {
	var out Health
	err := c.getJSON(ctx, healthEndpoint, nil, &out)
	return &out, err
}

func (c *Client) GetPositions(ctx context.Context, p PositionParams) ([]Position, error) {
	q := url.Values{}
	q.Set("user", p.User)
	p.Filter.appendQuery(q)
	setString(q, "sizeThreshold", p.SizeThreshold)
	setBool(q, "redeemable", p.Redeemable)
	setBool(q, "mergeable", p.Mergeable)
	setInt(q, "limit", boundedLimit(p.Limit, 500))
	setInt(q, "offset", p.Offset)
	setString(q, "sortBy", p.SortBy)
	setString(q, "sortDirection", p.SortDirection)
	setString(q, "title", p.Title)

	var out []Position
	err := c.getJSON(ctx, positionsEndpoint, q, &out)
	return out, err
}

func (c *Client) IterPositions(ctx context.Context, p PositionParams) iter.Seq2[Position, error] {
	return func(yield func(Position, error) bool) {
		offset := p.Offset
		limit := iteratorLimit(p.Limit, 100, 500)
		for {
			q := p
			q.Limit = limit
			q.Offset = offset
			items, err := c.GetPositions(ctx, q)
			if err != nil {
				yield(Position{}, err)
				return
			}
			if len(items) == 0 {
				return
			}
			for _, item := range items {
				if !yield(item, nil) {
					return
				}
			}
			if len(items) < limit {
				return
			}
			offset += len(items)
		}
	}
}

func (c *Client) GetClosedPositions(
	ctx context.Context,
	p ClosedPositionParams,
) ([]ClosedPosition, error) {
	q := url.Values{}
	q.Set("user", p.User)
	p.Filter.appendQuery(q)
	setString(q, "title", p.Title)
	setInt(q, "limit", boundedLimit(p.Limit, 50))
	setInt(q, "offset", p.Offset)
	setString(q, "sortBy", p.SortBy)
	setString(q, "sortDirection", p.SortDirection)

	var out []ClosedPosition
	err := c.getJSON(ctx, closedPositionsEndpoint, q, &out)
	return out, err
}

func (c *Client) IterClosedPositions(
	ctx context.Context,
	p ClosedPositionParams,
) iter.Seq2[ClosedPosition, error] {
	return func(yield func(ClosedPosition, error) bool) {
		offset := p.Offset
		limit := iteratorLimit(p.Limit, 50, 50)
		for {
			q := p
			q.Limit = limit
			q.Offset = offset
			items, err := c.GetClosedPositions(ctx, q)
			if err != nil {
				yield(ClosedPosition{}, err)
				return
			}
			if len(items) == 0 {
				return
			}
			for _, item := range items {
				if !yield(item, nil) {
					return
				}
			}
			if len(items) < limit {
				return
			}
			offset += len(items)
		}
	}
}

func (c *Client) GetValue(ctx context.Context, user string, markets []string) ([]Value, error) {
	q := url.Values{}
	q.Set("user", user)
	setCommaList(q, "market", markets)

	var out []Value
	err := c.getJSON(ctx, valueEndpoint, q, &out)
	return out, err
}

func (c *Client) GetTrades(ctx context.Context, p TradeParams) ([]Trade, error) {
	q := url.Values{}
	setString(q, "user", p.User)
	p.Filter.appendQuery(q)
	setInt(q, "limit", boundedLimit(p.Limit, 10000))
	setInt(q, "offset", p.Offset)
	setBool(q, "takerOnly", p.TakerOnly)
	if p.TradeFilter != nil {
		setString(q, "filterType", string(p.TradeFilter.FilterType))
		setString(q, "filterAmount", p.TradeFilter.FilterAmount.String())
	}
	setString(q, "side", p.Side)

	var out []Trade
	err := c.getJSON(ctx, tradesEndpoint, q, &out)
	return out, err
}

func (c *Client) IterTrades(ctx context.Context, p TradeParams) iter.Seq2[Trade, error] {
	return func(yield func(Trade, error) bool) {
		offset := p.Offset
		limit := iteratorLimit(p.Limit, 100, 10000)
		for {
			q := p
			q.Limit = limit
			q.Offset = offset
			items, err := c.GetTrades(ctx, q)
			if err != nil {
				yield(Trade{}, err)
				return
			}
			if len(items) == 0 {
				return
			}
			for _, item := range items {
				if !yield(item, nil) {
					return
				}
			}
			if len(items) < limit {
				return
			}
			offset += len(items)
		}
	}
}

func (c *Client) GetActivity(ctx context.Context, p ActivityParams) ([]Activity, error) {
	q := url.Values{}
	q.Set("user", p.User)
	p.Filter.appendQuery(q)
	setCommaList(q, "type", p.ActivityTypes)
	setInt(q, "limit", boundedLimit(p.Limit, 500))
	setInt(q, "offset", p.Offset)
	setInt64(q, "start", p.Start)
	setInt64(q, "end", p.End)
	setString(q, "sortBy", p.SortBy)
	setString(q, "sortDirection", p.SortDirection)
	setString(q, "side", p.Side)

	var out []Activity
	err := c.getJSON(ctx, activityEndpoint, q, &out)
	return out, err
}

func (c *Client) IterActivity(ctx context.Context, p ActivityParams) iter.Seq2[Activity, error] {
	return func(yield func(Activity, error) bool) {
		offset := p.Offset
		limit := iteratorLimit(p.Limit, 100, 500)
		for {
			q := p
			q.Limit = limit
			q.Offset = offset
			items, err := c.GetActivity(ctx, q)
			if err != nil {
				yield(Activity{}, err)
				return
			}
			if len(items) == 0 {
				return
			}
			for _, item := range items {
				if !yield(item, nil) {
					return
				}
			}
			if len(items) < limit {
				return
			}
			offset += len(items)
		}
	}
}

func (c *Client) GetHolders(ctx context.Context, p HoldersParams) ([]MetaHolder, error) {
	q := url.Values{}
	setCommaList(q, "market", p.Markets)
	setInt(q, "limit", boundedLimit(p.Limit, 20))
	setInt(q, "minBalance", p.MinBalance)

	var out []MetaHolder
	err := c.getJSON(ctx, holdersEndpoint, q, &out)
	return out, err
}

func (c *Client) GetTraded(ctx context.Context, user string) (*Traded, error) {
	q := url.Values{}
	q.Set("user", user)

	var out Traded
	err := c.getJSON(ctx, tradedEndpoint, q, &out)
	return &out, err
}

func (c *Client) GetOpenInterest(
	ctx context.Context,
	p OpenInterestParams,
) ([]OpenInterest, error) {
	q := url.Values{}
	setCommaList(q, "market", p.Markets)

	var out []OpenInterest
	err := c.getJSON(ctx, oiEndpoint, q, &out)
	return out, err
}

func (c *Client) GetLiveVolume(ctx context.Context, eventID int64) ([]LiveVolume, error) {
	q := url.Values{}
	setInt64(q, "id", eventID)

	var out []LiveVolume
	err := c.getJSON(ctx, liveVolumeEndpoint, q, &out)
	return out, err
}

func (c *Client) GetLeaderboard(
	ctx context.Context,
	p LeaderboardParams,
) ([]TraderLeaderboardEntry, error) {
	q := url.Values{}
	setString(q, "category", p.Category)
	setString(q, "timePeriod", p.TimePeriod)
	setString(q, "orderBy", p.SortBy)
	setInt(q, "limit", boundedLimit(p.Limit, 50))
	setInt(q, "offset", p.Offset)
	setString(q, "user", p.User)
	setString(q, "userName", p.UserName)

	var out []TraderLeaderboardEntry
	err := c.getJSON(ctx, leaderboardEndpoint, q, &out)
	return out, err
}

func (c *Client) IterLeaderboard(
	ctx context.Context,
	p LeaderboardParams,
) iter.Seq2[TraderLeaderboardEntry, error] {
	return func(yield func(TraderLeaderboardEntry, error) bool) {
		offset := p.Offset
		limit := iteratorLimit(p.Limit, 50, 50)
		for {
			q := p
			q.Limit = limit
			q.Offset = offset
			items, err := c.GetLeaderboard(ctx, q)
			if err != nil {
				yield(TraderLeaderboardEntry{}, err)
				return
			}
			if len(items) == 0 {
				return
			}
			for _, item := range items {
				if !yield(item, nil) {
					return
				}
			}
			if len(items) < limit {
				return
			}
			offset += len(items)
		}
	}
}

func (c *Client) GetBuilderLeaderboard(
	ctx context.Context,
	p BuilderLeaderboardParams,
) ([]BuilderLeaderboardEntry, error) {
	q := url.Values{}
	setString(q, "timePeriod", p.TimePeriod)
	setInt(q, "limit", boundedLimit(p.Limit, 50))
	setInt(q, "offset", p.Offset)

	var out []BuilderLeaderboardEntry
	err := c.getJSON(ctx, builderLeaderboardEndpoint, q, &out)
	return out, err
}

func (c *Client) IterBuilderLeaderboard(
	ctx context.Context,
	p BuilderLeaderboardParams,
) iter.Seq2[BuilderLeaderboardEntry, error] {
	return func(yield func(BuilderLeaderboardEntry, error) bool) {
		offset := p.Offset
		limit := iteratorLimit(p.Limit, 50, 50)
		for {
			q := p
			q.Limit = limit
			q.Offset = offset
			items, err := c.GetBuilderLeaderboard(ctx, q)
			if err != nil {
				yield(BuilderLeaderboardEntry{}, err)
				return
			}
			if len(items) == 0 {
				return
			}
			for _, item := range items {
				if !yield(item, nil) {
					return
				}
			}
			if len(items) < limit {
				return
			}
			offset += len(items)
		}
	}
}

func (c *Client) GetBuilderVolume(
	ctx context.Context,
	p BuilderVolumeParams,
) ([]BuilderVolumeEntry, error) {
	q := url.Values{}
	setString(q, "timePeriod", p.TimePeriod)

	var out []BuilderVolumeEntry
	err := c.getJSON(ctx, builderVolumeEndpoint, q, &out)
	return out, err
}

const (
	marketPositionsEndpoint    = "/v1/market-positions"
	accountingSnapshotEndpoint = "/v1/accounting/snapshot"
)

// GetMarketPositions returns all wallet positions for a specific market.
func (c *Client) GetMarketPositions(
	ctx context.Context,
	p MarketPositionParams,
) ([]MetaMarketPosition, error) {
	q := url.Values{}
	q.Set("market", p.Market)
	setString(q, "user", p.User)
	setString(q, "status", p.Status)
	setString(q, "sortBy", p.SortBy)
	setString(q, "sortDirection", p.SortDirection)
	setInt(q, "limit", boundedLimit(p.Limit, 500))
	setInt(q, "offset", p.Offset)

	var out []MetaMarketPosition
	err := c.getJSON(ctx, marketPositionsEndpoint, q, &out)
	return out, err
}

// IterMarketPositions returns an iterator over market positions.
func (c *Client) IterMarketPositions(
	ctx context.Context,
	p MarketPositionParams,
) iter.Seq2[MetaMarketPosition, error] {
	return func(yield func(MetaMarketPosition, error) bool) {
		offset := p.Offset
		limit := iteratorLimit(p.Limit, 100, 500)
		for {
			q := p
			q.Limit = limit
			q.Offset = offset
			items, err := c.GetMarketPositions(ctx, q)
			if err != nil {
				yield(MetaMarketPosition{}, err)
				return
			}
			if len(items) == 0 {
				return
			}
			for _, item := range items {
				if !yield(item, nil) {
					return
				}
			}
			if len(items) < limit {
				return
			}
			offset += len(items)
		}
	}
}

// DownloadAccountingSnapshot returns the raw bytes of an accounting snapshot archive for a user.
func (c *Client) DownloadAccountingSnapshot(
	ctx context.Context,
	user string,
) ([]byte, error) {
	q := url.Values{}
	q.Set("user", user)

	var out []byte
	err := c.getJSON(ctx, accountingSnapshotEndpoint, q, &out)
	return out, err
}

const comboPositionsEndpoint = "/v1/positions/combos"

// GetComboPositions returns combo positions for a wallet.
func (c *Client) GetComboPositions(
	ctx context.Context,
	p ComboPositionParams,
) ([]ComboPosition, error) {
	q := url.Values{}
	q.Set("user", p.User)
	setString(q, "status", p.Status)
	setString(q, "combo_condition_id", p.ConditionID)
	setString(q, "combo_position_id", p.PositionID)
	setInt(q, "limit", boundedLimit(p.Limit, 500))
	setInt(q, "offset", p.Offset)

	var out []ComboPosition
	err := c.getJSON(ctx, comboPositionsEndpoint, q, &out)
	return out, err
}

// IterComboPositions returns an iterator over combo positions.
func (c *Client) IterComboPositions(
	ctx context.Context,
	p ComboPositionParams,
) iter.Seq2[ComboPosition, error] {
	return func(yield func(ComboPosition, error) bool) {
		offset := p.Offset
		limit := iteratorLimit(p.Limit, 100, 500)
		for {
			q := p
			q.Limit = limit
			q.Offset = offset
			items, err := c.GetComboPositions(ctx, q)
			if err != nil {
				yield(ComboPosition{}, err)
				return
			}
			if len(items) == 0 {
				return
			}
			for _, item := range items {
				if !yield(item, nil) {
					return
				}
			}
			if len(items) < limit {
				return
			}
			offset += len(items)
		}
	}
}
