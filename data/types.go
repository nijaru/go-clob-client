package data

import (
	"time"

	"github.com/quagmt/udecimal"
)

type Decimal = udecimal.Decimal

type Side string

const (
	SideBuy  Side = "BUY"
	SideSell Side = "SELL"
)

type ActivityType string

const (
	ActivityTypeTrade          ActivityType = "TRADE"
	ActivityTypeSplit          ActivityType = "SPLIT"
	ActivityTypeMerge          ActivityType = "MERGE"
	ActivityTypeRedeem         ActivityType = "REDEEM"
	ActivityTypeReward         ActivityType = "REWARD"
	ActivityTypeConversion     ActivityType = "CONVERSION"
	ActivityTypeYield          ActivityType = "YIELD"
	ActivityTypeMakerRebate    ActivityType = "MAKER_REBATE"
	ActivityTypeReferralReward ActivityType = "REFERRAL_REWARD"
	ActivityTypeDeposit        ActivityType = "DEPOSIT"
	ActivityTypeWithdrawal     ActivityType = "WITHDRAWAL"
	ActivityTypeTakerRebate    ActivityType = "TAKER_REBATE"
)

type PositionSortBy string

const (
	PositionSortCurrent    PositionSortBy = "CURRENT"
	PositionSortInitial    PositionSortBy = "INITIAL"
	PositionSortTokens     PositionSortBy = "TOKENS"
	PositionSortCashPNL    PositionSortBy = "CASHPNL"
	PositionSortPercentPNL PositionSortBy = "PERCENTPNL"
	PositionSortTitle      PositionSortBy = "TITLE"
	PositionSortResolving  PositionSortBy = "RESOLVING"
	PositionSortPrice      PositionSortBy = "PRICE"
	PositionSortAvgPrice   PositionSortBy = "AVGPRICE"
)

type ClosedPositionSortBy string

const (
	ClosedPositionSortRealizedPNL ClosedPositionSortBy = "REALIZEDPNL"
	ClosedPositionSortTitle       ClosedPositionSortBy = "TITLE"
	ClosedPositionSortPrice       ClosedPositionSortBy = "PRICE"
	ClosedPositionSortAvgPrice    ClosedPositionSortBy = "AVGPRICE"
	ClosedPositionSortTimestamp   ClosedPositionSortBy = "TIMESTAMP"
)

type ActivitySortBy string

const (
	ActivitySortTimestamp ActivitySortBy = "TIMESTAMP"
	ActivitySortTokens    ActivitySortBy = "TOKENS"
	ActivitySortCash      ActivitySortBy = "CASH"
)

type SortDirection string

const (
	SortAsc  SortDirection = "ASC"
	SortDesc SortDirection = "DESC"
)

type FilterType string

const (
	FilterTypeCash   FilterType = "CASH"
	FilterTypeTokens FilterType = "TOKENS"
)

type TimePeriod string

const (
	TimePeriodDay   TimePeriod = "DAY"
	TimePeriodWeek  TimePeriod = "WEEK"
	TimePeriodMonth TimePeriod = "MONTH"
	TimePeriodAll   TimePeriod = "ALL"
)

type LeaderboardCategory string

const (
	LeaderboardCategoryOverall   LeaderboardCategory = "OVERALL"
	LeaderboardCategoryPolitics  LeaderboardCategory = "POLITICS"
	LeaderboardCategorySports    LeaderboardCategory = "SPORTS"
	LeaderboardCategoryCrypto    LeaderboardCategory = "CRYPTO"
	LeaderboardCategoryCulture   LeaderboardCategory = "CULTURE"
	LeaderboardCategoryMentions  LeaderboardCategory = "MENTIONS"
	LeaderboardCategoryWeather   LeaderboardCategory = "WEATHER"
	LeaderboardCategoryEconomics LeaderboardCategory = "ECONOMICS"
	LeaderboardCategoryTech      LeaderboardCategory = "TECH"
	LeaderboardCategoryFinance   LeaderboardCategory = "FINANCE"
)

type LeaderboardOrderBy string

const (
	LeaderboardOrderByPNL LeaderboardOrderBy = "PNL"
	LeaderboardOrderByVol LeaderboardOrderBy = "VOL"
)

