package gamma

import "time"

// Market represents a single Polymarket market outcome.
type Market struct {
	ID                              string               `json:"id"`
	Question                        string               `json:"question"`
	ConditionID                     string               `json:"conditionId"`
	Slug                            string               `json:"slug"`
	TwitterCardImage                string               `json:"twitterCardImage,omitzero"`
	ResolutionSource                string               `json:"resolutionSource,omitzero"`
	EndDate                         string               `json:"endDate,omitzero"`
	Category                        string               `json:"category,omitzero"`
	AmmType                         string               `json:"ammType,omitzero"`
	Liquidity                       string               `json:"liquidity,omitzero"`
	SponsorName                     string               `json:"sponsorName,omitzero"`
	SponsorImage                    string               `json:"sponsorImage,omitzero"`
	BestBid                         string               `json:"bestBid,omitzero"`
	BestAsk                         string               `json:"bestAsk,omitzero"`
	LastTradePrice                  string               `json:"lastTradePrice,omitzero"`
	Volume                          string               `json:"volume,omitzero"`
	Volume24h                       string               `json:"volume24h,omitzero"`
	OutcomePrices                   []string             `json:"outcomePrices,omitzero"`
	Outcomes                        []string             `json:"outcomes,omitzero"`
	CLOBTokenIDs                    []string             `json:"clobTokenIds,omitzero"`
	DescriptivePricing              []string             `json:"descriptivePricing,omitzero"`
	DayPercentChange                float64              `json:"dayPercentChange,omitzero"`
	ResolutionRules                 string               `json:"resolutionRules,omitzero"`
	Description                     string               `json:"description,omitzero"`
	MarketType                      string               `json:"marketType,omitzero"`
	Active                          bool                 `json:"active"`
	Closed                          bool                 `json:"closed"`
	Archived                        bool                 `json:"archived"`
	Resolved                        bool                 `json:"resolved"`
	Restricted                      bool                 `json:"restricted"`
	GroupWinner                     bool                 `json:"groupWinner"`
	Tracking                        bool                 `json:"tracking"`
	Hedge                           bool                 `json:"hedge"`
	OneToTwo                        bool                 `json:"oneToTwo"`
	Ready                           bool                 `json:"ready"`
	AcceptingOrders                 bool                 `json:"acceptingOrders"`
	NegativeRisk                    bool                 `json:"negativeRisk"`
	NegRiskMarketID                 string               `json:"negRiskMarketId,omitzero"`
	NegRiskRequestID                string               `json:"negRiskRequestId,omitzero"`
	ProxyAddress                    string               `json:"proxyAddress,omitzero"`
	OrderPriceMinTickSize           float64              `json:"orderPriceMinTickSize,omitzero"`
	OrderMinSize                    float64              `json:"orderMinSize,omitzero"`
	MaxOrderSize                    float64              `json:"maxOrderSize,omitzero"`
	RewardsMinSize                  float64              `json:"rewardsMinSize,omitzero"`
	RewardsMaxSpread                float64              `json:"rewardsMaxSpread,omitzero"`
	Spread                          float64              `json:"spread,omitzero"`
	GqlID                           string               `json:"gqlId,omitzero"`
	EventID                         string               `json:"eventId,omitzero"`
	CreatedAt                       time.Time            `json:"createdAt"`
	UpdatedAt                       time.Time            `json:"updatedAt"`
	Competitive                     float64              `json:"competitive,omitzero"`
	PagerDutyService                string               `json:"pagerDutyService,omitzero"`
	ApproveCurrentWorker            string               `json:"approveCurrentWorker,omitzero"`
	ResolutionServiceWorker         string               `json:"resolutionServiceWorker,omitzero"`
	Fee                             string               `json:"fee,omitzero"`
	Fpmm                            string               `json:"fpmm,omitzero"`
	OutcomeAssets                   []string             `json:"outcomeAssets,omitzero"`
	QuoterAddress                   string               `json:"quoterAddress,omitzero"`
	MinimumOrderSize                float64              `json:"minimumOrderSize,omitzero"`
	MinimumBaseWithdrawalAmount     float64              `json:"minimumBaseWithdrawalAmount,omitzero"`
}

