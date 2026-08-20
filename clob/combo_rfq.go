package clob

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/nijaru/go-clob-client/internal/polyauth"
	"github.com/nijaru/go-clob-client/internal/polyhttp"
	"github.com/quagmt/udecimal"
)

// Combo RFQ accept-outcome timing, mirroring the official TypeScript SDK:
// the gateway holds the accept through maker last look and responds with
// AWAITING_MAKER_CONFIRMATION when the outcome is still pending after its
// hold window; the SDK then resumes over the status endpoint.
const (
	comboAcceptOutcomeTimeout   = 30 * time.Second
	comboAcceptOutcomePollEvery = 500 * time.Millisecond
	comboMaxLegPositionIDs      = 50
	comboMinLegPositionIDs      = 2
)

// ComboRFQStatus is the lifecycle or execution status of a combo RFQ as
// reported by the builder gateway. The gateway merges on-chain execution
// progress into the top-level status: MATCHED surfaces as Executing.
type ComboRFQStatus string

const (
	ComboRFQAwaitingRequesterAcceptance ComboRFQStatus = "AWAITING_REQUESTER_ACCEPTANCE"
	ComboRFQAwaitingMakerConfirmation   ComboRFQStatus = "AWAITING_MAKER_CONFIRMATION"
	ComboRFQExecuting                   ComboRFQStatus = "EXECUTING"
	ComboRFQFilled                      ComboRFQStatus = "FILLED"
	ComboRFQFailed                      ComboRFQStatus = "FAILED"
	ComboRFQExpired                     ComboRFQStatus = "EXPIRED"
	ComboRFQCanceled                    ComboRFQStatus = "CANCELED"

	// Execution statuses merged into the top-level status projection.
	ComboRFQMatched   ComboRFQStatus = "MATCHED"
	ComboRFQMined     ComboRFQStatus = "MINED"
	ComboRFQConfirmed ComboRFQStatus = "CONFIRMED"
	ComboRFQRetrying  ComboRFQStatus = "RETRYING"
)

// Terminal reports whether the status is a final outcome for an acceptance.
func (s ComboRFQStatus) Terminal() bool {
	switch s {
	case ComboRFQFilled, ComboRFQFailed, ComboRFQExpired, ComboRFQCanceled:
		return true
	default:
		return false
	}
}

// Executing reports whether the trade was handed off for on-chain execution.
func (s ComboRFQStatus) Executing() bool {
	switch s {
	case ComboRFQExecuting, ComboRFQMatched, ComboRFQMined, ComboRFQConfirmed, ComboRFQRetrying:
		return true
	default:
		return false
	}
}

// ComboQuoteUnavailableReason is the reason no usable quote was returned for
// a combo quote request. A request that attracts no quotes is a normal
// outcome, not an error.
type ComboQuoteUnavailableReason string

const (
	ComboQuoteNoQuotes     ComboQuoteUnavailableReason = "NO_QUOTES"
	ComboQuoteSizeTooLarge ComboQuoteUnavailableReason = "SIZE_TOO_LARGE"
)

// ComboAcceptFailureReason is the reason an accepted combo quote did not
// proceed to a fill.
type ComboAcceptFailureReason string

const (
	ComboAcceptMakerDeclined   ComboAcceptFailureReason = "MAKER_DECLINED"
	ComboAcceptWindowExpired   ComboAcceptFailureReason = "ACCEPTANCE_WINDOW_EXPIRED"
	ComboAcceptExecutionFailed ComboAcceptFailureReason = "EXECUTION_FAILED"
)

// BuilderRfqError is a structured error carried inside builder gateway
// responses. The gateway error-code vocabulary grows independently of SDK
// releases, so Code is a plain string.
type BuilderRfqError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error implements the error interface.
func (e *BuilderRfqError) Error() string {
	return fmt.Sprintf("combo rfq error %s: %s", e.Code, e.Message)
}

