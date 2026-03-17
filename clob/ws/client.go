package ws

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	json "github.com/go-json-experiment/json"

	"github.com/coder/websocket"
	"github.com/nijaru/go-clob-client/clob"
	"github.com/nijaru/go-clob-client/internal/polyauth"
)

const (
	defaultMarketURL = "wss://ws-subscriptions-clob.polymarket.com/ws/market"
	pingInterval     = 10 * time.Second

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
	url   string
	creds *clob.Credentials

	mu       sync.Mutex
	conn     *websocket.Conn
	connDone chan struct{} // closed when conn.Close() completes

	events chan Event
	errs   chan error
	stop   chan struct{}
	cancel context.CancelFunc
	closed bool

	autoReconnect bool
	subsMu        sync.RWMutex
	subs          []subscription
}

// subscription tracks a single channel subscription for reconnect replay.
type subscription struct {
	// channelType is an internal identifier (e.g. "order_book", "user_events").
	channelType string
	// assetIDs is set for asset-scoped market channel subscriptions.
	assetIDs []string
	// markets is set for market-scoped user channel subscriptions.
	markets []string
	// initialDump requests a full book snapshot on subscribe (order_book only).
	initialDump bool
}

// NewClient creates a new unauthenticated WebSocket client.
func NewClient(url string) *Client {
	return newClient(url, nil)
}

// NewAuthenticatedClient creates a WebSocket client with API credentials for user channel subscriptions.
func NewAuthenticatedClient(url string, creds clob.Credentials) *Client {
	return newClient(url, &creds)
}

