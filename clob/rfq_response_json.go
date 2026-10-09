package clob

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
)

// RFQ-local normalization keeps identifiers exact without involving floating
// point. String identifiers may be opaque native IDs (not only CTF uint256s).
func rfqDecodeIntegerTextFields(data []byte, out any, names ...string) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for _, name := range names {
		raw := bytes.TrimSpace(fields[name])
		if len(raw) == 0 || raw[0] == '"' || bytes.Equal(raw, []byte("null")) {
			continue
		}
		value := string(raw)
		if _, ok := new(big.Int).SetString(value, 10); !ok {
			return fmt.Errorf("%s: expected integer or string", name)
		}
		fields[name], _ = json.Marshal(value)
	}
	normalized, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	return json.Unmarshal(normalized, out)
}

func (r *RFQRequest) UnmarshalJSON(data []byte) error {
	type plain RFQRequest
	var value plain
	if err := rfqDecodeIntegerTextFields(data, &value, "token", "complement", "expiry"); err != nil {
		return err
	}
	*r = RFQRequest(value)
	return nil
}

func (r *RFQQuote) UnmarshalJSON(data []byte) error {
	type plain RFQQuote
	var value plain
	if err := rfqDecodeIntegerTextFields(data, &value, "token", "complement", "expiry"); err != nil {
		return err
	}
	*r = RFQQuote(value)
	return nil
}

// Catalog arrays may be arrays or JSON-encoded arrays. Only prices accept
// numeric elements; outcome labels and opaque position IDs must remain strings.
func rfqDecodeCatalogArray(raw json.RawMessage, numeric bool) ([]string, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	if raw[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return nil, err
		}
		raw = []byte(text)
	}
	if !numeric {
		var values []string
		err := json.Unmarshal(raw, &values)
		return values, err
	}
	var values []DecimalString
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	result := make([]string, len(values))
	for i, value := range values {
		result[i] = string(value)
	}
	return result, nil
}

func (m *ComboMarket) UnmarshalJSON(data []byte) error {
	type plain ComboMarket
	var wire struct {
		plain
		Outcomes      json.RawMessage `json:"outcomes"`
		OutcomePrices json.RawMessage `json:"outcome_prices"`
		PositionIDs   json.RawMessage `json:"position_ids"`
	}
	if err := rfqDecodeIntegerTextFields(data, &wire, "id"); err != nil {
		return err
	}
	var err error
	if wire.plain.Outcomes, err = rfqDecodeCatalogArray(wire.Outcomes, false); err != nil {
		return fmt.Errorf("outcomes: %w", err)
	}
	if wire.plain.OutcomePrices, err = rfqDecodeCatalogArray(wire.OutcomePrices, true); err != nil {
		return fmt.Errorf("outcome_prices: %w", err)
	}
	if wire.plain.PositionIDs, err = rfqDecodeCatalogArray(wire.PositionIDs, false); err != nil {
		return fmt.Errorf("position_ids: %w", err)
	}
	*m = ComboMarket(wire.plain)
	return nil
}

func (p *CollateralReturnPlan) UnmarshalJSON(data []byte) error {
	type plain CollateralReturnPlan
	var value plain
	if err := rfqDecodeIntegerTextFields(data, &value, "block_number"); err != nil {
		return err
	}
	*p = CollateralReturnPlan(value)
	return nil
}

func (o *CollateralReturnOperation) UnmarshalJSON(data []byte) error {
	type plain CollateralReturnOperation
	var value plain
	if err := rfqDecodeIntegerTextFields(data, &value, "event_id"); err != nil {
		return err
	}
	*o = CollateralReturnOperation(value)
	return nil
}
