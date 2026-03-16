package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

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

	events chan any
	errs   chan error
	stop   chan struct{}
	cancel context.CancelFunc
	closed bool
}

// NewClient creates a new WebSocket client.
func NewClient(url string) *Client {
	if url == "" {
		url = defaultMarketURL
	}
	return &Client{
		url:    url,
		events: make(chan any, 100),
		errs:   make(chan error, 10),
		stop:   make(chan struct{}),
	}
}

// Connect opens the WebSocket connection and starts the read/heartbeat loops.
func (c *Client) Connect(ctx context.Context) error {
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
func (c *Client) SubscribeMarket(ctx context.Context, assetIDs []string) error {
	sub := MarketSubscription{
		Type:     ChannelMarket,
		AssetIDs: assetIDs,
	}
	return c.sendJSON(ctx, sub)
}

// SubscribeUser sends a user subscription message.
func (c *Client) SubscribeUser(ctx context.Context, auth clob.WSAuth) error {
	sub := UserSubscription{
		Type: ChannelUser,
		Auth: auth,
	}
	return c.sendJSON(ctx, sub)
}

// Events returns a channel of decoded events.
func (c *Client) Events() <-chan any {
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
		c.handleMessage(data)
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
			err := c.conn.Write(ctx, websocket.MessageText, []byte("PING"))
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

func (c *Client) handleMessage(data []byte) {
	var base BaseEvent
	if err := json.Unmarshal(data, &base); err != nil {
		// Non-JSON message (text heartbeat, etc.) — not an error.
		return
	}

	var event any
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

	c.events <- event
}