// ComboQuote is the winning quote for a combo quote request. Amounts are
// human-readable decimals converted from the gateway's e6 base units.
type ComboQuote struct {
	QuoteID string
	// BlendedPrice is the blended price across legs, in collateral per
	// outcome token.
	BlendedPrice string
	// MakerAmount is the requester's acceptance-order maker amount. For a
	// BUY this is collateral in USDC; for a SELL it is outcome tokens.
	MakerAmount string
	// TakerAmount is the requester's acceptance-order taker amount.
	TakerAmount string
	// TotalRequired is the total collateral (BUY) or position-share (SELL)
	// balance required to accept.
	TotalRequired string
	// ExpiresAt is the acceptance deadline as Unix milliseconds.
	ExpiresAt int64
}

// RequestComboQuoteParams describes a combo quote request.
type RequestComboQuoteParams struct {
	// LegPositionIDs are the position IDs of the combo legs: between 2 and
	// 50, no duplicates.
	LegPositionIDs []string
	// Direction is the trade direction: BUY spends collateral, SELL spends
	// outcome tokens.
	Direction RFQDirection
	// Amount is the collateral to spend in USDC when Direction is BUY,
	// including fees.
	Amount udecimal.Decimal
	// Size is the outcome tokens to sell when Direction is SELL.
	Size udecimal.Decimal
}

// RequestComboQuoteResult is the outcome of a combo quote request. When no
// usable quote was returned, Quote is nil and Reason explains why.
type RequestComboQuoteResult struct {
	RFQID string
	// Quote is the winning quote, or nil when none was returned.
	Quote *ComboQuote
	// Direction echoes the requested trade direction.
	Direction RFQDirection
	// YesPositionID is the combo YES position the acceptance order trades.
	YesPositionID string
	NoPositionID  string
	// ConditionID is the combo condition backing the position.
	ConditionID string
	// BuilderCode is the builder code the acceptance order must carry.
	BuilderCode string
	// Status is the gateway-reported RFQ status.
	Status ComboRFQStatus
	// Reason explains a nil Quote.
	Reason ComboQuoteUnavailableReason
	// Error carries the raw gateway error behind a rejection, when provided.
	Error *BuilderRfqError
}

// ComboQuoteReference identifies the quote being accepted, with the
// human-readable amounts returned alongside it.
type ComboQuoteReference struct {
	QuoteID string
	// MakerAmount is the quote's maker amount in human-readable units.
	MakerAmount string
	// TakerAmount is the quote's taker amount in human-readable units.
	TakerAmount string
}

// AcceptComboQuoteParams describes a combo quote acceptance.
type AcceptComboQuoteParams struct {
	// RFQID is the RFQ identifier returned by the quote request.
	RFQID string
	// Direction is the trade direction of the original request.
	Direction Side
	// PositionID is the combo YES position the acceptance order trades.
	PositionID string
	// BuilderCode is attached to the signed acceptance order.
	BuilderCode string
	// Quote is the winning quote to accept.
	Quote ComboQuoteReference
}

// AcceptComboQuoteResult is the outcome of accepting a combo quote. Executing
// means the trade was handed off for on-chain execution; a failed outcome is
// a normal result, not an error.
type AcceptComboQuoteResult struct {
	Status ComboRFQStatus
	RFQID  string
	// TakerOrderHash is the hash of the submitted acceptance order.
	TakerOrderHash string
	// TxHash is the on-chain transaction hash once execution starts.
	TxHash string
	// Reason explains a failed acceptance.
	Reason ComboAcceptFailureReason
	// Error carries the raw gateway error behind the failure, when provided.
	Error *BuilderRfqError
}

// ComboRFQStatusResult is the current gateway status of a combo RFQ.
type ComboRFQStatusResult struct {
	RFQID          string
	Status         ComboRFQStatus
	TakerOrderHash string
	TxHash         string
	Error          *BuilderRfqError
}

// ComboRFQRejectionError reports a combo RFQ request the gateway rejected
// with a code outside the known vocabulary. Known rejections surface as
// normal results instead.
type ComboRFQRejectionError struct {
	Code    string
	Message string
}

