package clob

import (
	"fmt"
	"strconv"

	json "github.com/go-json-experiment/json"

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

// RFQSignedOrder is the V1-style signed order used in RFQ accept/approve payloads.
// RFQ endpoints use V1 wire format (taker, nonce, feeRateBps) independently
// of the main order signing path.
type RFQSignedOrder struct {
	Salt          string        `json:"salt"`
	Maker         string        `json:"maker"`
	Signer        string        `json:"signer"`
	Taker         string        `json:"taker"`
	TokenID       string        `json:"tokenId"`
	MakerAmount   string        `json:"makerAmount"`
	TakerAmount   string        `json:"takerAmount"`
	Expiration    string        `json:"expiration"`
	Nonce         string        `json:"nonce"`
	FeeRateBps    string        `json:"feeRateBps"`
	Side          Side          `json:"side"`
	SignatureType SignatureType `json:"signatureType"`
	Signature     string        `json:"signature"`
}

// wireRFQOrder is the shared wire format for RFQ accept/approve payloads.
type wireRFQOrder struct {
	RequestID   string `json:"requestId"`
	QuoteID     string `json:"quoteId"`
	Owner       string `json:"owner"`
	Salt        uint64 `json:"salt"`
	Maker       string `json:"maker"`
	Signer      string `json:"signer"`
	Taker       string `json:"taker"`
	TokenID     string `json:"tokenId"`
	MakerAmount string `json:"makerAmount"`
	TakerAmount string `json:"takerAmount"`
	Expiration  uint64 `json:"expiration"`
	Nonce       uint64 `json:"nonce"`
	FeeRateBps  string `json:"feeRateBps"`
	Side        Side   `json:"side"`
	Signature   string `json:"signature"`
}

// marshalRFQOrder encodes an RFQ accept/approve payload with numeric salt, expiration, and nonce.
func marshalRFQOrder(requestID, quoteID, owner string, o RFQSignedOrder) ([]byte, error) {
	salt, err := strconv.ParseUint(o.Salt, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse order salt: %w", err)
	}
	expiration, err := strconv.ParseUint(o.Expiration, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse order expiration: %w", err)
	}
	nonce, err := strconv.ParseUint(o.Nonce, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse order nonce: %w", err)
	}

	return json.Marshal(wireRFQOrder{
		RequestID:   requestID,
		QuoteID:     quoteID,
		Owner:       owner,
		Salt:        salt,
		Maker:       o.Maker,
		Signer:      o.Signer,
		Taker:       o.Taker,
		TokenID:     o.TokenID,
		MakerAmount: o.MakerAmount,
		TakerAmount: o.TakerAmount,
		Expiration:  expiration,
		Nonce:       nonce,
		FeeRateBps:  o.FeeRateBps,
		Side:        o.Side,
		Signature:   o.Signature,
	})
}

// AcceptRFQQuoteRequest is the payload for accepting a specific quote.
// It includes the signed order from the requester.
type AcceptRFQQuoteRequest struct {
	RequestID string `json:"requestId"`
	QuoteID   string `json:"quoteId"`
	Owner     string `json:"owner"`
	RFQSignedOrder
}

// MarshalJSON encodes the accept request with salt and expiration as JSON numbers.
func (r AcceptRFQQuoteRequest) MarshalJSON() ([]byte, error) {
	return marshalRFQOrder(r.RequestID, r.QuoteID, r.Owner, r.RFQSignedOrder)
}

// AcceptRFQQuoteResponse is the response for accepting a quote.
// The server returns plain text "OK"; this type is a unit confirmation.
type AcceptRFQQuoteResponse struct{}

// ApproveRFQOrderResponse is the response for approving an RFQ order.
// It returns the resulting trade IDs queued for on-chain execution.
type ApproveRFQOrderResponse struct {
	TradeIDs []string `json:"tradeIds"`
}

// ApproveRFQOrderRequest is the payload for a quoter to approve an order.
type ApproveRFQOrderRequest struct {
	RequestID string `json:"requestId"`
	QuoteID   string `json:"quoteId"`
	Owner     string `json:"owner"`
	RFQSignedOrder
}

// MarshalJSON encodes the approve request with salt and expiration as JSON numbers.
func (r ApproveRFQOrderRequest) MarshalJSON() ([]byte, error) {
	return marshalRFQOrder(r.RequestID, r.QuoteID, r.Owner, r.RFQSignedOrder)
}

// RFQRequestResponse is the response for creating a RFQ request.
type RFQRequestResponse struct {
	RequestID string `json:"requestId"`
	Expiry    int64  `json:"expiry,omitzero"`
	Error     string `json:"error,omitzero"`
}

// RFQQuoteResponse is the response for creating a RFQ quote.
type RFQQuoteResponse struct {
	QuoteID string `json:"quoteId"`
	Error   string `json:"error,omitzero"`
}

// RFQRequestsResponse is the response for listing RFQ requests.
type RFQRequestsResponse Page[RFQRequest]

// RFQQuotesResponse is the response for listing RFQ quotes.
type RFQQuotesResponse Page[RFQQuote]

// RFQRequestFilterParams contains the filters for listing RFQ requests.
type RFQRequestFilterParams struct {
	Limit       int      `url:"limit,omitzero"`
	Offset      string   `url:"offset,omitzero"`
	State       string   `url:"state,omitzero"`
	RequestIDs  []string `url:"requestIds,omitzero"`
	Markets     []string `url:"markets,omitzero"`
	SizeMin     string   `url:"sizeMin,omitzero"`
	SizeMax     string   `url:"sizeMax,omitzero"`
	SizeUSDcMin string   `url:"sizeUsdcMin,omitzero"`
	SizeUSDcMax string   `url:"sizeUsdcMax,omitzero"`
	PriceMin    string   `url:"priceMin,omitzero"`
	PriceMax    string   `url:"priceMax,omitzero"`
	SortBy      string   `url:"sortBy,omitzero"`
	SortDir     string   `url:"sortDir,omitzero"`
}

// RFQQuoteFilterParams contains the filters for listing RFQ quotes.
type RFQQuoteFilterParams struct {
	Limit       int      `url:"limit,omitzero"`
	Offset      string   `url:"offset,omitzero"`
	State       string   `url:"state,omitzero"`
	QuoteIDs    []string `url:"quoteIds,omitzero"`
	RequestIDs  []string `url:"requestIds,omitzero"`
	Markets     []string `url:"markets,omitzero"`
	SizeMin     string   `url:"sizeMin,omitzero"`
	SizeMax     string   `url:"sizeMax,omitzero"`
	SizeUSDcMin string   `url:"sizeUsdcMin,omitzero"`
	SizeUSDcMax string   `url:"sizeUsdcMax,omitzero"`
	PriceMin    string   `url:"priceMin,omitzero"`
	PriceMax    string   `url:"priceMax,omitzero"`
	SortBy      string   `url:"sortBy,omitzero"`
	SortDir     string   `url:"sortDir,omitzero"`
}
