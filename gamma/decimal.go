package gamma

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/nijaru/go-clob-client/internal/polyjson"
)

// Decimal preserves the exact text of Gamma's string-or-number decimal values.
// The empty value represents an absent or null field, not zero. JSON encoding
// uses a string, as with Gamma's volume and liquidity fields.
type Decimal string

func (d *Decimal) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*d = ""
		return nil
	}
	var value polyjson.Decimal
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("gamma decimal: %w", err)
	}
	*d = Decimal(value)
	return nil
}

// Rat returns an exact rational value without rounding through float64.
func (d Decimal) Rat() (*big.Rat, error) {
	if !polyjson.ValidDecimal(string(d)) {
		return nil, fmt.Errorf("gamma decimal: invalid value %q", d)
	}
	value, ok := new(big.Rat).SetString(string(d))
	if !ok {
		return nil, fmt.Errorf("gamma decimal: invalid value %q", d)
	}
	return value, nil
}
