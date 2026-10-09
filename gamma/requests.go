package gamma

// MarketOptions controls optional fields on a single-market lookup.
type MarketOptions struct {
	IncludeTag *bool
	Locale     string
}

// EventOptions controls optional fields on a single-event lookup.
type EventOptions struct {
	IncludeChat      *bool
	IncludeTemplate  *bool
	IncludeBestLines *bool
	Locale           string
}

// SeriesOptions controls optional fields on a single-series lookup.
type SeriesOptions struct {
	IncludeChat *bool
	Locale      string
}

// TagOptions controls optional fields on a single-tag lookup.
type TagOptions struct {
	IncludeTemplate *bool
	Locale          string
}

// RelatedTagsOptions controls optional filtering for related-tag lookups.
type RelatedTagsOptions struct {
	OmitEmpty *bool
	Status    string
}

// TagFilterParams controls paginated tag listing.
type TagFilterParams struct {
	Ascending       *bool  `url:"ascending,omitzero"`
	IncludeTemplate *bool  `url:"include_template,omitzero"`
	IsCarousel      *bool  `url:"is_carousel,omitzero"`
	Locale          string `url:"locale,omitzero"`
	Order           string `url:"order,omitzero"`
	Limit           int    `url:"limit,omitzero"`
	Offset          int    `url:"offset,omitzero"`
}

// RelatedTagResourceParams controls optional filtering for related-tag
// resource lookups.
type RelatedTagResourceParams struct {
	Locale    string
	OmitEmpty *bool
	Status    string
}

// MarketClarificationsParams filters the market-clarifications endpoint.
type MarketClarificationsParams struct {
	MarketID       string
	MarketIDs      []string
	EventID        string
	EventIDs       []string
	QuestionID     string
	QuestionIDs    []string
	State          MarketClarificationState
	States         []MarketClarificationState
	ShowInFrontend *bool
	TxHash         string
	Order          string
	Ascending      *bool
	Limit          int
	Offset         int
}

// SearchParams defines parameters for Gamma public search. KeepClosedMarkets
// is an hour window for recently closed markets; nil omits it and &0 explicitly
// excludes them. Page is 1-based (zero selects the service default).
type SearchParams struct {
	Query             string     `url:"q"`
	LimitPerType      int        `url:"limit_per_type,omitzero"`
	Page              int        `url:"page,omitzero"`
	Ascending         *bool      `url:"ascending,omitzero"`
	Cache             *bool      `url:"cache,omitzero"`
	EventsStatus      string     `url:"events_status,omitzero"`
	EventsTag         []string   `url:"events_tag,omitzero"`
	ExcludeTagIDs     []int      `url:"exclude_tag_id,omitzero"`
	KeepClosedMarkets *int       `url:"keep_closed_markets,omitzero"`
	Optimized         *bool      `url:"optimized,omitzero"`
	Presets           []string   `url:"presets,omitzero"`
	Recurrence        string     `url:"recurrence,omitzero"`
	SearchProfiles    *bool      `url:"search_profiles,omitzero"`
	SearchTags        *bool      `url:"search_tags,omitzero"`
	Sort              SearchSort `url:"sort,omitzero"`
}

// MarketFilterParams defines filters for market listing.
type MarketFilterParams struct {
	Locale              string
	Decimalized         *bool
	PositionIDs         []string
	RfqEnabled          *bool
	TagMatch            string
	Active              *bool    `url:"active,omitzero"`
	IDs                 []string `url:"id,omitzero"`
	Closed              *bool    `url:"closed,omitzero"`
	Archived            *bool    `url:"archived,omitzero"`
	Resolved            *bool    `url:"resolved,omitzero"`
	Limit               int      `url:"limit,omitzero"`
	Offset              int      `url:"offset,omitzero"`
	Order               string   `url:"order,omitzero"`
	Ascending           *bool    `url:"ascending,omitzero"`
	TagID               string   `url:"tag_id,omitzero"`
	EventID             string   `url:"event_id,omitzero"`
	Slugs               []string `url:"slug,omitzero"`
	Slug                string   `url:"slug,omitzero"`
	NegativeRisk        *bool    `url:"negative_risk,omitzero"`
	AcceptingOrders     *bool    `url:"accepting_orders,omitzero"`
	ClobTokenIDs        []string `url:"clob_token_ids,omitzero"`
	ConditionIDs        []string `url:"condition_ids,omitzero"`
	MarketMakerAddress  []string `url:"market_maker_address,omitzero"`
	LiquidityNumMin     string   `url:"liquidity_num_min,omitzero"`
	LiquidityNumMax     string   `url:"liquidity_num_max,omitzero"`
	VolumeNumMin        string   `url:"volume_num_min,omitzero"`
	VolumeNumMax        string   `url:"volume_num_max,omitzero"`
	RelatedTags         *bool    `url:"related_tags,omitzero"`
	CYOM                *bool    `url:"cyom,omitzero"`
	UmaResolutionStatus string   `url:"uma_resolution_status,omitzero"`
	GameID              string   `url:"game_id,omitzero"`
	SportsMarketTypes   []string `url:"sports_market_types,omitzero"`
	RewardsMinSize      string   `url:"rewards_min_size,omitzero"`
	QuestionIDs         []string `url:"question_ids,omitzero"`
	IncludeTag          *bool    `url:"include_tag,omitzero"`
	StartDateMin        string   `url:"start_date_min,omitzero"`
	StartDateMax        string   `url:"start_date_max,omitzero"`
	EndDateMin          string   `url:"end_date_min,omitzero"`
	EndDateMax          string   `url:"end_date_max,omitzero"`
}

