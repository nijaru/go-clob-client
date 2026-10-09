package gamma

// Category represents a grouping for tags/events.
type Category struct {
	ID             string    `json:"id"`
	Label          *string   `json:"label,omitzero"`
	ParentCategory *string   `json:"parentCategory,omitzero"`
	Slug           *string   `json:"slug,omitzero"`
	CreatedAt      Timestamp `json:"createdAt,omitzero"`
	UpdatedAt      Timestamp `json:"updatedAt,omitzero"`
	PublishedAt    *string   `json:"publishedAt,omitzero"`
	CreatedBy      *string   `json:"createdBy,omitzero"`
	UpdatedBy      *string   `json:"updatedBy,omitzero"`
}

// Collection represents a high-level event grouping.
type Collection struct {
	DisqusThread         *string            `json:"disqusThread,omitzero"`
	ID                   string             `json:"id"`
	Ticker               *string            `json:"ticker,omitzero"`
	Slug                 *string            `json:"slug,omitzero"`
	Title                *string            `json:"title,omitzero"`
	CollectionType       *string            `json:"collectionType,omitzero"`
	Description          *string            `json:"description,omitzero"`
	Active               *bool              `json:"active,omitzero"`
	Closed               *bool              `json:"closed,omitzero"`
	Archived             *bool              `json:"archived,omitzero"`
	CreatedAt            Timestamp          `json:"createdAt,omitzero"`
	UpdatedAt            Timestamp          `json:"updatedAt,omitzero"`
	Subtitle             *string            `json:"subtitle,omitzero"`
	Tags                 *string            `json:"tags,omitzero"`
	Image                *string            `json:"image,omitzero"`
	Icon                 *string            `json:"icon,omitzero"`
	HeaderImage          *string            `json:"headerImage,omitzero"`
	Layout               *string            `json:"layout,omitzero"`
	New                  *bool              `json:"new,omitzero"`
	Featured             *bool              `json:"featured,omitzero"`
	Restricted           *bool              `json:"restricted,omitzero"`
	IsTemplate           *bool              `json:"isTemplate,omitzero"`
	TemplateVariables    *string            `json:"templateVariables,omitzero"`
	PublishedAt          *string            `json:"publishedAt,omitzero"`
	CreatedBy            *string            `json:"createdBy,omitzero"`
	UpdatedBy            *string            `json:"updatedBy,omitzero"`
	CommentsEnabled      *bool              `json:"commentsEnabled,omitzero"`
	ImageOptimized       *ImageOptimization `json:"imageOptimized,omitzero"`
	IconOptimized        *ImageOptimization `json:"iconOptimized,omitzero"`
	HeaderImageOptimized *ImageOptimization `json:"headerImageOptimized,omitzero"`
}

// Series represents a grouping of related events.
type Series struct {
	ID                  FlexibleID   `json:"id"`
	Title               *string      `json:"title,omitzero"`
	Slug                *string      `json:"slug,omitzero"`
	Description         *string      `json:"description,omitzero"`
	CreatedAt           Timestamp    `json:"createdAt,omitzero"`
	UpdatedAt           Timestamp    `json:"updatedAt,omitzero"`
	Ticker              *string      `json:"ticker,omitzero"`
	SeriesType          *string      `json:"seriesType,omitzero"`
	Recurrence          *string      `json:"recurrence,omitzero"`
	Active              *bool        `json:"active,omitzero"`
	Closed              *bool        `json:"closed,omitzero"`
	Subtitle            *string      `json:"subtitle,omitzero"`
	Image               *string      `json:"image,omitzero"`
	Icon                *string      `json:"icon,omitzero"`
	Layout              *string      `json:"layout,omitzero"`
	Archived            *bool        `json:"archived,omitzero"`
	New                 *bool        `json:"new,omitzero"`
	Featured            *bool        `json:"featured,omitzero"`
	Restricted          *bool        `json:"restricted,omitzero"`
	IsTemplate          *bool        `json:"isTemplate,omitzero"`
	TemplateVariables   *bool        `json:"templateVariables,omitzero"`
	PublishedAt         *string      `json:"publishedAt,omitzero"`
	CreatedBy           *string      `json:"createdBy,omitzero"`
	UpdatedBy           *string      `json:"updatedBy,omitzero"`
	CommentsEnabled     *bool        `json:"commentsEnabled,omitzero"`
	Competitive         Decimal      `json:"competitive,omitzero"`
	Volume24h           Decimal      `json:"volume24hr,omitzero"`
	Volume              Decimal      `json:"volume,omitzero"`
	Liquidity           Decimal      `json:"liquidity,omitzero"`
	StartDate           Timestamp    `json:"startDate,omitzero"`
	PythTokenID         *string      `json:"pythTokenID,omitzero"`
	CGAssetName         *string      `json:"cgAssetName,omitzero"`
	Score               *int         `json:"score,omitzero"`
	Events              []Event      `json:"events,omitzero"`
	Collections         []Collection `json:"collections,omitzero"`
	Categories          []Category   `json:"categories,omitzero"`
	Tags                []Tag        `json:"tags,omitzero"`
	CommentCount        *int         `json:"commentCount,omitzero"`
	Chats               []Chat       `json:"chats,omitzero"`
	RequiresTranslation *bool        `json:"requiresTranslation,omitzero"`
}

