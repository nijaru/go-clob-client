package gamma

import (
	"encoding/json"
	"time"
)

// Market is Gamma market metadata. Outcomes, prices and asset IDs retain wire order.
type Market struct {
	ID                          string          `json:"id"`
	Version                     ProtocolVersion `json:"version,omitzero"`
	ComboStatus                 ComboStatus     `json:"comboStatus,omitzero"`
	Question                    string          `json:"question,omitzero"`
	ConditionID                 string          `json:"conditionId,omitzero"`
	Slug                        string          `json:"slug,omitzero"`
	TwitterCardImage            string          `json:"twitterCardImage,omitzero"`
	ResolutionSource            string          `json:"resolutionSource,omitzero"`
	EndDate                     string          `json:"endDate,omitzero"`
	Category                    string          `json:"category,omitzero"`
	AmmType                     string          `json:"ammType,omitzero"`
	Liquidity                   Decimal         `json:"liquidity,omitzero"`
	SponsorName                 string          `json:"sponsorName,omitzero"`
	SponsorImage                string          `json:"sponsorImage,omitzero"`
	BestBid                     Decimal         `json:"bestBid,omitzero"`
	BestAsk                     Decimal         `json:"bestAsk,omitzero"`
	LastTradePrice              Decimal         `json:"lastTradePrice,omitzero"`
	Volume                      Decimal         `json:"volume,omitzero"`
	Volume24h                   Decimal         `json:"volume24hr,omitzero"`
	OutcomePrices               []string        `json:"outcomePrices,omitzero"`
	Outcomes                    []string        `json:"outcomes,omitzero"`
	CLOBTokenIDs                []string        `json:"clobTokenIds,omitzero"`
	DescriptivePricing          []string        `json:"descriptivePricing,omitzero"`
	DayPercentChange            Decimal         `json:"dayPercentChange,omitzero"`
	ResolutionRules             string          `json:"resolutionRules,omitzero"`
	Description                 string          `json:"description,omitzero"`
	MarketType                  string          `json:"marketType,omitzero"`
	Active                      bool            `json:"active"`
	Closed                      bool            `json:"closed"`
	Archived                    bool            `json:"archived"`
	Resolved                    bool            `json:"resolved"`
	Restricted                  bool            `json:"restricted"`
	GroupWinner                 bool            `json:"groupWinner"`
	Tracking                    bool            `json:"tracking"`
	Hedge                       bool            `json:"hedge"`
	OneToTwo                    bool            `json:"oneToTwo"`
	Ready                       bool            `json:"ready"`
	AcceptingOrders             bool            `json:"acceptingOrders"`
	NegativeRisk                bool            `json:"negRisk"`
	NegRiskMarketID             string          `json:"negRiskMarketID,omitzero"`
	NegRiskRequestID            string          `json:"negRiskRequestID,omitzero"`
	ProxyAddress                string          `json:"proxyAddress,omitzero"`
	OrderPriceMinTickSize       Decimal         `json:"orderPriceMinTickSize,omitzero"`
	OrderMinSize                Decimal         `json:"orderMinSize,omitzero"`
	MaxOrderSize                Decimal         `json:"maxOrderSize,omitzero"`
	RewardsMinSize              Decimal         `json:"rewardsMinSize,omitzero"`
	RewardsMaxSpread            Decimal         `json:"rewardsMaxSpread,omitzero"`
	Spread                      Decimal         `json:"spread,omitzero"`
	GqlID                       string          `json:"gqlId,omitzero"`
	EventID                     string          `json:"eventId,omitzero"`
	CreatedAt                   time.Time       `json:"createdAt"`
	UpdatedAt                   time.Time       `json:"updatedAt"`
	Competitive                 Decimal         `json:"competitive,omitzero"`
	PagerDutyService            string          `json:"pagerDutyService,omitzero"`
	ApproveCurrentWorker        string          `json:"approveCurrentWorker,omitzero"`
	ResolutionServiceWorker     string          `json:"resolutionServiceWorker,omitzero"`
	Fee                         Decimal         `json:"fee,omitzero"`
	Fpmm                        string          `json:"fpmm,omitzero"`
	OutcomeAssets               []string        `json:"outcomeAssets,omitzero"`
	QuoterAddress               string          `json:"quoterAddress,omitzero"`
	MinimumOrderSize            Decimal         `json:"minimumOrderSize,omitzero"`
	MinimumBaseWithdrawalAmount Decimal         `json:"minimumBaseWithdrawalAmount,omitzero"`

	FormatType                   string            `json:"formatType,omitzero"`
	LowerBound                   string            `json:"lowerBound,omitzero"`
	UpperBound                   string            `json:"upperBound,omitzero"`
	MarketGroup                  int               `json:"marketGroup,omitzero"`
	GroupItemTitle               string            `json:"groupItemTitle,omitzero"`
	GroupItemThreshold           string            `json:"groupItemThreshold,omitzero"`
	PositionIDs                  []string          `json:"positionIds,omitzero"`
	QuestionID                   string            `json:"questionID,omitzero"`
	UmaEndDate                   string            `json:"umaEndDate,omitzero"`
	UmaResolutionStatus          string            `json:"umaResolutionStatus,omitzero"`
	VolumeNum                    Decimal           `json:"volumeNum,omitzero"`
	LiquidityNum                 Decimal           `json:"liquidityNum,omitzero"`
	SecondsDelay                 int               `json:"secondsDelay,omitzero"`
	TeamAID                      string            `json:"teamAID,omitzero"`
	TeamBID                      string            `json:"teamBID,omitzero"`
	UmaBond                      string            `json:"umaBond,omitzero"`
	UmaReward                    Decimal           `json:"umaReward,omitzero"`
	Volume24hAmm                 Decimal           `json:"volume24hrAmm,omitzero"`
	Volume1wkAmm                 Decimal           `json:"volume1wkAmm,omitzero"`
	Volume1moAmm                 Decimal           `json:"volume1moAmm,omitzero"`
	Volume1yrAmm                 Decimal           `json:"volume1yrAmm,omitzero"`
	Volume24hClob                Decimal           `json:"volume24hrClob,omitzero"`
	Volume1wkClob                Decimal           `json:"volume1wkClob,omitzero"`
	Volume1moClob                Decimal           `json:"volume1moClob,omitzero"`
	Volume1yrClob                Decimal           `json:"volume1yrClob,omitzero"`
	VolumeAmm                    Decimal           `json:"volumeAmm,omitzero"`
	VolumeClob                   Decimal           `json:"volumeClob,omitzero"`
	LiquidityAmm                 Decimal           `json:"liquidityAmm,omitzero"`
	LiquidityClob                Decimal           `json:"liquidityClob,omitzero"`
	MakerBaseFee                 int               `json:"makerBaseFee,omitzero"`
	TakerBaseFee                 int               `json:"takerBaseFee,omitzero"`
	MakerRebatesFeeShareBps      int               `json:"makerRebatesFeeShareBps,omitzero"`
	CustomLiveness               int               `json:"customLiveness,omitzero"`
	NotificationsEnabled         bool              `json:"notificationsEnabled"`
	ClearBookOnStart             bool              `json:"clearBookOnStart"`
	ChartColor                   string            `json:"chartColor,omitzero"`
	SeriesColor                  string            `json:"seriesColor,omitzero"`
	ShowGmpSeries                bool              `json:"showGmpSeries"`
	ShowGmpOutcome               bool              `json:"showGmpOutcome"`
	ManualActivation             bool              `json:"manualActivation"`
	NegRiskOther                 bool              `json:"negRiskOther"`
	RfqEnabled                   bool              `json:"rfqEnabled"`
	HoldingRewardsEnabled        bool              `json:"holdingRewardsEnabled"`
	ClobRewards                  []ClobReward      `json:"clobRewards,omitzero"`
	StartDate                    *time.Time        `json:"startDate,omitzero"`
	XAxisValue                   string            `json:"xAxisValue,omitzero"`
	YAxisValue                   string            `json:"yAxisValue,omitzero"`
	DenominationToken            string            `json:"denominationToken,omitzero"`
	Image                        string            `json:"image,omitzero"`
	Icon                         string            `json:"icon,omitzero"`
	LowerBoundDate               string            `json:"lowerBoundDate,omitzero"`
	UpperBoundDate               string            `json:"upperBoundDate,omitzero"`
	MarketMakerAddress           string            `json:"marketMakerAddress,omitzero"`
	CreatedBy                    int               `json:"createdBy,omitzero"`
	UpdatedBy                    int               `json:"updatedBy,omitzero"`
	ClosedTime                   string            `json:"closedTime,omitzero"`
	WideFormat                   bool              `json:"wideFormat,omitzero"`
	New                          bool              `json:"new,omitzero"`
	MailchimpTag                 string            `json:"mailchimpTag,omitzero"`
	Featured                     bool              `json:"featured,omitzero"`
	ResolvedBy                   string            `json:"resolvedBy,omitzero"`
	EnableOrderBook              bool              `json:"enableOrderBook,omitzero"`
	CurationOrder                int               `json:"curationOrder,omitzero"`
	EndDateIso                   string            `json:"endDateIso,omitzero"`
	StartDateIso                 string            `json:"startDateIso,omitzero"`
	UmaEndDateIso                string            `json:"umaEndDateIso,omitzero"`
	HasReviewedDates             bool              `json:"hasReviewedDates,omitzero"`
	ReadyForCron                 bool              `json:"readyForCron,omitzero"`
	CommentsEnabled              bool              `json:"commentsEnabled,omitzero"`
	Volume1wk                    Decimal           `json:"volume1wk,omitzero"`
	Volume1mo                    Decimal           `json:"volume1mo,omitzero"`
	Volume1yr                    Decimal           `json:"volume1yr,omitzero"`
	GameStartTime                string            `json:"gameStartTime,omitzero"`
	DisqusThread                 string            `json:"disqusThread,omitzero"`
	ShortOutcomes                string            `json:"shortOutcomes,omitzero"`
	FpmmLive                     bool              `json:"fpmmLive,omitzero"`
	Score                        int               `json:"score,omitzero"`
	ImageOptimized               ImageOptimization `json:"imageOptimized,omitzero"`
	IconOptimized                ImageOptimization `json:"iconOptimized,omitzero"`
	Events                       []Event           `json:"events,omitzero"`
	Categories                   []Category        `json:"categories,omitzero"`
	Tags                         []Tag             `json:"tags,omitzero"`
	Creator                      string            `json:"creator,omitzero"`
	Funded                       bool              `json:"funded,omitzero"`
	PastSlugs                    string            `json:"pastSlugs,omitzero"`
	ReadyTimestamp               *time.Time        `json:"readyTimestamp,omitzero"`
	FundedTimestamp              *time.Time        `json:"fundedTimestamp,omitzero"`
	AcceptingOrdersTimestamp     *time.Time        `json:"acceptingOrdersTimestamp,omitzero"`
	AutomaticallyResolved        bool              `json:"automaticallyResolved,omitzero"`
	OneDayPriceChange            Decimal           `json:"oneDayPriceChange,omitzero"`
	OneHourPriceChange           Decimal           `json:"oneHourPriceChange,omitzero"`
	OneWeekPriceChange           Decimal           `json:"oneWeekPriceChange,omitzero"`
	OneMonthPriceChange          Decimal           `json:"oneMonthPriceChange,omitzero"`
	OneYearPriceChange           Decimal           `json:"oneYearPriceChange,omitzero"`
	AutomaticallyActive          bool              `json:"automaticallyActive,omitzero"`
	GameID                       string            `json:"gameId,omitzero"`
	GroupItemRange               string            `json:"groupItemRange,omitzero"`
	SportsMarketType             string            `json:"sportsMarketType,omitzero"`
	Line                         Decimal           `json:"line,omitzero"`
	UmaResolutionStatuses        string            `json:"umaResolutionStatuses,omitzero"`
	PendingDeployment            bool              `json:"pendingDeployment,omitzero"`
	Deploying                    bool              `json:"deploying,omitzero"`
	DeployingTimestamp           *time.Time        `json:"deployingTimestamp,omitzero"`
	ScheduledDeploymentTimestamp *time.Time        `json:"scheduledDeploymentTimestamp,omitzero"`
	EventStartTime               *time.Time        `json:"eventStartTime,omitzero"`
	SubmittedBy                  string            `json:"submittedBy,omitzero"`
	RequiresTranslation          bool              `json:"requiresTranslation,omitzero"`
	PagerDutyNotificationEnabled bool              `json:"pagerDutyNotificationEnabled,omitzero"`
	Approved                     bool              `json:"approved,omitzero"`
	CYOM                         bool              `json:"cyom,omitzero"`
	FeesEnabled                  bool              `json:"feesEnabled,omitzero"`
	SentDiscord                  bool              `json:"sentDiscord,omitzero"`
	// Rust describes a string-encoded epoch integer; TS describes a timestamp
	// string. Preserve the wire value rather than guessing its units.
	TwitterCardLastRefreshed json.RawMessage `json:"twitterCardLastRefreshed,omitzero"`
	TwitterCardLocation      string          `json:"twitterCardLocation,omitzero"`
	TwitterCardLastValidated string          `json:"twitterCardLastValidated,omitzero"`
	CategoryMailchimpTag     string          `json:"categoryMailchimpTag,omitzero"`
	Subcategory              string          `json:"subcategory,omitzero"`
	Markets                  []RelatedMarket `json:"markets,omitzero"`
	InternalUsers            []InternalUser  `json:"internalUsers,omitzero"`
	FeeType                  string          `json:"feeType,omitzero"`
	FeeSchedule              *FeeSchedule    `json:"feeSchedule,omitzero"`
}

