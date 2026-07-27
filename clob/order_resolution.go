package clob

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

const (
	postOrderTradeResolutionTimeout  = 30 * time.Second
	postOrderTradeResolutionInterval = 250 * time.Millisecond
)

// OrderSettlementOptions controls WaitForOrderFillSettlement.
type OrderSettlementOptions struct {
	// Timeout is the maximum time to wait for all fills to reach a terminal
	// state. Zero uses the 30-second default.
	Timeout time.Duration
	// PollInterval is the delay between trade-status requests. Zero uses the
	// 250-millisecond default.
	PollInterval time.Duration
}

func (o OrderSettlementOptions) normalized() (time.Duration, time.Duration, error) {
	if o.Timeout < 0 || o.PollInterval < 0 {
		return 0, 0, fmt.Errorf(
			"%w: timeout and poll interval must not be negative",
			ErrInvalidSettlementOptions,
		)
	}
	timeout := o.Timeout
	if timeout == 0 {
		timeout = postOrderTradeResolutionTimeout
	}
	interval := o.PollInterval
	if interval == 0 {
		interval = postOrderTradeResolutionInterval
	}
	return timeout, interval, nil
}

func shouldResolvePostOrder(request PostOrderRequest, response PostOrderResponse) bool {
	return !request.DeferExec &&
		len(response.TransactionsHashes) == 0 &&
		len(response.TradeIDs) > 0
}

func normalizedTradeStatus(status string) string {
	status = strings.ToUpper(strings.TrimSpace(status))
	return strings.TrimPrefix(status, "TRADE_STATUS_")
}

func tradeIsFailed(trade Trade) bool {
	return normalizedTradeStatus(trade.Status) == "FAILED"
}

func tradeIsResolved(trade Trade) bool {
	switch normalizedTradeStatus(trade.Status) {
	case "CONFIRMED", "FAILED":
		return true
	default:
		return false
	}
}

// WaitForOrderFillSettlement waits until every fill listed in an accepted
// order response reaches a terminal settlement outcome and returns the
// settlement transaction hashes.
//
// It covers only the fills identified by the response's TradeIDs. It does not
// wait for later fills of an order that remains open on the book. A failed fill
// contributes no hash; an error is returned only when every fill fails or the
// timeout expires.
func (c *AuthenticatedClient) WaitForOrderFillSettlement(
	ctx context.Context,
	order PostOrderResponse,
	opts OrderSettlementOptions,
) ([]string, error) {
	tradeIDs := uniqueNonEmptyStrings(order.TradeIDs)
	if len(tradeIDs) == 0 {
		return append([]string(nil), order.TransactionsHashes...), nil
	}

	timeout, interval, err := opts.normalized()
	if err != nil {
		return nil, err
	}
	settled, err := c.waitForResolvedTradesStrict(ctx, tradeIDs, timeout, interval)
	if err != nil {
		return nil, err
	}

	allFailed := true
	for _, trade := range settled {
		if !tradeIsFailed(trade) {
			allFailed = false
			break
		}
	}
	if allFailed {
		return nil, fmt.Errorf(
			"%w: %s",
			ErrSettlementFailed,
			strings.Join(tradeIDs, ", "),
		)
	}

	return transactionHashesForSettledTrades(tradeIDs, settled), nil
}

