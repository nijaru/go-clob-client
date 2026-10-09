package clob

import (
	"context"
	"fmt"

	"github.com/quagmt/udecimal"
)

// CalculateMarketPrice derives a marketable price from the current order book.
// For BUY orders, amount is the USDC notional to spend; for SELL orders, amount
// is the number of shares to sell.
func (c *Client) CalculateMarketPrice(
	ctx context.Context,
	tokenID string,
	side Side,
	amount udecimal.Decimal,
	orderType OrderType,
) (udecimal.Decimal, error) {
	if amount.Cmp(udecimal.Zero) <= 0 {
		return udecimal.Zero, fmt.Errorf("amount must be positive")
	}
	if orderType == "" {
		orderType = OrderTypeFOK
	}

	book, err := c.GetOrderBook(ctx, tokenID)
	if err != nil {
		return udecimal.Zero, err
	}

	var levels []OrderSummary
	switch side {
	case SideBuy:
		levels = book.Asks
	case SideSell:
		levels = book.Bids
	default:
		return udecimal.Zero, fmt.Errorf("invalid side %q", side)
	}

	if len(levels) == 0 {
		return udecimal.Zero, fmt.Errorf("no opposing orders for token %s", tokenID)
	}

	// Pre-parse the book levels to avoid string conversion in the loop
	type levelData struct {
		Price udecimal.Decimal
		Size  udecimal.Decimal
	}
	parsedLevels := make([]levelData, len(levels))
	for i, l := range levels {
		p, err := udecimal.Parse(l.Price)
		if err != nil {
			return udecimal.Zero, fmt.Errorf("parse price: %w", err)
		}
		s, err := udecimal.Parse(l.Size)
		if err != nil {
			return udecimal.Zero, fmt.Errorf("parse size: %w", err)
		}
		parsedLevels[i] = levelData{Price: p, Size: s}
	}

	sum := udecimal.Zero
	// Top of the book is at the end of the array (API returns Bids ASC, Asks DESC).
	for i := len(parsedLevels) - 1; i >= 0; i-- {
		level := parsedLevels[i]
		if side == SideBuy {
			// BUY: amount is USDC notional, accumulate size * price
			sum = sum.Add(level.Size.Mul(level.Price))
		} else {
			// SELL: amount is shares, accumulate size
			sum = sum.Add(level.Size)
		}

		if sum.Cmp(amount) >= 0 {
			return level.Price, nil
		}
	}

	if orderType == OrderTypeFOK {
		return udecimal.Zero, fmt.Errorf("insufficient liquidity to fill amount %s", amount)
	}

	return parsedLevels[0].Price, nil
}
