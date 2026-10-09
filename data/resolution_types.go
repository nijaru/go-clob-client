package data

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
)

// Resolution is returned by the Data API v2.
type Resolution struct {
	QuestionID             *string                        `json:"question_id"`
	ConditionID            *string                        `json:"condition_id"`
	Status                 ResolutionStatus               `json:"status"                   required:"true"`
	ExtendedReview         bool                           `json:"extended_review"          required:"true"`
	ExpectedSettlementTime *Timestamp                     `json:"expected_settlement_time"`
	SettlementTimeBasis    *ResolutionSettlementTimeBasis `json:"settlement_time_basis"`
	WasDisputed            bool                           `json:"was_disputed"             required:"true"`
	QuestionRulesUpdated   bool                           `json:"new_version_q"            required:"true"`
	ProposedPrice          *DecimalString                 `json:"proposed_price"`
	ReproposedPrice        *DecimalString                 `json:"reproposed_price"`
	Price                  *DecimalString                 `json:"price"`
	TransactionHash        *string                        `json:"transaction_hash"`
	LogIndex               *int64                         `json:"log_index"`
	LastUpdatedAt          Timestamp                      `json:"last_update_timestamp"    required:"true"`
	MarketType             *ResolutionMarketType          `json:"market_type"`
	// Payouts are USDC per share, normalized from the service's E6 base units.
	Payouts          *[2]DecimalString   `json:"payouts"`
	ResolutionSource *ResolutionSource   `json:"resolution_source"`
	Reporter         *ResolutionReporter `json:"reporter"`
	WasArbitrated    *bool               `json:"was_arbitrated"`
	ResolvedBlock    *int64              `json:"resolved_block"`
	ResolvedAt       *Timestamp          `json:"resolved_at"`
}

func (r *Resolution) UnmarshalJSON(raw []byte) error {
	type wire Resolution
	var value wire
	if err := validateWire(raw, reflect.TypeFor[wire]()); err != nil {
		return err
	}
	payload := struct {
		*wire
		LogIndex json.RawMessage `json:"log_index"`
		Payouts  []DecimalString `json:"payouts"`
	}{wire: &value}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	if len(payload.LogIndex) > 0 && !bytes.Equal(payload.LogIndex, []byte("null")) &&
		!bytes.Equal(payload.LogIndex, []byte(`""`)) {
		var index Integer
		if err := json.Unmarshal(payload.LogIndex, &index); err != nil {
			return err
		}
		n := int64(index)
		value.LogIndex = &n
	}
	if payload.Payouts != nil {
		if len(payload.Payouts) != 2 {
			return fmt.Errorf("data: expected exactly two resolution payouts")
		}
		payouts := [2]DecimalString{}
		for i, payout := range payload.Payouts {
			scaled, err := shiftDecimal(payout, -6)
			if err != nil {
				return err
			}
			payouts[i] = scaled
		}
		value.Payouts = &payouts
	}
	for _, price := range []**DecimalString{&value.ProposedPrice, &value.ReproposedPrice, &value.Price} {
		if *price != nil {
			amount, ok := new(big.Rat).SetString(string(**price))
			if ok && amount.Cmp(big.NewRat(69, 1)) == 0 {
				*price = nil
			}
		}
	}
	clearEmptyText(&value.TransactionHash)
	*r = Resolution(value)
	return nil
}