// ImageOptimization contains Gamma ImageOptimization metadata.
type ImageOptimization struct {
	ID                        *string    `json:"id,omitzero"`
	ImageURLSource            *string    `json:"imageUrlSource,omitzero"`
	ImageURLOptimized         *string    `json:"imageUrlOptimized,omitzero"`
	ImageSizeKBSource         *float64   `json:"imageSizeKbSource,omitzero"`
	ImageSizeKBOptimized      *float64   `json:"imageSizeKbOptimized,omitzero"`
	ImageOptimizedComplete    *bool      `json:"imageOptimizedComplete,omitzero"`
	ImageOptimizedLastUpdated *string    `json:"imageOptimizedLastUpdated,omitzero"`
	RelID                     FlexibleID `json:"relID,omitzero"`
	Field                     *string    `json:"field,omitzero"`
	Relname                   *string    `json:"relname,omitzero"`
}

// EventCreator contains Gamma EventCreator metadata.
type EventCreator struct {
	ID            *string   `json:"id,omitzero"`
	CreatorName   *string   `json:"creatorName,omitzero"`
	CreatorHandle *string   `json:"creatorHandle,omitzero"`
	CreatorURL    *string   `json:"creatorUrl,omitzero"`
	CreatorImage  *string   `json:"creatorImage,omitzero"`
	CreatedAt     Timestamp `json:"createdAt,omitzero"`
	UpdatedAt     Timestamp `json:"updatedAt,omitzero"`
}

// Chat contains Gamma Chat metadata.
type Chat struct {
	ID           *string   `json:"id,omitzero"`
	ChannelID    *string   `json:"channelId,omitzero"`
	ChannelName  *string   `json:"channelName,omitzero"`
	ChannelImage *string   `json:"channelImage,omitzero"`
	Live         *bool     `json:"live,omitzero"`
	StartTime    Timestamp `json:"startTime,omitzero"`
	EndTime      Timestamp `json:"endTime,omitzero"`
}

// Template contains Gamma Template metadata.
type Template struct {
	ID                      *string   `json:"id,omitzero"`
	EventTitle              *string   `json:"eventTitle,omitzero"`
	EventSlug               *string   `json:"eventSlug,omitzero"`
	EventImage              *string   `json:"eventImage,omitzero"`
	MarketTitle             *string   `json:"marketTitle,omitzero"`
	Description             *string   `json:"description,omitzero"`
	ResolutionSource        *string   `json:"resolutionSource,omitzero"`
	NegRisk                 *bool     `json:"negRisk,omitzero"`
	SortBy                  *string   `json:"sortBy,omitzero"`
	ShowMarketImages        *bool     `json:"showMarketImages,omitzero"`
	SeriesSlug              *string   `json:"seriesSlug,omitzero"`
	Outcomes                *string   `json:"outcomes,omitzero"`
	DisplayName             *string   `json:"displayName,omitzero"`
	MarketsOrder            *string   `json:"marketsOrder,omitzero"`
	MarketsNegRisk          *bool     `json:"marketsNegRisk,omitzero"`
	MarketsAugmentedNegRisk *bool     `json:"marketsAugmentedNegRisk,omitzero"`
	MarketsShowImages       *bool     `json:"marketsShowImages,omitzero"`
	Markets                 *string   `json:"markets,omitzero"`
	UserVariables           *string   `json:"userVariables,omitzero"`
	CreatorUserID           *string   `json:"creatorUserId,omitzero"`
	CreatedAt               Timestamp `json:"createdAt,omitzero"`
	UpdatedAt               Timestamp `json:"updatedAt,omitzero"`
}

// BestLine contains Gamma BestLine metadata.
type BestLine struct {
	ID       string  `json:"id,omitzero"`
	LineType string  `json:"lineType,omitzero"`
	Line     Decimal `json:"line,omitzero"`
}

// Partner contains Gamma Partner metadata.
type Partner struct {
	ID        int       `json:"id,omitzero"`
	Slug      string    `json:"slug,omitzero"`
	Name      string    `json:"name,omitzero"`
	CreatedAt Timestamp `json:"createdAt,omitzero"`
	UpdatedAt Timestamp `json:"updatedAt,omitzero"`
}

// EventExternalPartnerMapping contains Gamma EventExternalPartnerMapping metadata.
type EventExternalPartnerMapping struct {
	ID         int        `json:"id,omitzero"`
	EventID    FlexibleID `json:"eventId,omitzero"`
	PartnerID  int        `json:"partnerId,omitzero"`
	ExternalID string     `json:"externalId,omitzero"`
	Partner    *Partner   `json:"partner,omitzero"`
	CreatedAt  Timestamp  `json:"createdAt,omitzero"`
	UpdatedAt  Timestamp  `json:"updatedAt,omitzero"`
}

// InternalUser contains Gamma InternalUser metadata.
type InternalUser struct {
	ID       *string `json:"id,omitzero"`
	Username *string `json:"username,omitzero"`
}
