package clob

import (
	stdjson "encoding/json" //nolint:depguard // numeric reward wire normalization
	"fmt"
)

// UserEarning is a single daily user earnings row.
type UserEarning struct {
	Date         string `json:"date"`
	ConditionID  string `json:"condition_id"`
	AssetAddress string `json:"asset_address"`
	MakerAddress string `json:"maker_address"`
	Earnings     string `json:"earnings"`
	AssetRate    string `json:"asset_rate"`
}

// TotalUserEarning is a daily aggregate user earnings row. NativeEarnings and
// SponsoredEarnings break the total down by funding source and are nil when the
// service does not report them.
type TotalUserEarning struct {
	Date              string         `json:"date"`
	AssetAddress      string         `json:"asset_address"`
	MakerAddress      string         `json:"maker_address"`
	Earnings          string         `json:"earnings"`
	AssetRate         string         `json:"asset_rate"`
	NativeEarnings    *DecimalString `json:"native_earnings,omitzero"`
	SponsoredEarnings *DecimalString `json:"sponsored_earnings,omitzero"`
}

// RewardsPercentages maps market IDs to their reward percentages.
type RewardsPercentages map[string]string

// UnmarshalJSON accepts numeric or string percentage values from the rewards API.
func (p *RewardsPercentages) UnmarshalJSON(data []byte) error {
	var fields map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("reward percentages: decode object: %w", err)
	}
	if fields == nil {
		*p = nil
		return nil
	}

	percentages := make(RewardsPercentages, len(fields))
	for marketID, raw := range fields {
		value, err := decodeStringOrNumber(raw)
		if err != nil {
			return fmt.Errorf("reward percentages %s: %w", marketID, err)
		}
		percentages[marketID] = value
	}
	*p = percentages
	return nil
}

// RewardsConfig is the reward configuration for a market or user reward entry.
type RewardsConfig struct {
	ID           *int64 `json:"id,omitzero"`
	AssetAddress string `json:"asset_address"`
	StartDate    string `json:"start_date"`
	EndDate      string `json:"end_date"`
	RatePerDay   string `json:"rate_per_day"`
	TotalRewards string `json:"total_rewards"`
}

// MarketRewardsConfig is the rewards configuration shape returned from market reward endpoints.
type MarketRewardsConfig struct {
	ID           string `json:"id"`
	AssetAddress string `json:"asset_address"`
	StartDate    string `json:"start_date"`
	EndDate      string `json:"end_date"`
	RatePerDay   string `json:"rate_per_day"`
	TotalRewards string `json:"total_rewards"`
	TotalDays    string `json:"total_days"`
}

// UnmarshalJSON accepts numeric or string decimal fields from the rewards API.
func (r *MarketRewardsConfig) UnmarshalJSON(data []byte) error {
	type alias MarketRewardsConfig
	return unmarshalResponseDates(data, (*alias)(r),
		[]string{"id", "rate_per_day", "total_rewards", "total_days"}, true,
		"start_date", "end_date")
}

// Earning is an asset-specific earnings breakdown.
type Earning struct {
	AssetAddress string `json:"asset_address"`
	Earnings     string `json:"earnings"`
	AssetRate    string `json:"asset_rate"`
}

// CurrentReward is the current rewards summary for a market.
type CurrentReward struct {
	ConditionID        string          `json:"condition_id"`
	RewardsConfig      []RewardsConfig `json:"rewards_config"`
	RewardsMaxSpread   string          `json:"rewards_max_spread"`
	RewardsMinSize     string          `json:"rewards_min_size"`
	SponsoredDailyRate *DecimalString  `json:"sponsored_daily_rate,omitzero"`
	SponsorsCount      *int64          `json:"sponsors_count,omitzero"`
	NativeDailyRate    *DecimalString  `json:"native_daily_rate,omitzero"`
	TotalDailyRate     *DecimalString  `json:"total_daily_rate,omitzero"`
}

// MarketReward is the reward metadata for a specific market.
type MarketReward struct {
	ConditionID           string                `json:"condition_id"`
	Question              string                `json:"question"`
	MarketSlug            string                `json:"market_slug"`
	EventSlug             string                `json:"event_slug"`
	Image                 string                `json:"image"`
	RewardsMaxSpread      string                `json:"rewards_max_spread"`
	RewardsMinSize        string                `json:"rewards_min_size"`
	MarketCompetitiveness string                `json:"market_competitiveness"`
	Tokens                []OutcomeToken        `json:"tokens"`
	RewardsConfig         []MarketRewardsConfig `json:"rewards_config"`
}