func newClient(url string, creds *clob.Credentials) *Client {
	if url == "" {
		url = defaultMarketURL
	}
	return &Client{
		url:           url,
		creds:         creds,
		events:        make(chan Event, 100),
		errs:          make(chan error, 10),
		stop:          make(chan struct{}),
		autoReconnect: true,
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

	loopCtx, cancel := context.WithCancel(context.Background())

	c.mu.Lock()
	c.conn = conn
	c.connDone = make(chan struct{})
	c.cancel = cancel
	c.mu.Unlock()

	go c.readLoop(loopCtx)
	go c.heartbeatLoop(loopCtx)

	// Resubscribe if reconnecting.
	c.subsMu.RLock()
	subs := make([]subscription, len(c.subs))
	copy(subs, c.subs)
	c.subsMu.RUnlock()

	for _, sub := range subs {
		if err := c.replaySub(ctx, sub); err != nil {
			select {
			case c.errs <- fmt.Errorf("resubscribe %s: %w", sub.channelType, err):
			default:
			}
		}
	}

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
	done := c.connDone
	conn := c.conn
	c.mu.Unlock()

	close(c.stop)
	if cancel != nil {
		cancel()
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
	sub := subscription{
		channelType: channelTypeOrderBook,
		assetIDs:    assetIDs,
		initialDump: true,
	}
	return c.addAndSend(ctx, sub)
}

// UnsubscribeOrderBook unsubscribes from order book updates for the given asset IDs.
func (c *Client) UnsubscribeOrderBook(ctx context.Context, assetIDs []string) error {
	return c.removeAndSend(ctx, channelTypeOrderBook, assetIDs)
}

// SubscribeLastTradePrice subscribes to last trade price events for the given asset IDs.
func (c *Client) SubscribeLastTradePrice(ctx context.Context, assetIDs []string) error {
	sub := subscription{channelType: channelTypeLastTradePrice, assetIDs: assetIDs}
	return c.addAndSend(ctx, sub)
}

// SubscribePrices subscribes to price change (incremental order book) events for the given asset IDs.
func (c *Client) SubscribePrices(ctx context.Context, assetIDs []string) error {
	sub := subscription{channelType: channelTypePrices, assetIDs: assetIDs}
	return c.addAndSend(ctx, sub)
}

// UnsubscribePrices unsubscribes from price change events for the given asset IDs.
func (c *Client) UnsubscribePrices(ctx context.Context, assetIDs []string) error {
	return c.removeAndSend(ctx, channelTypePrices, assetIDs)
}

// SubscribeTickSizeChange subscribes to tick size change events for the given asset IDs.
func (c *Client) SubscribeTickSizeChange(ctx context.Context, assetIDs []string) error {
	sub := subscription{channelType: channelTypeTickSizeChange, assetIDs: assetIDs}
	return c.addAndSend(ctx, sub)
}

// UnsubscribeTickSizeChange unsubscribes from tick size change events for the given asset IDs.
func (c *Client) UnsubscribeTickSizeChange(ctx context.Context, assetIDs []string) error {
	return c.removeAndSend(ctx, channelTypeTickSizeChange, assetIDs)
}

// SubscribeMidpoints subscribes to midpoint events for the given asset IDs.
func (c *Client) SubscribeMidpoints(ctx context.Context, assetIDs []string) error {
	sub := subscription{channelType: channelTypeMidpoints, assetIDs: assetIDs}
	return c.addAndSend(ctx, sub)
}

// UnsubscribeMidpoints unsubscribes from midpoint events for the given asset IDs.
func (c *Client) UnsubscribeMidpoints(ctx context.Context, assetIDs []string) error {
	return c.removeAndSend(ctx, channelTypeMidpoints, assetIDs)
}

// SubscribeBestBidAsk subscribes to best bid/ask events for the given asset IDs.
func (c *Client) SubscribeBestBidAsk(ctx context.Context, assetIDs []string) error {
	sub := subscription{channelType: channelTypeBestBidAsk, assetIDs: assetIDs}
	return c.addAndSend(ctx, sub)
}

// SubscribeNewMarkets subscribes to new market creation events.
func (c *Client) SubscribeNewMarkets(ctx context.Context) error {
	sub := subscription{channelType: channelTypeNewMarkets}
	return c.addAndSend(ctx, sub)
}

// SubscribeMarketResolutions subscribes to market resolution events.
func (c *Client) SubscribeMarketResolutions(ctx context.Context) error {
	sub := subscription{channelType: channelTypeMarketRes}
	return c.addAndSend(ctx, sub)
}

// SubscribeUserEvents subscribes to all user events (orders and trades) for the given markets.
// Requires credentials set via NewAuthenticatedClient or WithCredentials.
func (c *Client) SubscribeUserEvents(ctx context.Context, markets []string) error {
	sub := subscription{channelType: channelTypeUserEvents, markets: markets}
	return c.addAndSend(ctx, sub)
}

// UnsubscribeUserEvents unsubscribes from user events for the given markets.
func (c *Client) UnsubscribeUserEvents(ctx context.Context, markets []string) error {
	return c.removeAndSend(ctx, channelTypeUserEvents, markets)
}

// SubscribeOrders subscribes to order status events for the given markets.
// Requires credentials set via NewAuthenticatedClient or WithCredentials.
func (c *Client) SubscribeOrders(ctx context.Context, markets []string) error {
	sub := subscription{channelType: channelTypeOrders, markets: markets}
	return c.addAndSend(ctx, sub)
}

// UnsubscribeOrders unsubscribes from order events for the given markets.
func (c *Client) UnsubscribeOrders(ctx context.Context, markets []string) error {
	return c.removeAndSend(ctx, channelTypeOrders, markets)
}

// SubscribeTrades subscribes to trade fill events for the given markets.
// Requires credentials set via NewAuthenticatedClient or WithCredentials.
func (c *Client) SubscribeTrades(ctx context.Context, markets []string) error {
	sub := subscription{channelType: channelTypeTrades, markets: markets}
	return c.addAndSend(ctx, sub)
}

// UnsubscribeTrades unsubscribes from trade events for the given markets.
func (c *Client) UnsubscribeTrades(ctx context.Context, markets []string) error {
	return c.removeAndSend(ctx, channelTypeTrades, markets)
}

// addAndSend records the subscription and sends the subscribe message.
func (c *Client) addAndSend(ctx context.Context, sub subscription) error {
	c.subsMu.Lock()
	c.subs = append(c.subs, sub)
	c.subsMu.Unlock()

	return c.replaySub(ctx, sub)
}

// removeAndSend removes matching subscriptions and sends the unsubscribe message.
func (c *Client) removeAndSend(ctx context.Context, channelType string, ids []string) error {
	c.subsMu.Lock()
	var remaining []subscription
	for _, s := range c.subs {
		if s.channelType == channelType {
			continue
		}
		remaining = append(remaining, s)
	}
	c.subs = remaining
	c.subsMu.Unlock()

	return c.sendUnsubscribeMessage(ctx, channelType, ids)
}

// replaySub sends the subscribe wire message for a stored subscription.
// Called both on initial subscribe and on reconnect replay.
func (c *Client) replaySub(ctx context.Context, sub subscription) error {
	if isUserChannel(sub.channelType) {
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
		Type:        ChannelMarket,
		AssetIDs:    sub.assetIDs,
		InitialDump: sub.initialDump,
	}
	return c.sendJSON(ctx, msg)
}

// sendUnsubscribeMessage sends an unsubscribe message for a channel.
func (c *Client) sendUnsubscribeMessage(
	ctx context.Context,
	channelType string,
	ids []string,
) error {
	if isUserChannel(channelType) {
		auth, err := c.deriveWSAuth(ctx)
		if err != nil {
			return err
		}
		msg := UserSubscription{
			Type:      ChannelUser,
			Auth:      auth,
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
	c.mu.Unlock()

	if creds == nil {
		return clob.WSAuth{}, errors.New(
			"user channel subscriptions require credentials: use NewAuthenticatedClient or WithCredentials",
		)
	}

	timestamp := time.Now().Unix()
	sig, err := polyauth.HMACSignature(creds.Secret, timestamp, "GET", "/ws/user", nil)
	if err != nil {
		return clob.WSAuth{}, fmt.Errorf("derive ws auth: %w", err)
	}

	return clob.WSAuth{
		Key:        creds.Key,
		Passphrase: creds.Passphrase,
		Timestamp:  strconv.FormatInt(timestamp, 10),
		Signature:  sig,
	}, nil
}

func (c *Client) readLoop(ctx context.Context) {
	defer close(c.connDone)

	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			// Context canceled means Close() was called — exit silently.
			if ctx.Err() != nil {
				return
			}

			// If auto-reconnect enabled, try to reconnect
			if c.autoReconnect {
				go c.attemptReconnect()
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
			continue
		}

		// Decode event
		c.handleMessage(ctx, data)
	}
}

func (c *Client) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Polymarket expects plain text "PING"
			c.mu.Lock()
			if c.conn == nil {
				c.mu.Unlock()
				return
			}
			err := c.conn.Write(ctx, websocket.MessageText, []byte("PING"))
			c.mu.Unlock()

			if err != nil {
				if ctx.Err() != nil {
					return
				}
				select {
				case c.errs <- fmt.Errorf("ping: %w", err):
				default:
				}
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

func (c *Client) handleMessage(ctx context.Context, data []byte) {
	var base BaseEvent
	if err := json.Unmarshal(data, &base); err != nil {
		// Non-JSON message (text heartbeat, etc.) — not an error.
		return
	}

	var event Event
	switch base.EventType {
	case EventTypeBook:
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
		case c.errs <- fmt.Errorf("unknown event type: %s", base.EventType):
		default:
		}
		return
	}

	if err := json.Unmarshal(data, event); err != nil {
		select {
		case c.errs <- fmt.Errorf("decode event %s: %w", base.EventType, err):
		default:
		}
		return
	}

	select {
	case c.events <- event:
	case <-ctx.Done():
	}
}

func (c *Client) attemptReconnect() {
	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second

	for {
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := c.connect(ctx)
		cancel()

		if err == nil {
			return
		}

		select {
		case <-c.stop:
			return
		case <-time.After(backoff):
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}
