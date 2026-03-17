package clob

import (
	"context"
	"iter"
	"net/url"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// GetEarningsForUserForDay returns all paginated earnings entries for a given day.
func (c *AuthenticatedClient) GetEarningsForUserForDay(
	ctx context.Context,
	date string,
) ([]UserEarning, error) {
	earnings := make([]UserEarning, 0, 64)
	for earning, err := range c.IterEarningsForUserForDay(ctx, date) {
		if err != nil {
			return nil, err
		}
		earnings = append(earnings, earning)
	}
	return earnings, nil
}

// IterEarningsForUserForDay returns an iterator over earnings entries for a given day.
func (c *AuthenticatedClient) IterEarningsForUserForDay(
	ctx context.Context,
	date string,
) iter.Seq2[UserEarning, error] {
	return func(yield func(UserEarning, error) bool) {
		cursor := initialCursor
		for cursor != endCursor {
			page, err := c.GetEarningsForUserForDayPage(ctx, date, cursor)
			if err != nil {
				var zero UserEarning
				yield(zero, err)
				return
			}
			for _, earning := range page.Data {
				if !yield(earning, nil) {
					return
				}
			}
			nextCursor, done := nextPageCursor(cursor, page.NextCursor)
			if done {
				return
			}
			cursor = nextCursor
		}
	}
}

// GetEarningsForUserForDayPage returns a single earnings page for a given day.
func (c *AuthenticatedClient) GetEarningsForUserForDayPage(
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
func (c *AuthenticatedClient) GetTotalEarningsForUserForDay(
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
func (c *AuthenticatedClient) GetUserEarningsAndMarketsConfig(
	ctx context.Context,
	params UserRewardsFilterParams,
) ([]UserRewardsEarning, error) {
	entries := make([]UserRewardsEarning, 0, 64)
	for entry, err := range c.IterUserEarningsAndMarketsConfig(ctx, params) {
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// IterUserEarningsAndMarketsConfig returns an iterator over user reward-and-market entries.
func (c *AuthenticatedClient) IterUserEarningsAndMarketsConfig(
	ctx context.Context,
	params UserRewardsFilterParams,
) iter.Seq2[UserRewardsEarning, error] {
	return func(yield func(UserRewardsEarning, error) bool) {
		cursor := initialCursor
		for cursor != endCursor {
			page, err := c.GetUserEarningsAndMarketsConfigPage(ctx, params, cursor)
			if err != nil {
				var zero UserRewardsEarning
				yield(zero, err)
				return
			}
			for _, entry := range page.Data {
				if !yield(entry, nil) {
					return
				}
			}
			nextCursor, done := nextPageCursor(cursor, page.NextCursor)
			if done {
				return
			}
			cursor = nextCursor
		}
	}
}

// GetUserEarningsAndMarketsConfigPage returns a single user reward-and-market page.
func (c *AuthenticatedClient) GetUserEarningsAndMarketsConfigPage(
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
func (c *AuthenticatedClient) GetRewardPercentages(
	ctx context.Context,
) (RewardsPercentages, error) {
	query := url.Values{}
	query.Set("signature_type", signatureTypeString(c.signatureType))

	var out RewardsPercentages
	err := c.getJSON(ctx, rewardsPercentagesEndpoint, query, polyhttp.AuthL2, &out)
	return out, err
}

// GetCurrentRewards returns all paginated current reward summaries.
func (c *AuthenticatedClient) GetCurrentRewards(ctx context.Context) ([]CurrentReward, error) {
	rewards := make([]CurrentReward, 0, 64)
	for reward, err := range c.IterCurrentRewards(ctx) {
		if err != nil {
			return nil, err
		}
		rewards = append(rewards, reward)
	}
	return rewards, nil
}

// IterCurrentRewards returns an iterator over current reward summaries.
func (c *AuthenticatedClient) IterCurrentRewards(
	ctx context.Context,
) iter.Seq2[CurrentReward, error] {
	return func(yield func(CurrentReward, error) bool) {
		cursor := initialCursor
		for cursor != endCursor {
			page, err := c.GetCurrentRewardsPage(ctx, cursor)
			if err != nil {
				var zero CurrentReward
				yield(zero, err)
				return
			}
			for _, reward := range page.Data {
				if !yield(reward, nil) {
					return
				}
			}
			nextCursor, done := nextPageCursor(cursor, page.NextCursor)
			if done {
				return
			}
			cursor = nextCursor
		}
	}
}

// GetCurrentRewardsPage returns a single current rewards page.
func (c *AuthenticatedClient) GetCurrentRewardsPage(
	ctx context.Context,
	nextCursor string,
) (*Page[CurrentReward], error) {
	query := rewardsCursorQuery(normalizedCursor(nextCursor))

	var out Page[CurrentReward]
	err := c.getJSON(ctx, rewardsMarketsCurrentEndpoint, query, polyhttp.AuthL2, &out)
	return &out, err
}

// GetRawRewardsForMarket is an alias for GetRewardsForMarket.
func (c *AuthenticatedClient) GetRawRewardsForMarket(
	ctx context.Context,
	conditionID string,
) ([]MarketReward, error) {
	return c.GetRewardsForMarket(ctx, conditionID)
}

// GetRewardsForMarket returns all paginated reward rows for a specific market.
func (c *AuthenticatedClient) GetRewardsForMarket(
	ctx context.Context,
	conditionID string,
) ([]MarketReward, error) {
	rewards := make([]MarketReward, 0, 64)
	for reward, err := range c.IterRewardsForMarket(ctx, conditionID) {
		if err != nil {
			return nil, err
		}
		rewards = append(rewards, reward)
	}
	return rewards, nil
}

// IterRewardsForMarket returns an iterator over reward rows for a specific market.
func (c *AuthenticatedClient) IterRewardsForMarket(
	ctx context.Context,
	conditionID string,
) iter.Seq2[MarketReward, error] {
	return func(yield func(MarketReward, error) bool) {
		cursor := initialCursor
		for cursor != endCursor {
			page, err := c.GetRewardsForMarketPage(ctx, conditionID, cursor)
			if err != nil {
				var zero MarketReward
				yield(zero, err)
				return
			}
			for _, reward := range page.Data {
				if !yield(reward, nil) {
					return
				}
			}
			nextCursor, done := nextPageCursor(cursor, page.NextCursor)
			if done {
				return
			}
			cursor = nextCursor
		}
	}
}

// GetRewardsForMarketPage returns a single reward page for a specific market.
func (c *AuthenticatedClient) GetRewardsForMarketPage(
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
