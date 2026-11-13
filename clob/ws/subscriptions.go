package ws

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"errors"

	"github.com/nijaru/go-clob-client/clob"
)

// SubscribeOrderBook subscribes to order book snapshots and incremental updates for the given asset IDs.
// A full book snapshot (initial_dump) is requested on subscribe.
func (c *Client) SubscribeOrderBook(ctx context.Context, assetIDs []string) error {
	return c.addAndSend(ctx, newMarketSubscription(channelTypeOrderBook, assetIDs, true, false))
}

// UnsubscribeOrderBook unsubscribes from order book updates for the given asset IDs.
func (c *Client) UnsubscribeOrderBook(ctx context.Context, assetIDs []string) error {
	return c.removeAndSend(ctx, channelTypeOrderBook, assetIDs)
}

// SubscribeLastTradePrice subscribes to last trade price events for the given asset IDs.
func (c *Client) SubscribeLastTradePrice(ctx context.Context, assetIDs []string) error {
	return c.addAndSend(
		ctx,
		newMarketSubscription(channelTypeLastTradePrice, assetIDs, false, false),
	)
}

// SubscribePrices subscribes to price change (incremental order book) events for the given asset IDs.
func (c *Client) SubscribePrices(ctx context.Context, assetIDs []string) error {
	return c.addAndSend(ctx, newMarketSubscription(channelTypePrices, assetIDs, false, false))
}

// UnsubscribePrices unsubscribes from price change events for the given asset IDs.
func (c *Client) UnsubscribePrices(ctx context.Context, assetIDs []string) error {
	return c.removeAndSend(ctx, channelTypePrices, assetIDs)
}

// SubscribeTickSizeChange subscribes to tick size change events for the given asset IDs.
func (c *Client) SubscribeTickSizeChange(ctx context.Context, assetIDs []string) error {
	return c.addAndSend(
		ctx,
		newMarketSubscription(channelTypeTickSizeChange, assetIDs, false, false),
	)
}

// UnsubscribeTickSizeChange unsubscribes from tick size change events for the given asset IDs.
func (c *Client) UnsubscribeTickSizeChange(ctx context.Context, assetIDs []string) error {
	return c.removeAndSend(ctx, channelTypeTickSizeChange, assetIDs)
}

// SubscribeMidpoints subscribes to midpoint events for the given asset IDs.
func (c *Client) SubscribeMidpoints(ctx context.Context, assetIDs []string) error {
	return c.addAndSend(ctx, newMarketSubscription(channelTypeMidpoints, assetIDs, true, false))
}

// UnsubscribeMidpoints unsubscribes from midpoint events for the given asset IDs.
func (c *Client) UnsubscribeMidpoints(ctx context.Context, assetIDs []string) error {
	return c.removeAndSend(ctx, channelTypeMidpoints, assetIDs)
}

// SubscribeBestBidAsk subscribes to best bid/ask events for the given asset IDs.
func (c *Client) SubscribeBestBidAsk(ctx context.Context, assetIDs []string) error {
	return c.addAndSend(ctx, newMarketSubscription(channelTypeBestBidAsk, assetIDs, false, true))
}

// SubscribeNewMarkets subscribes to new market creation events.
func (c *Client) SubscribeNewMarkets(ctx context.Context, assetIDs []string) error {
	return c.addAndSend(ctx, newMarketSubscription(channelTypeNewMarkets, assetIDs, false, true))
}

// SubscribeMarketResolutions subscribes to market resolution events.
func (c *Client) SubscribeMarketResolutions(ctx context.Context, assetIDs []string) error {
	return c.addAndSend(ctx, newMarketSubscription(channelTypeMarketRes, assetIDs, false, true))
}

// SubscribeUserEvents subscribes to all user events (orders and trades) for the given markets.
// Requires credentials set via NewAuthenticatedClient or WithCredentials.
func (c *Client) SubscribeUserEvents(ctx context.Context, markets []string) error {
	return c.addAndSend(ctx, newUserSubscription(channelTypeUserEvents, markets))
}

// UnsubscribeUserEvents unsubscribes from user events for the given markets.
func (c *Client) UnsubscribeUserEvents(ctx context.Context, markets []string) error {
	return c.removeAndSend(ctx, channelTypeUserEvents, markets)
}

// SubscribeOrders subscribes to order status events for the given markets.
// Requires credentials set via NewAuthenticatedClient or WithCredentials.
func (c *Client) SubscribeOrders(ctx context.Context, markets []string) error {
	return c.addAndSend(ctx, newUserSubscription(channelTypeOrders, markets))
}

