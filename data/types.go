package data

import "time"

type Position struct {
	ProxyWallet        string `json:"proxyWallet"`
	Asset              string `json:"asset"`
	ConditionID        string `json:"conditionId"`
	Size               string `json:"size"`
	AvgPrice           string `json:"avgPrice"`
	InitialValue       string `json:"initialValue"`
	CurrentValue       string `json:"currentValue"`
	CashPNL            string `json:"cashPnl"`
	PercentPNL         string `json:"percentPnl"`
	TotalBought        string `json:"totalBought"`
	RealizedPNL        string `json:"realizedPnl"`
	PercentRealizedPNL string `json:"percentRealizedPnl"`
	CurPrice           string `json:"curPrice"`
	Redeemable         bool   `json:"redeemable"`
	Mergeable          bool   `json:"mergeable"`
	Title              string `json:"title"`
	Slug               string `json:"slug"`
	Icon               string `json:"icon"`
	EventSlug          string `json:"eventSlug"`
	EventID            string `json:"eventId,omitzero"`
	Outcome            string `json:"outcome"`
	OutcomeIndex       int    `json:"outcomeIndex"`
	OppositeOutcome    string `json:"oppositeOutcome"`
	OppositeAsset      string `json:"oppositeAsset"`
	EndDate            string `json:"endDate,omitzero"`
	NegativeRisk       bool   `json:"negativeRisk"`
}

type ClosedPosition struct {
	ProxyWallet     string `json:"proxyWallet"`
	Asset           string `json:"asset"`
	ConditionID     string `json:"conditionId"`
	AvgPrice        string `json:"avgPrice"`
	TotalBought     string `json:"totalBought"`
	RealizedPNL     string `json:"realizedPnl"`
	CurPrice        string `json:"curPrice"`
	Timestamp       int64  `json:"timestamp"`
	Title           string `json:"title"`
	Slug            string `json:"slug"`
	Icon            string `json:"icon"`
	EventSlug       string `json:"eventSlug"`
	Outcome         string `json:"outcome"`
	OutcomeIndex    int    `json:"outcomeIndex"`
	OppositeOutcome string `json:"oppositeOutcome"`
	OppositeAsset   string `json:"oppositeAsset"`
	EndDate         string `json:"endDate"`
}

type Health struct {
	Data string `json:"data"`
}

type DataTrade struct {
	ProxyWallet           string `json:"proxyWallet"`
	Side                  string `json:"side"`
	Asset                 string `json:"asset"`
	ConditionID           string `json:"conditionId"`
	Size                  string `json:"size"`
	Price                 string `json:"price"`
	Timestamp             int64  `json:"timestamp"`
	Title                 string `json:"title"`
	Slug                  string `json:"slug"`
	Icon                  string `json:"icon"`
	EventSlug             string `json:"eventSlug"`
	Outcome               string `json:"outcome"`
	OutcomeIndex          int    `json:"outcomeIndex"`
	Name                  string `json:"name,omitzero"`
	Pseudonym             string `json:"pseudonym,omitzero"`
	Bio                   string `json:"bio,omitzero"`
	ProfileImage          string `json:"profileImage,omitzero"`
	ProfileImageOptimized string `json:"profileImageOptimized,omitzero"`
	TransactionHash       string `json:"transactionHash"`
}

type FilterType string

const (
	FilterTypeCash   FilterType = "CASH"
	FilterTypeTokens FilterType = "TOKENS"
)

type TradeFilter struct {
	FilterType   FilterType `json:"filterType"`
	FilterAmount string     `json:"filterAmount"`
}

