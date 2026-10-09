package clob

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func (c *Client) resolveBuilderFeeRateCached(
	ctx context.Context,
	builderCode string,
) (*BuilderFeeRateResponse, error) {
	if builderCode == "" || builderCode == zeroBytes32 {
		return &BuilderFeeRateResponse{}, nil
	}
	var load *builderFeeLoad
	var generation uint64
	for {
		c.orderMetadataMu.Lock()
		currentGeneration := c.orderMetadataGeneration
		if entry, ok := c.builderFeeCache[builderCode]; ok && time.Now().Before(entry.expiresAt) {
			value := entry.value
			c.orderMetadataMu.Unlock()
			return &value, nil
		}
		if existing, ok := c.builderFeeLoads[builderCode]; ok {
			c.orderMetadataMu.Unlock()
			if existing.generation != currentGeneration {
				select {
				case <-existing.done:
					continue
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			select {
			case <-existing.done:
				if ctx.Err() == nil &&
					(errors.Is(existing.err, context.Canceled) || errors.Is(existing.err, context.DeadlineExceeded)) {
					continue
				}
				if existing.err != nil {
					return nil, existing.err
				}
				value := existing.value
				return &value, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		generation = currentGeneration
		load = &builderFeeLoad{
			done:       make(chan struct{}),
			generation: generation,
		}
		c.builderFeeLoads[builderCode] = load
		c.orderMetadataMu.Unlock()
		break
	}

	value, err := c.GetBuilderFeeRate(ctx, builderCode)
	if err == nil && value == nil {
		err = fmt.Errorf("builder fee rate response is empty")
	}

	c.orderMetadataMu.Lock()
	load.err = err
	if err == nil && value != nil {
		load.value = *value
		if generation == c.orderMetadataGeneration {
			c.builderFeeCache[builderCode] = builderFeeEntry{
				value:     *value,
				expiresAt: time.Now().Add(c.orderMetadataTTL()),
			}
		}
	}
	if existing, ok := c.builderFeeLoads[builderCode]; ok && existing == load {
		delete(c.builderFeeLoads, builderCode)
	}
	close(load.done)
	c.orderMetadataMu.Unlock()
	return value, err
}

type builderFeeLoad struct {
	done       chan struct{}
	value      BuilderFeeRateResponse
	err        error
	generation uint64
}

type builderFeeEntry struct {
	value     BuilderFeeRateResponse
	expiresAt time.Time
}
