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
	if err := validatePagination("positions", p.Limit, p.Offset, 0, 500, 10_000); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("user", p.User)
	p.Filter.appendQuery(q)
	setString(q, "sizeThreshold", p.SizeThreshold)
	setBool(q, "redeemable", p.Redeemable)
	setBool(q, "mergeable", p.Mergeable)
	setInt(q, "limit", p.Limit)
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
		if err := validatePagination("positions", p.Limit, p.Offset, 0, 500, 10_000); err != nil {
			yield(Position{}, err)
			return
		}
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
	if err := validatePagination("closed_positions", p.Limit, p.Offset, 0, 50, 100_000); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("user", p.User)
	p.Filter.appendQuery(q)
	setString(q, "title", p.Title)
	setInt(q, "limit", p.Limit)
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
		if err := validatePagination("closed_positions", p.Limit, p.Offset, 0, 50, 100_000); err != nil {
			yield(ClosedPosition{}, err)
			return
		}
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
	if err := validatePagination("trades", p.Limit, p.Offset, 0, 10_000, 10_000); err != nil {
		return nil, err
	}
	q := url.Values{}
	setString(q, "user", p.User)
	p.Filter.appendQuery(q)
	setInt(q, "limit", p.Limit)
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
		if err := validatePagination("trades", p.Limit, p.Offset, 0, 10_000, 10_000); err != nil {
			yield(Trade{}, err)
			return
		}
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
	if err := validatePagination("activity", p.Limit, p.Offset, 0, 500, 10_000); err != nil {
		return nil, err
	}
	if err := activityTimeBoundsError(p); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("user", p.User)
	p.Filter.appendQuery(q)
	setCommaList(q, "type", p.ActivityTypes)
	// The service defaults this exclusion to true. Disable it unconditionally
	// so unfiltered activity includes account-level deposits and withdrawals.
	q.Set("excludeDepositsWithdrawals", "false")
	setInt(q, "limit", p.Limit)
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
		if err := validatePagination("activity", p.Limit, p.Offset, 0, 500, 10_000); err != nil {
			yield(Activity{}, err)
			return
		}
		if p.Start < 0 || p.End < 0 {
			yield(Activity{}, activityTimeBoundsError(p))
			return
		}
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
	if err := validateBound("holders.limit", p.Limit, 0, 20); err != nil {
		return nil, err
	}
	if err := validateBound("holders.min_balance", p.MinBalance, 0, 999_999); err != nil {
		return nil, err
	}
	q := url.Values{}
	setCommaList(q, "market", p.Markets)
	setInt(q, "limit", p.Limit)
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
	if err := validatePagination("leaderboard", p.Limit, p.Offset, 0, 50, 1_000); err != nil {
		return nil, err
	}
	q := url.Values{}
	setString(q, "category", p.Category)
	setString(q, "timePeriod", p.TimePeriod)
	setString(q, "orderBy", p.SortBy)
	setInt(q, "limit", p.Limit)
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
		if err := validatePagination("leaderboard", p.Limit, p.Offset, 0, 50, 1_000); err != nil {
			yield(TraderLeaderboardEntry{}, err)
			return
		}
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
	if err := validatePagination("builder_leaderboard", p.Limit, p.Offset, 0, 50, 1_000); err != nil {
		return nil, err
	}
	q := url.Values{}
	setString(q, "timePeriod", p.TimePeriod)
	setInt(q, "limit", p.Limit)
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
		if err := validatePagination("builder_leaderboard", p.Limit, p.Offset, 0, 50, 1_000); err != nil {
			yield(BuilderLeaderboardEntry{}, err)
			return
		}
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
	if err := validatePagination("market_positions", p.Limit, p.Offset, 0, 500, 10_000); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("market", p.Market)
	setString(q, "user", p.User)
	setString(q, "status", p.Status)
	setString(q, "sortBy", p.SortBy)
	setString(q, "sortDirection", p.SortDirection)
	setInt(q, "limit", p.Limit)
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
		if err := validatePagination("market_positions", p.Limit, p.Offset, 0, 500, 10_000); err != nil {
			yield(MetaMarketPosition{}, err)
			return
		}
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

type comboPagination struct {
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor"`
}

type comboPositionsResponse struct {
	Combos     []ComboPosition `json:"combos"`
	Pagination comboPagination `json:"pagination"`
}

type comboActivityResponse struct {
	Activity   []ComboActivity `json:"activity"`
	Pagination comboPagination `json:"pagination"`
}

func comboPositionQuery(p ComboPositionParams) url.Values {
	q := url.Values{}
	q.Set("user", p.User)
	setString(q, "status", p.Status)
	setString(q, "sort", p.Sort)
	if len(p.ConditionIDs) > 0 {
		setCommaList(q, "market_id", p.ConditionIDs)
	} else {
		setString(q, "market_id", p.ConditionID)
	}
	setInt64(q, "updated_after", p.UpdatedAfter)
	setInt64(q, "updated_before", p.UpdatedBefore)
	setInt(q, "limit", p.Limit)
	setString(q, "cursor", p.Cursor)
	// Keep the pre-keyset filters usable for callers migrating from the
	// previous unofficial implementation. Current official clients use the
	// market_id/cursor fields above.
	setString(q, "combo_position_id", p.PositionID)
	setInt(q, "offset", p.Offset)
	return q
}

