package clob

import (
	"context"
	"strings"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

const (
	postOrderTradeResolutionTimeout  = 30 * time.Second
	postOrderTradeResolutionInterval = 250 * time.Millisecond
)

func shouldResolvePostOrder(request PostOrderRequest, response PostOrderResponse) bool {
	return !request.DeferExec &&
		len(response.TransactionsHashes) == 0 &&
		len(response.TradeIDs) > 0
}

func tradeIsResolved(trade Trade) bool {
	return strings.EqualFold(strings.TrimSpace(trade.Status), "FAILED") ||
		strings.TrimSpace(trade.TransactionHash) != ""
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
		if !ok || strings.EqualFold(strings.TrimSpace(trade.Status), "FAILED") {
			continue
		}
		hash := strings.TrimSpace(trade.TransactionHash)
		if hash != "" {
			hashes = append(hashes, hash)
		}
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
