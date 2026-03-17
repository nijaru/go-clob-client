package clob

import (
	"context"
	"fmt"

	"github.com/quagmt/udecimal"
)

// CreateRFQRequestFromOrder is a convenience helper that takes user-friendly order parameters
// and converts them into the raw wire format for an RFQ request.
// It resolves the market tick size and rounds the price and size accordingly.
func (c *AuthenticatedClient) CreateRFQRequestFromOrder(
	ctx context.Context,
	tokenID string,
	side Side,
	price float64,
	size float64,
	options *CreateOrderOptions,
) (*RFQRequestResponse, error) {
	tickSize, err := c.resolveTickSize(ctx, tokenID, options)
	if err != nil {
		return nil, err
	}

	config, ok := roundingConfig[tickSize]
	if !ok {
		return nil, fmt.Errorf("unsupported tick size %q", tickSize)
	}

	dPrice := udecimal.MustFromFloat64(price)
	dSize := udecimal.MustFromFloat64(size)

	roundedPrice := roundNormal(dPrice, config.Price)
	roundedSize := roundDown(dSize, config.Size)

	sizeNum := roundedSize
	priceNum := roundedPrice

	var amountIn, amountOut udecimal.Decimal
	var assetIn, assetOut string

	if side == SideBuy {
		// Buying: pay USDC (asset 0), receive tokens (tokenID)
		amountIn = toTokenDecimals(roundedSize)
		amountOut = toTokenDecimals(roundNormal(sizeNum.Mul(priceNum), config.Amount))
		assetIn = tokenID
		assetOut = "0" // USDC
	} else {
		// Selling: pay tokens (tokenID), receive USDC (asset 0)
		amountIn = toTokenDecimals(roundNormal(sizeNum.Mul(priceNum), config.Amount))
		amountOut = toTokenDecimals(roundedSize)
		assetIn = "0" // USDC
		assetOut = tokenID
	}

	return c.CreateRFQRequest(ctx, CreateRFQRequestParams{
		AssetIn:   assetIn,
		AssetOut:  assetOut,
		AmountIn:  amountIn,
		AmountOut: amountOut,
		UserType:  int(c.signatureType),
	})
}

// CreateRFQQuoteFromOrder is a convenience helper that takes user-friendly order parameters
// and converts them into the raw wire format for an RFQ quote.
func (c *AuthenticatedClient) CreateRFQQuoteFromOrder(
	ctx context.Context,
	requestID string,
	tokenID string,
	side Side,
	price float64,
	size float64,
	options *CreateOrderOptions,
) (*RFQQuoteResponse, error) {
	tickSize, err := c.resolveTickSize(ctx, tokenID, options)
	if err != nil {
		return nil, err
	}

	config, ok := roundingConfig[tickSize]
	if !ok {
		return nil, fmt.Errorf("unsupported tick size %q", tickSize)
	}

	dPrice := udecimal.MustFromFloat64(price)
	dSize := udecimal.MustFromFloat64(size)

	roundedPrice := roundNormal(dPrice, config.Price)
	roundedSize := roundDown(dSize, config.Size)

	sizeNum := roundedSize
	priceNum := roundedPrice

	var amountIn, amountOut udecimal.Decimal
	var assetIn, assetOut string

	if side == SideSell {
		// Quoter selling tokens: receive USDC (asset 0), give tokens (tokenID)
		amountIn = toTokenDecimals(roundNormal(sizeNum.Mul(priceNum), config.Amount))
		amountOut = toTokenDecimals(roundedSize)
		assetIn = "0" // USDC
		assetOut = tokenID
	} else {
		// Quoter buying tokens: receive tokens (tokenID), give USDC (asset 0)
		amountIn = toTokenDecimals(roundedSize)
		amountOut = toTokenDecimals(roundNormal(sizeNum.Mul(priceNum), config.Amount))
		assetIn = tokenID
		assetOut = "0" // USDC
	}

	return c.CreateRFQQuote(ctx, CreateRFQQuoteParams{
		RequestID: requestID,
		AssetIn:   assetIn,
		AssetOut:  assetOut,
		AmountIn:  amountIn,
		AmountOut: amountOut,
		UserType:  int(c.signatureType),
	})
}
