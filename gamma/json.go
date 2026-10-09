package gamma

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/nijaru/go-clob-client/internal/polyjson"
)

// UnmarshalJSON decodes Gamma's direct or JSON-encoded outcome/asset arrays.
// V2 position IDs follow the same wire convention as CLOB token IDs.
func (m *Market) UnmarshalJSON(data []byte) error {
	type plain Market
	value := plain{}
	wire := struct {
		*plain
		Outcomes          json.RawMessage `json:"outcomes"`
		OutcomePrices     json.RawMessage `json:"outcomePrices"`
		TokenIDs          json.RawMessage `json:"clobTokenIds"`
		PositionIDs       json.RawMessage `json:"positionIds"`
		SubmittedBy       *string         `json:"submittedBy"`
		SubmittedByLegacy *string         `json:"submitted_by"`
	}{plain: &value}
	if err := json.Unmarshal(data, &wire); err != nil {
		return fmt.Errorf("gamma market: %w", err)
	}
	for _, field := range []struct {
		key     string
		raw     json.RawMessage
		target  *[]string
		numbers bool
	}{
		{"outcomes", wire.Outcomes, &value.Outcomes, false},
		{"outcomePrices", wire.OutcomePrices, &value.OutcomePrices, true},
		{"clobTokenIds", wire.TokenIDs, &value.CLOBTokenIDs, true},
		{"positionIds", wire.PositionIDs, &value.PositionIDs, true},
	} {
		items, err := decodeStringArray(field.raw, field.numbers)
		if err != nil {
			return fmt.Errorf("gamma market %s: %w", field.key, err)
		}
		if field.key == "outcomePrices" {
			for i, price := range items {
				if !polyjson.ValidDecimal(price) {
					return fmt.Errorf("gamma market outcomePrices: item %d is not a decimal", i)
				}
			}
		}
		*field.target = items
	}
	if wire.SubmittedBy != nil {
		value.SubmittedBy = *wire.SubmittedBy
	} else if wire.SubmittedByLegacy != nil {
		value.SubmittedBy = *wire.SubmittedByLegacy
	}
	*m = Market(value)
	return nil
}

func decodeStringArray(raw json.RawMessage, numbers bool) ([]string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}
	if trimmed[0] == '"' {
		var encoded string
		if err := json.Unmarshal(trimmed, &encoded); err != nil {
			return nil, err
		}
		encoded = strings.TrimSpace(encoded)
		if encoded == "" {
			return []string{}, nil
		}
		trimmed = []byte(encoded)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(trimmed, &items); err != nil {
		return nil, fmt.Errorf("expected array or JSON-encoded array: %w", err)
	}
	if items == nil {
		return nil, nil
	}
	values := make([]string, len(items))
	for i, item := range items {
		if bytes.Equal(bytes.TrimSpace(item), []byte("null")) {
			return nil, fmt.Errorf("item %d is null", i)
		}
		if err := json.Unmarshal(item, &values[i]); err == nil {
			continue
		}
		if !numbers {
			return nil, fmt.Errorf("item %d is not a string", i)
		}
		var number json.Number
		if err := json.Unmarshal(item, &number); err != nil {
			return nil, fmt.Errorf("item %d is not a string or number: %w", i, err)
		}
		values[i] = number.String()
	}
	return values, nil
}

// FlexibleID represents Gamma's string-or-integer identifiers without losing
// precision. Null is represented by the empty value.
type FlexibleID string

func (id *FlexibleID) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*id = ""
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*id = FlexibleID(text)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("gamma id: %w", err)
	}
	if _, ok := new(big.Int).SetString(number.String(), 10); !ok {
		return fmt.Errorf("gamma id: expected integer, got %s", number)
	}
	*id = FlexibleID(number.String())
	return nil
}

func (e *Event) UnmarshalJSON(data []byte) error {
	type plain Event
	value := plain{}
	wire := struct {
		*plain
		PublishedAt       *string `json:"publishedAt"`
		PublishedAtLegacy *string `json:"published_at"`
	}{plain: &value}
	if err := json.Unmarshal(data, &wire); err != nil {
		return fmt.Errorf("gamma event: %w", err)
	}
	if wire.PublishedAt != nil {
		value.PublishedAt = *wire.PublishedAt
	} else if wire.PublishedAtLegacy != nil {
		value.PublishedAt = *wire.PublishedAtLegacy
	}
	*e = Event(value)
	return nil
}

func (s *SportsMetadata) UnmarshalJSON(data []byte) error {
	// Rust SportsMetadata uses StringWithSeparator<CommaSeparator, String>.
	type plain SportsMetadata
	value := plain{}
	wire := struct {
		*plain
		Tags string `json:"tags"`
	}{plain: &value}
	if err := json.Unmarshal(data, &wire); err != nil {
		return fmt.Errorf("gamma sports: %w", err)
	}
	if wire.Tags != "" {
		value.Tags = strings.Split(wire.Tags, ",")
	}
	*s = SportsMetadata(value)
	return nil
}
