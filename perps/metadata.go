package perps

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

type PerpsInstrumentSettlement struct {
	Sequence       int64  `json:"sequence"`
	Timestamp      int64  `json:"timestamp"`
	Price          string `json:"price"`
	InsuranceDebit string `json:"insurance_debit"`
}
type PerpsPositionDeleveragedNotification struct {
	ID           string                `json:"id"`
	Type         PerpsNotificationType `json:"type"`
	InstrumentID int                   `json:"instrument_id"`
	Side         PerpsSide             `json:"side"`
	SizeClosed   string                `json:"size_closed"`
	Price        string                `json:"price"`
	PnL          string                `json:"pnl"`
	MarginType   PerpsMarginType       `json:"margin_type"`
}

// Fill fee totals are exact, including maker rebates. big.Int avoids the
// precision/size limits of fixed-width decimal libraries on response data.
func totalFillFee(exchange, builder string) (string, error) {
	if !fixedDecimalPattern.MatchString(exchange) || !fixedDecimalPattern.MatchString(builder) {
		return "", fmt.Errorf("perps: invalid fill fee")
	}
	_, ef, _ := strings.Cut(exchange, ".")
	_, bf, _ := strings.Cut(builder, ".")
	scale := max(len(ef), len(bf))
	units := func(value string) *big.Int {
		whole, fraction, _ := strings.Cut(value, ".")
		n, _ := new(big.Int).SetString(whole+fraction+strings.Repeat("0", scale-len(fraction)), 10)
		return n
	}
	sum := new(big.Int).Add(units(exchange), units(builder))
	negative := sum.Sign() < 0
	sum.Abs(sum)
	digits := sum.String()
	if scale > 0 {
		if len(digits) <= scale {
			digits = strings.Repeat("0", scale+1-len(digits)) + digits
		}
		fraction := strings.TrimRight(digits[len(digits)-scale:], "0")
		digits = digits[:len(digits)-scale]
		if fraction != "" {
			digits += "." + fraction
		}
	}
	if negative {
		digits = "-" + digits
	}
	return digits, nil
}

func (f *PerpsAccountFill) UnmarshalJSON(data []byte) error {
	type alias PerpsAccountFill
	var out alias
	if err := json.Unmarshal(data, &out); err != nil {
		return err
	}
	if out.BuilderFee == "" {
		out.BuilderFee = "0"
	}
	if out.TotalFee == "" {
		value, err := totalFillFee(out.Fee, out.BuilderFee)
		if err != nil {
			return err
		}
		out.TotalFee = value
	}
	*f = PerpsAccountFill(out)
	return nil
}
