package clob

import (
	"fmt"

	"github.com/quagmt/udecimal"
)

var tokenScaleFactor = udecimal.MustFromInt64(1000000, 0) // 10^6

var roundingConfig = map[TickSize]roundConfig{
	TickSizeTenth:       {Price: 1, Size: 2, Amount: 3},
	TickSizeHundredth:   {Price: 2, Size: 2, Amount: 4},
	TickSizeHalfCent:    {Price: 3, Size: 2, Amount: 5},
	TickSizeQuarterCent: {Price: 4, Size: 2, Amount: 6},
	TickSizeThousandth:  {Price: 3, Size: 2, Amount: 5},
	TickSizeTenThousand: {Price: 4, Size: 2, Amount: 6},
}

func roundDown(value udecimal.Decimal, places uint8) udecimal.Decimal {
	return value.Trunc(places)
}

func roundNormal(value udecimal.Decimal, places uint8) udecimal.Decimal {
	return value.RoundHAZ(places)
}

func roundUp(value udecimal.Decimal, places uint8) udecimal.Decimal {
	return value.RoundAwayFromZero(places)
}

// roundToAmount quantizes a computed maker/taker amount to the allowed precision.
// The two-pass approach (round-up with 4 extra digits, then round-down if still too long)
// mirrors the Rust SDK's order_builder.rs behavior: prefer rounding up to avoid
// underpaying, but clamp back down if the extra digits don't resolve cleanly.
func roundToAmount(value udecimal.Decimal, cfg roundConfig) udecimal.Decimal {
	if decimalPlaces(value) <= cfg.Amount {
		return value
	}
	v := roundUp(value, cfg.Amount+4)
	if decimalPlaces(v) > cfg.Amount {
		v = roundDown(v, cfg.Amount)
	}
	return v
}

func decimalPlaces(value udecimal.Decimal) uint8 {
	return value.PrecUint()
}

func toTokenDecimals(value udecimal.Decimal) udecimal.Decimal {
	return value.Mul(tokenScaleFactor).Trunc(0)
}

// adjustMarketBuyAmount shrinks a USDC buy amount to fit within the user's
// balance after accounting for platform and builder taker fees.
// Fee formula: platform_fee_rate = rate * (price * (1 - price))^exponent.
// This matches the Rust SDK's adjust_market_buy_amount.
func adjustMarketBuyAmount(
	amount udecimal.Decimal,
	userBalance udecimal.Decimal,
	price udecimal.Decimal,
	feeRate udecimal.Decimal,
	feeExponent uint32,
	builderTakerFeeRate udecimal.Decimal,
) (udecimal.Decimal, error) {
	one := udecimal.MustFromInt64(1, 0)
	base := price.Mul(one.Sub(price))

	// platform_fee_rate = rate * base^exponent. Both inputs are exact
	// decimals, so keep the computation in decimal arithmetic as well.
	platformFeeRate := feeRate.Mul(base.PowInt(int(feeExponent)))

	// platform_fee = amount / price * platform_fee_rate
	amountDivPrice, err := amount.Div(price)
	if err != nil {
		return udecimal.Zero, err
	}
	platformFee := amountDivPrice.Mul(platformFeeRate)

	// total_cost = amount + platform_fee + amount * builder_taker_fee_rate
	builderFee := amount.Mul(builderTakerFeeRate)
	totalCost := amount.Add(platformFee).Add(builderFee)

	var raw udecimal.Decimal
	if userBalance.Cmp(totalCost) <= 0 {
		// Balance insufficient: shrink amount to fit
		// divisor = 1 + platform_fee_rate / price + builder_taker_fee_rate
		feeRateDivPrice, err := platformFeeRate.Div(price)
		if err != nil {
			return udecimal.Zero, err
		}
		divisor := one.Add(feeRateDivPrice).Add(builderTakerFeeRate)
		val, err := userBalance.Div(divisor)
		if err != nil {
			return udecimal.Zero, err
		}
		raw = val
	} else {
		raw = amount
	}

	adjusted := raw.Trunc(6) // USDC_DECIMALS = 6
	if adjusted.IsZero() {
		return udecimal.Zero, fmt.Errorf(
			"user balance %s too small to cover fees at price %s",
			userBalance, price,
		)
	}
	return adjusted, nil
}

func parseTickSize(value TickSize) (udecimal.Decimal, error) {
	parsed, err := udecimal.Parse(string(value))
	if err != nil {
		return udecimal.Zero, fmt.Errorf("parse tick size %q: %w", value, err)
	}
	return parsed, nil
}
