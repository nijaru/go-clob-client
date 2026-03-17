package clob

import "github.com/quagmt/udecimal"

// Decimal is the numeric type used for prices, sizes, and amounts.
// It is re-exported here so callers do not need to import udecimal directly.
type Decimal = udecimal.Decimal

// Dec parses s as a decimal number. Returns an error if s is not a valid decimal.
func Dec(s string) (Decimal, error) {
	return udecimal.Parse(s)
}

// MustDec parses s as a decimal number. Panics if s is not valid.
// Intended for use in tests and package-level constants.
func MustDec(s string) Decimal {
	d, err := udecimal.Parse(s)
	if err != nil {
		panic("clob.MustDec: " + err.Error())
	}
	return d
}
