package clob

import (
	"context"
	"fmt"

	"github.com/quagmt/udecimal"
)

// CreateOrder builds and signs a limit order.
func (c *SignerClient) CreateOrder(
	ctx context.Context,
	userOrder OrderArgs,
	options *CreateOrderOptions,
) (*SignedOrder, error) {
	if err := validateLimitOrderArgs(userOrder); err != nil {
		return nil, err
	}

	assetID := orderAssetID(userOrder.TokenID, userOrder.PositionID)

	var metadata orderMarketMetadata
	var err error
	tickSizeOption := options != nil && options.TickSize != ""
	negRiskOption := userOrder.PositionID != "" || (options != nil && options.NegRisk != nil)
	marketTickSize, tickCached := c.cachedTickSize(assetID)
	cachedNegRisk, negRiskCached := c.cachedNegRisk(assetID)
	metadataLoaded := (!tickCached && !tickSizeOption) || (!negRiskCached && !negRiskOption)
	if metadataLoaded {
		metadata, err = c.resolveOrderMarketMetadata(ctx, assetID, false)
		if err != nil {
			return nil, err
		}
		if !tickCached {
			marketTickSize = metadata.TickSize
		}
		if !negRiskCached {
			cachedNegRisk = metadata.NegRisk
			negRiskCached = true
		}
	}

	var tickSize TickSize
	if tickSizeOption {
		if marketTickSize == "" {
			marketTickSize, err = c.resolveTickSize(ctx, assetID, nil)
			if err != nil {
				return nil, err
			}
		}
		smaller, err := isTickSizeSmaller(options.TickSize, marketTickSize)
		if err != nil {
			return nil, fmt.Errorf("invalid tick size option: %w", err)
		}
		if smaller {
			return nil, fmt.Errorf(
				"invalid tick size %q, minimum for market is %q",
				options.TickSize,
				marketTickSize,
			)
		}
		tickSize = options.TickSize
	} else if tickCached || metadataLoaded {
		tickSize = marketTickSize
		if tickSize == "" {
			tickSize, err = c.resolveTickSize(ctx, assetID, nil)
			if err != nil {
				return nil, err
			}
		}
	} else {
		tickSize, err = c.resolveTickSize(ctx, assetID, options)
		if err != nil {
			return nil, err
		}
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

	var isNegRisk bool
	if userOrder.PositionID != "" {
		isNegRisk = false
	} else if negRiskOption {
		isNegRisk = *options.NegRisk
	} else if negRiskCached {
		isNegRisk = cachedNegRisk
	} else {
		isNegRisk, err = c.resolveNegRisk(ctx, assetID, options)
		if err != nil {
			return nil, err
		}
	}

	return c.buildSignedLimitOrder(ctx, userOrder, CreateOrderOptions{
		TickSize: tickSize,
		NegRisk:  new(isNegRisk),
	})
}

func (c *SignerClient) buildSignedLimitOrder(
	ctx context.Context,
	userOrder OrderArgs,
	options CreateOrderOptions,
) (*SignedOrder, error) {
	roundConfig, ok := roundingConfig[options.TickSize]
	if !ok {
		return nil, fmt.Errorf("unsupported tick size %q", options.TickSize)
	}

	price := userOrder.Price
	size := userOrder.Size

	if decimalPlaces(size) > roundConfig.Size {
		return nil, fmt.Errorf(
			"size %s exceeds maximum %d decimal places for tick size %q",
			size, roundConfig.Size, options.TickSize,
		)
	}

	rawPrice := roundNormal(price, roundConfig.Price)

	var rawMakerAmount udecimal.Decimal
	var rawTakerAmount udecimal.Decimal

	switch userOrder.Side {
	case SideBuy:
		rawTakerAmount = roundDown(size, roundConfig.Size)
		rawMakerAmount = roundToAmount(rawTakerAmount.Mul(rawPrice), roundConfig)
		if userOrder.MaxSpend != nil {
			adjusted, err := c.adjustOrderBuyAmount(
				ctx,
				orderAssetID(userOrder.TokenID, userOrder.PositionID),
				rawMakerAmount,
				rawPrice,
				*userOrder.MaxSpend,
				userOrder.BuilderCode,
			)
			if err != nil {
				return nil, err
			}
			shares, err := adjusted.Div(rawPrice)
			if err != nil {
				return nil, err
			}
			rawTakerAmount = roundDown(shares, roundConfig.Size)
			rawMakerAmount = roundToAmount(rawTakerAmount.Mul(rawPrice), roundConfig)
		}
	case SideSell:
		rawMakerAmount = roundDown(size, roundConfig.Size)
		rawTakerAmount = roundToAmount(rawMakerAmount.Mul(rawPrice), roundConfig)
	default:
		return nil, fmt.Errorf("invalid side %q", userOrder.Side)
	}

	return c.signOrder(ctx, orderBuildInput{
		TokenID:       userOrder.TokenID,
		PositionID:    userOrder.PositionID,
		MakerAmount:   decimalBaseUnits(rawMakerAmount),
		TakerAmount:   decimalBaseUnits(rawTakerAmount),
		Side:          userOrder.Side,
		Expiration:    userOrder.Expiration,
		NegRisk:       derefBool(options.NegRisk),
		SignatureType: c.signatureType,
		Metadata:      userOrder.Metadata,
		BuilderCode:   userOrder.BuilderCode,
		Taker:         userOrder.Taker,
		Nonce:         userOrder.Nonce,
		FeeRateBps:    userOrder.FeeRateBps,
	})
}