// Error implements the error interface.
func (e *ComboRFQRejectionError) Error() string {
	return fmt.Sprintf("combo rfq rejected %s: %s", e.Code, e.Message)
}

// Wire types for the builder gateway. Field names follow the gateway contract.

type builderRfqRequestedSize struct {
	Unit    RFQRequestedSizeUnit `json:"unit"`
	ValueE6 string               `json:"value_e6"`
}

type builderRfqCreateRequest struct {
	SignerAddress  string                  `json:"signer_address"`
	MakerAddress   string                  `json:"maker_address"`
	SignatureType  SignatureType           `json:"signature_type"`
	LegPositionIDs []string                `json:"leg_position_ids"`
	Direction      RFQDirection            `json:"direction"`
	Side           RFQSide                 `json:"side"`
	RequestedSize  builderRfqRequestedSize `json:"requested_size"`
}

type builderRfqQuoteWire struct {
	QuoteID         string `json:"quote_id"`
	BlendedPriceE6  string `json:"blended_price_e6"`
	MakerAmountE6   string `json:"maker_amount_e6"`
	TakerAmountE6   string `json:"taker_amount_e6"`
	TotalRequiredE6 string `json:"total_required_e6"`
}

type builderRfqCreateResponseWire struct {
	RFQID       string `json:"rfq_id"`
	Status      string `json:"status"`
	ExpiresAt   int64  `json:"expires_at"`
	BuilderCode string `json:"builder_code"`
	Request     struct {
		ConditionID   string `json:"condition_id"`
		YesPositionID string `json:"yes_position_id"`
		NoPositionID  string `json:"no_position_id"`
	} `json:"request"`
	Quote *builderRfqQuoteWire `json:"quote"`
	Error *BuilderRfqError     `json:"error"`
}

type builderRfqStatusWire struct {
	RFQID          string           `json:"rfq_id"`
	Status         string           `json:"status"`
	TakerOrderHash string           `json:"taker_order_hash"`
	TxHash         string           `json:"tx_hash"`
	Error          *BuilderRfqError `json:"error"`
}

type comboSignedOrderWire struct {
	Salt          string        `json:"salt"`
	Maker         string        `json:"maker"`
	Signer        string        `json:"signer"`
	TokenID       string        `json:"tokenId"`
	MakerAmount   string        `json:"makerAmount"`
	TakerAmount   string        `json:"takerAmount"`
	Side          Side          `json:"side"`
	SignatureType SignatureType `json:"signatureType"`
	Timestamp     string        `json:"timestamp"`
	Builder       string        `json:"builder"`
	Expiration    string        `json:"expiration"`
	Metadata      string        `json:"metadata"`
	Signature     string        `json:"signature"`
}

type builderRfqAcceptRequest struct {
	QuoteID     string               `json:"quote_id"`
	SignedOrder comboSignedOrderWire `json:"signed_order"`
}

