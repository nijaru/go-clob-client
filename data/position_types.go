package data

// Position is returned by the Data API v2.
type Position struct {
	Wallet             string         `json:"proxy_wallet"         required:"true"`
	AssetID            string         `json:"token_id"             required:"true"`
	ConditionID        string         `json:"condition_id"         required:"true"`
	CurrentSize        DecimalString  `json:"current_size"         required:"true"`
	AvgPrice           DecimalString  `json:"avg_price"            required:"true"`
	EntryCostUSDC      DecimalString  `json:"entry_cost_usdc"      required:"true"`
	EntryFeesUSDC      DecimalString  `json:"entry_fees_usdc"      required:"true"`
	TotalCostUSDC      DecimalString  `json:"total_cost_usdc"      required:"true"`
	CurrentPrice       DecimalString  `json:"current_price"        required:"true"`
	CurrentValue       DecimalString  `json:"current_value"        required:"true"`
	TotalSize          DecimalString  `json:"total_size"           required:"true"`
	RealizedPnL        DecimalString  `json:"realized_pnl"         required:"true"`
	UnrealizedPnL      DecimalString  `json:"unrealized_pnl"       required:"true"`
	TotalPnL           DecimalString  `json:"total_pnl"            required:"true"`
	PercentPnL         DecimalString  `json:"percent_pnl"          required:"true"`
	PercentRealizedPnL DecimalString  `json:"percent_realized_pnl" required:"true"`
	Status             PositionStatus `json:"status"               required:"true"`
	Redeemable         bool           `json:"redeemable"           required:"true"`
	Mergeable          bool           `json:"mergeable"            required:"true"`
	NegativeRisk       bool           `json:"negative_risk"        required:"true"`
	Archived           bool           `json:"archived"             required:"true"`
	Verified           bool           `json:"verified"             required:"true"`
	Title              *string        `json:"title"`
	Slug               *string        `json:"slug"`
	Icon               *string        `json:"icon"`
	EventSlug          *string        `json:"event_slug"`
	Outcome            *string        `json:"outcome"`
	OppositeOutcome    *string        `json:"opposite_outcome"`
	Name               *string        `json:"name"`
	ProfileImage       *string        `json:"profile_image"`
	EventID            *EventID       `json:"event_id"`
	OutcomeIndex       *int64         `json:"outcome_index"`
	OppositeAssetID    *string        `json:"opposite_token_id"`
	EndDate            *string        `json:"end_date"`
	LastEventAt        *Timestamp     `json:"last_event_at"`
	FirstEntryAt       *Timestamp     `json:"first_entry_at"`
}

// ComboPositionMarketEvent is returned by the Data API v2.
type ComboPositionMarketEvent struct {
	EventID    *EventID `json:"event_id"`
	EventSlug  *string  `json:"event_slug"`
	EventTitle *string  `json:"event_title"`
	EventImage *string  `json:"event_image"`
}

// ComboPositionMarket is returned by the Data API v2.
type ComboPositionMarket struct {
	MarketID         *string                   `json:"market_id"`
	Slug             *string                   `json:"slug"`
	Title            *string                   `json:"title"`
	Question         *string                   `json:"question"`
	GroupItemTitle   *string                   `json:"group_item_title"`
	SportsMarketType *string                   `json:"sports_market_type"`
	Line             *DecimalString            `json:"line"`
	Outcomes         []string                  `json:"outcomes"`
	Outcome          *string                   `json:"outcome"`
	ImageURL         *string                   `json:"image_url"`
	IconURL          *string                   `json:"icon_url"`
	Category         *string                   `json:"category"`
	Subcategory      *string                   `json:"subcategory"`
	Tags             []string                  `json:"tags"`
	EndDate          *Timestamp                `json:"end_date"`
	Event            *ComboPositionMarketEvent `json:"event"`
}

// ComboPositionLeg is returned by the Data API v2.
type ComboPositionLeg struct {
	LegIndex        int64                `json:"leg_index"         required:"true"`
	LegPositionID   string               `json:"leg_position_id"   required:"true"`
	LegConditionID  string               `json:"leg_condition_id"  required:"true"`
	LegOutcomeIndex int64                `json:"leg_outcome_index" required:"true"`
	LegOutcomeLabel *string              `json:"leg_outcome_label"`
	LegStatus       ComboPositionStatus  `json:"leg_status"        required:"true"`
	LegResolvedAt   *Timestamp           `json:"leg_resolved_at"`
	LegCurrentPrice *DecimalString       `json:"leg_current_price"`
	Market          *ComboPositionMarket `json:"market"`
}

// ComboPosition is returned by the Data API v2.
type ComboPosition struct {
	ConditionID        string              `json:"combo_condition_id"    required:"true"`
	PositionID         string              `json:"combo_position_id"     required:"true"`
	Wallet             string              `json:"proxy_wallet"          required:"true"`
	OutcomeIndex       int64               `json:"outcome_index"         required:"true"`
	OutcomeLabel       string              `json:"outcome_label"         required:"true"`
	CurrentSize        DecimalString       `json:"current_size"          required:"true"`
	EntryAvgPriceUSDC  DecimalString       `json:"entry_avg_price_usdc"  required:"true"`
	EntryCostUSDC      DecimalString       `json:"entry_cost_usdc"       required:"true"`
	GrossEntryCostUSDC DecimalString       `json:"gross_entry_cost_usdc" required:"true"`
	EntryFeesUSDC      DecimalString       `json:"entry_fees_usdc"       required:"true"`
	RealizedPayoutUSDC DecimalString       `json:"realized_payout_usdc"  required:"true"`
	Status             ComboPositionStatus `json:"status"                required:"true"`
	Redeemable         bool                `json:"redeemable"            required:"true"`
	FirstEntryAt       Timestamp           `json:"first_entry_at"        required:"true"`
	ResolvedAt         *Timestamp          `json:"resolved_at"`
	UpdatedAt          Timestamp           `json:"updated_at"            required:"true"`
	LegsTotal          int64               `json:"legs_total"            required:"true"`
	LegsResolved       int64               `json:"legs_resolved"         required:"true"`
	LegsPending        int64               `json:"legs_pending"          required:"true"`
	Legs               []ComboPositionLeg  `json:"legs"                  required:"true"`
}