// Event represents a collection of markets.
type Event struct {
	ID                    string               `json:"id"`
	Ticker                string               `json:"ticker"`
	Slug                  string               `json:"slug"`
	Title                 string               `json:"title"`
	Description           string               `json:"description,omitzero"`
	ResolutionSource      string               `json:"resolutionSource,omitzero"`
	ResolutionRules       string               `json:"resolutionRules,omitzero"`
	Liquidity             float64              `json:"liquidity,omitzero"`
	Volume                float64              `json:"volume,omitzero"`
	Volume24h             float64              `json:"volume24h,omitzero"`
	StartDate             *time.Time           `json:"startDate,omitzero"`
	EndDate               *time.Time           `json:"endDate,omitzero"`
	Closed                bool                 `json:"closed"`
	Archived              bool                 `json:"archived"`
	Resolved              bool                 `json:"resolved"`
	Restricted            bool                 `json:"restricted"`
	Category              string               `json:"category,omitzero"`
	Icon                  string               `json:"icon,omitzero"`
	Image                 string               `json:"image,omitzero"`
	Banner                string               `json:"banner,omitzero"`
	Markets               []Market             `json:"markets,omitzero"`
	Tags                  []Tag                `json:"tags,omitzero"`
	Competitive           float64              `json:"competitive,omitzero"`
	CommentCount          int                  `json:"commentCount,omitzero"`
	NegativeRisk          bool                 `json:"negativeRisk"`
	NegRiskMarketID       string               `json:"negRiskMarketId,omitzero"`
	CreatedAt             time.Time            `json:"createdAt"`
	UpdatedAt             time.Time            `json:"updatedAt"`
}

// Tag represents metadata categorization.
type Tag struct {
	ID          string    `json:"id"`
	Label       string    `json:"label"`
	Slug        string    `json:"slug"`
	ForceShow   bool      `json:"forceShow"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Series represents a grouping of related events.
type Series struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Slug        string    `json:"slug"`
	Description string    `json:"description,omitzero"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Sport represents sports-specific metadata.
type Sport struct {
	ID          string    `json:"id"`
	Label       string    `json:"label"`
	Slug        string    `json:"slug"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Team represents a sports team.
type Team struct {
	ID          string    `json:"id"`
	Label       string    `json:"label"`
	Slug        string    `json:"slug"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// MarketType represents a valid sports market type.
type MarketType struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Comment represents a user comment.
type Comment struct {
	ID           string    `json:"id"`
	Comment      string    `json:"comment"`
	UserAddress  string    `json:"userAddress"`
	ConditionID  string    `json:"conditionId"`
	ParentID     string    `json:"parentId,omitzero"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// PublicProfile represents a user's public metadata.
type PublicProfile struct {
	Address               string  `json:"address"`
	Name                  string  `json:"name,omitzero"`
	Bio                   string  `json:"bio,omitzero"`
	ProfileImage          string  `json:"profileImage,omitzero"`
	ProfileImageOptimized string  `json:"profileImageOptimized,omitzero"`
	ProxyWallet           string  `json:"proxyWallet,omitzero"`
	Pseudonym             string  `json:"pseudonym,omitzero"`
	Verified              bool    `json:"verified"`
}

// MarketFilterParams defines filters for market listing.
type MarketFilterParams struct {
	Active          *bool   `url:"active,omitzero"`
	Closed          *bool   `url:"closed,omitzero"`
	Archived        *bool   `url:"archived,omitzero"`
	Resolved        *bool   `url:"resolved,omitzero"`
	Limit           int     `url:"limit,omitzero"`
	Offset          int     `url:"offset,omitzero"`
	Order           string  `url:"order,omitzero"`
	Ascending       *bool   `url:"ascending,omitzero"`
	TagID           string  `url:"tag_id,omitzero"`
	EventID         string  `url:"event_id,omitzero"`
	Slug            string  `url:"slug,omitzero"`
	NegativeRisk    *bool   `url:"negative_risk,omitzero"`
	AcceptingOrders *bool   `url:"accepting_orders,omitzero"`
}

// EventFilterParams defines filters for event listing.
type EventFilterParams struct {
	Active       *bool   `url:"active,omitzero"`
	Closed       *bool   `url:"closed,omitzero"`
	Archived     *bool   `url:"archived,omitzero"`
	Resolved     *bool   `url:"resolved,omitzero"`
	TagID        string  `url:"tag_id,omitzero"`
	Slug         string  `url:"slug,omitzero"`
	Limit        int     `url:"limit,omitzero"`
	Offset       int     `url:"offset,omitzero"`
	NegativeRisk *bool   `url:"negative_risk,omitzero"`
}

// CommentFilterParams defines filters for comments.
type CommentFilterParams struct {
	ConditionID string `url:"condition_id,omitzero"`
	Limit       int    `url:"limit,omitzero"`
	Offset      int    `url:"offset,omitzero"`
}
