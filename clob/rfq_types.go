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

// RFQErrorCode is a typed error code returned by the RFQ service.
// Values match the upstream TypeScript and Python SDK enums.
type RFQErrorCode string

const (
	RFQCodeAddressMismatch                 RFQErrorCode = "ADDRESS_MISMATCH"
	RFQCodeAllowanceValidationFailed       RFQErrorCode = "ALLOWANCE_VALIDATION_FAILED"
	RFQCodeBalanceValidationFailed         RFQErrorCode = "BALANCE_VALIDATION_FAILED"
	RFQCodeContradictoryLegs               RFQErrorCode = "CONTRADICTORY_LEGS"
	RFQCodeExpiredRFQ                      RFQErrorCode = "EXPIRED_RFQ"
	RFQCodeInvalidAcceptance               RFQErrorCode = "INVALID_ACCEPTANCE"
	RFQCodeInvalidConfirmation             RFQErrorCode = "INVALID_CONFIRMATION"
	RFQCodeInvalidExecutionResult          RFQErrorCode = "INVALID_EXECUTION_RESULT"
	RFQCodeInvalidIdentity                 RFQErrorCode = "INVALID_IDENTITY"
	RFQCodeInvalidMessage                  RFQErrorCode = "INVALID_MESSAGE"
	RFQCodeInvalidQuote                    RFQErrorCode = "INVALID_QUOTE"
	RFQCodeInvalidRFQ                      RFQErrorCode = "INVALID_RFQ"
	RFQCodeInvalidRFQState                 RFQErrorCode = "INVALID_RFQ_STATE"
	RFQCodeInvalidRole                     RFQErrorCode = "INVALID_ROLE"
	RFQCodeInvalidSignature                RFQErrorCode = "INVALID_SIGNATURE"
	RFQCodeInternalError                   RFQErrorCode = "INTERNAL_ERROR"
	RFQCodeLegMetadataUnavailable          RFQErrorCode = "LEG_METADATA_UNAVAILABLE"
	RFQCodeMakerAlreadyResponded           RFQErrorCode = "MAKER_ALREADY_RESPONDED"
	RFQCodeMakerNotRequired                RFQErrorCode = "MAKER_NOT_REQUIRED"
	RFQCodeMakerQuoteLimited               RFQErrorCode = "MAKER_QUOTE_LIMITED"
	RFQCodePreExecBalanceReservationFailed RFQErrorCode = "PRE_EXECUTION_BALANCE_RESERVATION_FAILED"
	RFQCodeQuoteMismatch                   RFQErrorCode = "QUOTE_MISMATCH"
	RFQCodeQuoteUnavailable                RFQErrorCode = "QUOTE_UNAVAILABLE"
	RFQCodeQuoteValidationTimeoutInternal  RFQErrorCode = "QUOTE_VALIDATION_TIMEOUT_INTERNAL"
	RFQCodeRateLimited                     RFQErrorCode = "RATE_LIMITED"
	RFQCodeRequestFailed                   RFQErrorCode = "REQUEST_FAILED"
	RFQCodeServiceUnavailable              RFQErrorCode = "SERVICE_UNAVAILABLE"
	RFQCodeSubmissionWindowClosed          RFQErrorCode = "SUBMISSION_WINDOW_CLOSED"
	RFQCodeTradeSubmissionFailed           RFQErrorCode = "TRADE_SUBMISSION_FAILED"
	RFQCodeUnauthenticated                 RFQErrorCode = "UNAUTHENTICATED"
	RFQCodeUnauthorizedRole                RFQErrorCode = "UNAUTHORIZED_ROLE"
	RFQCodeUnknownRFQ                      RFQErrorCode = "UNKNOWN_RFQ"
)

const (
	RFQStatusActive   = "active"
	RFQStatusInactive = "inactive"
)

// RFQDirection is the direction of an RFQ (buy or sell).
type RFQDirection string

const (
	RFQDirectionBuy  RFQDirection = "BUY"
	RFQDirectionSell RFQDirection = "SELL"
)

// RFQSide is the outcome side of an RFQ.
type RFQSide string

const (
	RFQSideYes RFQSide = "YES"
	RFQSideNo  RFQSide = "NO"
)

// RFQQuoteSource is the source of an RFQ quote.
type RFQQuoteSource string

const (
	RFQSourceCollateral RFQQuoteSource = "collateral"
	RFQSourceInventory  RFQQuoteSource = "inventory"
)

// RFQRequestedSizeUnit is the unit for an RFQ requested size.
type RFQRequestedSizeUnit string

const (
	RFQSizeUnitNotional RFQRequestedSizeUnit = "notional"
	RFQSizeUnitShares   RFQRequestedSizeUnit = "shares"
)

// RFQRequestedSize is the size of an RFQ request.
type RFQRequestedSize struct {
	Unit  RFQRequestedSizeUnit `json:"unit"`
	Value string               `json:"value"`
}

