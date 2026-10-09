package clob

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/quagmt/udecimal"
)

var errProtectedMarketAmounts = errors.New("unsafe protected market order amounts")

// CreateMarketOrder builds and signs a market order.
func (c *SignerClient) CreateMarketOrder(
	ctx context.Context,
	userOrder MarketOrderArgs,
	options *CreateOrderOptions,
) (*SignedOrder, error) {
	if err := validateMarketOrderArgs(userOrder); err != nil {
		return nil, err
	}

	protectedPrice := marketOrderPriceBound(userOrder)
	if protectedPrice != nil {
		// Explicit prices may be stricter than the protection, never weaker.
		if userOrder.Price.IsZero() ||
			(userOrder.Side == SideBuy && userOrder.Price.Cmp(*protectedPrice) > 0) ||
			(userOrder.Side == SideSell && userOrder.Price.Cmp(*protectedPrice) < 0) {
			userOrder.Price = *protectedPrice
		}
	}

	if userOrder.OrderType == "" {
		userOrder.OrderType = OrderTypeFOK
	}
	if userOrder.OrderType != OrderTypeFOK && userOrder.OrderType != OrderTypeFAK {
		return nil, fmt.Errorf("market orders only support FOK or FAK order types")
	}

	assetID := orderAssetID(userOrder.TokenID, userOrder.PositionID)

	tickSize, err := c.resolveTickSize(ctx, assetID, options)
	if err != nil {
		return nil, err
	}

	if protectedPrice != nil {
		tickSize, err = c.validateLimitPriceWithRefresh(
			ctx,
			assetID,
			*protectedPrice,
			tickSize,
			options,
		)
		if err != nil {
			return nil, err
		}
	}

	if userOrder.Price.IsZero() {
		price, err := c.CalculateMarketPrice(
			ctx,
			assetID,
			userOrder.Side,
			userOrder.Amount,
			userOrder.OrderType,
		)
		if err != nil {
			return nil, err
		}
		userOrder.Price = price
	}

	tickSize, err = c.validateLimitPriceWithRefresh(
		ctx,
		assetID,
		userOrder.Price,
		tickSize,
		options,
	)
	if err != nil {
		return nil, err
	}

	isNegRisk := false
	if userOrder.PositionID == "" {
		isNegRisk, err = c.resolveNegRisk(ctx, assetID, options)
		if err != nil {
			return nil, err
		}
	}

	order, err := c.buildSignedMarketOrder(ctx, userOrder, CreateOrderOptions{
		TickSize: tickSize,
		NegRisk:  new(isNegRisk),
	})
	if !errors.Is(err, errProtectedMarketAmounts) || (options != nil && options.TickSize != "") {
		return order, err
	}

	// A cap can be tick-aligned yet unsafe under a stale, coarse rounding
	// configuration. Refresh the whole market snapshot and rebuild once;
	// never relax the guard or override a caller-supplied tick.
	metadata, err := c.resolveOrderMarketMetadata(ctx, assetID, true)
	if err != nil {
		return nil, fmt.Errorf("refresh protected order metadata: %w", err)
	}
	for _, price := range []udecimal.Decimal{*protectedPrice, userOrder.Price} {
		if err := validateMarketPriceBound(price, metadata.TickSize); err != nil {
			return nil, err
		}
	}
	isNegRisk = false
	if userOrder.PositionID == "" {
		isNegRisk = metadata.NegRisk
	}
	if userOrder.PositionID == "" && options != nil && options.NegRisk != nil {
		isNegRisk = *options.NegRisk
	}
	return c.buildSignedMarketOrder(ctx, userOrder, CreateOrderOptions{
		TickSize: metadata.TickSize,
		NegRisk:  new(isNegRisk),
	})
}