// RequestComboQuote requests a quote for a combo of positions through the
// builder gateway and resolves when the quote competition window closes.
// A request that attracts no usable quotes is a normal outcome, returned as
// a nil Quote with a Reason rather than an error. Builder authorization is
// required.
func (c *AuthenticatedClient) RequestComboQuote(
	ctx context.Context,
	params RequestComboQuoteParams,
) (*RequestComboQuoteResult, error) {
	if err := validateComboQuoteRequest(params); err != nil {
		return nil, err
	}
	if c.builderAuth == nil {
		return nil, fmt.Errorf("combo RFQ requires builder authorization")
	}

	var value udecimal.Decimal
	unit := RFQSizeUnitNotional
	switch params.Direction {
	case RFQDirectionBuy:
		value = params.Amount
	case RFQDirectionSell:
		value = params.Size
		unit = RFQSizeUnitShares
	default:
		return nil, fmt.Errorf("combo RFQ direction must be BUY or SELL, got %q", params.Direction)
	}

	request := builderRfqCreateRequest{
		SignerAddress:  c.signer.Address().Hex(),
		MakerAddress:   c.comboMakerAddress(),
		SignatureType:  c.signatureType,
		LegPositionIDs: params.LegPositionIDs,
		Direction:      params.Direction,
		Side:           RFQSideYes,
		RequestedSize: builderRfqRequestedSize{
			Unit:    unit,
			ValueE6: decimalToE6(value),
		},
	}

	var wire builderRfqCreateResponseWire
	if err := c.gatewayHTTP.PostJSON(
		ctx,
		builderRFQRequestsEndpoint,
		request,
		polyhttp.AuthL2Builder,
		&wire,
	); err != nil {
		return nil, err
	}

	result := &RequestComboQuoteResult{
		RFQID:         wire.RFQID,
		Direction:     params.Direction,
		YesPositionID: wire.Request.YesPositionID,
		NoPositionID:  wire.Request.NoPositionID,
		ConditionID:   wire.Request.ConditionID,
		BuilderCode:   wire.BuilderCode,
		Status:        ComboRFQStatus(wire.Status),
		Error:         wire.Error,
	}

	if wire.Quote != nil {
		quote, err := comboQuoteFromWire(wire.Quote, wire.ExpiresAt)
		if err != nil {
			return nil, fmt.Errorf("combo rfq: decode quote: %w", err)
		}
		result.Quote = quote
		return result, nil
	}

	switch {
	case wire.Error == nil:
		result.Reason = ComboQuoteNoQuotes
	case wire.Error.Code == string(ComboQuoteSizeTooLarge):
		result.Reason = ComboQuoteSizeTooLarge
	case wire.Error.Code == string(ComboQuoteNoQuotes):
		result.Reason = ComboQuoteNoQuotes
	default:
		return nil, &ComboRFQRejectionError{Code: wire.Error.Code, Message: wire.Error.Message}
	}
	return result, nil
}

