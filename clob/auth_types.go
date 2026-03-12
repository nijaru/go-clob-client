package clob

// Credentials are the Polymarket API credentials used for authenticated CLOB requests.
type Credentials struct {
	Key        string `json:"key"`
	Secret     string `json:"secret"`
	Passphrase string `json:"passphrase"`
}

type apiKeyRaw struct {
	APIKey     string `json:"apiKey"`
	Secret     string `json:"secret"`
	Passphrase string `json:"passphrase"`
}

// APIKeysResponse is the response payload for listing API keys.
type APIKeysResponse struct {
	APIKeys []Credentials `json:"apiKeys"`
}

// BanStatus reports whether the account is currently restricted to closed-only mode.
type BanStatus struct {
	ClosedOnly bool `json:"closed_only"`
}

// ReadonlyAPIKeyResponse is the response from creating a readonly API key.
type ReadonlyAPIKeyResponse struct {
	APIKey string `json:"apiKey"`
}

// DeleteReadonlyAPIKeyRequest is the request payload for removing a readonly API key.
type DeleteReadonlyAPIKeyRequest struct {
	Key string `json:"key"`
}

// Notification is a Polymarket user notification.
type Notification struct {
	Type    int                 `json:"type"`
	Owner   string              `json:"owner"`
	Payload NotificationPayload `json:"payload"`
}

// NotificationPayload contains the event-specific details for a notification.
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

// DeleteNotificationsParams filters notification deletion requests.
type DeleteNotificationsParams struct {
	IDs []string
}

// AssetType identifies the Polymarket asset namespace used in allowance requests.
type AssetType string

const (
	// AssetTypeCollateral is the USDC collateral asset namespace.
	AssetTypeCollateral AssetType = "COLLATERAL"
	// AssetTypeConditional is the conditional token asset namespace.
	AssetTypeConditional AssetType = "CONDITIONAL"
)

// BalanceAllowanceParams configures a balance or allowance lookup.
type BalanceAllowanceParams struct {
	AssetType     AssetType
	TokenID       string
	SignatureType *SignatureType
}

// BalanceAllowanceResponse reports the current balance and spender allowances.
type BalanceAllowanceResponse struct {
	Balance    string            `json:"balance"`
	Allowances map[string]string `json:"allowances"`
}

// OrderScoringParams filters a single-order scoring lookup.
type OrderScoringParams struct {
	OrderID string
}

// OrderScoringResponse reports whether an order is scoring for rewards.
type OrderScoringResponse struct {
	Scoring bool `json:"scoring"`
}

// OrdersScoringParams configures a batch scoring lookup.
type OrdersScoringParams struct {
	OrderIDs []string
}

// OrdersScoringResponse maps order IDs to their scoring status.
type OrdersScoringResponse map[string]bool

// CancelMarketOrdersRequest scopes cancelation to a market and/or asset.
type CancelMarketOrdersRequest struct {
	Market  string `json:"market,omitempty"`
	AssetID string `json:"asset_id,omitempty"`
}
