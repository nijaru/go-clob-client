package clob

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const orderMetadataCacheTTL = 10 * time.Minute

func (c *Client) orderMetadataTTL() time.Duration {
	if c.cacheTTL > 0 {
		return c.cacheTTL
	}
	return orderMetadataCacheTTL
}

type orderMarketMetadata struct {
	ConditionID string
	TickSize    TickSize
	NegRisk     bool
	FeeInfo     FeeInfo
	TokenIDs    map[string]struct{}
}

type orderMetadataEntry struct {
	value     orderMarketMetadata
	expiresAt time.Time
}

type orderConditionLoad struct {
	done        chan struct{}
	conditionID string
	err         error
	generation  uint64
}

type orderMetadataLoad struct {
	done       chan struct{}
	value      orderMarketMetadata
	err        error
	generation uint64
}

// resolveOrderMarketMetadata reads and caches the market metadata used by
// order construction. Conditions are immutable mappings from token IDs, while
// market metadata expires so changes to tick size and fees are observed.
func (c *Client) resolveOrderMarketMetadata(
	ctx context.Context,
	tokenID string,
	force bool,
) (orderMarketMetadata, error) {
	conditionID, err := c.resolveOrderCondition(ctx, tokenID)
	if err != nil {
		return orderMarketMetadata{}, err
	}

	var load *orderMetadataLoad
	var generation uint64
	for {
		c.orderMetadataMu.Lock()
		currentGeneration := c.orderMetadataGeneration
		if !force {
			if entry, ok := c.orderMetadataCache[conditionID]; ok &&
				time.Now().Before(entry.expiresAt) {
				value := entry.value
				c.orderMetadataMu.Unlock()
				return validateOrderMarketToken(value, tokenID)
			}
		}
		if existing, ok := c.orderMetadataLoads[conditionID]; ok {
			c.orderMetadataMu.Unlock()
			if existing.generation != currentGeneration {
				select {
				case <-existing.done:
					continue
				case <-ctx.Done():
					return orderMarketMetadata{}, ctx.Err()
				}
			}
			select {
			case <-existing.done:
				if ctx.Err() == nil &&
					(errors.Is(existing.err, context.Canceled) || errors.Is(existing.err, context.DeadlineExceeded)) {
					continue
				}
				return validateOrderMarketToken(existing.value, tokenID, existing.err)
			case <-ctx.Done():
				return orderMarketMetadata{}, ctx.Err()
			}
		}
		generation = currentGeneration
		load = &orderMetadataLoad{
			done:       make(chan struct{}),
			generation: generation,
		}
		c.orderMetadataLoads[conditionID] = load
		c.orderMetadataMu.Unlock()
		break
	}

	value, err := c.fetchOrderMarketMetadata(ctx, tokenID, conditionID)
	if err == nil {
		value, err = validateOrderMarketToken(value, tokenID)
	}

	c.orderMetadataMu.Lock()
	load.value = value
	load.err = err
	if existing, ok := c.orderMetadataLoads[conditionID]; ok && existing == load {
		delete(c.orderMetadataLoads, conditionID)
	}
	if err != nil && len(value.TokenIDs) > 0 {
		if _, ok := value.TokenIDs[tokenID]; !ok {
			delete(c.orderConditionCache, tokenID)
			delete(c.orderMetadataCache, conditionID)
		}
	}
	if err == nil && generation == c.orderMetadataGeneration {
		c.orderMetadataCache[conditionID] = orderMetadataEntry{
			value:     value,
			expiresAt: time.Now().Add(c.orderMetadataTTL()),
		}
		for siblingTokenID := range value.TokenIDs {
			c.orderConditionCache[siblingTokenID] = conditionID
		}
		// A forced snapshot refresh also updates the independently fetchable
		// tick/risk fields so the next order cannot reuse the stale grid.
		if value.TickSize != "" {
			c.SetTickSize(tokenID, value.TickSize)
		}
		c.SetNegRisk(tokenID, value.NegRisk)
		for siblingTokenID := range value.TokenIDs {
			if siblingTokenID != tokenID {
				if value.TickSize != "" {
					c.SetTickSize(siblingTokenID, value.TickSize)
				}
				c.SetNegRisk(siblingTokenID, value.NegRisk)
			}
		}
	}
	close(load.done)
	c.orderMetadataMu.Unlock()
	return value, err
}

