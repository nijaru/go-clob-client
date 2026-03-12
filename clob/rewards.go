package clob

import (
	"context"
	"net/url"
)

import "github.com/nijaru/go-clob-client/internal/polyhttp"

func (c *Client) GetEarningsForUserForDay(
	ctx context.Context,
	date string,
	nextCursor string,
) (*Page[UserEarning], error) {
	query := rewardsCursorQuery(nextCursor)
	query.Set("date", date)
	query.Set("signature_type", signatureTypeString(c.signatureType))

	var out Page[UserEarning]
	err := c.getJSON(ctx, rewardsUserEndpoint, query, polyhttp.AuthL2, &out)
	return &out, err
}

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

func (c *Client) GetUserRewardsAndMarketsConfig(
	ctx context.Context,
	params UserRewardsFilterParams,
	nextCursor string,
) ([]UserRewardsEarning, error) {
	query := rewardsCursorQuery(nextCursor)
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

	var out []UserRewardsEarning
	err := c.getJSON(ctx, rewardsUserMarketsEndpoint, query, polyhttp.AuthL2, &out)
	return out, err
}

func (c *Client) GetRewardPercentages(ctx context.Context) (RewardsPercentages, error) {
	query := url.Values{}
	query.Set("signature_type", signatureTypeString(c.signatureType))

	var out RewardsPercentages
	err := c.getJSON(ctx, rewardsPercentagesEndpoint, query, polyhttp.AuthL2, &out)
	return out, err
}

func (c *Client) GetCurrentRewards(
	ctx context.Context,
	nextCursor string,
) (*Page[CurrentReward], error) {
	query := rewardsCursorQuery(nextCursor)

	var out Page[CurrentReward]
	err := c.getJSON(ctx, rewardsMarketsCurrentEndpoint, query, polyhttp.AuthL2, &out)
	return &out, err
}

func (c *Client) GetRewardsForMarket(
	ctx context.Context,
	conditionID string,
	nextCursor string,
) (*Page[MarketReward], error) {
	query := rewardsCursorQuery(nextCursor)

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
