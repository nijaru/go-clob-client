package bridge

// DepositRequest is the request to generate deposit addresses.
type DepositRequest struct {
	Address string `json:"address"` // Polymarket wallet address
}

// DepositAddresses holds deposit addresses for different blockchain networks.
type DepositAddresses struct {
	EVM string `json:"evm"`
	SVM string `json:"svm"`
	BTC string `json:"btc"`
}

// DepositResponse is the response from the /deposit endpoint.
type DepositResponse struct {
	Address DepositAddresses `json:"address"`
	Note    string           `json:"note,omitzero"`
}

// Token holds token information for a supported asset.
type Token struct {
	Name     string `json:"name"`
	Symbol   string `json:"symbol"`
	Address  string `json:"address"`
	Decimals uint8  `json:"decimals"`
}

// SupportedAsset is a supported asset with chain and token information.
type SupportedAsset struct {
	ChainID        string `json:"chainId"`
	ChainName      string `json:"chainName"`
	Token          Token  `json:"token"`
	MinCheckoutUSD string `json:"minCheckoutUsd"`
}

// SupportedAssetsResponse is the response from the /supported-assets endpoint.
type SupportedAssetsResponse struct {
	SupportedAssets []SupportedAsset `json:"supportedAssets"`
	Note            string           `json:"note,omitzero"`
}

// QuoteRequest is a request for a bridge quote.
type QuoteRequest struct {
	FromAmountBaseUnit string `json:"fromAmountBaseUnit"`
	FromChainID        string `json:"fromChainId"`
	FromTokenAddress   string `json:"fromTokenAddress"`
	RecipientAddress   string `json:"recipientAddress"`
	ToChainID          string `json:"toChainId"`
	ToTokenAddress     string `json:"toTokenAddress"`
}

// EstimatedFeeBreakdown holds the fee breakdown for a quote.
type EstimatedFeeBreakdown struct {
	AppFeeLabel     string  `json:"appFeeLabel"`
	AppFeePercent   float64 `json:"appFeePercent"`
	AppFeeUSD       float64 `json:"appFeeUsd"`
	FillCostPercent float64 `json:"fillCostPercent"`
	FillCostUSD     float64 `json:"fillCostUsd"`
	GasUSD          float64 `json:"gasUsd"`
	MaxSlippage     float64 `json:"maxSlippage"`
	MinReceived     float64 `json:"minReceived"`
	SwapImpact      float64 `json:"swapImpact"`
	SwapImpactUSD   float64 `json:"swapImpactUsd"`
	TotalImpact     float64 `json:"totalImpact"`
	TotalImpactUSD  float64 `json:"totalImpactUsd"`
}

// QuoteResponse is the response from the /quote endpoint.
type QuoteResponse struct {
	QuoteID            string                `json:"quoteId"`
	EstCheckoutTimeMs  uint64                `json:"estCheckoutTimeMs"`
	EstFeeBreakdown    EstimatedFeeBreakdown `json:"estFeeBreakdown"`
	EstInputUSD        float64               `json:"estInputUsd"`
	EstOutputUSD       float64               `json:"estOutputUsd"`
	EstToTokenBaseUnit string                `json:"estToTokenBaseUnit"`
}

// WithdrawRequest is a request to withdraw assets from Polymarket via the bridge.
type WithdrawRequest struct {
	Address        string `json:"address"`        // Source Polymarket wallet address on Polygon
	ToChainID      string `json:"toChainId"`      // Destination chain ID
	ToTokenAddress string `json:"toTokenAddress"` // Destination token contract address
	RecipientAddr  string `json:"recipientAddr"`  // Destination wallet address
}

// WithdrawalAddresses holds withdrawal destination addresses for different networks.
type WithdrawalAddresses struct {
	EVM string `json:"evm"`
	SVM string `json:"svm"`
	BTC string `json:"btc"`
}

// WithdrawResponse is the response from the /withdraw endpoint.
type WithdrawResponse struct {
	Address WithdrawalAddresses `json:"address"`
	Note    string              `json:"note"`
}

// DepositTransactionStatus is the status of a bridge deposit transaction.
type DepositTransactionStatus string

const (
	DepositStatusDetected        DepositTransactionStatus = "DEPOSIT_DETECTED"
	DepositStatusProcessing      DepositTransactionStatus = "PROCESSING"
	DepositStatusOriginConfirmed DepositTransactionStatus = "ORIGIN_TX_CONFIRMED"
	DepositStatusSubmitted       DepositTransactionStatus = "SUBMITTED"
	DepositStatusCompleted       DepositTransactionStatus = "COMPLETED"
	DepositStatusFailed          DepositTransactionStatus = "FAILED"
)

// DepositTransaction represents a single bridge deposit transaction.
type DepositTransaction struct {
	FromChainID        string                   `json:"fromChainId"`
	FromTokenAddress   string                   `json:"fromTokenAddress"`
	FromAmountBaseUnit string                   `json:"fromAmountBaseUnit"`
	ToChainID          string                   `json:"toChainId"`
	ToTokenAddress     string                   `json:"toTokenAddress"`
	Status             DepositTransactionStatus `json:"status"`
	TxHash             *string                  `json:"txHash,omitzero"`
	CreatedTimeMs      *uint64                  `json:"createdTimeMs,omitzero"`
}

// StatusResponse is the response from the /status endpoint.
type StatusResponse struct {
	Transactions []DepositTransaction `json:"transactions"`
}
