package clob

import (
	stdjson "encoding/json" //nolint:depguard // numeric reward wire normalization
	"fmt"
	"strconv"
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

// TotalUserEarning is a daily aggregate user earnings row.
type TotalUserEarning struct {
	Date         string `json:"date"`
	AssetAddress string `json:"asset_address"`
	MakerAddress string `json:"maker_address"`
	Earnings     string `json:"earnings"`
	AssetRate    string `json:"asset_rate"`
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

// UnmarshalJSON accepts numeric or string total_days values from the rewards API.
func (r *MarketRewardsConfig) UnmarshalJSON(data []byte) error {
	var fields map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("market rewards config: decode object: %w", err)
	}
	if raw, ok := fields["total_days"]; ok {
		value, err := decodeStringOrNumber(raw)
		if err != nil {
			return fmt.Errorf("market rewards config total_days: %w", err)
		}
		fields["total_days"] = stdjson.RawMessage(strconv.Quote(value))
	}
	normalized, err := stdjson.Marshal(fields)
	if err != nil {
		return fmt.Errorf("market rewards config: encode object: %w", err)
	}
	type alias MarketRewardsConfig
	return stdjson.Unmarshal(normalized, (*alias)(r))
}

// Earning is an asset-specific earnings breakdown.
type Earning struct {
	AssetAddress string `json:"asset_address"`
	Earnings     string `json:"earnings"`
	AssetRate    string `json:"asset_rate"`
}

// CurrentReward is the current rewards summary for a market.
type CurrentReward struct {
	ConditionID      string          `json:"condition_id"`
	RewardsConfig    []RewardsConfig `json:"rewards_config"`
	RewardsMaxSpread string          `json:"rewards_max_spread"`
	RewardsMinSize   string          `json:"rewards_min_size"`
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
	normalized, err := normalizeRewardStrings(data,
		"rewards_max_spread",
		"rewards_min_size",
		"market_competitiveness",
	)
	if err != nil {
		return err
	}
	type alias MarketReward
	return stdjson.Unmarshal(normalized, (*alias)(r))
}

func normalizeRewardStrings(data []byte, keys ...string) ([]byte, error) {
	var fields map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(data, &fields); err != nil {
		return nil, fmt.Errorf("rewards: decode object: %w", err)
	}
	for _, key := range keys {
		if raw, ok := fields[key]; ok {
			value, err := decodeStringOrNumber(raw)
			if err != nil {
				return nil, fmt.Errorf("rewards %s: %w", key, err)
			}
			fields[key] = stdjson.RawMessage(strconv.Quote(value))
		}
	}
	return stdjson.Marshal(fields)
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
