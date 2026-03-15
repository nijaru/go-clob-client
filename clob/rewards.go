package clob

import (
	"context"
	"net/url"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// GetEarningsForUserForDay returns all paginated earnings entries for a given day.
func (c *Client) GetEarningsForUserForDay(
	ctx context.Context,
	date string,
) ([]UserEarning, error) {
	cursor := initialCursor
	var earnings []UserEarning

	for cursor != endCursor {
		page, err := c.GetEarningsForUserForDayPage(ctx, date, cursor)
		if err != nil {
			return nil, err
		}
		earnings = append(earnings, page.Data...)

		nextCursor, done := nextPageCursor(cursor, page.NextCursor)
		if done {
			return earnings, nil
		}
		cursor = nextCursor
	}

	return earnings, nil
}

// GetEarningsForUserForDayPage returns a single earnings page for a given day.
func (c *Client) GetEarningsForUserForDayPage(
	ctx context.Context,
	date string,
	nextCursor string,
) (*Page[UserEarning], error) {
	query := rewardsCursorQuery(normalizedCursor(nextCursor))
	query.Set("date", date)
	query.Set("signature_type", signatureTypeString(c.signatureType))

	var out Page[UserEarning]
	err := c.getJSON(ctx, rewardsUserEndpoint, query, polyhttp.AuthL2, &out)
	return &out, err
}

// GetTotalEarningsForUserForDay returns the total earnings rows for a given day.
func (c *Client) GetTotalEarningsForUserForDay(
	ctx context.Context,
	date string,
) ([]TotalUserEarning, error) {
	query := url.Values{}
	query.Set("date", date)
	query.Set("signature_type", signatureTypeString(c.signatureType))

	var out []TotalUserEarning
	err := c.getJSON(ctx, rewardsUserTotalEndpoint, query, polyhttp.AuthL2, &out)
	return out, err
}

// GetUserEarningsAndMarketsConfig returns all paginated user reward-and-market entries.
func (c *Client) GetUserEarningsAndMarketsConfig(
	ctx context.Context,
	params UserRewardsFilterParams,
) ([]UserRewardsEarning, error) {
	cursor := initialCursor
	var entries []UserRewardsEarning

	for cursor != endCursor {
		page, err := c.GetUserEarningsAndMarketsConfigPage(ctx, params, cursor)
		if err != nil {
			return nil, err
		}
		entries = append(entries, page.Data...)

		nextCursor, done := nextPageCursor(cursor, page.NextCursor)
		if done {
			return entries, nil
		}
		cursor = nextCursor
	}

	return entries, nil
}

// GetUserEarningsAndMarketsConfigPage returns a single user reward-and-market page.
func (c *Client) GetUserEarningsAndMarketsConfigPage(
	ctx context.Context,
	params UserRewardsFilterParams,
	nextCursor string,
) (*Page[UserRewardsEarning], error) {
	query := rewardsCursorQuery(normalizedCursor(nextCursor))
	if params.Date != "" {
		query.Set("date", params.Date)
	}
	if params.OrderBy != "" {
		query.Set("order_by", params.OrderBy)
	}
	if params.Position != "" {
		query.Set("position", params.Position)
	}
	if params.NoCompetition {
		query.Set("no_competition", "true")
	}
	query.Set("signature_type", signatureTypeString(c.signatureType))

	var out Page[UserRewardsEarning]
	err := c.getJSON(ctx, rewardsUserMarketsEndpoint, query, polyhttp.AuthL2, &out)
	return &out, err
}

// GetRewardPercentages returns the liquidity reward percentages for the authenticated user.
func (c *Client) GetRewardPercentages(ctx context.Context) (RewardsPercentages, error) {
	query := url.Values{}
	query.Set("signature_type", signatureTypeString(c.signatureType))

	var out RewardsPercentages
	err := c.getJSON(ctx, rewardsPercentagesEndpoint, query, polyhttp.AuthL2, &out)
	return out, err
}

// GetCurrentRewards returns all paginated current reward summaries.
func (c *Client) GetCurrentRewards(ctx context.Context) ([]CurrentReward, error) {
	cursor := initialCursor
	var rewards []CurrentReward

	for cursor != endCursor {
		page, err := c.GetCurrentRewardsPage(ctx, cursor)
		if err != nil {
			return nil, err
		}
		rewards = append(rewards, page.Data...)

		nextCursor, done := nextPageCursor(cursor, page.NextCursor)
		if done {
			return rewards, nil
		}
		cursor = nextCursor
	}

	return rewards, nil
}

// GetCurrentRewardsPage returns a single current rewards page.
func (c *Client) GetCurrentRewardsPage(
	ctx context.Context,
	nextCursor string,
) (*Page[CurrentReward], error) {
	query := rewardsCursorQuery(normalizedCursor(nextCursor))

	var out Page[CurrentReward]
	err := c.getJSON(ctx, rewardsMarketsCurrentEndpoint, query, polyhttp.AuthL2, &out)
	return &out, err
}

// GetRawRewardsForMarket is an alias for GetRewardsForMarket.
func (c *Client) GetRawRewardsForMarket(
	ctx context.Context,
	conditionID string,
) ([]MarketReward, error) {
	return c.GetRewardsForMarket(ctx, conditionID)
}

// GetRewardsForMarket returns all paginated reward rows for a specific market.
func (c *Client) GetRewardsForMarket(
	ctx context.Context,
	conditionID string,
) ([]MarketReward, error) {
	cursor := initialCursor
	var rewards []MarketReward

	for cursor != endCursor {
		page, err := c.GetRewardsForMarketPage(ctx, conditionID, cursor)
		if err != nil {
			return nil, err
		}
		rewards = append(rewards, page.Data...)

		nextCursor, done := nextPageCursor(cursor, page.NextCursor)
		if done {
			return rewards, nil
		}
		cursor = nextCursor
	}

	return rewards, nil
}

// GetRewardsForMarketPage returns a single reward page for a specific market.
func (c *Client) GetRewardsForMarketPage(
	ctx context.Context,
	conditionID string,
	nextCursor string,
) (*Page[MarketReward], error) {
	query := rewardsCursorQuery(normalizedCursor(nextCursor))

	var out Page[MarketReward]
	err := c.getJSON(ctx, rewardsMarketsEndpoint+conditionID, query, polyhttp.AuthL2, &out)
	return &out, err
}

func rewardsCursorQuery(nextCursor string) url.Values {
	query := url.Values{}
	if nextCursor != "" {
		query.Set("next_cursor", nextCursor)
	}
	return query
}
