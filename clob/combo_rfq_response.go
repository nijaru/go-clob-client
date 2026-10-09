package clob

import (
	"fmt"
	"math/big"
	"slices"
	"strings"
)

// ComboRFQEchoMismatchError identifies an unsafe quote-ready response. No
// usable quote is returned when the gateway changes the submitted request.
type ComboRFQEchoMismatchError struct {
	RFQID  string
	Fields []string
}

func (e *ComboRFQEchoMismatchError) Error() string {
	return fmt.Sprintf(
		"combo RFQ %s did not echo submitted %s",
		e.RFQID,
		strings.Join(e.Fields, ", "),
	)
}

func validateComboQuoteEcho(wire builderRfqCreateResponseWire, sent builderRfqCreateRequest) error {
	echoed := wire.Request
	var fields []string
	if wire.RFQID == "" || echoed.RFQID != wire.RFQID {
		fields = append(fields, "rfq_id")
	}
	if echoed.Direction != sent.Direction {
		fields = append(fields, "direction")
	}
	if echoed.Side != sent.Side {
		fields = append(fields, "side")
	}
	// IDs and sizes are numeric string fields. Compare canonical integer values,
	// without imposing local arithmetic limits on the response.
	legs := make([]string, len(echoed.LegPositionIDs))
	for i, id := range echoed.LegPositionIDs {
		if !comboPositionValid(id) {
			fields = append(fields, "leg_position_ids")
			break
		}
		value, _ := new(big.Int).SetString(id, 10)
		legs[i] = value.String()
	}
	if !slices.Equal(legs, sent.LegPositionIDs) {
		fields = append(fields, "leg_position_ids")
	}
	size, ok := new(big.Int).SetString(echoed.RequestedSize.ValueE6, 10)
	if !ok || !isNumericString(echoed.RequestedSize.ValueE6) ||
		echoed.RequestedSize.Unit != sent.RequestedSize.Unit ||
		size.String() != sent.RequestedSize.ValueE6 {
		fields = append(fields, "requested_size")
	}
	if len(fields) != 0 {
		return &ComboRFQEchoMismatchError{RFQID: wire.RFQID, Fields: slices.Compact(fields)}
	}
	if ComboRFQStatus(wire.Status) != ComboRFQAwaitingRequesterAcceptance ||
		wire.Quote.QuoteID == "" ||
		!comboPositionValid(echoed.YesPositionID) ||
		!comboPositionValid(echoed.NoPositionID) ||
		echoed.YesPositionID == echoed.NoPositionID ||
		echoed.ConditionID == "" {
		return fmt.Errorf("combo rfq: malformed quote-ready response")
	}
	if err := validateBuilderCode(wire.BuilderCode); err != nil {
		return fmt.Errorf("combo rfq: %w", err)
	}
	return nil
}
