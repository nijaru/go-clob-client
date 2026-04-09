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