// AcceptComboQuote accepts a combo quote, signing the acceptance order for
// the authenticated account with the Exchange V3 domain. Accepting is
// idempotent: retrying an already-accepted RFQ reports its current status.
// Resolves at the maker last-look outcome; a maker decline or an expired
// acceptance window is a normal result, not an error. Builder authorization
// is required.
func (c *AuthenticatedClient) AcceptComboQuote(
	ctx context.Context,
	params AcceptComboQuoteParams,
) (*AcceptComboQuoteResult, error) {
	if err := validateComboAcceptRequest(params); err != nil {
		return nil, err
	}
	if c.builderAuth == nil {
		return nil, fmt.Errorf("combo RFQ requires builder authorization")
	}

	contracts, err := getContractConfig(c.chainID)
	if err != nil {
		return nil, err
	}
	if contracts.ExchangeV3 == "" {
		return nil, fmt.Errorf("exchange v3 contract not configured for chain %d", c.chainID)
	}

	makerAmount, err := decimalToE6String(params.Quote.MakerAmount)
	if err != nil {
		return nil, fmt.Errorf("combo accept: maker amount: %w", err)
	}
	takerAmount, err := decimalToE6String(params.Quote.TakerAmount)
	if err != nil {
		return nil, fmt.Errorf("combo accept: taker amount: %w", err)
	}

	salt, err := c.saltGenerator()
	if err != nil {
		return nil, fmt.Errorf("combo accept: generate order salt: %w", err)
	}

	order := comboSignedOrderWire{
		Salt:          strconv.FormatUint(salt, 10),
		Maker:         c.comboMakerAddress(),
		Signer:        c.signer.Address().Hex(),
		TokenID:       params.PositionID,
		MakerAmount:   makerAmount,
		TakerAmount:   takerAmount,
		Side:          params.Direction,
		SignatureType: c.signatureType,
		Timestamp:     strconv.FormatInt(time.Now().Unix(), 10),
		Builder:       params.BuilderCode,
		Expiration:    "0",
		Metadata:      zeroBytes32,
	}

	typedData := buildOrderTypedData(c.chainID, comboProtocolVersion, contracts.ExchangeV3, order.typedOrder())
	if c.signatureType == SignatureTypePoly1271 {
		order.Signature, err = signPoly1271Order(c.signer, typedData, c.chainID)
	} else {
		order.Signature, err = polyauth.SignTypedData(c.signer, typedData)
	}
	if err != nil {
		return nil, fmt.Errorf("combo accept: sign order: %w", err)
	}

	var wire builderRfqStatusWire
	acceptPath := builderRFQRequestsEndpoint + "/" + url.PathEscape(params.RFQID) + "/accept"
	acceptErr := c.gatewayHTTP.PostJSON(
		ctx,
		acceptPath,
		builderRfqAcceptRequest{QuoteID: params.Quote.QuoteID, SignedOrder: order},
		polyhttp.AuthL2Builder,
		&wire,
	)
	if acceptErr != nil {
		if expired, reason := comboAcceptanceExpired(acceptErr); expired {
			return &AcceptComboQuoteResult{
				Status: ComboRFQExpired,
				RFQID:  params.RFQID,
				Reason: reason,
				Error:  &BuilderRfqError{Code: string(ComboRFQExpired), Message: acceptErr.Error()},
			}, nil
		}
		return nil, acceptErr
	}

	// The gateway holds the accept through maker last look; resume over the
	// status endpoint while the outcome is still pending.
	deadline := time.Now().Add(comboAcceptOutcomeTimeout)
	for ComboRFQStatus(wire.Status) == ComboRFQAwaitingMakerConfirmation {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf(
				"combo accept: timed out waiting for the acceptance outcome of RFQ %s",
				params.RFQID,
			)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(comboAcceptOutcomePollEvery):
		}
		status, err := c.GetComboRFQStatus(ctx, params.RFQID)
		if err != nil {
			return nil, err
		}
		wire = builderRfqStatusWire{
			RFQID:          status.RFQID,
			Status:         string(status.Status),
			TakerOrderHash: status.TakerOrderHash,
			TxHash:         status.TxHash,
			Error:          status.Error,
		}
	}

	status := ComboRFQStatus(wire.Status)
	if status == ComboRFQFailed || status == ComboRFQExpired || status == ComboRFQCanceled {
		return &AcceptComboQuoteResult{
			Status: status,
			RFQID:  wire.RFQID,
			Reason: comboAcceptFailureReason(status, wire.Error),
			Error:  wire.Error,
		}, nil
	}

	takerOrderHash := wire.TakerOrderHash
	if takerOrderHash == "" {
		takerOrderHash = comboOrderHash(c.chainID, contracts.ExchangeV3, order)
	}
	return &AcceptComboQuoteResult{
		Status:         status,
		RFQID:          wire.RFQID,
		TakerOrderHash: takerOrderHash,
		TxHash:         wire.TxHash,
	}, nil
}

// GetComboRFQStatus returns the current builder-gateway status of a combo RFQ.
func (c *AuthenticatedClient) GetComboRFQStatus(
	ctx context.Context,
	rfqID string,
) (*ComboRFQStatusResult, error) {
	if rfqID == "" {
		return nil, fmt.Errorf("combo rfq status: rfqId is required")
	}
	var wire builderRfqStatusWire
	if err := c.gatewayHTTP.GetJSON(
		ctx,
		builderRFQRequestsEndpoint+"/"+url.PathEscape(rfqID),
		nil,
		polyhttp.AuthL2Builder,
		&wire,
	); err != nil {
		return nil, err
	}
	return &ComboRFQStatusResult{
		RFQID:          wire.RFQID,
		Status:         ComboRFQStatus(wire.Status),
		TakerOrderHash: wire.TakerOrderHash,
		TxHash:         wire.TxHash,
		Error:          wire.Error,
	}, nil
}

