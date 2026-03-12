package clob

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
	ConditionID      string                `json:"condition_id"`
	Question         string                `json:"question"`
	MarketSlug       string                `json:"market_slug"`
	EventSlug        string                `json:"event_slug"`
	Image            string                `json:"image"`
	RewardsMaxSpread string                `json:"rewards_max_spread"`
	RewardsMinSize   string                `json:"rewards_min_size"`
	Tokens           []OutcomeToken        `json:"tokens"`
	RewardsConfig    []MarketRewardsConfig `json:"rewards_config"`
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
