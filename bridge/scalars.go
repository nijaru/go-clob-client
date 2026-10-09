package bridge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/nijaru/go-clob-client/internal/polyjson"
)

// ChainID identifies an EVM or bridge-supported chain. Its wire representation
// is a base-10 JSON string, including for non-EVM networks such as Solana.
type ChainID uint64

func (c ChainID) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatUint(uint64(c), 10))
}

func (c *ChainID) UnmarshalJSON(raw []byte) error {
	value := string(bytes.TrimSpace(raw))
	if len(value) > 0 && value[0] == '"' {
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fmt.Errorf("bridge: invalid chain ID %q: %w", value, err)
	}
	*c = ChainID(parsed)
	return nil
}

// Decimal retains full wire precision for USD estimates and fee percentages.
// The service can return a JSON number or a quoted decimal. Arithmetic and
// its precision are explicitly chosen by the caller, never done with float64.
type Decimal = polyjson.Decimal

// BaseUnits is a canonical unsigned uint256 amount, not a human-readable token
// quantity. Use the token's Decimals to convert units before requesting a quote.
// Parsing and JSON marshaling reject negative, fractional and overflowing values.
type BaseUnits string

func ParseBaseUnits(value string) (BaseUnits, error) {
	if value == "" || len(value) > 78 || (len(value) > 1 && value[0] == '0') ||
		strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return "", fmt.Errorf("bridge: invalid base-unit amount %q", value)
	}
	amount, ok := new(big.Int).SetString(value, 10)
	if !ok || amount.BitLen() > 256 {
		return "", fmt.Errorf("bridge: base-unit amount exceeds uint256")
	}
	return BaseUnits(value), nil
}

func (b BaseUnits) String() string { return string(b) }

func (b BaseUnits) MarshalJSON() ([]byte, error) {
	if _, err := ParseBaseUnits(string(b)); err != nil {
		return nil, err
	}
	return json.Marshal(string(b))
}

func (b *BaseUnits) UnmarshalJSON(raw []byte) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	parsed, err := ParseBaseUnits(value)
	if err != nil {
		return err
	}
	*b = parsed
	return nil
}
