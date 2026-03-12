package clob

type ReadonlyAPIKeyResponse struct {
	APIKey string `json:"apiKey"`
}

type DeleteReadonlyAPIKeyRequest struct {
	Key string `json:"key"`
}

type Notification struct {
	Type    int                 `json:"type"`
	Owner   string              `json:"owner"`
	Payload NotificationPayload `json:"payload"`
}

type NotificationPayload struct {
	AssetID         string `json:"asset_id"`
	ConditionID     string `json:"condition_id"`
	EventSlug       string `json:"eventSlug"`
	Icon            string `json:"icon"`
	Image           string `json:"image"`
	Market          string `json:"market"`
	MarketSlug      string `json:"market_slug"`
	MatchedSize     string `json:"matched_size"`
	OrderID         string `json:"order_id"`
	OriginalSize    string `json:"original_size"`
	Outcome         string `json:"outcome"`
	OutcomeIndex    int64  `json:"outcome_index"`
	Owner           string `json:"owner"`
	Price           string `json:"price"`
	Question        string `json:"question"`
	RemainingSize   string `json:"remaining_size"`
	SeriesSlug      string `json:"seriesSlug"`
	Side            Side   `json:"side"`
	TradeID         string `json:"trade_id"`
	TransactionHash string `json:"transaction_hash"`
	OrderType       string `json:"type"`
}

type DeleteNotificationsParams struct {
	IDs []string
}

type AssetType string

const (
	AssetTypeCollateral  AssetType = "COLLATERAL"
	AssetTypeConditional AssetType = "CONDITIONAL"
)

type BalanceAllowanceParams struct {
	AssetType     AssetType
	TokenID       string
	SignatureType *SignatureType
}

type BalanceAllowanceResponse struct {
	Balance    string            `json:"balance"`
	Allowances map[string]string `json:"allowances"`
}

type OrderScoringParams struct {
	OrderID string
}

type OrderScoringResponse struct {
	Scoring bool `json:"scoring"`
}

type OrdersScoringParams struct {
	OrderIDs []string
}

type OrdersScoringResponse map[string]bool

type CancelMarketOrdersRequest struct {
	Market  string `json:"market,omitempty"`
	AssetID string `json:"asset_id,omitempty"`
}

type UserRewardsFilterParams struct {
	Date          string
	OrderBy       string
	Position      string
	NoCompetition bool
}