type Position struct {
	ProxyWallet        string  `json:"proxyWallet"`
	Asset              string  `json:"asset"`
	ConditionID        string  `json:"conditionId"`
	Size               Decimal `json:"size"`
	AvgPrice           Decimal `json:"avgPrice"`
	InitialValue       Decimal `json:"initialValue"`
	CurrentValue       Decimal `json:"currentValue"`
	CashPNL            Decimal `json:"cashPnl"`
	PercentPNL         Decimal `json:"percentPnl"`
	TotalBought        Decimal `json:"totalBought"`
	RealizedPNL        Decimal `json:"realizedPnl"`
	PercentRealizedPNL Decimal `json:"percentRealizedPnl"`
	CurPrice           Decimal `json:"curPrice"`
	Redeemable         bool    `json:"redeemable"`
	Mergeable          bool    `json:"mergeable"`
	Title              string  `json:"title"`
	Slug               string  `json:"slug"`
	Icon               string  `json:"icon"`
	EventSlug          string  `json:"eventSlug"`
	EventID            string  `json:"eventId,omitzero"`
	Outcome            string  `json:"outcome"`
	OutcomeIndex       int     `json:"outcomeIndex"`
	OppositeOutcome    string  `json:"oppositeOutcome"`
	OppositeAsset      string  `json:"oppositeAsset"`
	EndDate            string  `json:"endDate,omitzero"`
	NegativeRisk       bool    `json:"negativeRisk"`
}

type ClosedPosition struct {
	ProxyWallet     string  `json:"proxyWallet"`
	Asset           string  `json:"asset"`
	ConditionID     string  `json:"conditionId"`
	AvgPrice        Decimal `json:"avgPrice"`
	TotalBought     Decimal `json:"totalBought"`
	RealizedPNL     Decimal `json:"realizedPnl"`
	CurPrice        Decimal `json:"curPrice"`
	Timestamp       int64   `json:"timestamp"`
	Title           string  `json:"title"`
	Slug            string  `json:"slug"`
	Icon            string  `json:"icon"`
	EventSlug       string  `json:"eventSlug"`
	Outcome         string  `json:"outcome"`
	OutcomeIndex    int     `json:"outcomeIndex"`
	OppositeOutcome string  `json:"oppositeOutcome"`
	OppositeAsset   string  `json:"oppositeAsset"`
	EndDate         string  `json:"endDate"`
}

type Health struct {
	Data string `json:"data"`
}

type Trade struct {
	ProxyWallet           string  `json:"proxyWallet"`
	Side                  Side    `json:"side"`
	Asset                 string  `json:"asset"`
	ConditionID           string  `json:"conditionId"`
	Size                  Decimal `json:"size"`
	Price                 Decimal `json:"price"`
	Timestamp             int64   `json:"timestamp"`
	Title                 string  `json:"title"`
	Slug                  string  `json:"slug"`
	Icon                  string  `json:"icon"`
	EventSlug             string  `json:"eventSlug"`
	Outcome               string  `json:"outcome"`
	OutcomeIndex          int     `json:"outcomeIndex"`
	Name                  string  `json:"name,omitzero"`
	Pseudonym             string  `json:"pseudonym,omitzero"`
	Bio                   string  `json:"bio,omitzero"`
	ProfileImage          string  `json:"profileImage,omitzero"`
	ProfileImageOptimized string  `json:"profileImageOptimized,omitzero"`
	TransactionHash       string  `json:"transactionHash"`
}

type TradeFilter struct {
	FilterType   FilterType `json:"filterType"`
	FilterAmount Decimal    `json:"filterAmount"`
}

type Activity struct {
	ProxyWallet           string       `json:"proxyWallet"`
	Timestamp             int64        `json:"timestamp"`
	ConditionID           string       `json:"conditionId,omitzero"`
	Type                  ActivityType `json:"type"`
	IsCombo               bool         `json:"isCombo,omitzero"`
	Size                  Decimal      `json:"size"`
	USDCSize              Decimal      `json:"usdcSize"`
	TransactionHash       string       `json:"transactionHash"`
	Price                 *Decimal     `json:"price,omitzero"`
	Asset                 string       `json:"asset,omitzero"`
	Side                  string       `json:"side,omitzero"`
	OutcomeIndex          *int         `json:"outcomeIndex,omitzero"`
	Title                 string       `json:"title,omitzero"`
	Slug                  string       `json:"slug,omitzero"`
	Icon                  string       `json:"icon,omitzero"`
	EventSlug             string       `json:"eventSlug,omitzero"`
	Outcome               string       `json:"outcome,omitzero"`
	Name                  string       `json:"name,omitzero"`
	Pseudonym             string       `json:"pseudonym,omitzero"`
	Bio                   string       `json:"bio,omitzero"`
	ProfileImage          string       `json:"profileImage,omitzero"`
	ProfileImageOptimized string       `json:"profileImageOptimized,omitzero"`
}

