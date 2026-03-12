package clob

type HealthResponse string

type Market struct {
	EnableOrderBook      bool           `json:"enable_order_book"`
	Active               bool           `json:"active"`
	Closed               bool           `json:"closed"`
	Archived             bool           `json:"archived"`
	AcceptingOrders      bool           `json:"accepting_orders"`
	AcceptingOrderTime   *string        `json:"accepting_order_timestamp"`
	MinimumOrderSize     string         `json:"minimum_order_size"`
	MinimumTickSize      string         `json:"minimum_tick_size"`
	ConditionID          *string        `json:"condition_id"`
	QuestionID           *string        `json:"question_id"`
	Question             string         `json:"question"`
	Description          string         `json:"description"`
	MarketSlug           string         `json:"market_slug"`
	EndDateISO           *string        `json:"end_date_iso"`
	GameStartTime        *string        `json:"game_start_time"`
	SecondsDelay         int64          `json:"seconds_delay"`
	FPMM                 *string        `json:"fpmm"`
	MakerBaseFee         string         `json:"maker_base_fee"`
	TakerBaseFee         string         `json:"taker_base_fee"`
	NotificationsEnabled bool           `json:"notifications_enabled"`
	NegRisk              bool           `json:"neg_risk"`
	NegRiskMarketID      *string        `json:"neg_risk_market_id"`
	NegRiskRequestID     *string        `json:"neg_risk_request_id"`
	Icon                 string         `json:"icon"`
	Image                string         `json:"image"`
	Rewards              Rewards        `json:"rewards"`
	IsFiftyFiftyOutcome  bool           `json:"is_50_50_outcome"`
	Tokens               []OutcomeToken `json:"tokens"`
	Tags                 []string       `json:"tags"`
}

type SimplifiedMarket struct {
	ConditionID     *string        `json:"condition_id"`
	Tokens          []OutcomeToken `json:"tokens"`
	Rewards         Rewards        `json:"rewards"`
	Active          bool           `json:"active"`
	Closed          bool           `json:"closed"`
	Archived        bool           `json:"archived"`
	AcceptingOrders bool           `json:"accepting_orders"`
}

type OutcomeToken struct {
	TokenID string `json:"token_id"`
	Outcome string `json:"outcome"`
	Price   string `json:"price"`
	Winner  bool   `json:"winner"`
}

type RewardRate struct {
	AssetAddress     string `json:"asset_address"`
	RewardsDailyRate string `json:"rewards_daily_rate"`
}

type Rewards struct {
	Rates     []RewardRate `json:"rates"`
	MinSize   string       `json:"min_size"`
	MaxSpread string       `json:"max_spread"`
}

type MarketPrice struct {
	T int64   `json:"t"`
	P float64 `json:"p"`
}

type PriceHistoryInterval string

const (
	PriceHistoryIntervalMax      PriceHistoryInterval = "max"
	PriceHistoryIntervalOneWeek  PriceHistoryInterval = "1w"
	PriceHistoryIntervalOneDay   PriceHistoryInterval = "1d"
	PriceHistoryIntervalSixHours PriceHistoryInterval = "6h"
	PriceHistoryIntervalOneHour  PriceHistoryInterval = "1h"
)

type PriceHistoryFilterParams struct {
	Market   string
	StartTs  int64
	EndTs    int64
	Fidelity int
	Interval PriceHistoryInterval
}

type MarketTradeEvent struct {
	EventType       string                 `json:"event_type"`
	Market          MarketTradeEventMarket `json:"market"`
	User            MarketTradeEventUser   `json:"user"`
	Side            Side                   `json:"side"`
	Size            string                 `json:"size"`
	FeeRateBps      string                 `json:"fee_rate_bps"`
	Price           string                 `json:"price"`
	Outcome         string                 `json:"outcome"`
	OutcomeIndex    int                    `json:"outcome_index"`
	TransactionHash string                 `json:"transaction_hash"`
	Timestamp       string                 `json:"timestamp"`
}

type MarketTradeEventMarket struct {
	ConditionID string `json:"condition_id"`
	AssetID     string `json:"asset_id"`
	Question    string `json:"question"`
	Icon        string `json:"icon"`
	Slug        string `json:"slug"`
}

type MarketTradeEventUser struct {
	Address                 string `json:"address"`
	Username                string `json:"username"`
	ProfilePicture          string `json:"profile_picture"`
	OptimizedProfilePicture string `json:"optimized_profile_picture"`
	Pseudonym               string `json:"pseudonym"`
}

type UserEarning struct {
	Date         string `json:"date"`
	ConditionID  string `json:"condition_id"`
	AssetAddress string `json:"asset_address"`
	MakerAddress string `json:"maker_address"`
	Earnings     string `json:"earnings"`
	AssetRate    string `json:"asset_rate"`
}

type TotalUserEarning struct {
	Date         string `json:"date"`
	AssetAddress string `json:"asset_address"`
	MakerAddress string `json:"maker_address"`
	Earnings     string `json:"earnings"`
	AssetRate    string `json:"asset_rate"`
}

type RewardsPercentages map[string]string

type RewardsConfig struct {
	AssetAddress string `json:"asset_address"`
	StartDate    string `json:"start_date"`
	EndDate      string `json:"end_date"`
	RatePerDay   string `json:"rate_per_day"`
	TotalRewards string `json:"total_rewards"`
}

type MarketRewardsConfig struct {
	ID           string `json:"id"`
	AssetAddress string `json:"asset_address"`
	StartDate    string `json:"start_date"`
	EndDate      string `json:"end_date"`
	RatePerDay   string `json:"rate_per_day"`
	TotalRewards string `json:"total_rewards"`
	TotalDays    string `json:"total_days"`
}

type Earning struct {
	AssetAddress string `json:"asset_address"`
	Earnings     string `json:"earnings"`
	AssetRate    string `json:"asset_rate"`
}

type CurrentReward struct {
	ConditionID      string          `json:"condition_id"`
	RewardsConfig    []RewardsConfig `json:"rewards_config"`
	RewardsMaxSpread string          `json:"rewards_max_spread"`
	RewardsMinSize   string          `json:"rewards_min_size"`
}

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
