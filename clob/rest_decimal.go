package clob

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/quagmt/udecimal"
)

var responseDecimalPattern = regexp.MustCompile(
	`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`,
)

// Decimal converts exactly to the order-math range. It never rounds or truncates.
// Empty/missing response values denote zero, as in the compact fee defaults.
func (d DecimalString) Decimal() (udecimal.Decimal, error) {
	text := string(d)
	if text == "" {
		return udecimal.Zero, nil
	}
	if !responseDecimalPattern.MatchString(text) {
		return udecimal.Zero, fmt.Errorf("invalid decimal %q", text)
	}
	sign := ""
	if text[0] == '-' || text[0] == '+' {
		if text[0] == '-' {
			sign = "-"
		}
		text = text[1:]
	}
	exponent := int64(0)
	if index := strings.IndexAny(text, "eE"); index >= 0 {
		var err error
		exponent, err = strconv.ParseInt(text[index+1:], 10, 64)
		if err != nil {
			return udecimal.Zero, fmt.Errorf("decimal exponent outside order-math range: %w", err)
		}
		text = text[:index]
	}
	fraction := int64(0)
	if index := strings.IndexByte(text, '.'); index >= 0 {
		fraction = int64(len(text) - index - 1)
		text = text[:index] + text[index+1:]
	}
	text = strings.TrimLeft(text, "0")
	if text == "" {
		return udecimal.Zero, nil
	}
	// Removing trailing zeros before expansion admits exact values such as 1e-19
	// and 0.010000000000000000000, without accepting inexact tiny fee rates.
	trimmed := strings.TrimRight(text, "0")
	zeros := int64(len(text) - len(trimmed))
	text = trimmed
	// Bound exponent before arithmetic and allocation. udecimal's coefficient
	// fits 128 bits and its maximum scale is 19.
	if exponent < -int64(len(d))-19 || exponent > int64(len(d))+39 {
		return udecimal.Zero, fmt.Errorf("decimal %q outside order-math range", d)
	}
	scale := fraction - exponent - zeros
	if scale > 19 || int64(len(text))-scale > 39 {
		return udecimal.Zero, fmt.Errorf("decimal %q outside order-math range", d)
	}
	if scale <= 0 {
		text += strings.Repeat("0", int(-scale))
	} else if scale >= int64(len(text)) {
		text = "0." + strings.Repeat("0", int(scale)-len(text)) + text
	} else {
		index := len(text) - int(scale)
		text = text[:index] + "." + text[index:]
	}
	value, err := udecimal.Parse(sign + text)
	if err != nil {
		return udecimal.Zero, fmt.Errorf("decimal %q outside order-math range: %w", d, err)
	}
	return value, nil
}
