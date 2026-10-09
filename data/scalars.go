package data

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyjson"
)

// DecimalString preserves a monetary or share amount without float64 rounding.
// The service can send a JSON number or a decimal string. Parse it with a
// decimal library when arithmetic is needed; its String method is lossless.
type DecimalString = polyjson.Decimal

// Timestamp is an instant in UTC. Data v2 uses epoch seconds (sometimes quoted)
// and RFC3339 timestamps, depending on the resource. Calendar dates are midnight
// UTC. The embedded time.Time provides the usual Go time operations.
type Timestamp struct{ time.Time }

func (t *Timestamp) UnmarshalJSON(raw []byte) error {
	value := string(bytes.TrimSpace(raw))
	if len(value) > 0 && value[0] == '"' {
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
	}
	if value == "" {
		t.Time = time.Time{}
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.DateOnly} {
		if parsed, err := time.Parse(layout, value); err == nil {
			t.Time = parsed.UTC()
			return nil
		}
	}
	if !polyjson.ValidDecimal(value) {
		return fmt.Errorf("data: invalid timestamp %q", value)
	}
	// Reject enormous epochs before exact fractional-second conversion.
	approximate, err := strconv.ParseFloat(value, 64)
	if err != nil || approximate < -62135596800 || approximate > 253402300799 {
		return fmt.Errorf("data: timestamp outside years 0001–9999")
	}
	seconds, ok := new(big.Rat).SetString(value)
	if !ok {
		return fmt.Errorf("data: invalid timestamp %q", value)
	}
	seconds.Mul(seconds, big.NewRat(1_000_000_000, 1))
	nanos := new(big.Int).Quo(seconds.Num(), seconds.Denom())
	whole, remainder := new(big.Int), new(big.Int)
	whole.QuoRem(nanos, big.NewInt(1_000_000_000), remainder)
	if !whole.IsInt64() || whole.Int64() < -62135596800 || whole.Int64() > 253402300799 {
		return fmt.Errorf("data: timestamp outside years 0001–9999")
	}
	t.Time = time.Unix(whole.Int64(), remainder.Int64()).UTC()
	return nil
}

// EventID accepts the numeric and string spellings used by the service.
type EventID string

func (id *EventID) UnmarshalJSON(raw []byte) error {
	value := string(bytes.TrimSpace(raw))
	if len(value) > 0 && value[0] == '"' {
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
	}
	if value == "" {
		*id = ""
		return nil
	}
	if _, err := strconv.ParseUint(value, 10, 31); err != nil {
		return fmt.Errorf("data: invalid event ID %q", value)
	}
	*id = EventID(value)
	return nil
}

func (id EventID) MarshalJSON() ([]byte, error) { return json.Marshal(string(id)) }

// Integer accepts integral JSON numbers and quoted integers. Some resolution
// metadata, such as log_index, is quoted even when other resources use numbers.
type Integer int64

func (n *Integer) UnmarshalJSON(raw []byte) error {
	value := string(bytes.TrimSpace(raw))
	if len(value) > 0 && value[0] == '"' {
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("data: invalid integer %q", value)
	}
	*n = Integer(parsed)
	return nil
}

// shiftDecimal scales a wire decimal by a power of ten without rounding. The
// bounded exponent prevents a malformed response from demanding a huge string.
func shiftDecimal(value DecimalString, places int) (DecimalString, error) {
	s := string(value)
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign, s = "-", s[1:]
	} else {
		s = strings.TrimPrefix(s, "+")
	}
	exponent := 0
	if index := strings.IndexAny(s, "eE"); index >= 0 {
		var err error
		exponent, err = strconv.Atoi(s[index+1:])
		if err != nil || exponent < -10000 || exponent > 10000 {
			return "", fmt.Errorf("data: payout exponent out of range")
		}
		s = s[:index]
	}
	point := strings.IndexByte(s, '.')
	if point < 0 {
		point = len(s)
	}
	digits := strings.ReplaceAll(s, ".", "")
	point += exponent + places
	switch {
	case point <= 0:
		s = "0." + strings.Repeat("0", -point) + digits
	case point >= len(digits):
		s = digits + strings.Repeat("0", point-len(digits))
	default:
		s = digits[:point] + "." + digits[point:]
	}
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return DecimalString(sign + s), nil
}
