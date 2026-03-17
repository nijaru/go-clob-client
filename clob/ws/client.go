package ws

import (
	"context"
	"fmt"
	"sync"
	"time"

	json "github.com/go-json-experiment/json"

	"github.com/coder/websocket"
	"github.com/nijaru/go-clob-client/clob"
)

const (
	defaultMarketURL = "wss://ws-subscriptions-clob.polymarket.com/ws/market"
	pingInterval     = 10 * time.Second
)

// Client is a WebSocket client for the Polymarket CLOB.
type Client struct {
	url string

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
	marketSubs    []marketSub
	userSub       *clob.WSAuth
}

type marketSub struct {
	assetIDs             []string
	markets              []string
	initialDump          bool
	customFeatureEnabled bool
}

// NewClient creates a new WebSocket client.
func NewClient(url string) *Client {
	if url == "" {
		url = defaultMarketURL
	}
	return &Client{
		url:           url,
		events:        make(chan Event, 100),
		errs:          make(chan error, 10),
		stop:          make(chan struct{}),
		autoReconnect: true,
	}
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

	// Resubscribe if reconnecting
	c.subsMu.RLock()
	marketSubs := c.marketSubs
	userSub := c.userSub
	c.subsMu.RUnlock()

	for _, sub := range marketSubs {
		if err := c.SubscribeMarket(ctx, sub.assetIDs, sub.markets, sub.initialDump, sub.customFeatureEnabled); err != nil {
			// Log error but continue?
		}
	}
	if userSub != nil {
		if err := c.SubscribeUser(ctx, *userSub); err != nil {
			// Log error
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

// SubscribeMarket sends a market subscription message.
func (c *Client) SubscribeMarket(
	ctx context.Context,
	assetIDs, markets []string,
	initialDump, customFeatureEnabled bool,
) error {
	sub := MarketSubscription{
		Type:                 ChannelMarket,
		AssetIDs:             assetIDs,
		Markets:              markets,
		InitialDump:          initialDump,
		CustomFeatureEnabled: customFeatureEnabled,
	}

	c.subsMu.Lock()
	c.marketSubs = append(c.marketSubs, marketSub{
		assetIDs:             assetIDs,
		markets:              markets,
		initialDump:          initialDump,
		customFeatureEnabled: customFeatureEnabled,
	})
	c.subsMu.Unlock()

	return c.sendJSON(ctx, sub)
}

// SubscribeUser sends a user subscription message.
func (c *Client) SubscribeUser(ctx context.Context, auth clob.WSAuth) error {
	sub := UserSubscription{
		Type: ChannelUser,
		Auth: auth,
	}

	c.subsMu.Lock()
	c.userSub = &auth
	c.subsMu.Unlock()

	return c.sendJSON(ctx, sub)
}

// Events returns a channel of decoded events.
func (c *Client) Events() <-chan Event {
	return c.events
}

// Errors returns a channel of asynchronous errors (e.g. from the read loop).
func (c *Client) Errors() <-chan error {
	return c.errs
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