type Holder struct {
	ProxyWallet           string  `json:"proxyWallet"`
	Bio                   string  `json:"bio,omitzero"`
	Asset                 string  `json:"asset"`
	Pseudonym             string  `json:"pseudonym,omitzero"`
	Amount                Decimal `json:"amount"`
	DisplayUsernamePublic *bool   `json:"displayUsernamePublic,omitzero"`
	OutcomeIndex          int     `json:"outcomeIndex"`
	Name                  string  `json:"name,omitzero"`
	ProfileImage          string  `json:"profileImage,omitzero"`
	ProfileImageOptimized string  `json:"profileImageOptimized,omitzero"`
	Verified              *bool   `json:"verified,omitzero"`
}

type MetaHolder struct {
	Token   string   `json:"token"`
	Holders []Holder `json:"holders"`
}

type Traded struct {
	User   string `json:"user"`
	Traded int    `json:"traded"`
}

type Value struct {
	User  string  `json:"user"`
	Value Decimal `json:"value"`
}

type OpenInterest struct {
	Market string  `json:"market"`
	Value  Decimal `json:"value"`
}

type MarketVolume struct {
	Market string  `json:"market"`
	Value  Decimal `json:"value"`
}

type LiveVolume struct {
	Total   Decimal        `json:"total"`
	Markets []MarketVolume `json:"markets"`
}

type BuilderLeaderboardEntry struct {
	Rank        int     `json:"rank,string"`
	Builder     string  `json:"builder"`
	Volume      Decimal `json:"volume"`
	ActiveUsers int     `json:"activeUsers"`
	Verified    bool    `json:"verified"`
	BuilderLogo string  `json:"builderLogo,omitzero"`
}

type BuilderVolumeEntry struct {
	Timestamp   time.Time `json:"dt"`
	Builder     string    `json:"builder"`
	BuilderLogo string    `json:"builderLogo,omitzero"`
	Verified    bool      `json:"verified"`
	Volume      Decimal   `json:"volume"`
	ActiveUsers int       `json:"activeUsers"`
	Rank        int       `json:"rank,string"`
}

type TraderLeaderboardEntry struct {
	Rank         int     `json:"rank,string"`
	ProxyWallet  string  `json:"proxyWallet"`
	Username     string  `json:"userName,omitzero"`
	Volume       Decimal `json:"vol"`
	PNL          Decimal `json:"pnl"`
	ProfileImage string  `json:"profileImage,omitzero"`
	XUsername    string  `json:"xUsername,omitzero"`
	Verified     bool    `json:"verifiedBadge,omitzero"`
}

type PositionParams struct {
	User          string
	Filter        MarketFilter
	SizeThreshold string
	Redeemable    *bool
	Mergeable     *bool
	Limit         int
	Offset        int
	SortBy        PositionSortBy
	SortDirection SortDirection
	Title         string
}

type ClosedPositionParams struct {
	User          string
	Filter        MarketFilter
	Title         string
	Limit         int
	Offset        int
	SortBy        ClosedPositionSortBy
	SortDirection SortDirection
}

type TradeParams struct {
	User        string
	Filter      MarketFilter
	Limit       int
	Offset      int
	TakerOnly   *bool
	TradeFilter *TradeFilter
	Side        Side
}

type ActivityParams struct {
	User          string
	Filter        MarketFilter
	ActivityTypes []ActivityType
	Limit         int
	Offset        int
	Start         int64
	End           int64
	SortBy        ActivitySortBy
	SortDirection SortDirection
	Side          Side
}

type HoldersParams struct {
	Markets    []string
	Limit      int
	MinBalance int
}

type OpenInterestParams struct {
	Markets []string
}

type LeaderboardParams struct {
	Category   LeaderboardCategory
	TimePeriod TimePeriod
	SortBy     LeaderboardOrderBy
	Limit      int
	Offset     int
	User       string
	UserName   string
}

type BuilderLeaderboardParams struct {
	TimePeriod TimePeriod
	Limit      int
	Offset     int
}

type BuilderVolumeParams struct {
	TimePeriod TimePeriod
}