func (c *AuthenticatedClient) waitForResolvedTradesStrict(
	ctx context.Context,
	tradeIDs []string,
	timeout time.Duration,
	interval time.Duration,
) ([]Trade, error) {
	deadline := time.Now().Add(timeout)
	resolved := make(map[string]Trade, len(tradeIDs))

	for len(resolved) < len(tradeIDs) {
		for _, tradeID := range tradeIDs {
			if _, ok := resolved[tradeID]; ok {
				continue
			}

			var page Page[Trade]
			err := c.getJSON(
				ctx,
				tradesEndpoint,
				tradesQuery(TradeParams{ID: tradeID}, ""),
				polyhttp.AuthL2Builder,
				&page,
			)
			if err != nil {
				return nil, err
			}
			for _, trade := range page.Data {
				if trade.ID == tradeID && tradeIsResolved(trade) {
					resolved[tradeID] = trade
					break
				}
			}
		}

		if len(resolved) == len(tradeIDs) {
			break
		}
		remaining := unresolvedTradeIDs(tradeIDs, resolved)
		if time.Now().After(deadline) {
			return nil, fmt.Errorf(
				"%w: %s",
				ErrSettlementTimeout,
				strings.Join(remaining, ", "),
			)
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	settled := make([]Trade, 0, len(resolved))
	for _, tradeID := range tradeIDs {
		settled = append(settled, resolved[tradeID])
	}
	return settled, nil
}

func unresolvedTradeIDs(tradeIDs []string, resolved map[string]Trade) []string {
	remaining := make([]string, 0, len(tradeIDs)-len(resolved))
	for _, tradeID := range tradeIDs {
		if _, ok := resolved[tradeID]; !ok {
			remaining = append(remaining, tradeID)
		}
	}
	return remaining
}

// waitForResolvedTrades best-effort polls the authenticated trade endpoint for
// the settlement state associated with a post-order response. All IDs share a
// single timeout so a batch cannot multiply the resolution wait per order.
func (c *AuthenticatedClient) waitForResolvedTrades(
	ctx context.Context,
	tradeIDs []string,
) []Trade {
	orderedIDs := uniqueNonEmptyStrings(tradeIDs)
	if len(orderedIDs) == 0 {
		return nil
	}

	resolveCtx, cancel := context.WithTimeout(ctx, postOrderTradeResolutionTimeout)
	defer cancel()

	resolved := make(map[string]Trade, len(orderedIDs))
	for len(resolved) < len(orderedIDs) {
		for _, tradeID := range orderedIDs {
			if _, ok := resolved[tradeID]; ok {
				continue
			}

			var page Page[Trade]
			err := c.getJSON(
				resolveCtx,
				tradesEndpoint,
				tradesQuery(TradeParams{ID: tradeID}, ""),
				polyhttp.AuthL2Builder,
				&page,
			)
			if err != nil {
				continue
			}
			for _, trade := range page.Data {
				if trade.ID == tradeID && tradeIsResolved(trade) {
					resolved[tradeID] = trade
					break
				}
			}
		}

		if len(resolved) == len(orderedIDs) || resolveCtx.Err() != nil {
			break
		}

		timer := time.NewTimer(postOrderTradeResolutionInterval)
		select {
		case <-resolveCtx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}

	trades := make([]Trade, 0, len(resolved))
	for _, tradeID := range orderedIDs {
		if trade, ok := resolved[tradeID]; ok {
			trades = append(trades, trade)
		}
	}
	return trades
}

func transactionHashesForTradeIDs(tradeIDs []string, trades []Trade) []string {
	byID := make(map[string]Trade, len(trades))
	for _, trade := range trades {
		byID[trade.ID] = trade
	}

	hashes := make([]string, 0, len(tradeIDs))
	for _, tradeID := range tradeIDs {
		trade, ok := byID[tradeID]
		if !ok || tradeIsFailed(trade) {
			continue
		}
		hash := strings.TrimSpace(trade.TransactionHash)
		if hash != "" {
			hashes = append(hashes, hash)
		}
	}
	return hashes
}

func transactionHashesForSettledTrades(tradeIDs []string, trades []Trade) []string {
	byID := make(map[string]Trade, len(trades))
	for _, trade := range trades {
		byID[trade.ID] = trade
	}

	hashes := make([]string, 0, len(trades))
	seen := make(map[string]struct{}, len(trades))
	for _, tradeID := range tradeIDs {
		trade, ok := byID[tradeID]
		if !ok || tradeIsFailed(trade) {
			continue
		}
		hash := strings.TrimSpace(trade.TransactionHash)
		if hash == "" {
			continue
		}
		if _, ok := seen[hash]; ok {
			continue
		}
		seen[hash] = struct{}{}
		hashes = append(hashes, hash)
	}
	return hashes
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}
