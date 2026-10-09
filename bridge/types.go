package bridge

import "github.com/ethereum/go-ethereum/common"

// DepositRequest is the request to generate deposit addresses.
type DepositRequest struct {
	Address common.Address `json:"address"` // Polymarket wallet address
}

// DepositAddresses holds deposit addresses for different blockchain networks.
type DepositAddresses struct {
	EVM common.Address `json:"evm"`
	SVM string         `json:"svm"`
	BTC string         `json:"btc"`
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
	ChainID        ChainID `json:"chainId"`
	ChainName      string  `json:"chainName"`
	Token          Token   `json:"token"`
	MinCheckoutUSD Decimal `json:"minCheckoutUsd"`
}

// SupportedAssetsResponse is the response from the /supported-assets endpoint.
type SupportedAssetsResponse struct {
	SupportedAssets []SupportedAsset `json:"supportedAssets"`
	Note            string           `json:"note,omitzero"`
}

// QuoteRequest is a request for a bridge quote.
type QuoteRequest struct {
	FromAmountBaseUnit BaseUnits `json:"fromAmountBaseUnit"`
	FromChainID        ChainID   `json:"fromChainId"`
	FromTokenAddress   string    `json:"fromTokenAddress"`
	RecipientAddress   string    `json:"recipientAddress"`
	ToChainID          ChainID   `json:"toChainId"`
	ToTokenAddress     string    `json:"toTokenAddress"`
}

// EstimatedFeeBreakdown holds the fee breakdown for a quote.
type EstimatedFeeBreakdown struct {
	AppFeeLabel     string  `json:"appFeeLabel"`
	AppFeePercent   Decimal `json:"appFeePercent"`
	AppFeeUSD       Decimal `json:"appFeeUsd"`
	FillCostPercent Decimal `json:"fillCostPercent"`
	FillCostUSD     Decimal `json:"fillCostUsd"`
	GasUSD          Decimal `json:"gasUsd"`
	MaxSlippage     Decimal `json:"maxSlippage"`
	MinReceived     Decimal `json:"minReceived"`
	SwapImpact      Decimal `json:"swapImpact"`
	SwapImpactUSD   Decimal `json:"swapImpactUsd"`
	TotalImpact     Decimal `json:"totalImpact"`
	TotalImpactUSD  Decimal `json:"totalImpactUsd"`
}

// QuoteResponse is the response from the /quote endpoint.
type QuoteResponse struct {
	QuoteID            string                `json:"quoteId"`
	EstCheckoutTimeMs  uint64                `json:"estCheckoutTimeMs"`
	EstFeeBreakdown    EstimatedFeeBreakdown `json:"estFeeBreakdown"`
	EstInputUSD        Decimal               `json:"estInputUsd"`
	EstOutputUSD       Decimal               `json:"estOutputUsd"`
	EstToTokenBaseUnit BaseUnits             `json:"estToTokenBaseUnit"`
}

// WithdrawRequest is a request to withdraw assets from Polymarket via the bridge.
type WithdrawRequest struct {
	Address        common.Address `json:"address"`        // Source Polymarket wallet address on Polygon
	ToChainID      ChainID        `json:"toChainId"`      // Destination chain ID
	ToTokenAddress string         `json:"toTokenAddress"` // Destination token contract address
	RecipientAddr  string         `json:"recipientAddr"`  // Destination wallet address
}

// WithdrawalAddresses holds withdrawal destination addresses for different networks.
type WithdrawalAddresses struct {
	EVM common.Address `json:"evm"`
	SVM string         `json:"svm"`
	BTC string         `json:"btc"`
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
	FromChainID        ChainID                  `json:"fromChainId"`
	FromTokenAddress   string                   `json:"fromTokenAddress"`
	FromAmountBaseUnit BaseUnits                `json:"fromAmountBaseUnit"`
	ToChainID          ChainID                  `json:"toChainId"`
	ToTokenAddress     common.Address           `json:"toTokenAddress"`
	Status             DepositTransactionStatus `json:"status"`
	TxHash             *string                  `json:"txHash,omitzero"`
	CreatedTimeMs      *uint64                  `json:"createdTimeMs,omitzero"`
}

// StatusResponse is the response from the /status endpoint.
type StatusResponse struct {
	Transactions []DepositTransaction `json:"transactions"`
}
