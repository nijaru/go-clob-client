package perps

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

// CommandError preserves service rejection identifiers independently of HTTP
// status. Batch item rejections remain in their typed results.
type CommandError struct{ Operation, Code string }

func (e *CommandError) Error() string {
	return fmt.Sprintf("perps: %s rejected: %s", e.Operation, e.Code)
}

func commandRejection(op, status, code string) error {
	if status == "ok" {
		return nil
	}
	if status != "err" {
		return fmt.Errorf("perps: %s returned invalid status %q", op, status)
	}
	if code == "" {
		code = "command_rejected"
	}
	return &CommandError{Operation: op, Code: code}
}

var fixedDecimalPattern = regexp.MustCompile(`^-?\d+(?:\.\d+)?$`)

// Exact fixed-point inputs avoid JS numeric coercion. Advanced execution uses
// the venue's 96-bit decimal coefficient and maximum scale 28.
func parseFixedDecimal(value string, positive bool) (*big.Rat, error) {
	if !fixedDecimalPattern.MatchString(value) {
		return nil, fmt.Errorf("perps: invalid fixed-point decimal %q", value)
	}
	rat, ok := new(big.Rat).SetString(value)
	if !ok || positive && rat.Sign() <= 0 {
		return nil, fmt.Errorf("perps: decimal must be positive")
	}
	return rat, nil
}

func boundedDecimal(value string, positive bool) (string, error) {
	rat, err := parseFixedDecimal(value, positive)
	if err != nil {
		return "", err
	}
	if rat.Sign() < 0 {
		return "", fmt.Errorf("perps: decimal must be nonnegative")
	}
	whole, fraction, _ := strings.Cut(value, ".")
	fraction = strings.TrimRight(fraction, "0")
	coefficient, _ := new(big.Int).SetString(whole+fraction, 10)
	max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 96), big.NewInt(1))
	if len(fraction) > 28 || coefficient.Cmp(max) > 0 {
		return "", fmt.Errorf("perps: decimal exceeds venue precision")
	}
	whole = strings.TrimLeft(whole, "0")
	if whole == "" {
		whole = "0"
	}
	if fraction != "" {
		return whole + "." + fraction, nil
	}
	return whole, nil
}

func validateBuilderFeeRate(value string) error {
	if strings.HasPrefix(value, "-") {
		return fmt.Errorf("perps: builder rate must be nonnegative")
	}
	if _, err := parseFixedDecimal(value, false); err != nil {
		return err
	}
	_, fraction, _ := strings.Cut(value, ".")
	if len(fraction) > 28 {
		return fmt.Errorf("perps: builder rate scale exceeds 28")
	}
	return nil
}

func validateMarketRange(id int, start, end int64) error {
	if !validInstrumentID(id) || start < 0 || end < start {
		return fmt.Errorf("perps: invalid instrument or time range")
	}
	return nil
}

func validInstrumentID(id int) bool { return id >= 0 && uint64(id) <= 4294967295 }