// MarketPositionDetail holds one wallet's position in a specific market.
type MarketPositionDetail struct {
	Wallet       string  `json:"proxyWallet,omitzero"`
	Name         string  `json:"name,omitzero"`
	ProfileImage string  `json:"profileImage,omitzero"`
	Verified     bool    `json:"verified,omitzero"`
	TokenID      string  `json:"asset,omitzero"`
	ConditionID  string  `json:"conditionId,omitzero"`
	AvgPrice     Decimal `json:"avgPrice,omitzero"`
	Size         Decimal `json:"size,omitzero"`
	CurPrice     Decimal `json:"currPrice,omitzero"`
	CurrentValue Decimal `json:"currentValue,omitzero"`
	CashPnl      Decimal `json:"cashPnl,omitzero"`
	TotalBought  Decimal `json:"totalBought,omitzero"`
	RealizedPnl  Decimal `json:"realizedPnl,omitzero"`
	TotalPnl     Decimal `json:"totalPnl,omitzero"`
	Outcome      string  `json:"outcome,omitzero"`
	OutcomeIndex int     `json:"outcomeIndex,omitzero"`
}

// MetaMarketPosition groups all wallet positions for a single token in a market.
type MetaMarketPosition struct {
	Token     string                 `json:"token,omitzero"`
	Positions []MarketPositionDetail `json:"positions,omitzero"`
}

// MarketPositionStatus filters market-position queries by lifecycle state.
type MarketPositionStatus string

const (
	MarketPositionStatusOpen   MarketPositionStatus = "open"
	MarketPositionStatusClosed MarketPositionStatus = "closed"
	MarketPositionStatusAll    MarketPositionStatus = "all"
)

// MarketPositionSortBy controls ordering in market-position queries.
type MarketPositionSortBy string

const (
	MarketPositionSortByCashPnl      MarketPositionSortBy = "cashPnl"
	MarketPositionSortByTotalPnl     MarketPositionSortBy = "totalPnl"
	MarketPositionSortBySize         MarketPositionSortBy = "size"
	MarketPositionSortByAvgPrice     MarketPositionSortBy = "avgPrice"
	MarketPositionSortByCurrentValue MarketPositionSortBy = "currentValue"
)

// MarketPositionParams filters the /v1/market-positions endpoint.
type MarketPositionParams struct {
	Market        string
	User          string
	Status        MarketPositionStatus
	SortBy        MarketPositionSortBy
	SortDirection SortDirection
	Limit         int
	Offset        int
}

// ComboPositionStatus represents the lifecycle state of a combo position.
type ComboPositionStatus string

const (
	ComboPositionStatusOpen            ComboPositionStatus = "OPEN"
	ComboPositionStatusPartial         ComboPositionStatus = "PARTIAL"
	ComboPositionStatusResolvedPartial ComboPositionStatus = "RESOLVED_PARTIAL"
	ComboPositionStatusResolvedWin     ComboPositionStatus = "RESOLVED_WIN"
	ComboPositionStatusResolvedLoss    ComboPositionStatus = "RESOLVED_LOSS"
)

// ComboPositionOutcome identifies the outcome represented by a combo position.
type ComboPositionOutcome string

const (
	ComboPositionOutcomeYes ComboPositionOutcome = "YES"
	ComboPositionOutcomeNo  ComboPositionOutcome = "NO"
)

// ComboPositionMarketEvent holds event metadata for a combo leg's market.
type ComboPositionMarketEvent struct {
	EventID    string `json:"event_id,omitzero"`
	EventSlug  string `json:"event_slug,omitzero"`
	EventTitle string `json:"event_title,omitzero"`
	EventImage string `json:"event_image,omitzero"`
}

// ComboPositionMarket holds market metadata for a combo leg.
type ComboPositionMarket struct {
	MarketID    string                    `json:"market_id,omitzero"`
	Slug        string                    `json:"slug,omitzero"`
	Title       string                    `json:"title,omitzero"`
	Outcome     string                    `json:"outcome,omitzero"`
	ImageURL    string                    `json:"image_url,omitzero"`
	IconURL     string                    `json:"icon_url,omitzero"`
	Category    string                    `json:"category,omitzero"`
	Subcategory string                    `json:"subcategory,omitzero"`
	Tags        []string                  `json:"tags,omitzero"`
	EndDate     string                    `json:"end_date,omitzero"`
	Event       *ComboPositionMarketEvent `json:"event,omitzero"`
}

// ComboPositionLeg represents one leg of a combo position.
type ComboPositionLeg struct {
	LegIndex        int                  `json:"leg_index"`
	LegPositionID   string               `json:"leg_position_id"`
	LegConditionID  string               `json:"leg_condition_id"`
	LegOutcomeIndex int                  `json:"leg_outcome_index"`
	LegOutcomeLabel string               `json:"leg_outcome_label,omitzero"`
	LegStatus       ComboPositionStatus  `json:"leg_status"`
	LegResolvedAt   string               `json:"leg_resolved_at,omitzero"`
	LegCurrentPrice *Decimal             `json:"leg_current_price,omitzero"`
	Market          *ComboPositionMarket `json:"market,omitzero"`
}