// UnsubscribeOrders unsubscribes from order events for the given markets.
func (c *Client) UnsubscribeOrders(ctx context.Context, markets []string) error {
	return c.removeAndSend(ctx, channelTypeOrders, markets)
}

// SubscribeTrades subscribes to trade fill events for the given markets.
// Requires credentials set via NewAuthenticatedClient or WithCredentials.
func (c *Client) SubscribeTrades(ctx context.Context, markets []string) error {
	return c.addAndSend(ctx, newUserSubscription(channelTypeTrades, markets))
}

// UnsubscribeTrades unsubscribes from trade events for the given markets.
func (c *Client) UnsubscribeTrades(ctx context.Context, markets []string) error {
	return c.removeAndSend(ctx, channelTypeTrades, markets)
}

// addAndSend records the subscription and sends the subscribe message.
func (c *Client) addAndSend(ctx context.Context, sub subscription) error {
	if err := validateSubscriptionInput(sub.target, sub.assetIDsOrMarkets()); err != nil {
		return err
	}

	sendSub, shouldSend := c.recordSubscription(sub)
	if !shouldSend {
		return nil
	}
	if err := c.replaySub(ctx, sendSub); err != nil {
		c.rollbackRecordedSubscription(sub)
		return err
	}
	return nil
}

// removeAndSend removes matching subscriptions and sends the unsubscribe message.
func (c *Client) removeAndSend(ctx context.Context, channelType string, ids []string) error {
	target := subscriptionTargetMarket
	if isUserChannel(channelType) {
		target = subscriptionTargetUser
	}
	if err := validateSubscriptionInput(target, ids); err != nil {
		return err
	}

	removedSub, toUnsubscribe, ok := c.removeRecordedSubscription(target, channelType, ids)
	if !ok || len(toUnsubscribe) == 0 {
		return nil
	}
	if err := c.sendUnsubscribeMessage(ctx, target, toUnsubscribe); err != nil {
		c.rollbackRemovedSubscription(removedSub)
		return err
	}
	return nil
}

// replaySub sends the subscribe wire message for a stored subscription.
// Called both on initial subscribe and on reconnect replay.
func (c *Client) replaySub(ctx context.Context, sub subscription) error {
	if sub.target.isUser() {
		return c.sendUserSubscribeMessage(ctx, sub)
	}
	return c.sendMarketSubscribeMessage(ctx, sub)
}

func isUserChannel(channelType string) bool {
	switch channelType {
	case channelTypeUserEvents, channelTypeOrders, channelTypeTrades:
		return true
	}
	return false
}

// sendMarketSubscribeMessage sends a market channel subscribe message.
func (c *Client) sendMarketSubscribeMessage(ctx context.Context, sub subscription) error {
	msg := MarketSubscription{
		Type:                 ChannelMarket,
		Operation:            "subscribe",
		AssetIDs:             sub.assetIDs,
		InitialDump:          sub.initialDump,
		CustomFeatureEnabled: sub.customFeatureEnabled,
	}
	return c.sendJSON(ctx, msg)
}

// sendUnsubscribeMessage sends an unsubscribe message for a channel.
func (c *Client) sendUnsubscribeMessage(
	ctx context.Context,
	target subscriptionTarget,
	ids []string,
) error {
	if target.isUser() {
		auth, err := c.deriveWSAuth(ctx)
		if err != nil {
			return err
		}
		msg := UserSubscription{
			Type:      ChannelUser,
			Auth:      auth,
			Markets:   ids,
			Operation: "unsubscribe",
		}
		return c.sendJSON(ctx, msg)
	}
	msg := MarketSubscription{
		Type:      ChannelMarket,
		Operation: "unsubscribe",
		AssetIDs:  ids,
	}
	return c.sendJSON(ctx, msg)
}

// sendUserSubscribeMessage derives WSAuth from stored credentials and sends a user subscribe message.
func (c *Client) sendUserSubscribeMessage(ctx context.Context, sub subscription) error {
	auth, err := c.deriveWSAuth(ctx)
	if err != nil {
		return err
	}
	msg := UserSubscription{
		Type:        ChannelUser,
		Auth:        auth,
		Markets:     sub.markets,
		Operation:   "subscribe",
		InitialDump: true,
	}
	return c.sendJSON(ctx, msg)
}

