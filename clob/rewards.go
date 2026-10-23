package clob

import (
	"context"
	"fmt"
	"iter"
	"math/big"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
	"github.com/nijaru/go-clob-client/internal/polyjson"
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
	return paginate("", func(cursor string) (*Page[UserEarning], error) {
		return c.GetEarningsForUserForDayPage(ctx, date, cursor)
	})
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
	params TotalEarningsParams,
) ([]TotalUserEarning, error) {
	query := url.Values{}
	query.Set("date", params.Date)
	query.Set("signature_type", signatureTypeString(c.signatureType))
	boolQuery(query, "sponsored", params.Sponsored)

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
	return paginate("", func(cursor string) (*Page[UserRewardsEarning], error) {
		return c.GetUserEarningsAndMarketsConfigPage(ctx, params, cursor)
	})
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
func (c *AuthenticatedClient) GetCurrentRewards(
	ctx context.Context,
	params CurrentRewardsParams,
) ([]CurrentReward, error) {
	rewards := make([]CurrentReward, 0, 64)
	for reward, err := range c.IterCurrentRewards(ctx, params) {
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
	params CurrentRewardsParams,
) iter.Seq2[CurrentReward, error] {
	return paginate(params.Cursor, func(cursor string) (*Page[CurrentReward], error) {
		params.Cursor = cursor
		return c.GetCurrentRewardsPage(ctx, params)
	})
}

// GetCurrentRewardsPage returns a single current rewards page.
func (c *AuthenticatedClient) GetCurrentRewardsPage(
	ctx context.Context,
	params CurrentRewardsParams,
) (*Page[CurrentReward], error) {
	query := rewardsCursorQuery(normalizedCursor(params.Cursor))
	boolQuery(query, "sponsored", params.Sponsored)

	var out Page[CurrentReward]
	err := c.getJSON(ctx, rewardsMarketsCurrentEndpoint, query, polyhttp.AuthL2, &out)
	return &out, err
}

// GetRewardsForMarket returns all paginated reward rows for a specific market.
func (c *AuthenticatedClient) GetRewardsForMarket(
	ctx context.Context,
	params MarketRewardsParams,
) ([]MarketReward, error) {
	rewards := make([]MarketReward, 0, 64)
	for reward, err := range c.IterRewardsForMarket(ctx, params) {
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
	params MarketRewardsParams,
) iter.Seq2[MarketReward, error] {
	return paginate(params.Cursor, func(cursor string) (*Page[MarketReward], error) {
		params.Cursor = cursor
		return c.GetRewardsForMarketPage(ctx, params)
	})
}

// GetRewardsForMarketPage returns a single reward page for a specific market.
func (c *AuthenticatedClient) GetRewardsForMarketPage(
	ctx context.Context,
	params MarketRewardsParams,
) (*Page[MarketReward], error) {
	query := rewardsCursorQuery(normalizedCursor(params.Cursor))
	boolQuery(query, "sponsored", params.Sponsored)

	var out Page[MarketReward]
	err := c.getJSON(
		ctx,
		rewardsMarketsEndpoint+url.PathEscape(params.ConditionID),
		query,
		polyhttp.AuthL2,
		&out,
	)
	return &out, err
}

// GetRewardMarkets returns all active reward markets matching the filters.
func (c *AuthenticatedClient) GetRewardMarkets(
	ctx context.Context,
	params RewardMarketsParams,
) ([]RewardMarket, error) {
	markets := make([]RewardMarket, 0, 64)
	for market, err := range c.IterRewardMarkets(ctx, params) {
		if err != nil {
			return nil, err
		}
		markets = append(markets, market)
	}
	return markets, nil
}

// IterRewardMarkets returns an iterator over active reward markets.
func (c *AuthenticatedClient) IterRewardMarkets(
	ctx context.Context,
	params RewardMarketsParams,
) iter.Seq2[RewardMarket, error] {
	return paginate(params.Cursor, func(cursor string) (*Page[RewardMarket], error) {
		params.Cursor = cursor
		return c.GetRewardMarketsPage(ctx, params)
	})
}

// GetRewardMarketsPage returns a single page of active reward markets.
func (c *AuthenticatedClient) GetRewardMarketsPage(
	ctx context.Context,
	params RewardMarketsParams,
) (*Page[RewardMarket], error) {
	query, err := rewardMarketsQuery(params)
	if err != nil {
		return nil, err
	}

	var out Page[RewardMarket]
	err = c.getJSON(ctx, rewardsMarketsMultiEndpoint, query, polyhttp.AuthNone, &out)
	return &out, err
}

// GetCurrentRebates returns the maker fee rebates for one calendar day. A maker
// with no rebates for the date returns an empty slice.
func (c *AuthenticatedClient) GetCurrentRebates(
	ctx context.Context,
	params RebatesParams,
) ([]RebatedFee, error) {
	if _, err := time.Parse(time.DateOnly, params.Date); err != nil {
		return nil, fmt.Errorf("rewards: date must be a YYYY-MM-DD calendar date: %w", err)
	}
	if !common.IsHexAddress(params.MakerAddress) {
		return nil, fmt.Errorf("rewards: maker_address must be a hex EVM address")
	}
	if common.HexToAddress(params.MakerAddress) == (common.Address{}) {
		return nil, fmt.Errorf("rewards: maker_address must be nonzero")
	}
	query := url.Values{}
	query.Set("date", params.Date)
	query.Set("maker_address", params.MakerAddress)

	var out []RebatedFee
	err := c.getJSON(ctx, rebatesCurrentEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

func rewardMarketsQuery(params RewardMarketsParams) (url.Values, error) {
	query := url.Values{}
	if params.PageSize < 0 || params.PageSize > 500 {
		return nil, fmt.Errorf("rewards: page_size must be between 1 and 500")
	}
	if params.PageSize > 0 {
		query.Set("page_size", strconv.Itoa(params.PageSize))
	}
	if params.Query != "" {
		query.Set("q", params.Query)
	}
	if params.EventTitle != "" {
		query.Set("event_title", params.EventTitle)
	}
	for _, slug := range params.TagSlugs {
		if slug == "" {
			return nil, fmt.Errorf("rewards: tag_slug must be nonempty")
		}
		query.Add("tag_slug", slug)
	}
	if params.MarketID != nil {
		if *params.MarketID <= 0 {
			return nil, fmt.Errorf("rewards: market_id must be a positive integer")
		}
		query.Set("market_id", strconv.FormatInt(*params.MarketID, 10))
	}
	for _, id := range params.EventIDs {
		if id <= 0 {
			return nil, fmt.Errorf("rewards: event_id must contain positive integers")
		}
		query.Add("event_id", strconv.FormatInt(id, 10))
	}
	if params.OrderBy != "" && !slices.Contains(rewardMarketSorts, params.OrderBy) {
		return nil, fmt.Errorf("rewards: invalid reward market sort field %q", params.OrderBy)
	}
	if params.OrderBy != "" {
		query.Set("order_by", string(params.OrderBy))
	}
	switch params.SortDirection {
	case "":
	case RewardMarketSortAscending, RewardMarketSortDescending:
		query.Set("position", string(params.SortDirection))
	default:
		return nil, fmt.Errorf(
			"rewards: position must be ASC or DESC, got %q",
			params.SortDirection,
		)
	}
	for key, bound := range map[string]*DecimalString{
		"min_volume_24hr": params.MinVolume24hr,
		"max_volume_24hr": params.MaxVolume24hr,
		"min_spread":      params.MinSpread,
		"max_spread":      params.MaxSpread,
		"min_price":       params.MinPrice,
		"max_price":       params.MaxPrice,
	} {
		if bound == nil {
			continue
		}
		if err := validateNonnegative(*bound); err != nil {
			return nil, fmt.Errorf("rewards: %s must be a finite nonnegative number", key)
		}
		query.Set(key, string(*bound))
	}
	if cursor := normalizedCursor(params.Cursor); cursor != "" {
		query.Set("next_cursor", cursor)
	}
	return query, nil
}

var rewardMarketSorts = []RewardMarketSort{
	RewardMarketSortMarketID,
	RewardMarketSortCreatedAt,
	RewardMarketSortVolume24hr,
	RewardMarketSortSpread,
	RewardMarketSortCompetitiveness,
	RewardMarketSortMaxSpread,
	RewardMarketSortMinSize,
	RewardMarketSortQuestion,
	RewardMarketSortOneDayPriceChange,
	RewardMarketSortRatePerDay,
	RewardMarketSortPrice,
	RewardMarketSortEndDate,
	RewardMarketSortStartDate,
	RewardMarketSortRewardEndDate,
}

func boolQuery(query url.Values, key string, value *bool) {
	if value != nil {
		query.Set(key, strconv.FormatBool(*value))
	}
}

func rewardsCursorQuery(nextCursor string) url.Values {
	query := url.Values{}
	if nextCursor != "" {
		query.Set("next_cursor", nextCursor)
	}
	return query
}

// validateNonnegative rejects bounds the service cannot express: non-decimal
// text, infinities, and negative values.
func validateNonnegative(value DecimalString) error {
	if !polyjson.ValidDecimal(string(value)) {
		return fmt.Errorf("rewards: invalid decimal %q", value)
	}
	rational, ok := new(big.Rat).SetString(string(value))
	if !ok || rational.Sign() < 0 {
		return fmt.Errorf("rewards: invalid decimal %q", value)
	}
	return nil
}
