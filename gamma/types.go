package gamma

import "time"

// Market represents a single Polymarket market outcome.
type Market struct {
	ID                          string    `json:"id"`
	Question                    string    `json:"question,omitzero"`
	ConditionID                 string    `json:"conditionId,omitzero"`
	Slug                        string    `json:"slug,omitzero"`
	TwitterCardImage            string    `json:"twitterCardImage,omitzero"`
	ResolutionSource            string    `json:"resolutionSource,omitzero"`
	EndDate                     string    `json:"endDate,omitzero"`
	Category                    string    `json:"category,omitzero"`
	AmmType                     string    `json:"ammType,omitzero"`
	Liquidity                   string    `json:"liquidity,omitzero"`
	SponsorName                 string    `json:"sponsorName,omitzero"`
	SponsorImage                string    `json:"sponsorImage,omitzero"`
	BestBid                     string    `json:"bestBid,omitzero"`
	BestAsk                     string    `json:"bestAsk,omitzero"`
	LastTradePrice              string    `json:"lastTradePrice,omitzero"`
	Volume                      string    `json:"volume,omitzero"`
	Volume24h                   string    `json:"volume24h,omitzero"`
	OutcomePrices               []string  `json:"outcomePrices,omitzero"`
	Outcomes                    []string  `json:"outcomes,omitzero"`
	CLOBTokenIDs                []string  `json:"clobTokenIds,omitzero"`
	DescriptivePricing          []string  `json:"descriptivePricing,omitzero"`
	DayPercentChange            float64   `json:"dayPercentChange,omitzero"`
	ResolutionRules             string    `json:"resolutionRules,omitzero"`
	Description                 string    `json:"description,omitzero"`
	MarketType                  string    `json:"marketType,omitzero"`
	Active                      bool      `json:"active"`
	Closed                      bool      `json:"closed"`
	Archived                    bool      `json:"archived"`
	Resolved                    bool      `json:"resolved"`
	Restricted                  bool      `json:"restricted"`
	GroupWinner                 bool      `json:"groupWinner"`
	Tracking                    bool      `json:"tracking"`
	Hedge                       bool      `json:"hedge"`
	OneToTwo                    bool      `json:"oneToTwo"`
	Ready                       bool      `json:"ready"`
	AcceptingOrders             bool      `json:"acceptingOrders"`
	NegativeRisk                bool      `json:"negativeRisk"`
	NegRiskMarketID             string    `json:"negRiskMarketId,omitzero"`
	NegRiskRequestID            string    `json:"negRiskRequestId,omitzero"`
	ProxyAddress                string    `json:"proxyAddress,omitzero"`
	OrderPriceMinTickSize       float64   `json:"orderPriceMinTickSize,omitzero"`
	OrderMinSize                float64   `json:"orderMinSize,omitzero"`
	MaxOrderSize                float64   `json:"maxOrderSize,omitzero"`
	RewardsMinSize              float64   `json:"rewardsMinSize,omitzero"`
	RewardsMaxSpread            float64   `json:"rewardsMaxSpread,omitzero"`
	Spread                      float64   `json:"spread,omitzero"`
	GqlID                       string    `json:"gqlId,omitzero"`
	EventID                     string    `json:"eventId,omitzero"`
	CreatedAt                   time.Time `json:"createdAt"`
	UpdatedAt                   time.Time `json:"updatedAt"`
	Competitive                 float64   `json:"competitive,omitzero"`
	PagerDutyService            string    `json:"pagerDutyService,omitzero"`
	ApproveCurrentWorker        string    `json:"approveCurrentWorker,omitzero"`
	ResolutionServiceWorker     string    `json:"resolutionServiceWorker,omitzero"`
	Fee                         string    `json:"fee,omitzero"`
	Fpmm                        string    `json:"fpmm,omitzero"`
	OutcomeAssets               []string  `json:"outcomeAssets,omitzero"`
	QuoterAddress               string    `json:"quoterAddress,omitzero"`
	MinimumOrderSize            float64   `json:"minimumOrderSize,omitzero"`
	MinimumBaseWithdrawalAmount float64   `json:"minimumBaseWithdrawalAmount,omitzero"`

	// Extended fields for parity
	FormatType              string       `json:"formatType,omitzero"`
	LowerBound              string       `json:"lowerBound,omitzero"`
	UpperBound              string       `json:"upperBound,omitzero"`
	MarketGroup             int          `json:"marketGroup,omitzero"`
	GroupItemTitle          string       `json:"groupItemTitle,omitzero"`
	GroupItemThreshold      string       `json:"groupItemThreshold,omitzero"`
	QuestionID              string       `json:"questionID,omitzero"`
	UmaEndDate              string       `json:"umaEndDate,omitzero"`
	UmaResolutionStatus     string       `json:"umaResolutionStatus,omitzero"`
	VolumeNum               string       `json:"volumeNum,omitzero"`
	LiquidityNum            string       `json:"liquidityNum,omitzero"`
	SecondsDelay            int          `json:"secondsDelay,omitzero"`
	TeamAID                 string       `json:"teamAID,omitzero"`
	TeamBID                 string       `json:"teamBID,omitzero"`
	UmaBond                 string       `json:"umaBond,omitzero"`
	UmaReward               string       `json:"umaReward,omitzero"`
	Volume24hAmm            string       `json:"volume24hrAmm,omitzero"`
	Volume1wkAmm            string       `json:"volume1wkAmm,omitzero"`
	Volume1moAmm            string       `json:"volume1moAmm,omitzero"`
	Volume1yrAmm            string       `json:"volume1yrAmm,omitzero"`
	Volume24hClob           string       `json:"volume24hrClob,omitzero"`
	Volume1wkClob           string       `json:"volume1wkClob,omitzero"`
	Volume1moClob           string       `json:"volume1moClob,omitzero"`
	Volume1yrClob           string       `json:"volume1yrClob,omitzero"`
	VolumeAmm               string       `json:"volumeAmm,omitzero"`
	VolumeClob              string       `json:"volumeClob,omitzero"`
	LiquidityAmm            string       `json:"liquidityAmm,omitzero"`
	LiquidityClob           string       `json:"liquidityClob,omitzero"`
	MakerBaseFee            int          `json:"makerBaseFee,omitzero"`
	TakerBaseFee            int          `json:"takerBaseFee,omitzero"`
	MakerRebatesFeeShareBps int          `json:"makerRebatesFeeShareBps,omitzero"`
	CustomLiveness          int          `json:"customLiveness,omitzero"`
	NotificationsEnabled    bool         `json:"notificationsEnabled"`
	ClearBookOnStart        bool         `json:"clearBookOnStart"`
	ChartColor              string       `json:"chartColor,omitzero"`
	SeriesColor             string       `json:"seriesColor,omitzero"`
	ShowGmpSeries           bool         `json:"showGmpSeries"`
	ShowGmpOutcome          bool         `json:"showGmpOutcome"`
	ManualActivation        bool         `json:"manualActivation"`
	NegRiskOther            bool         `json:"negRiskOther"`
	RfqEnabled              bool         `json:"rfqEnabled"`
	HoldingRewardsEnabled   bool         `json:"holdingRewardsEnabled"`
	ClobRewards             []ClobReward `json:"clobRewards,omitzero"`
}

