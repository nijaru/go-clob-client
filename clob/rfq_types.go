package clob

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/quagmt/udecimal"
)

// RFQRequest identifies a single Request for Quote.
type RFQRequest struct {
	ID              string  `json:"requestId"`
	UserAddress     string  `json:"userAddress"`
	ProxyAddress    string  `json:"proxyAddress"`
	Token           string  `json:"token"`
	Complement      string  `json:"complement"`
	Condition       string  `json:"condition"`
	Side            string  `json:"side"`
	SizeIn          string  `json:"sizeIn"`
	SizeOut         string  `json:"sizeOut"`
	Price           float64 `json:"price"`
	AcceptedQuoteID string  `json:"acceptedQuoteId"`
	State           string  `json:"state"`
	Expiry          string  `json:"expiry"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
}

// RFQQuote identifies a single quote responded to an RFQ request.
type RFQQuote struct {
	ID           string  `json:"quoteId"`
	RequestID    string  `json:"requestId"`
	UserAddress  string  `json:"userAddress"`
	ProxyAddress string  `json:"proxyAddress"`
	Token        string  `json:"token"`
	Complement   string  `json:"complement"`
	Condition    string  `json:"condition"`
	Side         string  `json:"side"`
	SizeIn       string  `json:"sizeIn"`
	SizeOut      string  `json:"sizeOut"`
	Price        float64 `json:"price"`
	State        string  `json:"state"`
	Expiry       string  `json:"expiry"`
	MatchType    string  `json:"matchType"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    string  `json:"updatedAt"`
}

const (
	RFQStatusActive   = "active"
	RFQStatusInactive = "inactive"
)

// CreateRFQRequestParams contains the inputs for creating a new RFQ request.
type CreateRFQRequestParams struct {
	AssetIn   string           `json:"assetIn"`
	AssetOut  string           `json:"assetOut"`
	AmountIn  udecimal.Decimal `json:"amountIn"`
	AmountOut udecimal.Decimal `json:"amountOut"`
	UserType  int              `json:"userType"` // 0=EOA, 1=POLY_PROXY, 2=POLY_GNOSIS_SAFE
}

// CreateRFQQuoteParams contains the inputs for a quoter to respond to an RFQ request.
type CreateRFQQuoteParams struct {
	RequestID string           `json:"requestId"`
	AssetIn   string           `json:"assetIn"`
	AssetOut  string           `json:"assetOut"`
	AmountIn  udecimal.Decimal `json:"amountIn"`
	AmountOut udecimal.Decimal `json:"amountOut"`
	UserType  int              `json:"userType"`
}

// AcceptRFQQuoteRequest is the payload for accepting a specific quote.
// It includes the signed order from the requester.
type AcceptRFQQuoteRequest struct {
	RequestID string `json:"requestId"`
	QuoteID   string `json:"quoteId"`
	Owner     string `json:"owner"`
	SignedOrder
}

// MarshalJSON encodes the accept request with salt and expiration as JSON numbers.
func (r AcceptRFQQuoteRequest) MarshalJSON() ([]byte, error) {
	salt, err := strconv.ParseUint(r.Salt, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse order salt: %w", err)
	}
	expiration, err := strconv.ParseUint(r.Expiration, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse order expiration: %w", err)
	}

	type wireAccept struct {
		RequestID     string        `json:"requestId"`
		QuoteID       string        `json:"quoteId"`
		Owner         string        `json:"owner"`
		Salt          uint64        `json:"salt"`
		Maker         string        `json:"maker"`
		Signer        string        `json:"signer"`
		Taker         string        `json:"taker"`
		TokenID       string        `json:"tokenId"`
		MakerAmount   string        `json:"makerAmount"`
		TakerAmount   string        `json:"takerAmount"`
		Expiration    uint64        `json:"expiration"`
		Nonce         string        `json:"nonce"`
		FeeRateBps    string        `json:"feeRateBps"`
		Side          Side          `json:"side"`
		SignatureType SignatureType `json:"signatureType"`
		Signature     string        `json:"signature"`
	}

	return json.Marshal(wireAccept{
		RequestID:     r.RequestID,
		QuoteID:       r.QuoteID,
		Owner:         r.Owner,
		Salt:          salt,
		Maker:         r.Maker,
		Signer:        r.Signer,
		Taker:         r.Taker,
		TokenID:       r.TokenID,
		MakerAmount:   r.MakerAmount,
		TakerAmount:   r.TakerAmount,
		Expiration:    expiration,
		Nonce:         r.Nonce,
		FeeRateBps:    r.FeeRateBps,
		Side:          r.Side,
		SignatureType: r.SignatureType,
		Signature:     r.Signature,
	})
}