// deriveWSAuth derives a WSAuth from the stored credentials.
func (c *Client) deriveWSAuth(_ context.Context) (clob.WSAuth, error) {
	c.mu.Lock()
	creds := c.creds
	c.mu.Unlock()

	if creds == nil {
		return clob.WSAuth{}, errors.New(
			"user channel subscriptions require credentials: use NewAuthenticatedClient or WithCredentials",
		)
	}

	return clob.WSAuth{
		Key:        creds.Key,
		Secret:     creds.Secret,
		Passphrase: creds.Passphrase,
	}, nil
}

func newMarketSubscription(
	channelType string,
	assetIDs []string,
	initialDump bool,
	customFeatureEnabled bool,
) subscription {
	assetIDs = slices.Clone(assetIDs)
	return subscription{
		target:               subscriptionTargetMarket,
		channelType:          channelType,
		key:                  subscriptionKey(subscriptionTargetMarket, assetIDs),
		assetIDs:             assetIDs,
		initialDump:          initialDump,
		customFeatureEnabled: customFeatureEnabled,
	}
}

func newUserSubscription(channelType string, markets []string) subscription {
	markets = slices.Clone(markets)
	return subscription{
		target:      subscriptionTargetUser,
		channelType: channelType,
		key:         subscriptionKey(subscriptionTargetUser, markets),
		markets:     markets,
	}
}

func (s subscription) assetIDsOrMarkets() []string {
	if s.target.isUser() {
		return s.markets
	}
	return s.assetIDs
}

func validateSubscriptionInput(target subscriptionTarget, ids []string) error {
	// Rust's market subscription manager rejects empty asset lists. Empty user
	// market lists are meaningful: they subscribe to all user markets.
	if !target.isUser() && len(ids) == 0 {
		return errors.New("market websocket subscription requires at least one asset ID")
	}
	return nil
}

func subscriptionKey(target subscriptionTarget, ids []string) string {
	normalized := slices.Clone(ids)
	slices.Sort(normalized)
	prefix := "market"
	if target.isUser() {
		prefix = "user"
	}
	return prefix + ":" + strings.Join(normalized, ",")
}

func (c *Client) replayTrackedSubscriptions(ctx context.Context) {
	c.subsMu.RLock()
	if len(c.subs) == 0 {
		c.subsMu.RUnlock()
		return
	}

	marketActive := false
	userActive := false
	initialDump := false
	customFeature := c.customFeature
	marketAssets := make([]string, 0, len(c.marketRefs))
	for assetID := range c.marketRefs {
		marketAssets = append(marketAssets, assetID)
	}
	userMarkets := make([]string, 0, len(c.userRefs))
	for market := range c.userRefs {
		userMarkets = append(userMarkets, market)
	}
	for _, sub := range c.subs {
		if sub.target.isUser() {
			userActive = true
			continue
		}
		marketActive = true
		initialDump = initialDump || sub.initialDump
	}
	c.subsMu.RUnlock()

	slices.Sort(marketAssets)
	slices.Sort(userMarkets)

	if marketActive {
		if err := c.replaySub(ctx, subscription{
			target:               subscriptionTargetMarket,
			assetIDs:             marketAssets,
			initialDump:          initialDump,
			customFeatureEnabled: customFeature,
		}); err != nil {
			select {
			case c.errs <- fmt.Errorf("resubscribe market: %w", err):
			default:
			}
		}
	}
	if userActive {
		if err := c.replaySub(ctx, subscription{
			target:  subscriptionTargetUser,
			markets: userMarkets,
		}); err != nil {
			select {
			case c.errs <- fmt.Errorf("resubscribe user: %w", err):
			default:
			}
		}
	}
}