func comboQuoteFromWire(wire *builderRfqQuoteWire, expiresAt int64) (*ComboQuote, error) {
	decode := func(raw, name string) (string, error) {
		value, err := e6ToDecimal(raw)
		if err != nil {
			return "", fmt.Errorf("%s: %w", name, err)
		}
		return value, nil
	}
	blendedPrice, err := decode(wire.BlendedPriceE6, "blended_price_e6")
	if err != nil {
		return nil, err
	}
	makerAmount, err := decode(wire.MakerAmountE6, "maker_amount_e6")
	if err != nil {
		return nil, err
	}
	takerAmount, err := decode(wire.TakerAmountE6, "taker_amount_e6")
	if err != nil {
		return nil, err
	}
	totalRequired, err := decode(wire.TotalRequiredE6, "total_required_e6")
	if err != nil {
		return nil, err
	}
	return &ComboQuote{
		QuoteID:       wire.QuoteID,
		BlendedPrice:  blendedPrice,
		MakerAmount:   makerAmount,
		TakerAmount:   takerAmount,
		TotalRequired: totalRequired,
		ExpiresAt:     expiresAt,
	}, nil
}

// typedOrder adapts the wire order to the shared EIP-712 order builder.
// The acceptance order reuses the standard V2/V3 order shape; only the
// domain (Exchange V3, version "3") differs.
func (o comboSignedOrderWire) typedOrder() SignedOrder {
	return SignedOrder{
		Order: Order{
			Salt:          o.Salt,
			Maker:         o.Maker,
			Signer:        o.Signer,
			TokenID:       o.TokenID,
			MakerAmount:   o.MakerAmount,
			TakerAmount:   o.TakerAmount,
			Side:          o.Side,
			SignatureType: o.SignatureType,
			Timestamp:     o.Timestamp,
			Metadata:      o.Metadata,
			Builder:       o.Builder,
		},
		Expiration: o.Expiration,
	}
}

// comboMakerAddress resolves the maker address for combo RFQ flows with the
// same fallback as the main order path: the funder when configured, otherwise
// the signer address (EOA wallets carry no separate funder).
func (c *SignerClient) comboMakerAddress() string {
	if c.funderAddress != "" {
		return c.funderAddress
	}
	return c.signer.Address().Hex()
}

// comboProtocolVersion is the Exchange V3 EIP-712 domain version.
const comboProtocolVersion = "3"

// comboAcceptanceExpired classifies an accept-request transport error whose
// gateway code reports an expired RFQ as a normal failed outcome.
func comboAcceptanceExpired(err error) (bool, ComboAcceptFailureReason) {
	var apiErr *polyhttp.APIError
	if !errors.As(err, &apiErr) || apiErr.Code == nil {
		return false, ""
	}
	if *apiErr.Code != "EXPIRED_RFQ" {
		return false, ""
	}
	return true, ComboAcceptWindowExpired
}

// comboAcceptFailureReason maps a terminal acceptance status to its
// failure reason, mirroring the official TypeScript SDK mapping.
func comboAcceptFailureReason(status ComboRFQStatus, gatewayErr *BuilderRfqError) ComboAcceptFailureReason {
	switch status {
	case ComboRFQExpired:
		return ComboAcceptWindowExpired
	case ComboRFQCanceled:
		return ComboAcceptMakerDeclined
	case ComboRFQFailed:
		if gatewayErr != nil && gatewayErr.Code == "EXPIRED_RFQ" {
			return ComboAcceptWindowExpired
		}
		return ComboAcceptExecutionFailed
	default:
		return ComboAcceptExecutionFailed
	}
}