// ClobReward represents CLOB rewards configuration for a market.
type ClobReward struct {
	ID               string `json:"id,omitzero"`
	AssetAddress     string `json:"assetAddress,omitzero"`
	ConditionID      string `json:"conditionId,omitzero"`
	StartDate        string `json:"startDate,omitzero"`
	EndDate          string `json:"endDate,omitzero"`
	RewardsAmount    string `json:"rewardsAmount,omitzero"`
	RewardsDailyRate string `json:"rewardsDailyRate,omitzero"`
}

// Event represents a collection of markets.
type Event struct {
	ID               string     `json:"id"`
	Ticker           string     `json:"ticker,omitzero"`
	Slug             string     `json:"slug,omitzero"`
	Title            string     `json:"title,omitzero"`
	Subtitle         string     `json:"subtitle,omitzero"`
	Description      string     `json:"description,omitzero"`
	ResolutionSource string     `json:"resolutionSource,omitzero"`
	ResolutionRules  string     `json:"resolutionRules,omitzero"`
	Liquidity        float64    `json:"liquidity,omitzero"`
	Volume           float64    `json:"volume,omitzero"`
	Volume24h        float64    `json:"volume24h,omitzero"`
	Volume1wk        float64    `json:"volume1wk,omitzero"`
	Volume1mo        float64    `json:"volume1mo,omitzero"`
	Volume1yr        float64    `json:"volume1yr,omitzero"`
	StartDate        *time.Time `json:"startDate,omitzero"`
	EndDate          *time.Time `json:"endDate,omitzero"`
	CreationDate     *time.Time `json:"creationDate,omitzero"`
	Closed           bool       `json:"closed"`
	Archived         bool       `json:"archived"`
	Resolved         bool       `json:"resolved"`
	Restricted       bool       `json:"restricted"`
	Category         string     `json:"category,omitzero"`
	Subcategory      string     `json:"subcategory,omitzero"`
	Icon             string     `json:"icon,omitzero"`
	Image            string     `json:"image,omitzero"`
	Banner           string     `json:"banner,omitzero"`
	Markets          []Market   `json:"markets,omitzero"`
	Tags             []Tag      `json:"tags,omitzero"`
	Competitive      float64    `json:"competitive,omitzero"`
	CommentCount     int        `json:"commentCount,omitzero"`
	CommentsEnabled  bool       `json:"commentsEnabled"`
	NegativeRisk     bool       `json:"negativeRisk"`
	NegRiskMarketID  string     `json:"negRiskMarketId,omitzero"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`

	// Extended fields
	Featured          bool         `json:"featured"`
	New               bool         `json:"new"`
	SortBy            string       `json:"sortBy,omitzero"`
	IsTemplate        bool         `json:"isTemplate"`
	TemplateVariables string       `json:"templateVariables,omitzero"`
	CreatedBy         string       `json:"createdBy,omitzero"`
	UpdatedBy         string       `json:"updatedBy,omitzero"`
	OpenInterest      float64      `json:"openInterest,omitzero"`
	LiquidityAmm      float64      `json:"liquidityAmm,omitzero"`
	LiquidityClob     float64      `json:"liquidityClob,omitzero"`
	NegRiskFeeBips    int          `json:"negRiskFeeBips,omitzero"`
	SubEvents         []string     `json:"subEvents,omitzero"`
	Series            []Series     `json:"series,omitzero"`
	Categories        []Category   `json:"categories,omitzero"`
	Collections       []Collection `json:"collections,omitzero"`
}

