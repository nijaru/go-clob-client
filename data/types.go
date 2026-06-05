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
	ActivityTypeTrade       ActivityType = "TRADE"
	ActivityTypeSplit       ActivityType = "SPLIT"
	ActivityTypeMerge       ActivityType = "MERGE"
	ActivityTypeRedeem      ActivityType = "REDEEM"
	ActivityTypeReward      ActivityType = "REWARD"
	ActivityTypeConversion  ActivityType = "CONVERSION"
	ActivityTypeYield       ActivityType = "YIELD"
	ActivityTypeMakerRebate ActivityType = "MAKERREBATE"
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
	ComboPositionStatusOpen         ComboPositionStatus = "OPEN"
	ComboPositionStatusPartial      ComboPositionStatus = "PARTIAL"
	ComboPositionStatusResolvedWin  ComboPositionStatus = "RESOLVED_WIN"
	ComboPositionStatusResolvedLoss ComboPositionStatus = "RESOLVED_LOSS"
)

// ComboPositionMarketEvent holds event metadata for a combo leg's market.
type ComboPositionMarketEvent struct {
	EventID    string `json:"eventId,omitzero"`
	EventSlug  string `json:"eventSlug,omitzero"`
	EventTitle string `json:"eventTitle,omitzero"`
	EventImage string `json:"eventImage,omitzero"`
}

// ComboPositionMarket holds market metadata for a combo leg.
type ComboPositionMarket struct {
	MarketID    string                    `json:"marketId,omitzero"`
	Slug        string                    `json:"slug,omitzero"`
	Title       string                    `json:"title,omitzero"`
	Outcome     string                    `json:"outcome,omitzero"`
	ImageURL    string                    `json:"imageUrl,omitzero"`
	IconURL     string                    `json:"iconUrl,omitzero"`
	Category    string                    `json:"category,omitzero"`
	Subcategory string                    `json:"subcategory,omitzero"`
	Tags        []string                  `json:"tags,omitzero"`
	EndDate     string                    `json:"endDate,omitzero"`
	Event       *ComboPositionMarketEvent `json:"event,omitzero"`
}

// ComboPositionLeg represents one leg of a combo position.
type ComboPositionLeg struct {
	LegIndex        int                  `json:"legIndex"`
	LegPositionID   string               `json:"legPositionId"`
	LegConditionID  string               `json:"legConditionId"`
	LegOutcomeIndex int                  `json:"legOutcomeIndex"`
	LegOutcomeLabel string               `json:"legOutcomeLabel,omitzero"`
	LegStatus       ComboPositionStatus  `json:"legStatus"`
	LegResolvedAt   string               `json:"legResolvedAt,omitzero"`
	LegCurrentPrice *Decimal             `json:"legCurrentPrice,omitzero"`
	Market          *ComboPositionMarket `json:"market,omitzero"`
}

// ComboPosition represents a multi-leg combo position.
type ComboPosition struct {
	ConditionID       string              `json:"conditionId"`
	PositionID        string              `json:"positionId"`
	ModuleID          int                 `json:"moduleId"`
	UserAddress       string              `json:"userAddress"`
	Shares            Decimal             `json:"shares"`
	EntryAvgPriceUsdc *Decimal            `json:"entryAvgPriceUsdc,omitzero"`
	EntryCostUsdc     *Decimal            `json:"entryCostUsdc,omitzero"`
	Status            ComboPositionStatus `json:"status"`
	FirstEntryAt      string              `json:"firstEntryAt"`
	ResolvedAt        string              `json:"resolvedAt,omitzero"`
	LegsTotal         int                 `json:"legsTotal"`
	LegsResolved      int                 `json:"legsResolved"`
	LegsPending       int                 `json:"legsPending"`
	Legs              []ComboPositionLeg  `json:"legs"`
}

// ComboPositionParams filters the /v1/positions/combos endpoint.
type ComboPositionParams struct {
	User        string
	Status      ComboPositionStatus
	ConditionID string
	PositionID  string
	Limit       int
	Offset      int
}
