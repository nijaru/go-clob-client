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

type builderAPIKeyRaw struct {
	Key        string `json:"key"`
	APIKey     string `json:"apiKey"`
	Secret     string `json:"secret"`
	Passphrase string `json:"passphrase"`
}

func (r builderAPIKeyRaw) credentials() Credentials {
	key := r.Key
	if key == "" {
		key = r.APIKey
	}
	return Credentials{
		Key:        key,
		Secret:     r.Secret,
		Passphrase: r.Passphrase,
	}
}

// WSAuth contains the raw credentials for authenticated CLOB websocket
// subscriptions. Unlike HTTP L2 auth, the websocket user channel does not
// use an HMAC timestamp/signature envelope.
type WSAuth struct {
	Key        string `json:"apiKey"`
	Secret     string `json:"secret"`
	Passphrase string `json:"passphrase"`
	Timestamp  string `json:"timestamp,omitempty"`
	Signature  string `json:"signature,omitempty"`
}

// APIKeysResponse is the response payload for listing API keys.
type APIKeysResponse struct {
	APIKeys []string `json:"apiKeys"`
}

// BanStatus reports whether the account is currently restricted to closed-only mode.
type BanStatus struct {
	ClosedOnly bool `json:"closed_only"`
}

// ReadonlyAPIKeyResponse is the response from creating a readonly API key.
type ReadonlyAPIKeyResponse struct {
	APIKey string `json:"apiKey"`
}

// ReadonlyAPIKeysResponse is the response from listing readonly API keys.
type ReadonlyAPIKeysResponse struct {
	ReadonlyAPIKeys []string `json:"readonly_api_keys"`
}

// DeleteReadonlyAPIKeyRequest is the request payload for removing a readonly API key.
type DeleteReadonlyAPIKeyRequest struct {
	Key string `json:"key"`
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
	Balance    DecimalString            `json:"balance"`
	Allowances map[string]DecimalString `json:"allowances"`
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
	Market  string `json:"market,omitzero"`
	AssetID string `json:"asset_id,omitzero"`
}