// UnmarshalJSON accepts numeric Decimal fields emitted by the Rust-compatible
// rewards endpoint while retaining Go's string representation.
func (r *MarketReward) UnmarshalJSON(data []byte) error {
	type alias MarketReward
	return unmarshalResponseStrings(data, (*alias)(r),
		"rewards_max_spread",
		"rewards_min_size",
		"market_competitiveness",
	)
}

// UnmarshalJSON accepts numeric or string decimal fields from the rewards API.
func (e *UserEarning) UnmarshalJSON(data []byte) error {
	type alias UserEarning
	return unmarshalResponseDates(
		data,
		(*alias)(e),
		[]string{"earnings", "asset_rate"},
		true,
		"date",
	)
}

// UnmarshalJSON accepts numeric or string decimal fields from the rewards API.
func (e *TotalUserEarning) UnmarshalJSON(data []byte) error {
	type alias TotalUserEarning
	return unmarshalResponseDates(
		data,
		(*alias)(e),
		[]string{"earnings", "asset_rate", "native_earnings", "sponsored_earnings"},
		true,
		"date",
	)
}

// UnmarshalJSON accepts numeric or string decimal fields from the rewards API.
func (c *RewardsConfig) UnmarshalJSON(data []byte) error {
	type alias RewardsConfig
	return unmarshalResponseDates(
		data,
		(*alias)(c),
		[]string{"rate_per_day", "total_rewards"},
		true,
		"start_date",
		"end_date",
	)
}

// UnmarshalJSON accepts numeric or string decimal fields from the rewards API.
func (e *Earning) UnmarshalJSON(data []byte) error {
	type alias Earning
	return unmarshalResponseStrings(data, (*alias)(e), "earnings", "asset_rate")
}

// UnmarshalJSON accepts numeric or string decimal fields from the rewards API.
func (r *CurrentReward) UnmarshalJSON(data []byte) error {
	type alias CurrentReward
	return unmarshalResponseStrings(data, (*alias)(r),
		"rewards_max_spread", "rewards_min_size",
	)
}

// UnmarshalJSON accepts numeric or string decimal fields from the rewards API.
func (r *UserRewardsEarning) UnmarshalJSON(data []byte) error {
	type alias UserRewardsEarning
	return unmarshalResponseStrings(data, (*alias)(r),
		"rewards_max_spread", "rewards_min_size", "market_competitiveness",
		"earning_percentage",
	)
}

// UserRewardsEarning is the user-facing reward-and-market earnings entry.
type UserRewardsEarning struct {
	ConditionID           string          `json:"condition_id"`
	Question              string          `json:"question"`
	MarketSlug            string          `json:"market_slug"`
	EventSlug             string          `json:"event_slug"`
	Image                 string          `json:"image"`
	RewardsMaxSpread      string          `json:"rewards_max_spread"`
	RewardsMinSize        string          `json:"rewards_min_size"`
	MarketCompetitiveness string          `json:"market_competitiveness"`
	Tokens                []OutcomeToken  `json:"tokens"`
	RewardsConfig         []RewardsConfig `json:"rewards_config"`
	MakerAddress          string          `json:"maker_address"`
	EarningPercentage     string          `json:"earning_percentage"`
	Earnings              []Earning       `json:"earnings"`
}

// UserRewardsFilterParams filters user reward-and-market queries.
type UserRewardsFilterParams struct {
	Date          string
	OrderBy       string
	Position      string
	NoCompetition bool
}

// CurrentRewardsParams filters current reward summaries. Cursor resumes a page
// traversal; the empty value starts at the first page. Sponsored restricts rows
// to sponsored (true) or native (false) rewards; nil uses the service default.
type CurrentRewardsParams struct {
	Sponsored *bool
	Cursor    string
}

// MarketRewardsParams filters one market's reward rows. ConditionID selects the
// market; Cursor resumes a page traversal. Sponsored restricts rows to
// sponsored (true) or native (false) rewards; nil uses the service default.
type MarketRewardsParams struct {
	ConditionID string
	Sponsored   *bool
	Cursor      string
}

// TotalEarningsParams selects a day's total earnings. Sponsored restricts rows
// to sponsored (true) or native (false) earnings; nil uses the service default.
type TotalEarningsParams struct {
	Date      string
	Sponsored *bool
}

// RewardMarketSort selects the ordering of reward market discovery.
type RewardMarketSort string

