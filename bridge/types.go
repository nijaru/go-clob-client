package bridge

// SupportedAsset represents a token supported on a specific chain for bridging.
type SupportedAsset struct {
	ChainID      string `json:"chainId"`
	ChainName    string `json:"chainName"`
	TokenAddress string `json:"tokenAddress"`
	TokenSymbol  string `json:"tokenSymbol"`
	MinDeposit   string `json:"minDeposit,omitzero"`
}

// SupportedAssetsResponse is the response from the /supported-assets endpoint.
type SupportedAssetsResponse struct {
	Assets []SupportedAsset `json:"assets"`
}

// DepositRequest is the request to generate a deposit address.
type DepositRequest struct {
	Address string `json:"address"` // Polymarket wallet address
}

// DepositAddress represents a unique deposit address for a specific network.
type DepositAddress struct {
	Network string `json:"network"`
	Address string `json:"address"`
}

// DepositResponse is the response from the /deposit endpoint.
type DepositResponse struct {
	Addresses []DepositAddress `json:"addresses"`
}

// QuoteRequest represents a request for a bridge quote.
type QuoteRequest struct {
	FromChain   string  `json:"fromChain"`
	ToChain     string  `json:"toChain"`
	FromToken   string  `json:"fromToken"`
	ToToken     string  `json:"toToken"`
	Amount      string  `json:"amount"`
	UserAddress string  `json:"userAddress"`
	Slippage    float64 `json:"slippage,omitzero"`
}

// QuoteResponse represents a bridge quote.
type QuoteResponse struct {
	QuoteID       string `json:"quoteId"`
	FromAmount    string `json:"fromAmount"`
	ToAmount      string `json:"toAmount"`
	Fee           string `json:"fee"`
	EstimatedTime int    `json:"estimatedTime"` // in seconds
}

// WithdrawRequest represents a request to withdraw assets via the bridge (2026 standard).
type WithdrawRequest struct {
	ToAddress string `json:"toAddress"`
	Amount    string `json:"amount"`
	FromToken string `json:"fromToken"`
	ToToken   string `json:"toToken"`
	ToChain   string `json:"toChain"`
}

// WithdrawResponse represents the response from a withdrawal request.
type WithdrawResponse struct {
	TransactionID string `json:"transactionId"`
	Status        string `json:"status"`
}

// StatusResponse represents the status of bridge transactions for an address.
type StatusResponse struct {
	Transactions []BridgeTransaction `json:"transactions"`
}

// BridgeTransaction represents a single bridge transaction.
type BridgeTransaction struct {
	ID                   string `json:"id"`
	Status               string `json:"status"`
	FromAmountBaseUnit   string `json:"fromAmountBaseUnit"`
	FromTokenAddress     string `json:"fromTokenAddress"`
	FromChainID          string `json:"fromChainId"`
	ToChainID            string `json:"toChainId"`
	TransactionHash      string `json:"transactionHash"`
}
