package clob

import (
	"fmt"
	"net/http"
	"time"
)

const DefaultComboRFQQuoterURL = "wss://combos-rfq-gateway-quoter.polymarket.com/ws/rfq"

// ComboRFQSessionOptions configures a quoter connection, not the REST gateway.
// Zero timeouts use 30 seconds. Events are buffered up to 1024 entries; overflow
// fails the session rather than silently losing trading events.
type ComboRFQSessionOptions struct {
	URL         string
	Header      http.Header
	HTTPClient  *http.Client
	AuthTimeout time.Duration
	AckTimeout  time.Duration
}

type ComboQuoteSource string

const (
	ComboQuoteCollateral ComboQuoteSource = "collateral"
	ComboQuoteInventory  ComboQuoteSource = "inventory"
)

// ComboRFQQuoteResponse uses exact human-readable decimals (at most six places).
// Empty Source defaults to collateral; empty Size uses the entire request.
type ComboRFQQuoteResponse struct {
	Price  string
	Size   string
	Source ComboQuoteSource
}

type ComboRFQQuoteReference struct {
	RFQID   string
	QuoteID string
}
type ComboRFQConfirmationDecision string

const (
	ComboRFQConfirm ComboRFQConfirmationDecision = "CONFIRM"
	ComboRFQDecline ComboRFQConfirmationDecision = "DECLINE"
)

type ComboRFQConfirmationAck struct {
	ComboRFQQuoteReference
	Decision ComboRFQConfirmationDecision
}

// ComboRFQEvent is one of the four stable quoter broadcasts. Commands live on
// the session so events remain plain data and may be handled concurrently.
type ComboRFQEvent interface{ comboRFQEvent() }

type ComboRFQRequestedSize struct {
	Unit  RFQRequestedSizeUnit
	Value string
}
type ComboRFQQuoteRequest struct {
	RFQID              string
	RequestorPublicID  string
	LegPositionIDs     []string
	ConditionID        string
	YesPositionID      string
	NoPositionID       string
	Direction          RFQDirection
	Side               RFQSide
	RequestedSize      ComboRFQRequestedSize
	SubmissionDeadline int64 // Unix milliseconds
}

func (ComboRFQQuoteRequest) comboRFQEvent() {}

type ComboRFQConfirmationRequest struct {
	ComboRFQQuoteReference
	SignerAddress  string
	MakerAddress   string
	SignatureType  SignatureType
	LegPositionIDs []string
	ConditionID    string
	YesPositionID  string
	NoPositionID   string
	Direction      RFQDirection
	Side           RFQSide
	FillSize       string
	Price          string
	ConfirmBy      int64 // Unix milliseconds
}

func (ComboRFQConfirmationRequest) comboRFQEvent() {}

type ComboRFQExecutionUpdate struct {
	RFQID  string
	Status ComboRFQStatus
	TxHash string
}

func (ComboRFQExecutionUpdate) comboRFQEvent() {}

type ComboRFQTrade struct {
	RFQID          string
	RequesterID    string
	ConditionID    string
	LegPositionIDs []string
	Direction      RFQDirection
	Side           RFQSide
	Price          string
	Size           string
	ExecutedAt     int64 // Unix milliseconds
}

func (ComboRFQTrade) comboRFQEvent() {}

// ComboRFQCommandError preserves the gateway's open-ended error vocabulary and
// identifiers, correlated to the failed command.
type ComboRFQCommandError struct {
	RequestType string
	RFQID       string
	QuoteID     string
	Code        string
	ErrorID     string
	Message     string
}

func (e *ComboRFQCommandError) Error() string {
	return fmt.Sprintf(
		"combo RFQ %s rejected (%s, %s): %s",
		e.RequestType,
		e.RFQID,
		e.Code,
		e.Message,
	)
}