// RFQConfirmationDecision is the decision for an RFQ confirmation.
type RFQConfirmationDecision string

const (
	RFQDecisionConfirm RFQConfirmationDecision = "CONFIRM"
	RFQDecisionDecline RFQConfirmationDecision = "DECLINE"
)

// RFQExecutionStatus is the execution status of an RFQ trade.
type RFQExecutionStatus string

const (
	RFQExecutionMatched   RFQExecutionStatus = "MATCHED"
	RFQExecutionMined     RFQExecutionStatus = "MINED"
	RFQExecutionConfirmed RFQExecutionStatus = "CONFIRMED"
	RFQExecutionRetrying  RFQExecutionStatus = "RETRYING"
	RFQExecutionFailed    RFQExecutionStatus = "FAILED"
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

// RFQError is a structured error returned by the RFQ WebSocket or REST API.
type RFQError struct {
	Code        RFQErrorCode   `json:"code"`
	ErrorID     string         `json:"errorId,omitzero"`
	Message     string         `json:"error"`
	RequestID   string         `json:"rfqId,omitzero"`
	QuoteID     string         `json:"quoteId,omitzero"`
	RequestType string         `json:"requestType,omitzero"`
	Request     map[string]any `json:"request,omitzero"`
}

// Error implements the error interface.
func (e *RFQError) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("rfq error %s: %s (rfq=%s)", e.Code, e.Message, e.RequestID)
	}
	return fmt.Sprintf("rfq error %s: %s", e.Code, e.Message)
}

// ComboMarketOutcome is a single outcome in a Combo market catalog entry.
type ComboMarketOutcome struct {
	Label      string `json:"label"`
	PositionID string `json:"positionId"`
	Price      string `json:"price"`
}

// ComboMarketOutcomes contains the yes/no outcomes for a binary Combo market.
type ComboMarketOutcomes struct {
	Yes ComboMarketOutcome `json:"yes"`
	No  ComboMarketOutcome `json:"no"`
}

// ComboMarket is a market available for Combo trading.
// Wire format uses flat arrays for outcomes/prices; the [ComboMarketOutcomes]
// accessor parses them into a structured yes/no pair.
type ComboMarket struct {
	ID            string   `json:"id"`
	ConditionID   string   `json:"condition_id"`
	Slug          string   `json:"slug"`
	Title         string   `json:"title"`
	Outcomes      []string `json:"outcomes"`
	OutcomePrices []string `json:"outcome_prices"`
	PositionIDs   []string `json:"position_ids"`
	Image         string   `json:"image"`
	Volume        float64  `json:"volume"`
	Tags          []string `json:"tags"`
}

// ParsedOutcomes returns the yes/no outcome pair.
// Returns an error if the market is not binary (exactly 2 outcomes).
func (m *ComboMarket) ParsedOutcomes() (ComboMarketOutcomes, error) {
	if len(m.Outcomes) != 2 {
		return ComboMarketOutcomes{}, fmt.Errorf("expected 2 outcomes, got %d", len(m.Outcomes))
	}
	if len(m.PositionIDs) != 2 {
		return ComboMarketOutcomes{}, fmt.Errorf(
			"expected 2 position IDs, got %d",
			len(m.PositionIDs),
		)
	}
	if len(m.OutcomePrices) != 2 {
		return ComboMarketOutcomes{}, fmt.Errorf(
			"expected 2 outcome prices, got %d",
			len(m.OutcomePrices),
		)
	}
	return ComboMarketOutcomes{
		Yes: ComboMarketOutcome{
			Label:      m.Outcomes[0],
			PositionID: m.PositionIDs[0],
			Price:      m.OutcomePrices[0],
		},
		No: ComboMarketOutcome{
			Label:      m.Outcomes[1],
			PositionID: m.PositionIDs[1],
			Price:      m.OutcomePrices[1],
		},
	}, nil
}

// ComboMarketsPage is a paginated response for combo market listings.
type ComboMarketsPage struct {
	Markets    []ComboMarket `json:"markets"`
	NextCursor string        `json:"next_cursor,omitzero"`
}

// ComboMarketFilterParams contains the filters for listing combo markets.
type ComboMarketFilterParams struct {
	Limit   int      `url:"limit,omitzero"`
	Cursor  string   `url:"cursor,omitzero"`
	Exclude []string `url:"exclude,omitzero"`
}

// RFQTradeEvent is a confirmed combo trade broadcast visible to all authenticated quoters.
type RFQTradeEvent struct {
	Type           string       `json:"type"`
	RFQID          string       `json:"rfq_id"`
	RequesterID    string       `json:"requester_id"`
	ConditionID    string       `json:"condition_id"`
	LegPositionIDs []string     `json:"leg_position_ids"`
	Direction      RFQDirection `json:"direction"`
	Side           RFQSide      `json:"side"`
	PriceE6        string       `json:"price_e6"`
	SizeE6         string       `json:"size_e6"`
	ExecutedAt     int64        `json:"executed_at"`
}
