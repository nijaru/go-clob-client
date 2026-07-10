package ws

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	json "github.com/go-json-experiment/json"

	"github.com/coder/websocket"
	"github.com/nijaru/go-clob-client/clob"
	"github.com/nijaru/go-clob-client/internal/polyauth"
)

const (
	defaultMarketURL         = "wss://ws-subscriptions-clob.polymarket.com/ws/market"
	defaultUserURL           = "wss://ws-subscriptions-clob.polymarket.com/ws/user"
	defaultHeartbeatInterval = 5 * time.Second
	defaultHeartbeatTimeout  = 15 * time.Second

	channelTypeOrderBook      = "order_book"
	channelTypeLastTradePrice = "last_trade_price"
	channelTypePrices         = "prices"
	channelTypeTickSizeChange = "tick_size_change"
	channelTypeMidpoints      = "midpoints"
	channelTypeBestBidAsk     = "best_bid_ask"
	channelTypeNewMarkets     = "new_markets"
	channelTypeMarketRes      = "market_resolutions"
	channelTypeUserEvents     = "user_events"
	channelTypeOrders         = "orders"
	channelTypeTrades         = "trades"
)

// Client is a WebSocket client for the Polymarket CLOB.
type Client struct {
	url           string
	creds         *clob.Credentials
	decodedSecret []byte

	mu       sync.Mutex
	conn     *websocket.Conn
	connDone chan struct{} // closed when conn.Close() completes

	events       chan Event
	errs         chan error
	stop         chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc
	connCancel   context.CancelFunc
	closed       bool
	reconnecting bool

	autoReconnect bool
	subsMu        sync.RWMutex
	subs          []subscription
	marketRefs    map[string]int
	userRefs      map[string]int
	customFeature bool

	heartbeatInterval time.Duration
	heartbeatTimeout  time.Duration
}

type subscriptionTarget uint8

const (
	subscriptionTargetMarket subscriptionTarget = iota
	subscriptionTargetUser
)

func (t subscriptionTarget) isUser() bool {
	return t == subscriptionTargetUser
}

// subscription tracks a single logical subscription for refcounting and reconnect replay.
type subscription struct {
	target subscriptionTarget
	key    string
	// assetIDs is set for asset-scoped market channel subscriptions.
	assetIDs []string
	// markets is set for market-scoped user channel subscriptions.
	markets []string
	// initialDump requests a full book snapshot on subscribe (order_book only).
	initialDump          bool
	customFeatureEnabled bool
}

// NewClient creates a new unauthenticated WebSocket client.
func NewClient(url string) *Client {
	if url == "" {
		url = defaultMarketURL
	}
	return newClient(url, nil)
}

// NewAuthenticatedClient creates a WebSocket client with API credentials for user channel subscriptions.
func NewAuthenticatedClient(url string, creds clob.Credentials) *Client {
	if url == "" {
		url = defaultUserURL
	}
	return newClient(url, &creds)
}

func newClient(url string, creds *clob.Credentials) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		url:               url,
		creds:             creds,
		events:            make(chan Event, 100),
		errs:              make(chan error, 10),
		stop:              make(chan struct{}),
		ctx:               ctx,
		cancel:            cancel,
		autoReconnect:     true,
		marketRefs:        make(map[string]int),
		userRefs:          make(map[string]int),
		heartbeatInterval: defaultHeartbeatInterval,
		heartbeatTimeout:  defaultHeartbeatTimeout,
	}
}

// WithCredentials sets API credentials for user channel subscriptions.
// Returns the client to allow chaining.
func (c *Client) WithCredentials(creds clob.Credentials) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.creds = &creds
	return c
}

// Connect opens the WebSocket connection and starts the read/heartbeat loops.
func (c *Client) Connect(ctx context.Context) error {
	return c.connect(ctx)
}

func (c *Client) connect(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("client closed")
	}
	c.mu.Unlock()

	conn, _, err := websocket.Dial(ctx, c.url, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	loopCtx, cancel := context.WithCancel(c.ctx)

	c.mu.Lock()
	c.conn = conn
	c.connDone = make(chan struct{})
	oldCancel := c.connCancel
	c.connCancel = cancel
	c.mu.Unlock()

	if oldCancel != nil {
		oldCancel()
	}

	pongs := make(chan time.Time, 1)
	go c.readLoop(loopCtx, conn, c.connDone, pongs)
	go c.heartbeatLoop(loopCtx, conn, pongs)

	c.replayTrackedSubscriptions(ctx)

	return nil
}

