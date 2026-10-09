package gamma

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
)

// Decimal preserves the exact text of Gamma's string-or-number decimal values.
// The empty value represents an absent or null field, not zero. JSON encoding
// uses a string, as with Gamma's volume and liquidity fields.
type Decimal string

var decimalPattern = regexp.MustCompile(
	`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`,
)

func (d *Decimal) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*d = ""
		return nil
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		var n json.Number
		if err := json.Unmarshal(data, &n); err != nil {
			return fmt.Errorf("gamma decimal: %w", err)
		}
		value = n.String()
	}
	if !decimalPattern.MatchString(value) {
		return fmt.Errorf("gamma decimal: invalid value %q", value)
	}
	*d = Decimal(value)
	return nil
}

// Rat returns an exact rational value without rounding through float64.
func (d Decimal) Rat() (*big.Rat, error) {
	if !decimalPattern.MatchString(string(d)) {
		return nil, fmt.Errorf("gamma decimal: invalid value %q", d)
	}
	value, ok := new(big.Rat).SetString(string(d))
	if !ok {
		return nil, fmt.Errorf("gamma decimal: invalid value %q", d)
	}
	return value, nil
}