// ComboPosition represents a multi-leg combo position.
type ComboPosition struct {
	ConditionID        string               `json:"combo_condition_id"`
	PositionID         string               `json:"combo_position_id"`
	Outcome            ComboPositionOutcome `json:"side"`
	ModuleID           int                  `json:"module_id"`
	UserAddress        string               `json:"user_address"`
	Shares             Decimal              `json:"shares_balance"`
	EntryAvgPriceUsdc  *Decimal             `json:"entry_avg_price_usdc,omitzero"`
	EntryCostUsdc      *Decimal             `json:"entry_cost_usdc,omitzero"`
	RealizedPayoutUsdc *Decimal             `json:"realized_payout_usdc,omitzero"`
	TotalCostUsdc      *Decimal             `json:"total_cost_usdc,omitzero"`
	Status             ComboPositionStatus  `json:"status"`
	Redeemable         bool                 `json:"redeemable"`
	FirstEntryAt       string               `json:"first_entry_at"`
	ResolvedAt         string               `json:"resolved_at,omitzero"`
	UpdatedAt          string               `json:"updated_at,omitzero"`
	LegsTotal          int                  `json:"legs_total"`
	LegsResolved       int                  `json:"legs_resolved"`
	LegsPending        int                  `json:"legs_pending"`
	Legs               []ComboPositionLeg   `json:"legs"`
}

// ComboPositionPage is one cursor-paginated combo-position response.
type ComboPositionPage struct {
	Items      []ComboPosition
	Limit      int
	Offset     int
	HasMore    bool
	NextCursor string
}

// ComboPositionParams filters the /v1/positions/combos endpoint.
type ComboPositionParams struct {
	User          string
	Status        ComboPositionStatus
	Sort          ComboPositionSort
	ConditionID   string
	ConditionIDs  []string
	PositionID    string // Deprecated: the current official filter is condition-based.
	UpdatedAfter  int64
	UpdatedBefore int64
	Limit         int
	Cursor        string
	Offset        int // Deprecated: the current API uses Cursor pagination.
}

// ComboPositionSort controls combo-position ordering.
type ComboPositionSort string

const (
	ComboPositionSortCurrentValueDesc ComboPositionSort = "current_value_desc"
	ComboPositionSortFirstEntryDesc   ComboPositionSort = "first_entry_desc"
	ComboPositionSortEntryCostDesc    ComboPositionSort = "entry_cost_desc"
	ComboPositionSortResolvedAtDesc   ComboPositionSort = "resolved_at_desc"
	ComboPositionSortUpdatedAsc       ComboPositionSort = "updated_asc"
)

// ComboActivityType identifies a combo lifecycle event.
type ComboActivityType string

const (
	ComboActivityTypeSplit    ComboActivityType = "SPLIT"
	ComboActivityTypeMerge    ComboActivityType = "MERGE"
	ComboActivityTypeConvert  ComboActivityType = "CONVERT"
	ComboActivityTypeCompress ComboActivityType = "COMPRESS"
	ComboActivityTypeWrap     ComboActivityType = "WRAP"
	ComboActivityTypeUnwrap   ComboActivityType = "UNWRAP"
	ComboActivityTypeRedeem   ComboActivityType = "REDEEM"
)

// ComboActivity is one combo lifecycle event from the Data API.
type ComboActivity struct {
	ID              string             `json:"id"`
	Type            ComboActivityType  `json:"type"`
	UserAddress     string             `json:"user_address"`
	ConditionID     string             `json:"combo_condition_id"`
	PositionID      string             `json:"combo_position_id"`
	ModuleID        int                `json:"module_id"`
	AmountUsdc      *Decimal           `json:"amount_usdc"`
	PayoutUsdc      *Decimal           `json:"payout_usdc"`
	Timestamp       int64              `json:"timestamp"`
	TransactionAt   string             `json:"tx_dttm"`
	TransactionHash string             `json:"tx_hash"`
	LogIndex        int                `json:"log_index"`
	BlockNumber     int                `json:"block_number"`
	Legs            []ComboPositionLeg `json:"legs"`
}

// ComboActivityPage is one cursor-paginated combo-activity response.
type ComboActivityPage struct {
	Items      []ComboActivity
	Limit      int
	Offset     int
	HasMore    bool
	NextCursor string
}

// ComboActivityParams filters combo lifecycle activity.
type ComboActivityParams struct {
	User         string
	ConditionID  string
	ConditionIDs []string
	Limit        int
	Cursor       string
}