// ClobReward represents CLOB rewards configuration for a market.
type ClobReward struct {
	ID               string  `json:"id,omitzero"`
	AssetAddress     string  `json:"assetAddress,omitzero"`
	ConditionID      string  `json:"conditionId,omitzero"`
	StartDate        string  `json:"startDate,omitzero"`
	EndDate          string  `json:"endDate,omitzero"`
	RewardsAmount    Decimal `json:"rewardsAmount,omitzero"`
	RewardsDailyRate Decimal `json:"rewardsDailyRate,omitzero"`
}

// FeeSchedule contains Gamma FeeSchedule metadata.
type FeeSchedule struct {
	Exponent   float64 `json:"exponent,omitzero"`
	Rate       Decimal `json:"rate,omitzero"`
	TakerOnly  bool    `json:"takerOnly,omitzero"`
	RebateRate Decimal `json:"rebateRate,omitzero"`
}

// RelatedMarket contains Gamma RelatedMarket metadata.
type RelatedMarket struct {
	ID            string     `json:"id,omitzero"`
	ConditionID   string     `json:"conditionId,omitzero"`
	Slug          string     `json:"slug,omitzero"`
	Image         string     `json:"image,omitzero"`
	Volume        Decimal    `json:"volume,omitzero"`
	Question      string     `json:"question,omitzero"`
	Outcomes      string     `json:"outcomes,omitzero"`
	OutcomePrices string     `json:"outcomePrices,omitzero"`
	StartDate     *time.Time `json:"startDate,omitzero"`
	EventSlug     string     `json:"eventSlug,omitzero"`
}

// ProtocolVersion is the market protocol version a market trades under.
// Known values are v1 (CTF tokens) and v2 (Poly positions). Unknown values
// pass through as-is so newer server versions do not fail decoding.
type ProtocolVersion string

const (
	// ProtocolVersionV1 is the CTF-token market protocol.
	ProtocolVersionV1 ProtocolVersion = "v1"
	// ProtocolVersionV2 is the Polymarket V2 position protocol.
	ProtocolVersionV2 ProtocolVersion = "v2"
)

// ComboStatus is a market's combo eligibility state. Known values are
// pending, enabled, and disabled. Unknown values pass through as-is.
type ComboStatus string

const (
	// ComboStatusPending is a combo-eligible market awaiting enablement.
	ComboStatusPending ComboStatus = "pending"
	// ComboStatusEnabled marks a market available for combo bets.
	ComboStatusEnabled ComboStatus = "enabled"
	// ComboStatusDisabled marks a market withdrawn from combo betting.
	ComboStatusDisabled ComboStatus = "disabled"
)
