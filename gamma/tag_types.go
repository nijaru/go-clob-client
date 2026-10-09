package gamma

// Tag represents metadata categorization.
type Tag struct {
	Templates           []Template `json:"templates,omitzero"`
	ID                  string     `json:"id"`
	Label               *string    `json:"label,omitzero"`
	Slug                *string    `json:"slug,omitzero"`
	ForceShow           *bool      `json:"forceShow,omitzero"`
	CreatedAt           Timestamp  `json:"createdAt,omitzero"`
	UpdatedAt           Timestamp  `json:"updatedAt,omitzero"`
	PublishedAt         *string    `json:"publishedAt,omitzero"`
	CreatedBy           *int       `json:"createdBy,omitzero"`
	UpdatedBy           *int       `json:"updatedBy,omitzero"`
	ForceHide           *bool      `json:"forceHide,omitzero"`
	IsCarousel          *bool      `json:"isCarousel,omitzero"`
	RequiresTranslation *bool      `json:"requiresTranslation,omitzero"`
	ActiveEventsCount   *int       `json:"activeEventsCount,omitzero"`
}

// RelatedTag represents a relationship between tags.
type RelatedTag struct {
	ID           FlexibleID `json:"id"`
	TagID        FlexibleID `json:"tagID,omitzero"`
	RelatedTagID FlexibleID `json:"relatedTagID,omitzero"`
	Rank         *int       `json:"rank,omitzero"`
}

// RelatedTagsStatus identifies the status filter accepted by Gamma related-tag
// endpoints. It is intentionally string-backed so new server values remain
// forward-compatible.
type RelatedTagsStatus string

const (
	RelatedTagsStatusActive RelatedTagsStatus = "active"
	RelatedTagsStatusClosed RelatedTagsStatus = "closed"
	RelatedTagsStatusAll    RelatedTagsStatus = "all"
)