func (c *Client) recordSubscription(sub subscription) (subscription, bool) {
	c.subsMu.Lock()
	defer c.subsMu.Unlock()

	c.subs = append(c.subs, sub)
	if sub.channelType == channelTypeMidpoints {
		for _, assetID := range sub.assetIDs {
			c.midpointRefs[assetID]++
		}
	}
	sendSub := sub
	if sub.target.isUser() {
		if len(sub.markets) == 0 {
			return sendSub, true
		}
		// Use a fresh slice — sendSub and the stored sub share a backing array;
		// appending into a resliced copy would corrupt the stored subscription.
		sendSub.markets = make([]string, 0, len(sub.markets))
		for _, market := range sub.markets {
			if c.userRefs[market] == 0 {
				sendSub.markets = append(sendSub.markets, market)
			}
			c.userRefs[market]++
		}
		return sendSub, len(sendSub.markets) > 0
	}

	if len(sub.assetIDs) == 0 {
		c.customFeature = c.customFeature || sub.customFeatureEnabled
		return sendSub, true
	}

	// Use a fresh slice — same reason as markets above.
	sendSub.assetIDs = make([]string, 0, len(sub.assetIDs))
	for _, assetID := range sub.assetIDs {
		if c.marketRefs[assetID] == 0 {
			sendSub.assetIDs = append(sendSub.assetIDs, assetID)
		}
		c.marketRefs[assetID]++
	}
	if sub.customFeatureEnabled {
		c.customFeature = true
		if len(sendSub.assetIDs) == 0 {
			sendSub.assetIDs = slices.Clone(sub.assetIDs)
		}
		return sendSub, true
	}
	return sendSub, len(sendSub.assetIDs) > 0
}

func (c *Client) rollbackRecordedSubscription(sub subscription) {
	c.subsMu.Lock()
	defer c.subsMu.Unlock()

	c.removeOneSubscriptionLocked(sub)
	c.decrementSubscriptionRefsLocked(sub)
	c.recomputeCustomFeatureLocked()
}

func (c *Client) removeRecordedSubscription(
	target subscriptionTarget,
	channelType string,
	ids []string,
) (subscription, []string, bool) {
	c.subsMu.Lock()
	defer c.subsMu.Unlock()

	key := subscriptionKey(target, ids)
	fallbackIndex := -1
	for idx, sub := range c.subs {
		if sub.target != target || sub.key != key {
			continue
		}
		if sub.channelType != channelType {
			if fallbackIndex < 0 {
				fallbackIndex = idx
			}
			continue
		}
		fallbackIndex = idx
		break
	}
	if fallbackIndex < 0 {
		return subscription{}, nil, false
	}

	sub := c.subs[fallbackIndex]
	c.subs = append(c.subs[:fallbackIndex], c.subs[fallbackIndex+1:]...)
	toUnsubscribe := c.decrementSubscriptionRefsLocked(sub)
	c.recomputeCustomFeatureLocked()
	return sub, toUnsubscribe, true
}

func (c *Client) rollbackRemovedSubscription(sub subscription) {
	c.subsMu.Lock()
	defer c.subsMu.Unlock()

	c.subs = append(c.subs, sub)
	if sub.target.isUser() {
		for _, market := range sub.markets {
			c.userRefs[market]++
		}
	} else {
		for _, assetID := range sub.assetIDs {
			c.marketRefs[assetID]++
			if sub.channelType == channelTypeMidpoints {
				c.midpointRefs[assetID]++
			}
		}
	}
	c.recomputeCustomFeatureLocked()
}

func (c *Client) removeOneSubscriptionLocked(sub subscription) {
	for idx, existing := range c.subs {
		if existing.target != sub.target || existing.channelType != sub.channelType ||
			existing.key != sub.key {
			continue
		}
		c.subs = append(c.subs[:idx], c.subs[idx+1:]...)
		return
	}
}

func (c *Client) decrementSubscriptionRefsLocked(sub subscription) []string {
	toUnsubscribe := make([]string, 0, len(sub.assetIDs)+len(sub.markets))
	if sub.target.isUser() {
		for _, market := range sub.markets {
			switch count := c.userRefs[market]; {
			case count <= 1:
				delete(c.userRefs, market)
				toUnsubscribe = append(toUnsubscribe, market)
			default:
				c.userRefs[market] = count - 1
			}
		}
		return toUnsubscribe
	}

	for _, assetID := range sub.assetIDs {
		if sub.channelType == channelTypeMidpoints {
			if count := c.midpointRefs[assetID]; count <= 1 {
				delete(c.midpointRefs, assetID)
			} else {
				c.midpointRefs[assetID] = count - 1
			}
		}
		switch count := c.marketRefs[assetID]; {
		case count <= 1:
			delete(c.marketRefs, assetID)
			toUnsubscribe = append(toUnsubscribe, assetID)
		default:
			c.marketRefs[assetID] = count - 1
		}
	}
	return toUnsubscribe
}

func (c *Client) recomputeCustomFeatureLocked() {
	c.customFeature = false
	for _, sub := range c.subs {
		if !sub.target.isUser() && sub.customFeatureEnabled {
			c.customFeature = true
			return
		}
	}
}