func (c *Client) resolveOrderCondition(
	ctx context.Context,
	tokenID string,
) (string, error) {
	var load *orderConditionLoad
	var generation uint64
	for {
		c.orderMetadataMu.Lock()
		currentGeneration := c.orderMetadataGeneration
		if conditionID, ok := c.orderConditionCache[tokenID]; ok {
			c.orderMetadataMu.Unlock()
			return conditionID, nil
		}
		if existing, ok := c.orderConditionLoads[tokenID]; ok {
			c.orderMetadataMu.Unlock()
			if existing.generation != currentGeneration {
				select {
				case <-existing.done:
					continue
				case <-ctx.Done():
					return "", ctx.Err()
				}
			}
			select {
			case <-existing.done:
				if ctx.Err() == nil &&
					(errors.Is(existing.err, context.Canceled) || errors.Is(existing.err, context.DeadlineExceeded)) {
					continue
				}
				if existing.err != nil {
					return "", existing.err
				}
				return existing.conditionID, nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		generation = currentGeneration
		load = &orderConditionLoad{
			done:       make(chan struct{}),
			generation: generation,
		}
		c.orderConditionLoads[tokenID] = load
		c.orderMetadataMu.Unlock()
		break
	}

	market, err := c.GetMarketByToken(ctx, tokenID)
	conditionID := ""
	if err == nil {
		conditionID = market.ConditionID
		if conditionID == "" {
			err = fmt.Errorf("resolve token %s to market: missing condition ID", tokenID)
		}
	}
	if err != nil {
		err = fmt.Errorf("resolve token %s to market: %w", tokenID, err)
	}

	c.orderMetadataMu.Lock()
	load.conditionID = conditionID
	load.err = err
	if existing, ok := c.orderConditionLoads[tokenID]; ok && existing == load {
		delete(c.orderConditionLoads, tokenID)
	}
	if err == nil && generation == c.orderMetadataGeneration {
		c.orderConditionCache[tokenID] = conditionID
	}
	close(load.done)
	c.orderMetadataMu.Unlock()
	return conditionID, err
}

func (c *Client) fetchOrderMarketMetadata(
	ctx context.Context,
	tokenID string,
	conditionID string,
) (orderMarketMetadata, error) {
	market, err := c.GetClobMarket(ctx, conditionID)
	if err != nil {
		return orderMarketMetadata{}, fmt.Errorf(
			"fetch market %s for token %s: %w",
			conditionID,
			tokenID,
			err,
		)
	}
	if market.ConditionID == "" {
		market.ConditionID = conditionID
	}

	var tokenIDs map[string]struct{}
	if len(market.Tokens) > 0 {
		tokenIDs = make(map[string]struct{}, len(market.Tokens))
		for _, token := range market.Tokens {
			if token != nil {
				tokenIDs[token.TokenID] = struct{}{}
			}
		}
	}
	feeInfo, err := feeInfoFromMarket(*market)
	if err != nil {
		return orderMarketMetadata{}, fmt.Errorf("market %s fee: %w", conditionID, err)
	}
	return orderMarketMetadata{
		ConditionID: conditionID,
		TickSize:    TickSize(market.MinTickSize),
		NegRisk:     market.NegRisk,
		FeeInfo:     feeInfo,
		TokenIDs:    tokenIDs,
	}, nil
}

func validateOrderMarketToken(
	value orderMarketMetadata,
	tokenID string,
	err ...error,
) (orderMarketMetadata, error) {
	if len(err) > 0 && err[0] != nil {
		return orderMarketMetadata{}, err[0]
	}
	if len(value.TokenIDs) > 0 {
		if _, ok := value.TokenIDs[tokenID]; !ok {
			return orderMarketMetadata{}, fmt.Errorf(
				"market %s does not contain token %s",
				value.ConditionID,
				tokenID,
			)
		}
	}
	return value, nil
}

func feeInfoFromMarket(market ClobMarketInfoResponse) (FeeInfo, error) {
	if market.FeeDetails == nil {
		return FeeInfo{}, nil
	}
	rate, err := market.FeeDetails.Rate.Decimal()
	if err != nil {
		return FeeInfo{}, err
	}
	return FeeInfo{Rate: rate, Exponent: market.FeeDetails.Exponent}, nil
}

func (c *Client) clearOrderMetadata(tokenID string) {
	c.orderMetadataMu.Lock()
	c.orderMetadataGeneration++
	conditionID := c.orderConditionCache[tokenID]
	delete(c.orderConditionCache, tokenID)
	if conditionID != "" {
		delete(c.orderMetadataCache, conditionID)
	}
	c.orderMetadataMu.Unlock()
}

func (c *Client) clearAllOrderMetadata() {
	c.orderMetadataMu.Lock()
	c.orderMetadataGeneration++
	clear(c.orderConditionCache)
	clear(c.orderMetadataCache)
	clear(c.builderFeeCache)
	c.orderMetadataMu.Unlock()
}