// EventFilterParams defines filters for event listing.
type EventFilterParams struct {
	TagIDs           []string
	SeriesIDs        []string
	GameIDs          []string
	TagMatch         string
	IncludeBestLines *bool
	IncludeChildren  *bool
	Ended            *bool
	Live             *bool
	FeaturedOrder    *bool
	EventDate        string
	EventWeek        *int
	StartTimeMin     string
	StartTimeMax     string
	ParentEventID    string
	PartnerSlug      string
	TitleSearch      string
	Locale           string
	Ascending        *bool
	Active           *bool    `url:"active,omitzero"`
	Closed           *bool    `url:"closed,omitzero"`
	Archived         *bool    `url:"archived,omitzero"`
	Resolved         *bool    `url:"resolved,omitzero"`
	IDs              []string `url:"id,omitzero"`
	Orders           []string `url:"order,omitzero"`
	Order            string   `url:"order,omitzero"`
	TagID            string   `url:"tag_id,omitzero"`
	ExcludeTagIDs    []string `url:"exclude_tag_id,omitzero"`
	Slugs            []string `url:"slug,omitzero"`
	Slug             string   `url:"slug,omitzero"`
	Limit            int      `url:"limit,omitzero"`
	Offset           int      `url:"offset,omitzero"`
	NegativeRisk     *bool    `url:"negative_risk,omitzero"`
	TagSlug          string   `url:"tag_slug,omitzero"`
	RelatedTags      *bool    `url:"related_tags,omitzero"`
	Featured         *bool    `url:"featured,omitzero"`
	CYOM             *bool    `url:"cyom,omitzero"`
	IncludeChat      *bool    `url:"include_chat,omitzero"`
	IncludeTemplate  *bool    `url:"include_template,omitzero"`
	Recurrence       string   `url:"recurrence,omitzero"`
	LiquidityMin     string   `url:"liquidity_min,omitzero"`
	LiquidityMax     string   `url:"liquidity_max,omitzero"`
	VolumeMin        string   `url:"volume_min,omitzero"`
	VolumeMax        string   `url:"volume_max,omitzero"`
	StartDateMin     string   `url:"start_date_min,omitzero"`
	StartDateMax     string   `url:"start_date_max,omitzero"`
	EndDateMin       string   `url:"end_date_min,omitzero"`
	EndDateMax       string   `url:"end_date_max,omitzero"`
}

// CommentFilterParams identifies a parent thread and controls its ordering,
// optional holder/position data, and pagination. Limit counts root comments.
type CommentFilterParams struct {
	ParentEntityType ParentEntityType
	ParentEntityID   string
	GetPositions     *bool
	HoldersOnly      *bool
	Order            string
	Ascending        *bool
	Limit            int
	Offset           int
}

// CommentsByUserAddressParams controls paginated comments-by-user listing.
type CommentsByUserAddressParams struct {
	Ascending *bool  `url:"ascending,omitzero"`
	Order     string `url:"order,omitzero"`
	Limit     int    `url:"limit,omitzero"`
	Offset    int    `url:"offset,omitzero"`
}

// SeriesFilterParams defines filters for listing series.
type SeriesFilterParams struct {
	Ascending        *bool    `url:"ascending,omitzero"`
	Closed           *bool    `url:"closed,omitzero"`
	ExcludeEvents    *bool    `url:"exclude_events,omitzero"`
	Locale           string   `url:"locale,omitzero"`
	Order            string   `url:"order,omitzero"`
	Recurrence       string   `url:"recurrence,omitzero"`
	Slugs            []string `url:"slug,omitzero"`
	Slug             string   `url:"slug,omitzero"`
	CategoriesIDs    []string `url:"categories_ids,omitzero"`
	CategoriesLabels []string `url:"categories_labels,omitzero"`
	IncludeChat      *bool    `url:"include_chat,omitzero"`
	Limit            int      `url:"limit,omitzero"`
	Offset           int      `url:"offset,omitzero"`
}

// TeamFilterParams defines filters for listing teams.
type TeamFilterParams struct {
	Abbreviations []string `url:"abbreviation,omitzero"`
	Abbreviation  string   `url:"abbreviation,omitzero"`
	Ascending     *bool    `url:"ascending,omitzero"`
	Leagues       []string `url:"league,omitzero"`
	League        string   `url:"league,omitzero"`
	Names         []string `url:"name,omitzero"`
	Name          string   `url:"name,omitzero"`
	Order         string   `url:"order,omitzero"`
	ProviderID    int      `url:"provider_id,omitzero"`
	ProviderIDs   []int
	Limit         int `url:"limit,omitzero"`
	Offset        int `url:"offset,omitzero"`
}

// CommentOptions controls comment-thread lookups.
type CommentOptions struct{ GetPositions *bool }

// ParentEntityType identifies the Gamma entity whose comments are requested.
type ParentEntityType string

const (
	ParentEntityTypeEvent  ParentEntityType = "Event"
	ParentEntityTypeSeries ParentEntityType = "Series"
	ParentEntityTypeMarket ParentEntityType = "market"
)
