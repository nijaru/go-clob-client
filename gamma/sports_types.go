package gamma

// Sport represents sports-specific metadata.
type Sport struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	Slug      string    `json:"slug"`
	Active    *bool     `json:"active,omitzero"`
	CreatedAt Timestamp `json:"createdAt,omitzero"`
	UpdatedAt Timestamp `json:"updatedAt,omitzero"`
}

// Team represents a sports team.
type Team struct {
	ID           int     `json:"id"`
	Name         *string `json:"name,omitzero"`
	League       *string `json:"league,omitzero"`
	Record       *string `json:"record,omitzero"`
	Logo         *string `json:"logo,omitzero"`
	Abbreviation *string `json:"abbreviation,omitzero"`
	Alias        *string `json:"alias,omitzero"`
	// Ordering is the team's home/away ordering within its event
	// (py-sdk "home"/"away" passthrough).
	Ordering   *string   `json:"ordering,omitzero"`
	CreatedAt  Timestamp `json:"createdAt,omitzero"`
	UpdatedAt  Timestamp `json:"updatedAt,omitzero"`
	Color      *string   `json:"color,omitzero"`
	ProviderID *int      `json:"providerId,omitzero"`
}

// SportsMetadata describes a supported sports feed in Gamma.
type SportsMetadata struct {
	ID         int       `json:"id,omitzero"`
	Sport      string    `json:"sport"`
	Image      string    `json:"image"`
	Resolution string    `json:"resolution"`
	Ordering   string    `json:"ordering"`
	Tags       []string  `json:"tags,omitzero"`
	Series     string    `json:"series"`
	CreatedAt  Timestamp `json:"createdAt,omitzero"`
	Name       *string   `json:"name,omitzero"`
	UpdatedAt  Timestamp `json:"updatedAt,omitzero"`
}

// SportsMarketTypesResponse wraps the valid sports market types response.
type SportsMarketTypesResponse struct {
	MarketTypes []string `json:"marketTypes,omitzero"`
}