// Close closes the connection and stops the loops.
// Safe to call multiple times.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	cancel := c.cancel
	connCancel := c.connCancel
	done := c.connDone
	conn := c.conn
	c.mu.Unlock()

	close(c.stop)
	if cancel != nil {
		cancel()
	}
	if connCancel != nil {
		connCancel()
	}
	if conn != nil {
		err := conn.Close(websocket.StatusNormalClosure, "")
		if done != nil {
			<-done
		}
		return err
	}
	return nil
}

func (c *Client) scheduleReconnect() {
	c.mu.Lock()
	if c.closed || !c.autoReconnect || c.reconnecting {
		c.mu.Unlock()
		return
	}
	c.reconnecting = true
	c.mu.Unlock()

	go c.attemptReconnect()
}

// IsConnected reports whether the client has an active WebSocket connection.
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil && !c.closed
}

// SubscriptionCount returns the number of active subscriptions.
func (c *Client) SubscriptionCount() int {
	c.subsMu.RLock()
	defer c.subsMu.RUnlock()
	return len(c.subs)
}

// Events returns a channel of decoded events.
func (c *Client) Events() <-chan Event {
	return c.events
}

// Errors returns a channel of asynchronous errors (e.g. from the read loop).
func (c *Client) Errors() <-chan error {
	return c.errs
}

// SubscribeOrderBook subscribes to order book snapshots and incremental updates for the given asset IDs.
// A full book snapshot (initial_dump) is requested on subscribe.
func (c *Client) SubscribeOrderBook(ctx context.Context, assetIDs []string) error {
	return c.addAndSend(ctx, newMarketSubscription(assetIDs, true, true))
}

// UnsubscribeOrderBook unsubscribes from order book updates for the given asset IDs.
func (c *Client) UnsubscribeOrderBook(ctx context.Context, assetIDs []string) error {
	return c.removeAndSend(ctx, channelTypeOrderBook, assetIDs)
}

// SubscribeLastTradePrice subscribes to last trade price events for the given asset IDs.
func (c *Client) SubscribeLastTradePrice(ctx context.Context, assetIDs []string) error {
	return c.addAndSend(ctx, newMarketSubscription(assetIDs, false, false))
}

// SubscribePrices subscribes to price change (incremental order book) events for the given asset IDs.
func (c *Client) SubscribePrices(ctx context.Context, assetIDs []string) error {
	return c.addAndSend(ctx, newMarketSubscription(assetIDs, false, false))
}

// UnsubscribePrices unsubscribes from price change events for the given asset IDs.
func (c *Client) UnsubscribePrices(ctx context.Context, assetIDs []string) error {
	return c.removeAndSend(ctx, channelTypePrices, assetIDs)
}

// SubscribeTickSizeChange subscribes to tick size change events for the given asset IDs.
func (c *Client) SubscribeTickSizeChange(ctx context.Context, assetIDs []string) error {
	return c.addAndSend(ctx, newMarketSubscription(assetIDs, false, false))
}

// UnsubscribeTickSizeChange unsubscribes from tick size change events for the given asset IDs.
func (c *Client) UnsubscribeTickSizeChange(ctx context.Context, assetIDs []string) error {
	return c.removeAndSend(ctx, channelTypeTickSizeChange, assetIDs)
}

// SubscribeMidpoints subscribes to midpoint events for the given asset IDs.
func (c *Client) SubscribeMidpoints(ctx context.Context, assetIDs []string) error {
	return c.addAndSend(ctx, newMarketSubscription(assetIDs, false, false))
}

// UnsubscribeMidpoints unsubscribes from midpoint events for the given asset IDs.
func (c *Client) UnsubscribeMidpoints(ctx context.Context, assetIDs []string) error {
	return c.removeAndSend(ctx, channelTypeMidpoints, assetIDs)
}

// SubscribeBestBidAsk subscribes to best bid/ask events for the given asset IDs.
func (c *Client) SubscribeBestBidAsk(ctx context.Context, assetIDs []string) error {
	return c.addAndSend(ctx, newMarketSubscription(assetIDs, false, true))
}

// SubscribeNewMarkets subscribes to new market creation events.
func (c *Client) SubscribeNewMarkets(ctx context.Context, assetIDs []string) error {
	return c.addAndSend(ctx, newMarketSubscription(assetIDs, false, true))
}

// SubscribeMarketResolutions subscribes to market resolution events.
func (c *Client) SubscribeMarketResolutions(ctx context.Context, assetIDs []string) error {
	return c.addAndSend(ctx, newMarketSubscription(assetIDs, false, true))
}

// SubscribeUserEvents subscribes to all user events (orders and trades) for the given markets.
// Requires credentials set via NewAuthenticatedClient or WithCredentials.
func (c *Client) SubscribeUserEvents(ctx context.Context, markets []string) error {
	return c.addAndSend(ctx, newUserSubscription(markets))
}

