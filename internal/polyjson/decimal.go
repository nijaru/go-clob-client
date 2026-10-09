// Package polyjson holds wire scalars shared by Polymarket HTTP APIs.
package polyjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
)

// Decimal preserves the exact spelling of a finite decimal without float64
// rounding. Wire numbers and quoted decimals decode identically; marshaling
// uses a string. Consumers choose their arithmetic library and precision.
type Decimal string

var decimalPattern = regexp.MustCompile(
	`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`,
)

func (d Decimal) String() string { return string(d) }

func (d *Decimal) UnmarshalJSON(raw []byte) error {
	value := string(bytes.TrimSpace(raw))
	if len(value) > 0 && value[0] == '"' {
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
	}
	if !ValidDecimal(value) {
		return fmt.Errorf("invalid decimal %q", value)
	}
	*d = Decimal(value)
	return nil
}

func (d Decimal) MarshalJSON() ([]byte, error) {
	if !ValidDecimal(string(d)) {
		return nil, fmt.Errorf("invalid decimal %q", d)
	}
	return json.Marshal(string(d))
}

func ValidDecimal(value string) bool { return decimalPattern.MatchString(value) }
