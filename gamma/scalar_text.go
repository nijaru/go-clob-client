package gamma

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ScalarText preserves nullable string-or-number metadata exactly. Gamma's
// legacy bounds, bonds and estimates are strings in Rust's wire contract, even
// when they are not decimals; newer contracts also admit numeric estimates.
// Empty means absent/null/empty text. For arithmetic, Decimal(value).Rat rejects
// nondecimal text instead of guessing a numeric value.
type ScalarText string

func (s *ScalarText) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*s = ""
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*s = ScalarText(text)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("gamma scalar text: %w", err)
	}
	*s = ScalarText(number.String())
	return nil
}