// AcceptRFQQuoteResponse is the response for accepting a quote.
// It returns the resulting trade IDs.
type AcceptRFQQuoteResponse struct {
	TradeIDs []string `json:"tradeIds"`
}

// ApproveRFQOrderRequest is the payload for a quoter to approve an order.
type ApproveRFQOrderRequest struct {
	RequestID string `json:"requestId"`
	QuoteID   string `json:"quoteId"`
	Owner     string `json:"owner"`
	SignedOrder
}

// MarshalJSON encodes the approve request with salt and expiration as JSON numbers.
func (r ApproveRFQOrderRequest) MarshalJSON() ([]byte, error) {
	salt, err := strconv.ParseUint(r.Salt, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse order salt: %w", err)
	}
	expiration, err := strconv.ParseUint(r.Expiration, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse order expiration: %w", err)
	}

	type wireApprove struct {
		RequestID     string        `json:"requestId"`
		QuoteID       string        `json:"quoteId"`
		Owner         string        `json:"owner"`
		Salt          uint64        `json:"salt"`
		Maker         string        `json:"maker"`
		Signer        string        `json:"signer"`
		Taker         string        `json:"taker"`
		TokenID       string        `json:"tokenId"`
		MakerAmount   string        `json:"makerAmount"`
		TakerAmount   string        `json:"takerAmount"`
		Expiration    uint64        `json:"expiration"`
		Nonce         string        `json:"nonce"`
		FeeRateBps    string        `json:"feeRateBps"`
		Side          Side          `json:"side"`
		SignatureType SignatureType `json:"signatureType"`
		Signature     string        `json:"signature"`
	}

	return json.Marshal(wireApprove{
		RequestID:     r.RequestID,
		QuoteID:       r.QuoteID,
		Owner:         r.Owner,
		Salt:          salt,
		Maker:         r.Maker,
		Signer:        r.Signer,
		Taker:         r.Taker,
		TokenID:       r.TokenID,
		MakerAmount:   r.MakerAmount,
		TakerAmount:   r.TakerAmount,
		Expiration:    expiration,
		Nonce:         r.Nonce,
		FeeRateBps:    r.FeeRateBps,
		Side:          r.Side,
		SignatureType: r.SignatureType,
		Signature:     r.Signature,
	})
}

// RFQRequestResponse is the response for creating an RFQ request.
type RFQRequestResponse struct {
	RequestID string `json:"requestId"`
	Error     string `json:"error,omitempty"`
}

// RFQQuoteResponse is the response for creating an RFQ quote.
type RFQQuoteResponse struct {
	QuoteID string `json:"quoteId"`
	Error   string `json:"error,omitempty"`
}

// RFQRequestsResponse is the response for listing RFQ requests.
type RFQRequestsResponse Page[RFQRequest]

// RFQQuotesResponse is the response for listing RFQ quotes.
type RFQQuotesResponse Page[RFQQuote]

// RFQRequestFilterParams contains the filters for listing RFQ requests.
type RFQRequestFilterParams struct {
	Limit      int      `url:"limit,omitempty"`
	Offset     string   `url:"offset,omitempty"`
	State      string   `url:"state,omitempty"`
	RequestIDs []string `url:"requestIds,omitempty"`
	Markets    []string `url:"markets,omitempty"`
}

// RFQQuoteFilterParams contains the filters for listing RFQ quotes.
type RFQQuoteFilterParams struct {
	Limit      int      `url:"limit,omitempty"`
	Offset     string   `url:"offset,omitempty"`
	RequestIDs []string `url:"requestIds,omitempty"`
	QuoteIDs   []string `url:"quoteIds,omitempty"`
}
