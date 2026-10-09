package data

// Holder is returned by the Data API v2.
type Holder struct {
	Wallet                string         `json:"proxy_wallet"            required:"true"`
	AssetID               string         `json:"token_id"                required:"true"`
	Amount                DecimalString  `json:"amount"                  required:"true"`
	OutcomeIndex          *int64         `json:"outcome_index"`
	DisplayUsernamePublic bool           `json:"display_username_public" required:"true"`
	Verified              bool           `json:"verified"                required:"true"`
	Name                  *string        `json:"name"`
	Pseudonym             *string        `json:"pseudonym"`
	Bio                   *string        `json:"bio"`
	ProfileImage          *string        `json:"profile_image"`
	ProfileImageOptimized *string        `json:"profile_image_optimized"`
	AvgPrice              *DecimalString `json:"avg_price"`
	EntryCostUSDC         *DecimalString `json:"entry_cost_usdc"`
	CurrentPrice          *DecimalString `json:"current_price"`
	CurrentValue          *DecimalString `json:"current_value"`
	RealizedPnL           *DecimalString `json:"realized_pnl"`
	UnrealizedPnL         *DecimalString `json:"unrealized_pnl"`
	TotalPnL              *DecimalString `json:"total_pnl"`
}

// MetaHolder is returned by the Data API v2.
type MetaHolder struct {
	AssetID string   `json:"token_id" required:"true"`
	Holders []Holder `json:"holders"  required:"true"`
}

// OpenInterest is returned by the Data API v2.
type OpenInterest struct {
	ConditionID *string       `json:"condition_id" required:"true"`
	Value       DecimalString `json:"value"        required:"true"`
}

// MarketLiveVolume is returned by the Data API v2.
type MarketLiveVolume struct {
	ConditionID *string       `json:"condition_id" required:"true"`
	TakerVolume DecimalString `json:"taker_volume" required:"true"`
}

// LiveVolume is returned by the Data API v2.
type LiveVolume struct {
	TakerVolumeTotal DecimalString      `json:"taker_volume_total" required:"true"`
	Markets          []MarketLiveVolume `json:"conditions"         required:"true"`
}

// PriceHistoryPoint is returned by the Data API v2.
type PriceHistoryPoint struct {
	Timestamp         Timestamp     `json:"timestamp"          required:"true"`
	Price             DecimalString `json:"price"              required:"true"`
	ResolutionSeconds int64         `json:"resolution_seconds" required:"true"`
}

// Trade is returned by the Data API v2.
type Trade struct {
	Wallet                string        `json:"proxy_wallet"            required:"true"`
	AssetID               string        `json:"token_id"                required:"true"`
	ConditionID           string        `json:"condition_id"            required:"true"`
	Side                  Side          `json:"side"                    required:"true"`
	Size                  DecimalString `json:"size"                    required:"true"`
	Price                 DecimalString `json:"price"                   required:"true"`
	Timestamp             Timestamp     `json:"timestamp"               required:"true"`
	TransactionHash       string        `json:"transaction_hash"        required:"true"`
	Title                 *string       `json:"title"`
	Slug                  *string       `json:"slug"`
	Icon                  *string       `json:"icon"`
	EventSlug             *string       `json:"event_slug"`
	Outcome               *string       `json:"outcome"`
	OutcomeIndex          *int64        `json:"outcome_index"`
	Name                  *string       `json:"name"`
	Pseudonym             *string       `json:"pseudonym"`
	Bio                   *string       `json:"bio"`
	ProfileImage          *string       `json:"profile_image"`
	ProfileImageOptimized *string       `json:"profile_image_optimized"`
}