// GetComboPositionsPage returns one official cursor-paginated response.
func (c *Client) GetComboPositionsPage(
	ctx context.Context,
	p ComboPositionParams,
) (ComboPositionPage, error) {
	if err := validatePagination("combo_positions", p.Limit, p.Offset, 0, 500, 10_000); err != nil {
		return ComboPositionPage{}, err
	}
	if p.UpdatedAfter < 0 {
		return ComboPositionPage{}, &ParameterBoundsError{
			Parameter: "combo_positions.updated_after",
			Value:     int(p.UpdatedAfter),
			Minimum:   0,
		}
	}
	if p.UpdatedBefore < 0 {
		return ComboPositionPage{}, &ParameterBoundsError{
			Parameter: "combo_positions.updated_before",
			Value:     int(p.UpdatedBefore),
			Minimum:   0,
		}
	}
	var out comboPositionsResponse
	if err := c.getJSON(ctx, comboPositionsEndpoint, comboPositionQuery(p), &out); err != nil {
		return ComboPositionPage{}, err
	}
	return ComboPositionPage{
		Items:      out.Combos,
		Limit:      out.Pagination.Limit,
		Offset:     out.Pagination.Offset,
		HasMore:    out.Pagination.HasMore,
		NextCursor: out.Pagination.NextCursor,
	}, nil
}

// GetComboPositions returns combo positions for a wallet.
func (c *Client) GetComboPositions(
	ctx context.Context,
	p ComboPositionParams,
) ([]ComboPosition, error) {
	page, err := c.GetComboPositionsPage(ctx, p)
	return page.Items, err
}

// IterComboPositions returns an iterator over combo positions.
func (c *Client) IterComboPositions(
	ctx context.Context,
	p ComboPositionParams,
) iter.Seq2[ComboPosition, error] {
	return func(yield func(ComboPosition, error) bool) {
		cursor := p.Cursor
		for {
			q := p
			q.Cursor = cursor
			q.Offset = 0
			page, err := c.GetComboPositionsPage(ctx, q)
			if err != nil {
				yield(ComboPosition{}, err)
				return
			}
			for _, item := range page.Items {
				if !yield(item, nil) {
					return
				}
			}
			if !page.HasMore || page.NextCursor == "" || page.NextCursor == cursor {
				return
			}
			cursor = page.NextCursor
		}
	}
}

const comboActivityEndpoint = "/v1/activity/combos"

func comboActivityQuery(p ComboActivityParams) url.Values {
	q := url.Values{}
	q.Set("user", p.User)
	if len(p.ConditionIDs) > 0 {
		setCommaList(q, "market_id", p.ConditionIDs)
	} else {
		setString(q, "market_id", p.ConditionID)
	}
	setInt(q, "limit", p.Limit)
	setString(q, "cursor", p.Cursor)
	return q
}

// GetComboActivityPage returns one official cursor-paginated response.
func (c *Client) GetComboActivityPage(
	ctx context.Context,
	p ComboActivityParams,
) (ComboActivityPage, error) {
	if err := validateBound("combo_activity.limit", p.Limit, 0, 500); err != nil {
		return ComboActivityPage{}, err
	}
	var out comboActivityResponse
	if err := c.getJSON(ctx, comboActivityEndpoint, comboActivityQuery(p), &out); err != nil {
		return ComboActivityPage{}, err
	}
	return ComboActivityPage{
		Items:      out.Activity,
		Limit:      out.Pagination.Limit,
		Offset:     out.Pagination.Offset,
		HasMore:    out.Pagination.HasMore,
		NextCursor: out.Pagination.NextCursor,
	}, nil
}

// GetComboActivity returns the first page of combo lifecycle activity.
func (c *Client) GetComboActivity(
	ctx context.Context,
	p ComboActivityParams,
) ([]ComboActivity, error) {
	page, err := c.GetComboActivityPage(ctx, p)
	return page.Items, err
}

// IterComboActivity returns an iterator over all combo lifecycle activity pages.
func (c *Client) IterComboActivity(
	ctx context.Context,
	p ComboActivityParams,
) iter.Seq2[ComboActivity, error] {
	return func(yield func(ComboActivity, error) bool) {
		cursor := p.Cursor
		for {
			q := p
			q.Cursor = cursor
			page, err := c.GetComboActivityPage(ctx, q)
			if err != nil {
				yield(ComboActivity{}, err)
				return
			}
			for _, item := range page.Items {
				if !yield(item, nil) {
					return
				}
			}
			if !page.HasMore || page.NextCursor == "" || page.NextCursor == cursor {
				return
			}
			cursor = page.NextCursor
		}
	}
}
