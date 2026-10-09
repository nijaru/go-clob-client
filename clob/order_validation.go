package clob

import (
	"fmt"
	"time"

	"github.com/quagmt/udecimal"
)

// minGTDExpirationSeconds is the minimum lead time a GTD (Good-Til-Date)
// limit order expiration must have. The CLOB rejects GTD orders expiring
// sooner, so this fails fast before signing. Mirrors the ts-sdk and py-sdk
// 3-minute client guard (not enforced by the Rust SDK, which is permissive).
const minGTDExpirationSeconds = 180

// validateGTDExpiration rejects GTD expirations closer than
// minGTDExpirationSeconds to now. A zero expiration (GTC semantics) is
// always valid. now is the current Unix time in seconds.
func validateGTDExpiration(expiration uint64, now int64) error {
	if expiration == 0 {
		return nil
	}
	minimum := uint64(now) + minGTDExpirationSeconds
	if expiration < minimum {
		return fmt.Errorf(
			"GTD expiration %d must be at least %d seconds (%d minutes) in the future",
			expiration, minGTDExpirationSeconds, minGTDExpirationSeconds/60,
		)
	}
	return nil
}

func validateLimitOrderArgs(order OrderArgs) error {
	if order.TokenID == "" && order.PositionID == "" {
		return fmt.Errorf("order requires exactly one of TokenID or PositionID")
	}
	if order.TokenID != "" && order.PositionID != "" {
		return fmt.Errorf("order must not set both TokenID and PositionID")
	}
	if order.Size.Cmp(udecimal.Zero) <= 0 {
		return fmt.Errorf("size must be positive")
	}
	if order.Price.Cmp(udecimal.Zero) <= 0 {
		return fmt.Errorf("price must be positive")
	}
	if order.Side != SideBuy && order.Side != SideSell {
		return fmt.Errorf("invalid side %q", order.Side)
	}
	if order.MaxSpend != nil && (order.Side != SideBuy || order.MaxSpend.Cmp(udecimal.Zero) <= 0) {
		return fmt.Errorf("MaxSpend must be positive and is only valid for BUY orders")
	}
	if err := validateGTDExpiration(order.Expiration, time.Now().Unix()); err != nil {
		return err
	}
	return nil
}

func validatePrice(price udecimal.Decimal, tickSize TickSize) error {
	value := price
	minimum, err := parseTickSize(tickSize)
	if err != nil {
		return err
	}
	maximum := udecimal.MustFromInt64(1, 0).Sub(minimum)
	if value.Cmp(minimum) >= 0 && value.Cmp(maximum) <= 0 {
		return nil
	}
	return fmt.Errorf("invalid price (%s), min: %s - max: %s", price, minimum, maximum)
}

func validateLimitPricePrecision(price udecimal.Decimal, tickSize TickSize) error {
	minimum, err := parseTickSize(tickSize)
	if err != nil {
		return err
	}
	if decimalPlaces(price) <= decimalPlaces(minimum) {
		return nil
	}
	return fmt.Errorf(
		"price %s exceeds maximum %d decimal places for tick size %q",
		price,
		decimalPlaces(minimum),
		tickSize,
	)
}

func sideValue(side Side) int {
	if side == SideSell {
		return 1
	}
	return 0
}

func normalizeTaker(taker string) string {
	if taker == "" {
		return zeroAddress
	}
	return taker
}

// validateTickAlignment checks that price is aligned to the minimum tick size grid.
// This mirrors the Rust SDK's price_aligned_to_tick_size check.
func validateTickAlignment(price udecimal.Decimal, tickSize TickSize) error {
	minimum, err := parseTickSize(tickSize)
	if err != nil {
		return err
	}
	rem, err := price.Mod(minimum)
	if err != nil {
		return fmt.Errorf("tick alignment check: %w", err)
	}
	if !rem.IsZero() {
		return fmt.Errorf(
			"price %s is not aligned to the minimum tick size %s",
			price, tickSize,
		)
	}
	return nil
}

func isTickSizeSmaller(a, b TickSize) (bool, error) {
	aParsed, err := udecimal.Parse(string(a))
	if err != nil {
		return false, fmt.Errorf("parse tick size %q: %w", a, err)
	}
	bParsed, err := udecimal.Parse(string(b))
	if err != nil {
		return false, fmt.Errorf("parse tick size %q: %w", b, err)
	}
	return aParsed.Cmp(bParsed) < 0, nil
}
