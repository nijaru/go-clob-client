package gamma

// Market represents a Polymarket Gamma market record.
type Market struct {
	ID               string   `json:"id"`
	Question         string   `json:"question"`
	ConditionID      string   `json:"conditionId"`
	Slug             string   `json:"slug"`
	ResolutionSource string   `json:"resolutionSource"`
	EndDate          string   `json:"endDate"`
	Liquidity        string   `json:"liquidity"`
	Volume           string   `json:"volume"`
	Outcomes         []string `json:"outcomes"`
	OutcomePrices    []string `json:"outcomePrices"`
	CLOBTokenIDs     []string `json:"clobTokenIds"`
	Active           bool     `json:"active"`
	Closed           bool     `json:"closed"`
	Archived         bool     `json:"archived"`
	AcceptingOrders  bool     `json:"acceptingOrders"`
	MinTickSize      string   `json:"minTickSize"`
	Category         string   `json:"category,omitzero"`
	Notifications    bool     `json:"notificationsEnabled,omitzero"`
	Description      string   `json:"description,omitzero"`
	Image            string   `json:"image,omitzero"`
	Icon             string   `json:"icon,omitzero"`
}

// Event represents a collection of markets related to a single real-world event.
type Event struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Image       string   `json:"image,omitzero"`
	Icon        string   `json:"icon,omitzero"`
	Category    string   `json:"category,omitzero"`
	Markets     []Market `json:"markets,omitzero"`
	Active      bool     `json:"active"`
	Closed      bool     `json:"closed"`
}

// Series represents a top-level grouping of events.
type Series struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Image       string `json:"image,omitzero"`
	Icon        string `json:"icon,omitzero"`
}

// Tag represents a category or descriptive label for markets.
type Tag struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Slug  string `json:"slug"`
}

// Sport represents a sport-specific grouping of markets.
type Sport struct {
	Sport      string `json:"sport"`
	Image      string `json:"image"`
	Resolution string `json:"resolution"`
	Ordering   string `json:"ordering,omitzero"`
	Tags       string `json:"tags,omitzero"`
	Series     string `json:"series,omitzero"`
}

// MarketType represents a valid sports market type.
type MarketType struct {
	Name string `json:"name"`
}

// Comment represents a user comment on a market.
type Comment struct {
	ID          string `json:"id"`
	Body        string `json:"body"`
	UserID      string `json:"userId"`
	ConditionID string `json:"conditionId"`
	CreatedAt   string `json:"createdAt"`
}

// CommentFilterParams defines query parameters for fetching comments.
type CommentFilterParams struct {
	ConditionID string
	Limit       int
	Offset      int
}

// PublicProfile represents a user's public metadata on Polymarket.
type PublicProfile struct {
	DisplayName  string `json:"displayName,omitzero"`
	Username     string `json:"username,omitzero"`
	Bio          string `json:"bio,omitzero"`
	Image        string `json:"image,omitzero"`
	ProxyAddress string `json:"proxyAddress,omitzero"`
}

// MarketFilterParams defines query parameters for fetching markets.
type MarketFilterParams struct {
	Active    *bool
	Closed    *bool
	Archived  *bool
	Limit     int
	Offset    int
	Order     string
	Ascending *bool
	TagID     string
	EventID   string
}

// EventFilterParams defines query parameters for fetching events.
type EventFilterParams struct {
	Active *bool
	Closed *bool
	TagID  string
	Slug   string
	Limit  int
	Offset int
}
