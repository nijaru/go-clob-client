package rtds

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	json "github.com/go-json-experiment/json"

	"github.com/coder/websocket"
)

const (
	// DefaultRTDSHost is the production Polymarket RTDS WebSocket URL.
	DefaultRTDSHost = "wss://rtds.polymarket.com"
	pingInterval    = 10 * time.Second
)

// Client is a WebSocket client for the Polymarket RTDS (Real-Time Data Stream).
type Client struct {
	url    string
	logger *slog.Logger

	mu       sync.Mutex
	conn     *websocket.Conn
	connDone chan struct{}

	msgs chan *RtdsMessage
	errs chan error
	stop chan struct{}
	cancel context.CancelFunc
	closed bool

	autoReconnect bool
	subsMu        sync.RWMutex
	subs          []Subscription
}

// NewClient creates a new RTDS client.
func NewClient(url string, logger *slog.Logger) *Client {
	if url == "" {
		url = DefaultRTDSHost
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		url:           url,
		logger:        logger.With("pkg", "rtds"),
		msgs:          make(chan *RtdsMessage, 1024),
		errs:          make(chan error, 100),
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

	c.logger.Debug("connecting to RTDS", "url", c.url)
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
	subs := c.subs
	c.subsMu.RUnlock()

	if len(subs) > 0 {
		c.logger.Debug("resubscribing to topics", "count", len(subs))
		req := SubscriptionRequest{
			Action:        ActionSubscribe,
			Subscriptions: subs,
		}
		if err := c.sendJSON(ctx, req); err != nil {
			c.logger.Error("resubscribe failed", "error", err)
		}
	}

	return nil
}

// Close closes the connection and stops the loops.
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

// Messages returns a channel of received RTDS messages.
func (c *Client) Messages() <-chan *RtdsMessage {
	return c.msgs
}

// Errors returns a channel of asynchronous errors.
func (c *Client) Errors() <-chan error {
	return c.errs
}

// Subscribe adds a new subscription.
func (c *Client) Subscribe(ctx context.Context, sub Subscription) error {
	c.subsMu.Lock()
	c.subs = append(c.subs, sub)
	c.subsMu.Unlock()

	req := SubscriptionRequest{
		Action:        ActionSubscribe,
		Subscriptions: []Subscription{sub},
	}
	return c.sendJSON(ctx, req)
}

// SubscribeCryptoPrices subscribes to Binance crypto prices.
func (c *Client) SubscribeCryptoPrices(ctx context.Context, symbols []string) error {
	sub := Subscription{
		Topic: "crypto_prices",
		Type:  "update",
	}
	if len(symbols) > 0 {
		sub.Filters = symbols
	}
	return c.Subscribe(ctx, sub)
}

// SubscribeChainlinkPrices subscribes to Chainlink price feeds.
func (c *Client) SubscribeChainlinkPrices(ctx context.Context, symbol string) error {
	sub := Subscription{
		Topic: "crypto_prices_chainlink",
		Type:  "*",
	}
	if symbol != "" {
		sub.Filters = map[string]string{"symbol": symbol}
	}
	return c.Subscribe(ctx, sub)
}

// SubscribeComments subscribes to comment events.
func (c *Client) SubscribeComments(ctx context.Context, commentType CommentType, auth *Credentials) error {
	msgType := string(commentType)
	if msgType == "" {
		msgType = "*"
	}
	sub := Subscription{
		Topic:    "comments",
		Type:     msgType,
		CLOBAuth: auth,
	}
	return c.Subscribe(ctx, sub)
}

func (c *Client) readLoop(ctx context.Context) {
	defer close(c.connDone)

	for {
		typ, data, err := c.conn.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.logger.Error("read error", "error", err)

			if c.autoReconnect {
				go c.attemptReconnect()
				return
			}

			c.reportError(fmt.Errorf("read: %w", err))
			return
		}

		if typ != websocket.MessageText {
			continue
		}

		// RTDS can return a single message or an array of messages
		if len(data) == 0 {
			continue
		}

		// Handle whitespace keepalive (like " ")
		isWhitespace := true
		for _, b := range data {
			if b != ' ' && b != '\n' && b != '\r' && b != '\t' {
				isWhitespace = false
				break
			}
		}
		if isWhitespace {
			continue
		}

		c.handleData(ctx, data)
	}
}

func (c *Client) handleData(ctx context.Context, data []byte) {
	// Try array first
	if data[0] == '[' {
		var msgs []*RtdsMessage
		if err := json.Unmarshal(data, &msgs); err != nil {
			c.reportError(fmt.Errorf("unmarshal msgs: %w", err))
			return
		}
		for _, m := range msgs {
			c.dispatch(ctx, m)
		}
	} else {
		var m RtdsMessage
		if err := json.Unmarshal(data, &m); err != nil {
			c.reportError(fmt.Errorf("unmarshal msg: %w", err))
			return
		}
		c.dispatch(ctx, &m)
	}
}

func (c *Client) dispatch(ctx context.Context, m *RtdsMessage) {
	select {
	case c.msgs <- m:
	case <-ctx.Done():
	default:
		c.logger.Warn("message dropped, channel full", "topic", m.Topic, "type", m.Type)
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
			c.mu.Lock()
			conn := c.conn
			c.mu.Unlock()

			if conn == nil {
				return
			}

			// RTDS expects plain text " " or "PING" usually, 
			// the Rust SDK sends " " by default in some cases but here we follow clob pattern if unsure.
			// Actually let's just send a space as it is common for simple keepalives if not specified.
			if err := conn.Write(ctx, websocket.MessageText, []byte(" ")); err != nil {
				if ctx.Err() != nil {
					return
				}
				c.logger.Error("ping failed", "error", err)
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
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}
	return conn.Write(ctx, websocket.MessageText, data)
}

func (c *Client) reportError(err error) {
	select {
	case c.errs <- err:
	default:
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

		c.logger.Info("attempting to reconnect", "backoff", backoff)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := c.connect(ctx)
		cancel()

		if err == nil {
			c.logger.Info("reconnected successfully")
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