const (
	RewardMarketSortMarketID          RewardMarketSort = "market_id"
	RewardMarketSortCreatedAt         RewardMarketSort = "created_at"
	RewardMarketSortVolume24hr        RewardMarketSort = "volume_24hr"
	RewardMarketSortSpread            RewardMarketSort = "spread"
	RewardMarketSortCompetitiveness   RewardMarketSort = "competitiveness"
	RewardMarketSortMaxSpread         RewardMarketSort = "max_spread"
	RewardMarketSortMinSize           RewardMarketSort = "min_size"
	RewardMarketSortQuestion          RewardMarketSort = "question"
	RewardMarketSortOneDayPriceChange RewardMarketSort = "one_day_price_change"
	RewardMarketSortRatePerDay        RewardMarketSort = "rate_per_day"
	RewardMarketSortPrice             RewardMarketSort = "price"
	RewardMarketSortEndDate           RewardMarketSort = "end_date"
	RewardMarketSortStartDate         RewardMarketSort = "start_date"
	RewardMarketSortRewardEndDate     RewardMarketSort = "reward_end_date"
)

// RewardMarketSortDirection is the ordering direction for reward market
// discovery. An empty value leaves the service default (ascending).
type RewardMarketSortDirection string

const (
	RewardMarketSortAscending  RewardMarketSortDirection = "ASC"
	RewardMarketSortDescending RewardMarketSortDirection = "DESC"
)

// RewardMarket is an active market with its reward configurations and discovery
// metadata. Configurations can be empty. CreatedAt and EndDate are ISO
// timestamps normalized from the service's epoch milliseconds; EndDate is empty
// when the market has no reward end.
type RewardMarket struct {
	ConditionID           string          `json:"condition_id"`
	MarketID              string          `json:"market_id"`
	MarketSlug            string          `json:"market_slug"`
	Question              string          `json:"question"`
	Image                 string          `json:"image"`
	MarketCompetitiveness string          `json:"market_competitiveness"`
	RewardsConfig         []RewardsConfig `json:"rewards_config"`
	RewardsMaxSpread      string          `json:"rewards_max_spread"`
	RewardsMinSize        string          `json:"rewards_min_size"`
	Spread                string          `json:"spread"`
	Tokens                []OutcomeToken  `json:"tokens"`
	GroupItemTitle        string          `json:"group_item_title"`
	Volume24hr            string          `json:"volume_24hr"`
	EventID               string          `json:"event_id"`
	EventSlug             string          `json:"event_slug"`
	CreatedAt             string          `json:"created_at"`
	OneDayPriceChange     string          `json:"one_day_price_change"`
	EndDate               string          `json:"end_date"`
}

// UnmarshalJSON accepts numeric or string Decimal fields and epoch-millisecond
// dates from the reward discovery endpoint.
func (r *RewardMarket) UnmarshalJSON(data []byte) error {
	type alias RewardMarket
	return unmarshalResponseDates(
		data,
		(*alias)(r),
		[]string{
			"market_competitiveness",
			"rewards_max_spread",
			"rewards_min_size",
			"spread",
			"volume_24hr",
			"one_day_price_change",
		},
		true,
		"created_at",
		"end_date",
	)
}

// RewardMarketsParams filters reward market discovery. Zero bounds are absent.
// Per the service contract, positive minimum bounds are exclusive and maximum
// bounds are inclusive. PageSize 0 uses the service default.
type RewardMarketsParams struct {
	Query         string
	TagSlugs      []string
	MarketID      *int64
	EventIDs      []int64
	EventTitle    string
	OrderBy       RewardMarketSort
	SortDirection RewardMarketSortDirection
	MinVolume24hr *DecimalString
	MaxVolume24hr *DecimalString
	MinSpread     *DecimalString
	MaxSpread     *DecimalString
	MinPrice      *DecimalString
	MaxPrice      *DecimalString
	PageSize      int
	Cursor        string
}

// RebatedFee is a maker fee rebate for a calendar day, in USDC units.
type RebatedFee struct {
	Date            string        `json:"date"`
	ConditionID     string        `json:"condition_id"`
	AssetAddress    string        `json:"asset_address"`
	MakerAddress    string        `json:"maker_address"`
	RebatedFeesUSDC DecimalString `json:"rebated_fees_usdc"`
}

// UnmarshalJSON accepts numeric or string USDC amounts.
func (r *RebatedFee) UnmarshalJSON(data []byte) error {
	type alias RebatedFee
	return unmarshalResponseStrings(data, (*alias)(r), "rebated_fees_usdc")
}

// RebatesParams selects a maker's rebates for one calendar day.
type RebatesParams struct {
	Date         string
	MakerAddress string
}