// UnsubscribeUserEvents unsubscribes from user events for the given markets.
func (c *Client) UnsubscribeUserEvents(ctx context.Context, markets []string) error {
	return c.removeAndSend(ctx, channelTypeUserEvents, markets)
}

// SubscribeOrders subscribes to order status events for the given markets.
// Requires credentials set via NewAuthenticatedClient or WithCredentials.
func (c *Client) SubscribeOrders(ctx context.Context, markets []string) error {
	return c.addAndSend(ctx, newUserSubscription(markets))
}

// UnsubscribeOrders unsubscribes from order events for the given markets.
func (c *Client) UnsubscribeOrders(ctx context.Context, markets []string) error {
	return c.removeAndSend(ctx, channelTypeOrders, markets)
}

// SubscribeTrades subscribes to trade fill events for the given markets.
// Requires credentials set via NewAuthenticatedClient or WithCredentials.
func (c *Client) SubscribeTrades(ctx context.Context, markets []string) error {
	return c.addAndSend(ctx, newUserSubscription(markets))
}

// UnsubscribeTrades unsubscribes from trade events for the given markets.
func (c *Client) UnsubscribeTrades(ctx context.Context, markets []string) error {
	return c.removeAndSend(ctx, channelTypeTrades, markets)
}

// addAndSend records the subscription and sends the subscribe message.
func (c *Client) addAndSend(ctx context.Context, sub subscription) error {
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

	removedSub, toUnsubscribe, ok := c.removeRecordedSubscription(target, ids)
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
		Type:    ChannelUser,
		Auth:    auth,
		Markets: sub.markets,
	}
	return c.sendJSON(ctx, msg)
}

// deriveWSAuth derives a WSAuth from the stored credentials.
func (c *Client) deriveWSAuth(ctx context.Context) (clob.WSAuth, error) {
	c.mu.Lock()
	creds := c.creds
	decodedSecret := c.decodedSecret
	c.mu.Unlock()

	if creds == nil {
		return clob.WSAuth{}, errors.New(
			"user channel subscriptions require credentials: use NewAuthenticatedClient or WithCredentials",
		)
	}

	timestamp := time.Now().Unix()
	sig := polyauth.HMACSignatureBytes(decodedSecret, timestamp, "GET", "/ws/user", nil)

	return clob.WSAuth{
		Key:        creds.Key,
		Passphrase: creds.Passphrase,
		Timestamp:  strconv.FormatInt(timestamp, 10),
		Signature:  sig,
	}, nil
}

func newMarketSubscription(
	assetIDs []string,
	initialDump bool,
	customFeatureEnabled bool,
) subscription {
	assetIDs = slices.Clone(assetIDs)
	return subscription{
		target:               subscriptionTargetMarket,
		key:                  subscriptionKey(subscriptionTargetMarket, assetIDs),
		assetIDs:             assetIDs,
		initialDump:          initialDump,
		customFeatureEnabled: customFeatureEnabled,
	}
}

func newUserSubscription(markets []string) subscription {
	markets = slices.Clone(markets)
	return subscription{
		target:  subscriptionTargetUser,
		key:     subscriptionKey(subscriptionTargetUser, markets),
		markets: markets,
	}
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
	ids []string,
) (subscription, []string, bool) {
	c.subsMu.Lock()
	defer c.subsMu.Unlock()

	key := subscriptionKey(target, ids)
	for idx, sub := range c.subs {
		if sub.target != target || sub.key != key {
			continue
		}
		c.subs = append(c.subs[:idx], c.subs[idx+1:]...)
		toUnsubscribe := c.decrementSubscriptionRefsLocked(sub)
		c.recomputeCustomFeatureLocked()
		return sub, toUnsubscribe, true
	}
	return subscription{}, nil, false
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
		}
	}
	c.recomputeCustomFeatureLocked()
}

