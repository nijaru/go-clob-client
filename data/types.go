package data

import "time"

// Position represents an open market position.
type Position struct {
	ProxyWallet        string    `json:"proxyWallet"`
	Asset              string    `json:"asset"`
	ConditionID        string    `json:"conditionId"`
	Size               string    `json:"size"`
	AvgPrice           string    `json:"avgPrice"`
	InitialValue       string    `json:"initialValue"`
	CurrentValue       string    `json:"currentValue"`
	CashPNL            string    `json:"cashPnl"`
	PercentPNL         string    `json:"percentPnl"`
	TotalBought        string    `json:"totalBought"`
	RealizedPNL        string    `json:"realizedPnl"`
	PercentRealizedPNL string    `json:"percentRealizedPnl"`
	CurPrice           string    `json:"curPrice"`
	Redeemable         bool      `json:"redeemable"`
	Mergeable          bool      `json:"mergeable"`
	Title              string    `json:"title"`
	Slug               string    `json:"slug"`
	Icon               string    `json:"icon"`
	EventSlug          string    `json:"eventSlug"`
	EventID            string    `json:"eventId,omitzero"`
	Outcome            string    `json:"outcome"`
	OutcomeIndex       int       `json:"outcomeIndex"`
	OppositeOutcome    string    `json:"oppositeOutcome"`
	OppositeAsset      string    `json:"oppositeAsset"`
	EndDate            string    `json:"endDate,omitzero"`
	NegativeRisk       bool      `json:"negativeRisk"`
}

// ClosedPosition represents a historical market position.
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

// DataTrade represents a trade record from the Data API.
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

// Activity represents an on-chain activity record.
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

// Holder represents a single token holder.
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

// MetaHolder groups holders by token.
type MetaHolder struct {
	Token   string   `json:"token"`
	Holders []Holder `json:"holders"`
}

// Traded reports the unique markets count for a user.
type Traded struct {
	User   string `json:"user"`
	Traded int    `json:"traded"`
}

// OpenInterest reports open interest for a market.
type OpenInterest struct {
	Market string `json:"market"`
	Value  string `json:"value"`
}

// MarketVolume reports volume for a specific market.
type MarketVolume struct {
	Market string `json:"market"`
	Value  string `json:"value"`
}

// LiveVolume reports live trading volume for an event.
type LiveVolume struct {
	Total   string         `json:"total"`
	Markets []MarketVolume `json:"markets"`
}

// BuilderLeaderboardEntry represents a builder's performance.
type BuilderLeaderboardEntry struct {
	Rank        int    `json:"rank,string"`
	Builder     string `json:"builder"`
	Volume      string `json:"volume"`
	ActiveUsers int    `json:"activeUsers"`
	Verified    bool   `json:"verified"`
	BuilderLogo string `json:"builderLogo,omitzero"`
}

// BuilderVolumeEntry represents daily builder volume.
type BuilderVolumeEntry struct {
	Timestamp   time.Time `json:"dt"`
	Builder     string    `json:"builder"`
	BuilderLogo string    `json:"builderLogo,omitzero"`
	Verified    bool      `json:"verified"`
	Volume      string    `json:"volume"`
	ActiveUsers int       `json:"activeUsers"`
	Rank        int       `json:"rank,string"`
}

// TraderLeaderboardEntry represents a trader's performance.
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

// LeaderboardParams filters trader leaderboard requests.
type LeaderboardParams struct {
	Category   string `url:"category,omitzero"`
	TimePeriod string `url:"timePeriod,omitzero"`
	SortBy     string `url:"orderBy,omitzero"`
	Limit      int    `url:"limit,omitzero"`
	Offset     int    `url:"offset,omitzero"`
	User       string `url:"user,omitzero"`
	UserName   string `url:"userName,omitzero"`
}

// TradeParams filters trade history requests.
type TradeParams struct {
	User      string   `url:"user,omitzero"`
	Markets   []string `url:"market,omitzero"`
	EventIDs  []string `url:"eventID,omitzero"`
	Limit     int      `url:"limit,omitzero"`
	Offset    int      `url:"offset,omitzero"`
	TakerOnly *bool    `url:"takerOnly,omitzero"`
	Side      string   `url:"side,omitzero"`
}

// ActivityParams filters activity history requests.
type ActivityParams struct {
	User          string   `url:"user"`
	Markets       []string `url:"market,omitzero"`
	EventIDs      []string `url:"eventID,omitzero"`
	ActivityTypes []string `url:"type,omitzero"`
	Limit         int      `url:"limit,omitzero"`
	Offset        int      `url:"offset,omitzero"`
	Start         int64    `url:"start,omitzero"`
	End           int64    `url:"end,omitzero"`
	SortBy        string   `url:"sortBy,omitzero"`
	SortDirection string   `url:"sortDirection,omitzero"`
	Side          string   `url:"side,omitzero"`
}

// PositionParams filters current position requests.
type PositionParams struct {
	User          string   `url:"user"`
	Markets       []string `url:"market,omitzero"`
	EventIDs      []string `url:"eventID,omitzero"`
	SizeThreshold string   `url:"sizeThreshold,omitzero"`
	Redeemable    *bool    `url:"redeemable,omitzero"`
	Mergeable     *bool    `url:"mergeable,omitzero"`
	Limit         int      `url:"limit,omitzero"`
	Offset        int      `url:"offset,omitzero"`
	SortBy        string   `url:"sortBy,omitzero"`
	SortDirection string   `url:"sortDirection,omitzero"`
	Title         string   `url:"title,omitzero"`
}

// ClosedPositionParams filters closed position requests.
type ClosedPositionParams struct {
	User          string   `url:"user"`
	Markets       []string `url:"market,omitzero"`
	EventIDs      []string `url:"eventID,omitzero"`
	Title         string   `url:"title,omitzero"`
	Limit         int      `url:"limit,omitzero"`
	Offset        int      `url:"offset,omitzero"`
	SortBy        string   `url:"sortBy,omitzero"`
	SortDirection string   `url:"sortDirection,omitzero"`
}

// HoldersParams filters market holder requests.
type HoldersParams struct {
	Markets    []string `url:"market"`
	Limit      int      `url:"limit,omitzero"`
	MinBalance int      `url:"minBalance,omitzero"`
}

// OpenInterestParams filters open interest requests.
type OpenInterestParams struct {
	Markets []string `url:"market,omitzero"`
}

// BuilderLeaderboardParams filters builder leaderboard requests.
type BuilderLeaderboardParams struct {
	TimePeriod string `url:"timePeriod,omitzero"`
	Limit      int    `url:"limit,omitzero"`
	Offset     int    `url:"offset,omitzero"`
}

// BuilderVolumeParams filters builder volume requests.
type BuilderVolumeParams struct {
	TimePeriod string `url:"timePeriod,omitzero"`
}

type positionValueResponse struct {
	Value string `json:"value"`
}