func (c *SignerClient) buildSignedMarketOrder(
	ctx context.Context,
	userOrder MarketOrderArgs,
	options CreateOrderOptions,
) (*SignedOrder, error) {
	roundConfig, ok := roundingConfig[options.TickSize]
	if !ok {
		return nil, fmt.Errorf("unsupported tick size %q", options.TickSize)
	}

	price := roundDown(userOrder.Price, roundConfig.Price)
	amount := userOrder.Amount

	var rawMakerAmount udecimal.Decimal
	var rawTakerAmount udecimal.Decimal

	switch userOrder.Side {
	case SideBuy:
		// BUY: Amount is USDC notional. Adjust for fees if MaxSpend is set.
		adjustedAmount := amount
		if userOrder.MaxSpend != nil {
			var err error
			adjustedAmount, err = c.adjustOrderBuyAmount(
				ctx,
				orderAssetID(userOrder.TokenID, userOrder.PositionID),
				amount,
				price,
				*userOrder.MaxSpend,
				userOrder.BuilderCode,
			)
			if err != nil {
				return nil, err
			}
		}
		// Preserve the full USDC amount; only the derived share quantity is quantized.
		rawMakerAmount = adjustedAmount
		if userOrder.MaxPrice != nil {
			// Derive shares from the exact USDC amount that will be signed.
			rawMakerAmount = roundDown(rawMakerAmount, 6)
		}
		val, err := rawMakerAmount.Div(price)
		if err != nil {
			return nil, fmt.Errorf("calculation error: %w", err)
		}
		if userOrder.MaxPrice != nil {
			// Rounding shares up can put maker/taker below the resting ask.
			rawTakerAmount = roundDown(val, roundConfig.Amount)
		} else {
			rawTakerAmount = roundToAmount(val, roundConfig)
		}
	case SideSell:
		// SELL: Amount is shares.
		rawMakerAmount = roundDown(amount, roundConfig.Size)
		rawTakerAmount = roundToAmount(rawMakerAmount.Mul(price), roundConfig)
	default:
		return nil, fmt.Errorf("invalid side %q", userOrder.Side)
	}

	makerAmount, takerAmount := toTokenDecimals(rawMakerAmount), toTokenDecimals(rawTakerAmount)
	if marketOrderPriceBound(userOrder) != nil {
		if err := validateProtectedMarketAmounts(makerAmount, takerAmount, userOrder.Side, price); err != nil {
			return nil, err
		}
	}

	return c.signOrder(ctx, orderBuildInput{
		TokenID:       userOrder.TokenID,
		PositionID:    userOrder.PositionID,
		MakerAmount:   decimalInteger(makerAmount),
		TakerAmount:   decimalInteger(takerAmount),
		Side:          userOrder.Side,
		Expiration:    0,
		NegRisk:       derefBool(options.NegRisk),
		SignatureType: c.signatureType,
		Metadata:      userOrder.Metadata,
		BuilderCode:   userOrder.BuilderCode,
		Taker:         userOrder.Taker,
		Nonce:         userOrder.Nonce,
		FeeRateBps:    userOrder.FeeRateBps,
	})
}

func validateMarketOrderArgs(order MarketOrderArgs) error {
	if order.TokenID == "" && order.PositionID == "" {
		return fmt.Errorf("order requires exactly one of TokenID or PositionID")
	}
	if order.TokenID != "" && order.PositionID != "" {
		return fmt.Errorf("order must not set both TokenID and PositionID")
	}
	if order.Amount.Cmp(udecimal.Zero) <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	if order.Price.Cmp(udecimal.Zero) < 0 {
		return fmt.Errorf("price cannot be negative")
	}
	if order.Side != SideBuy && order.Side != SideSell {
		return fmt.Errorf("invalid side %q", order.Side)
	}
	if order.Side == SideBuy && order.MinPrice != nil {
		return fmt.Errorf("MinPrice is only valid for SELL market orders")
	}
	if order.Side == SideSell && (order.MaxPrice != nil || order.MaxSpend != nil) {
		return fmt.Errorf("MaxPrice and MaxSpend are only valid for BUY market orders")
	}
	for _, bound := range []struct {
		name  string
		value *udecimal.Decimal
	}{{"MaxPrice", order.MaxPrice}, {"MinPrice", order.MinPrice}, {"MaxSpend", order.MaxSpend}} {
		if bound.value != nil && bound.value.Cmp(udecimal.Zero) <= 0 {
			return fmt.Errorf("%s must be positive", bound.name)
		}
	}
	return nil
}

func marketOrderPriceBound(order MarketOrderArgs) *udecimal.Decimal {
	if order.Side == SideBuy {
		return order.MaxPrice
	}
	return order.MinPrice
}

func validateMarketPriceBound(price udecimal.Decimal, tick TickSize) error {
	if err := validatePrice(price, tick); err != nil {
		return err
	}
	return validateTickAlignment(price, tick)
}

// Compare final wire amounts exactly, without a rounded price quotient. BUY
// rounding may encode a price slightly above the cap, but must exclude the
// next possible ask even if the tick refines after signing (py-sdk 1ed8b7cd).
func validateProtectedMarketAmounts(
	maker, taker udecimal.Decimal,
	side Side,
	price udecimal.Decimal,
) error {
	if maker.IsZero() || taker.IsZero() {
		return fmt.Errorf("%w: amount rounds to zero", errProtectedMarketAmounts)
	}
	makerRatio, _ := new(big.Rat).SetString(maker.String())
	takerRatio, _ := new(big.Rat).SetString(taker.String())
	limit, _ := new(big.Rat).SetString(price.String())
	if side == SideSell {
		if new(big.Rat).Quo(takerRatio, makerRatio).Cmp(limit) < 0 {
			return fmt.Errorf(
				"%w: cannot preserve MinPrice with this SELL amount and tick precision",
				errProtectedMarketAmounts,
			)
		}
		return nil
	}
	encoded := new(big.Rat).Quo(makerRatio, takerRatio)
	nextAsk := new(big.Rat).Add(limit, big.NewRat(1, 10000))
	if encoded.Cmp(limit) < 0 || encoded.Cmp(nextAsk) >= 0 {
		return fmt.Errorf(
			"%w: cannot preserve MaxPrice with this BUY amount and tick precision; increase Amount or choose a different price",
			errProtectedMarketAmounts,
		)
	}
	return nil
}
