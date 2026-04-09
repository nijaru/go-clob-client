package bridge

import (
	"fmt"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	json "github.com/go-json-experiment/json"
	"github.com/quagmt/udecimal"
)

// ChainID identifies an EVM or bridge-supported chain.
// It marshals to the wire format Polymarket expects: a base-10 JSON string.
type ChainID int64

func (c ChainID) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatInt(int64(c), 10))
}

func (c *ChainID) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err == nil {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("parse chain id: %w", err)
		}
		*c = ChainID(parsed)
		return nil
	}

	var numeric int64
	if err := json.Unmarshal(data, &numeric); err != nil {
		return fmt.Errorf("decode chain id: %w", err)
	}
	*c = ChainID(numeric)
	return nil
}

// Decimal is the numeric type used for bridge decimal amounts.
type Decimal = udecimal.Decimal

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
	FromAmountBaseUnit string  `json:"fromAmountBaseUnit"`
	FromChainID        ChainID `json:"fromChainId"`
	FromTokenAddress   string  `json:"fromTokenAddress"`
	RecipientAddress   string  `json:"recipientAddress"`
	ToChainID          ChainID `json:"toChainId"`
	ToTokenAddress     string  `json:"toTokenAddress"`
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
	FromAmountBaseUnit string                   `json:"fromAmountBaseUnit"`
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