func validateComboQuoteRequest(params RequestComboQuoteParams) error {
	if len(params.LegPositionIDs) < comboMinLegPositionIDs ||
		len(params.LegPositionIDs) > comboMaxLegPositionIDs {
		return fmt.Errorf(
			"combo RFQ requires between %d and %d leg position IDs, got %d",
			comboMinLegPositionIDs, comboMaxLegPositionIDs, len(params.LegPositionIDs),
		)
	}
	seen := make(map[string]bool, len(params.LegPositionIDs))
	for _, id := range params.LegPositionIDs {
		if id == "" {
			return fmt.Errorf("combo RFQ leg position ID must not be empty")
		}
		if seen[id] {
			return fmt.Errorf("combo RFQ duplicate leg position ID %q", id)
		}
		seen[id] = true
	}
	switch params.Direction {
	case RFQDirectionBuy:
		if params.Amount.Cmp(udecimal.Zero) <= 0 {
			return fmt.Errorf("combo RFQ buy amount must be positive")
		}
	case RFQDirectionSell:
		if params.Size.Cmp(udecimal.Zero) <= 0 {
			return fmt.Errorf("combo RFQ sell size must be positive")
		}
	default:
		return fmt.Errorf("combo RFQ direction must be BUY or SELL, got %q", params.Direction)
	}
	return nil
}

func validateComboAcceptRequest(params AcceptComboQuoteParams) error {
	if params.RFQID == "" {
		return fmt.Errorf("combo accept: rfqId is required")
	}
	if params.Direction != SideBuy && params.Direction != SideSell {
		return fmt.Errorf("combo accept: direction must be BUY or SELL, got %q", params.Direction)
	}
	if params.PositionID == "" {
		return fmt.Errorf("combo accept: positionId is required")
	}
	if _, err := strconv.ParseUint(params.PositionID, 10, 64); err != nil || !isNumericString(params.PositionID) {
		return fmt.Errorf("combo accept: positionId must be a numeric string")
	}
	if params.Quote.QuoteID == "" {
		return fmt.Errorf("combo accept: quoteId is required")
	}
	if err := validateBuilderCode(params.BuilderCode); err != nil {
		return fmt.Errorf("combo accept: %w", err)
	}
	return nil
}

func validateBuilderCode(code string) error {
	if !strings.HasPrefix(code, "0x") || len(code) != 66 {
		return fmt.Errorf("builderCode must be a 0x-prefixed 32-byte hex string")
	}
	for _, r := range code[2:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return fmt.Errorf("builderCode must be a 0x-prefixed 32-byte hex string")
		}
	}
	return nil
}

func isNumericString(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// comboOrderHash computes the EIP-712 digest of a signed acceptance order.
// It matches the exchange's order hash and is used as the taker-order hash
// when the gateway does not return one.
func comboOrderHash(chainID int64, exchangeV3 string, order comboSignedOrderWire) string {
	digest, _, err := apitypes.TypedDataAndHash(
		buildOrderTypedData(chainID, comboProtocolVersion, exchangeV3, order.typedOrder()),
	)
	if err != nil {
		return ""
	}
	return "0x" + fmt.Sprintf("%x", digest)
}

// decimalToE6 converts a human-readable decimal amount to the gateway's
// 6-decimal base-unit integer string, truncating any sub-unit remainder.
func decimalToE6(value udecimal.Decimal) string {
	return value.Mul(udecimal.MustFromInt64(1_000_000, 0)).Trunc(0).StringFixed(0)
}

// decimalToE6String parses a human-readable decimal string and converts it
// to the gateway's 6-decimal base-unit integer string.
func decimalToE6String(value string) (string, error) {
	parsed, err := udecimal.Parse(value)
	if err != nil {
		return "", fmt.Errorf("parse decimal %q: %w", value, err)
	}
	if parsed.Cmp(udecimal.Zero) <= 0 {
		return "", fmt.Errorf("amount %q must be positive", value)
	}
	return decimalToE6(parsed), nil
}

// e6ToDecimal converts the gateway's 6-decimal base-unit integer string to a
// human-readable decimal string, trimming trailing fractional zeros.
func e6ToDecimal(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("empty e6 value")
	}
	raw, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return "", fmt.Errorf("parse e6 value %q: %w", value, err)
	}
	whole := raw / 1_000_000
	fraction := strconv.FormatUint(raw%1_000_000, 10)
	fraction = strings.TrimRight(strings.Repeat("0", 6-len(fraction))+fraction, "0")
	if fraction == "" {
		return strconv.FormatUint(whole, 10), nil
	}
	return strconv.FormatUint(whole, 10) + "." + fraction, nil
}