type Activity struct {
	ProxyWallet           string `json:"proxyWallet"`
	Timestamp             int64  `json:"timestamp"`
	ConditionID           string `json:"conditionId,omitzero"`
	Type                  string `json:"type"`
	Size                  string `json:"size"`
	USDCSize              string `json:"usdcSize"`
	TransactionHash       string `json:"transactionHash"`
	Price                 string `json:"price,omitzero"`
	Asset                 string `json:"asset,omitzero"`
	Side                  string `json:"side,omitzero"`
	OutcomeIndex          *int   `json:"outcomeIndex,omitzero"`
	Title                 string `json:"title,omitzero"`
	Slug                  string `json:"slug,omitzero"`
	Icon                  string `json:"icon,omitzero"`
	EventSlug             string `json:"eventSlug,omitzero"`
	Outcome               string `json:"outcome,omitzero"`
	Name                  string `json:"name,omitzero"`
	Pseudonym             string `json:"pseudonym,omitzero"`
	Bio                   string `json:"bio,omitzero"`
	ProfileImage          string `json:"profileImage,omitzero"`
	ProfileImageOptimized string `json:"profileImageOptimized,omitzero"`
}

type Holder struct {
	ProxyWallet           string `json:"proxyWallet"`
	Bio                   string `json:"bio,omitzero"`
	Asset                 string `json:"asset"`
	Pseudonym             string `json:"pseudonym,omitzero"`
	Amount                string `json:"amount"`
	DisplayUsernamePublic *bool  `json:"displayUsernamePublic,omitzero"`
	OutcomeIndex          int    `json:"outcomeIndex"`
	Name                  string `json:"name,omitzero"`
	ProfileImage          string `json:"profileImage,omitzero"`
	ProfileImageOptimized string `json:"profileImageOptimized,omitzero"`
	Verified              *bool  `json:"verified,omitzero"`
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
	User  string `json:"user"`
	Value string `json:"value"`
}

type OpenInterest struct {
	Market string `json:"market"`
	Value  string `json:"value"`
}

type MarketVolume struct {
	Market string `json:"market"`
	Value  string `json:"value"`
}

type LiveVolume struct {
	Total   string         `json:"total"`
	Markets []MarketVolume `json:"markets"`
}

type BuilderLeaderboardEntry struct {
	Rank        int    `json:"rank,string"`
	Builder     string `json:"builder"`
	Volume      string `json:"volume"`
	ActiveUsers int    `json:"activeUsers"`
	Verified    bool   `json:"verified"`
	BuilderLogo string `json:"builderLogo,omitzero"`
}

type BuilderVolumeEntry struct {
	Timestamp   time.Time `json:"dt"`
	Builder     string    `json:"builder"`
	BuilderLogo string    `json:"builderLogo,omitzero"`
	Verified    bool      `json:"verified"`
	Volume      string    `json:"volume"`
	ActiveUsers int       `json:"activeUsers"`
	Rank        int       `json:"rank,string"`
}

type TraderLeaderboardEntry struct {
	Rank         int    `json:"rank,string"`
	ProxyWallet  string `json:"proxyWallet"`
	Username     string `json:"userName,omitzero"`
	Volume       string `json:"vol"`
	PNL          string `json:"pnl"`
	ProfileImage string `json:"profileImage,omitzero"`
	XUsername    string `json:"xUsername,omitzero"`
	Verified     bool   `json:"verifiedBadge,omitzero"`
}

type PositionParams struct {
	User          string
	Filter        MarketFilter
	SizeThreshold string
	Redeemable    *bool
	Mergeable     *bool
	Limit         int
	Offset        int
	SortBy        string
	SortDirection string
	Title         string
}

type ClosedPositionParams struct {
	User          string
	Filter        MarketFilter
	Title         string
	Limit         int
	Offset        int
	SortBy        string
	SortDirection string
}

type TradeParams struct {
	User        string
	Filter      MarketFilter
	Limit       int
	Offset      int
	TakerOnly   *bool
	TradeFilter *TradeFilter
	Side        string
}

type ActivityParams struct {
	User          string
	Filter        MarketFilter
	ActivityTypes []string
	Limit         int
	Offset        int
	Start         int64
	End           int64
	SortBy        string
	SortDirection string
	Side          string
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
	Category   string
	TimePeriod string
	SortBy     string
	Limit      int
	Offset     int
	User       string
	UserName   string
}

type BuilderLeaderboardParams struct {
	TimePeriod string
	Limit      int
	Offset     int
}

type BuilderVolumeParams struct {
	TimePeriod string
}
