package data

import "time"

// Selectors chooses markets or events, never both. Condition IDs are normalized
// according to the resource's protocol grammar and deduplicated before sending.
type Selectors struct {
	ConditionIDs []string
	EventIDs     []int32
}

// TimeWindow uses absolute times, encoded as integer UTC epoch seconds.
// Trades, activity and user-volume default to the service's recent window;
// FullHistory requests their entire history. Positions are already unbounded.
// FullHistory cannot be combined with either explicit bound.
type TimeWindow struct {
	Start, End  *time.Time
	FullHistory bool
}

type TradesParams struct {
	User         string
	Selectors    Selectors
	Window       TimeWindow
	Side         Side
	TakerOnly    *bool
	FilterType   FilterType
	FilterAmount *DecimalString
	Page         PageParams
}

type ActivityParams struct {
	User          string
	Selectors     Selectors
	Window        TimeWindow
	Types         []ActivityType
	Side          Side
	SortDirection SortDirection
	Page          PageParams
}

type ComboActivityParams struct {
	User         string
	ConditionIDs []string
	Page         PageParams
}

type PositionsParams struct {
	User            string
	Selectors       Selectors
	Window          TimeWindow
	Status          PositionStatus
	Title           string
	FilterType      FilterType
	FilterAmount    *DecimalString
	IncludeArchived *bool
	SortBy          PositionSortBy
	SortDirection   SortDirection
	Page            PageParams
}

type ComboPositionsParams struct {
	User          string
	ConditionIDs  []string
	Statuses      []ComboPositionStatus
	SortBy        ComboPositionSortBy
	SortDirection SortDirection
	// UpdatedAfter/UpdatedBefore are inclusive epoch-second watermarks. Unlike
	// other timestamp filters, the Unix epoch itself is valid here.
	UpdatedAfter, UpdatedBefore *time.Time
	Page                        PageParams
}

type ValueParams struct {
	User         string
	ConditionIDs []string
}

type UserPnLParams struct {
	User     string
	Interval UserPnLInterval
	Fidelity UserPnLFidelity
}

type UserVolumeParams struct {
	User   string
	Window TimeWindow
}

type HoldersParams struct {
	ConditionIDs []string
	MinBalance   *DecimalString
	IncludePnL   *bool
	Page         PageParams
}

// PriceHistoryParams selects exactly one of Interval, Start, or AsOf. Explicit
// windows span at most 15 days. AsOf disallows buckets and page-size overrides.
type PriceHistoryParams struct {
	AssetID          string
	Interval         PriceHistoryInterval
	Start, End, AsOf *time.Time
	BucketSeconds    int
	Page             PageParams
}

type ResolutionsParams struct {
	QuestionID   string
	ConditionIDs []string
	EventIDs     []int32
}

type LeaderboardParams struct {
	Category string
	Window   LeaderboardWindow
	SortBy   LeaderboardSort
	Page     PageParams
}

type LeaderboardStandingParams struct {
	User, Category string
	Window         LeaderboardWindow
}

type BiggestWinnersParams struct {
	Category string
	Window   LeaderboardWindow
	Page     PageParams
}

type BuilderLeaderboardParams struct {
	Window LeaderboardWindow
	Page   PageParams
}

type BuilderVolumeParams struct {
	Interval    BuilderVolumeInterval
	BucketLimit int
}
