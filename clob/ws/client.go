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
	defaultUserURL   = "wss://ws-subscriptions-clob.polymarket.com/ws/user"
	pingInterval     = 10 * time.Second
)

// Client is a WebSocket client for the Polymarket CLOB.
type Client struct {
	url string

	mu   sync.Mutex
	conn *websocket.Conn

	events chan interface{}
	errs   chan error
	stop   chan struct{}

	handler func(interface{})
}

// NewClient creates a new WebSocket client.
func NewClient(url string) *Client {
	if url == "" {
		url = defaultMarketURL
	}
	return &Client{
		url:    url,
		events: make(chan interface{}, 100),
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

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	go c.readLoop()
	go c.heartbeatLoop()

	return nil
}

// Close closes the connection and stops the loops.
func (c *Client) Close() error {
	close(c.stop)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn.Close(websocket.StatusNormalClosure, "")
	}
	return nil
}

// SubscribeMarket sends a market subscription message.
func (c *Client) SubscribeMarket(ctx context.Context, assetIDs []string, customFeature bool) error {
	sub := MarketSubscription{
		Type:                 ChannelMarket,
		AssetIDs:             assetIDs,
		CustomFeatureEnabled: customFeature,
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
func (c *Client) Events() <-chan interface{} {
	return c.events
}

// Errors returns a channel of asynchronous errors (e.g. from the read loop).
func (c *Client) Errors() <-chan error {
	return c.errs
}

func (c *Client) readLoop() {
	for {
		select {
		case <-c.stop:
			return
		default:
			_, data, err := c.conn.Read(context.Background())
			if err != nil {
				// Avoid reporting error on closure
				select {
				case <-c.stop:
					return
				default:
					c.errs <- fmt.Errorf("read: %w", err)
					return
				}
			}

			// Handle PONG
			if string(data) == "PONG" {
				continue
			}

			// Decode event
			c.handleMessage(data)
		}
	}
}

func (c *Client) heartbeatLoop() {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stop:
			return
		case <-ticker.C:
			// Polymarket expects plain text "PING"
			err := c.conn.Write(context.Background(), websocket.MessageText, []byte("PING"))
			if err != nil {
				c.errs <- fmt.Errorf("ping: %w", err)
				return
			}
		}
	}
}

func (c *Client) sendJSON(ctx context.Context, v interface{}) error {
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
		return // Silently ignore non-JSON or malformed (might be PONG if missed earlier)
	}

	var event interface{}
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
		// Unknown event
		return
	}

	if err := json.Unmarshal(data, event); err != nil {
		c.errs <- fmt.Errorf("decode event %s: %w", base.EventType, err)
		return
	}

	c.events <- event
}
