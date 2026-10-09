package clob

import (
	"context"
	"fmt"
	"time"

	"github.com/quagmt/udecimal"
)

func (c *Client) validateLimitPriceWithRefresh(
	ctx context.Context,
	tokenID string,
	price udecimal.Decimal,
	tickSize TickSize,
	options *CreateOrderOptions,
) (TickSize, error) {
	validate := func(size TickSize) error {
		if err := validatePrice(price, size); err != nil {
			return err
		}
		if err := validateLimitPricePrecision(price, size); err != nil {
			return err
		}
		return validateTickAlignment(price, size)
	}
	if err := validate(tickSize); err == nil {
		return tickSize, nil
	} else if options != nil && options.TickSize != "" {
		return tickSize, err
	} else {
		metadata, refreshErr := c.resolveOrderMarketMetadata(ctx, tokenID, true)
		if refreshErr == nil && metadata.TickSize != "" {
			if refreshedErr := validate(metadata.TickSize); refreshedErr == nil {
				return metadata.TickSize, nil
			} else {
				return metadata.TickSize, refreshedErr
			}
		}
		return tickSize, err
	}
}

func (c *Client) cachedTickSize(tokenID string) (TickSize, bool) {
	c.tickSizeMu.RLock()
	cached, ok := c.tickSizeCache[tokenID]
	ts := c.tickSizeTimestamps[tokenID]
	c.tickSizeMu.RUnlock()
	if ok && (c.cacheTTL == 0 || time.Since(ts) < c.cacheTTL) {
		return cached, true
	}
	return "", false
}

func (c *Client) cachedNegRisk(tokenID string) (bool, bool) {
	c.negRiskMu.RLock()
	cached, ok := c.negRiskCache[tokenID]
	ts := c.negRiskTimestamps[tokenID]
	c.negRiskMu.RUnlock()
	if ok && (c.cacheTTL == 0 || time.Since(ts) < c.cacheTTL) {
		return cached, true
	}
	return false, false
}

func (c *Client) resolveTickSize(
	ctx context.Context,
	tokenID string,
	options *CreateOrderOptions,
) (TickSize, error) {
	c.tickSizeMu.RLock()
	cached, ok := c.tickSizeCache[tokenID]
	ts := c.tickSizeTimestamps[tokenID]
	generation := c.tickSizeGeneration
	c.tickSizeMu.RUnlock()

	var marketTickSize TickSize
	if ok && (c.cacheTTL == 0 || time.Since(ts) < c.cacheTTL) {
		marketTickSize = cached
	} else {
		response, err := c.GetTickSize(ctx, tokenID)
		if err != nil {
			return "", err
		}
		marketTickSize = response.MinimumTickSize

		c.tickSizeMu.Lock()
		if generation == c.tickSizeGeneration {
			c.tickSizeCache[tokenID] = marketTickSize
			c.tickSizeTimestamps[tokenID] = time.Now()
		}
		c.tickSizeMu.Unlock()
	}

	if options != nil && options.TickSize != "" {
		smaller, err := isTickSizeSmaller(options.TickSize, marketTickSize)
		if err != nil {
			return "", fmt.Errorf("invalid tick size option: %w", err)
		}
		if smaller {
			return "", fmt.Errorf(
				"invalid tick size %q, minimum for market is %q",
				options.TickSize,
				marketTickSize,
			)
		}
		return options.TickSize, nil
	}

	return marketTickSize, nil
}

func (c *Client) resolveNegRisk(
	ctx context.Context,
	tokenID string,
	options *CreateOrderOptions,
) (bool, error) {
	if options != nil && options.NegRisk != nil {
		return *options.NegRisk, nil
	}

	c.negRiskMu.RLock()
	cached, ok := c.negRiskCache[tokenID]
	ts := c.negRiskTimestamps[tokenID]
	generation := c.negRiskGeneration
	c.negRiskMu.RUnlock()

	if ok && (c.cacheTTL == 0 || time.Since(ts) < c.cacheTTL) {
		return cached, nil
	}

	response, err := c.GetNegRisk(ctx, tokenID)
	if err != nil {
		return false, err
	}

	c.negRiskMu.Lock()
	if generation == c.negRiskGeneration {
		c.negRiskCache[tokenID] = response.NegRisk
		c.negRiskTimestamps[tokenID] = time.Now()
	}
	c.negRiskMu.Unlock()

	return response.NegRisk, nil
}