func (c *Client) removeOneSubscriptionLocked(sub subscription) {
	for idx, existing := range c.subs {
		if existing.target != sub.target || existing.key != sub.key {
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

func (c *Client) readLoop(
	ctx context.Context,
	conn *websocket.Conn,
	done chan struct{},
	pongs chan<- time.Time,
) {
	defer close(done)

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			// Context canceled means Close() was called — exit silently.
			if ctx.Err() != nil {
				return
			}

			// If auto-reconnect enabled, try to reconnect
			if c.autoReconnect {
				c.scheduleReconnect()
				return
			}

			select {
			case c.errs <- fmt.Errorf("read: %w", err):
			default:
			}
			return
		}

		// Handle PONG
		if string(data) == "PONG" {
			select {
			case pongs <- time.Now():
			default:
			}
			continue
		}

		// Decode event
		c.handleMessage(ctx, data)
	}
}

func (c *Client) heartbeatLoop(
	ctx context.Context,
	conn *websocket.Conn,
	pongs <-chan time.Time,
) {
	ticker := time.NewTicker(c.heartbeatInterval)
	defer ticker.Stop()

	var timeout *time.Timer
	defer func() {
		if timeout != nil {
			timeout.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for {
				select {
				case <-pongs:
				default:
					goto drained
				}
			}
		drained:
			// Polymarket expects plain text "PING"
			if err := conn.Write(ctx, websocket.MessageText, []byte("PING")); err != nil {
				if ctx.Err() != nil {
					return
				}
				c.scheduleReconnect()
				return
			}

			if timeout == nil {
				timeout = time.NewTimer(c.heartbeatTimeout)
			} else {
				timeout.Reset(c.heartbeatTimeout)
			}

			select {
			case <-ctx.Done():
				return
			case <-pongs:
				if !timeout.Stop() {
					select {
					case <-timeout.C:
					default:
					}
				}
			case <-timeout.C:
				_ = conn.Close(websocket.StatusPolicyViolation, "heartbeat timeout")
				c.scheduleReconnect()
				return
			}
		}
	}
}

func (c *Client) sendJSON(ctx context.Context, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	return c.conn.Write(ctx, websocket.MessageText, data)
}

var eventTypeKey = []byte(`"event_type"`)

// extractEventType scans JSON bytes for the "event_type" key using a direct
// byte search so the hot path avoids decoder allocations.
func extractEventType(data []byte) (EventType, bool) {
	idx := bytes.Index(data, eventTypeKey)
	if idx < 0 {
		return "", false
	}

	i := idx + len(eventTypeKey)
	for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
		i++
	}
	if i >= len(data) || data[i] != ':' {
		return "", false
	}
	i++
	for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
		i++
	}
	if i >= len(data) || data[i] != '"' {
		return "", false
	}
	i++
	start := i
	for i < len(data) {
		switch data[i] {
		case '\\':
			// Event types are fixed identifiers, not escaped strings.
			return "", false
		case '"':
			return EventType(data[start:i]), true
		default:
			i++
		}
	}
	return "", false
}

func (c *Client) handleMessage(ctx context.Context, data []byte) {
	eventType, ok := extractEventType(data)
	if !ok {
		// Non-JSON message (text heartbeat, etc.) — not an error.
		return
	}

	var event Event
	switch eventType {
	case EventTypeBook:
		// The market endpoint sends the initial dump as a JSON array of
		// per-asset book snapshots. Keep compatibility with older/synthetic
		// single-object payloads, but flatten the live wire batch into the
		// same Event stream exposed by the official clients.
		var books []BookEvent
		if err := json.Unmarshal(data, &books); err == nil {
			for i := range books {
				books[i].BaseEvent = BaseEvent{EventType: EventTypeBook}
				c.emitEvent(ctx, &books[i])
			}
			return
		}
		event = &BookEvent{}
	case EventTypePriceChange:
		event = &PriceChangeEvent{}
	case EventTypeTickSizeChange:
		event = &TickSizeChangeEvent{}
	case EventTypeLastTradePrice:
		event = &LastTradePriceEvent{}
	case EventTypeOrder:
		event = &OrderEvent{}
	case EventTypeTrade:
		event = &TradeEvent{}
	case EventTypeBestBidAsk:
		event = &BestBidAskEvent{}
	case EventTypeNewMarket:
		event = &NewMarketEvent{}
	case EventTypeMarketResolved:
		event = &MarketResolvedEvent{}
	default:
		// Unknown event type — report so callers can notice new API events.
		select {
		case c.errs <- fmt.Errorf("unknown event type: %s", eventType):
		default:
		}
		return
	}

	if err := json.Unmarshal(data, event); err != nil {
		c.reportDecodeError(eventType, err)
		return
	}
	c.emitEvent(ctx, event)
}

func (c *Client) reportDecodeError(eventType EventType, err error) {
	select {
	case c.errs <- fmt.Errorf("decode event %s: %w", eventType, err):
	default:
	}
}

func (c *Client) emitEvent(ctx context.Context, event Event) {
	select {
	case c.events <- event:
	case <-ctx.Done():
	}
}

func (c *Client) attemptReconnect() {
	defer func() {
		c.mu.Lock()
		c.reconnecting = false
		c.mu.Unlock()
	}()

	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second
	timer := time.NewTimer(backoff)
	defer timer.Stop()

	for {
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()

		ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
		err := c.connect(ctx)
		cancel()

		if err == nil {
			return
		}

		select {
		case <-c.ctx.Done():
			return
		case <-timer.C:
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}

			// Add ±25% jitter to prevent thundering herd
			jitter := time.Duration(rand.Float64()*0.5-0.25) * backoff
			nextBackoff := backoff + jitter

			timer.Reset(nextBackoff)
		}
	}
}
