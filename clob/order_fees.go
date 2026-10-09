package clob

import (
	"context"
	"fmt"

	"github.com/quagmt/udecimal"
)

func (c *Client) adjustOrderBuyAmount(
	ctx context.Context,
	assetID string,
	amount, price, budget udecimal.Decimal,
	builderCode string,
) (udecimal.Decimal, error) {
	metadata, err := c.resolveOrderMarketMetadata(ctx, assetID, false)
	if err != nil {
		return udecimal.Zero, fmt.Errorf("resolve order fee metadata: %w", err)
	}
	builderFee, err := c.resolveBuilderFeeRateCached(ctx, builderCode)
	if err != nil {
		return udecimal.Zero, fmt.Errorf("resolve builder fee rate: %w", err)
	}
	return adjustMarketBuyAmount(
		amount,
		budget,
		price,
		metadata.FeeInfo.Rate,
		metadata.FeeInfo.Exponent,
		udecimal.MustFromInt64(int64(builderFee.BuilderTakerFeeRateBps), 4),
	)
}