// Tag represents metadata categorization.
type Tag struct {
	ID          string    `json:"id"`
	Label       string    `json:"label,omitzero"`
	Slug        string    `json:"slug,omitzero"`
	ForceShow   bool      `json:"forceShow"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	PublishedAt string    `json:"publishedAt,omitzero"`
	CreatedBy   int       `json:"createdBy,omitzero"`
	UpdatedBy   int       `json:"updatedBy,omitzero"`
	ForceHide   bool      `json:"forceHide"`
	IsCarousel  bool      `json:"isCarousel"`
}

// RelatedTag represents a relationship between tags.
type RelatedTag struct {
	ID           string `json:"id"`
	TagID        string `json:"tagID,omitzero"`
	RelatedTagID string `json:"relatedTagID,omitzero"`
	Rank         int    `json:"rank,omitzero"`
}

// Category represents a grouping for tags/events.
type Category struct {
	ID             string    `json:"id"`
	Label          string    `json:"label,omitzero"`
	ParentCategory string    `json:"parentCategory,omitzero"`
	Slug           string    `json:"slug,omitzero"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Collection represents a high-level event grouping.
type Collection struct {
	ID             string    `json:"id"`
	Ticker         string    `json:"ticker,omitzero"`
	Slug           string    `json:"slug,omitzero"`
	Title          string    `json:"title,omitzero"`
	CollectionType string    `json:"collectionType,omitzero"`
	Description    string    `json:"description,omitzero"`
	Active         bool      `json:"active"`
	Closed         bool      `json:"closed"`
	Archived       bool      `json:"archived"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Series represents a grouping of related events.
type Series struct {
	ID          string    `json:"id"`
	Title       string    `json:"title,omitzero"`
	Slug        string    `json:"slug,omitzero"`
	Description string    `json:"description,omitzero"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Ticker      string    `json:"ticker,omitzero"`
	SeriesType  string    `json:"seriesType,omitzero"`
	Recurrence  string    `json:"recurrence,omitzero"`
	Active      bool      `json:"active"`
	Closed      bool      `json:"closed"`
}

// Sport represents sports-specific metadata.
type Sport struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	Slug      string    `json:"slug"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Team represents a sports team.
type Team struct {
	ID           int       `json:"id"`
	Name         string    `json:"name,omitzero"`
	League       string    `json:"league,omitzero"`
	Record       string    `json:"record,omitzero"`
	Logo         string    `json:"logo,omitzero"`
	Abbreviation string    `json:"abbreviation,omitzero"`
	Alias        string    `json:"alias,omitzero"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// MarketType represents a valid sports market type.
type MarketType struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// Comment represents a user comment.
type Comment struct {
	ID               string         `json:"id"`
	Comment          string         `json:"comment"`
	UserAddress      string         `json:"userAddress"`
	ConditionID      string         `json:"conditionId,omitzero"`
	ParentID         string         `json:"parentId,omitzero"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	Profile          CommentProfile `json:"profile,omitzero"`
	Reactions        []Reaction     `json:"reactions,omitzero"`
	ReportCount      int            `json:"reportCount,omitzero"`
	ReactionCount    int            `json:"reactionCount,omitzero"`
	ParentCommentID  string         `json:"parentCommentID,omitzero"`
	ParentEntityType string         `json:"parentEntityType,omitzero"`
	ParentEntityID   int            `json:"parentEntityID,omitzero"`
}

// CommentProfile contains author information for a comment.
type CommentProfile struct {
	Name                  string            `json:"name,omitzero"`
	Pseudonym             string            `json:"pseudonym,omitzero"`
	DisplayUsernamePublic bool              `json:"displayUsernamePublic"`
	Bio                   string            `json:"bio,omitzero"`
	IsMod                 bool              `json:"isMod"`
	IsCreator             bool              `json:"isCreator"`
	ProxyWallet           string            `json:"proxyWallet,omitzero"`
	BaseAddress           string            `json:"baseAddress,omitzero"`
	ProfileImage          string            `json:"profileImage,omitzero"`
	Positions             []CommentPosition `json:"positions,omitzero"`
}

// CommentPosition represents a user's position relative to a comment.
type CommentPosition struct {
	TokenID      string `json:"tokenID,omitzero"`
	PositionSize string `json:"positionSize,omitzero"`
}

// Reaction represents a reaction to a comment.
type Reaction struct {
	ID           string         `json:"id"`
	CommentID    int            `json:"commentID,omitzero"`
	ReactionType string         `json:"reactionType,omitzero"`
	Icon         string         `json:"icon,omitzero"`
	UserAddress  string         `json:"userAddress,omitzero"`
	CreatedAt    time.Time      `json:"createdAt"`
	Profile      CommentProfile `json:"profile,omitzero"`
}

// PublicProfile represents a user's public metadata.
type PublicProfile struct {
	Address               string              `json:"address"`
	Name                  string              `json:"name,omitzero"`
	Bio                   string              `json:"bio,omitzero"`
	ProfileImage          string              `json:"profileImage,omitzero"`
	ProfileImageOptimized string              `json:"profileImageOptimized,omitzero"`
	ProxyWallet           string              `json:"proxyWallet,omitzero"`
	Pseudonym             string              `json:"pseudonym,omitzero"`
	Verified              bool                `json:"verified"`
	CreatedAt             time.Time           `json:"createdAt"`
	Users                 []PublicProfileUser `json:"users,omitzero"`
	XUsername             string              `json:"xUsername,omitzero"`
	VerifiedBadge         bool                `json:"verifiedBadge"`
}

// PublicProfileUser represents a user record in a public profile.
type PublicProfileUser struct {
	ID      string `json:"id"`
	Creator bool   `json:"creator"`
	Mod     bool   `json:"mod"`
}

// MarketFilterParams defines filters for market listing.
type MarketFilterParams struct {
	Active          *bool  `url:"active,omitzero"`
	Closed          *bool  `url:"closed,omitzero"`
	Archived        *bool  `url:"archived,omitzero"`
	Resolved        *bool  `url:"resolved,omitzero"`
	Limit           int    `url:"limit,omitzero"`
	Offset          int    `url:"offset,omitzero"`
	Order           string `url:"order,omitzero"`
	Ascending       *bool  `url:"ascending,omitzero"`
	TagID           string `url:"tag_id,omitzero"`
	EventID         string `url:"event_id,omitzero"`
	Slug            string `url:"slug,omitzero"`
	NegativeRisk    *bool  `url:"negative_risk,omitzero"`
	AcceptingOrders *bool  `url:"accepting_orders,omitzero"`
}

// EventFilterParams defines filters for event listing.
type EventFilterParams struct {
	Active       *bool  `url:"active,omitzero"`
	Closed       *bool  `url:"closed,omitzero"`
	Archived     *bool  `url:"archived,omitzero"`
	Resolved     *bool  `url:"resolved,omitzero"`
	TagID        string `url:"tag_id,omitzero"`
	Slug         string `url:"slug,omitzero"`
	Limit        int    `url:"limit,omitzero"`
	Offset       int    `url:"offset,omitzero"`
	NegativeRisk *bool  `url:"negative_risk,omitzero"`
}

// CommentFilterParams defines filters for comments.
type CommentFilterParams struct {
	ConditionID string `url:"condition_id,omitzero"`
	Limit       int    `url:"limit,omitzero"`
	Offset      int    `url:"offset,omitzero"`
}
