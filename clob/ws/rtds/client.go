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
	DefaultRTDSHost          = "wss://ws-live-data.polymarket.com"
	defaultHeartbeatInterval = 5 * time.Second
	defaultHeartbeatTimeout  = 15 * time.Second
)

// Client is a WebSocket client for the Polymarket RTDS (Real-Time Data Stream).
type Client struct {
	url    string
	logger *slog.Logger

	mu       sync.Mutex
	conn     *websocket.Conn
	connDone chan struct{}

	msgs         chan *RtdsMessage
	errs         chan error
	stop         chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc
	connCancel   context.CancelFunc
	closed       bool
	reconnecting bool

	autoReconnect bool
	subsMu        sync.RWMutex
	subs          []Subscription
	creds         *Credentials

	heartbeatInterval time.Duration
	heartbeatTimeout  time.Duration
}

// NewClient creates a new RTDS client.
func NewClient(url string, logger *slog.Logger) *Client {
	if url == "" {
		url = DefaultRTDSHost
	}
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		url:               url,
		logger:            logger.With("pkg", "rtds"),
		msgs:              make(chan *RtdsMessage, 1024),
		errs:              make(chan error, 100),
		stop:              make(chan struct{}),
		ctx:               ctx,
		cancel:            cancel,
		autoReconnect:     true,
		heartbeatInterval: defaultHeartbeatInterval,
		heartbeatTimeout:  defaultHeartbeatTimeout,
	}
}

// WithCredentials sets the credentials for authenticated subscriptions.
func (c *Client) WithCredentials(creds *Credentials) *Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.creds = creds
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

	c.logger.Debug("connecting to RTDS", "url", c.url)
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

	// Resubscribe if reconnecting
	c.subsMu.RLock()
	subs := append([]Subscription(nil), c.subs...)
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

// Close closes the connection and stops the loops.
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

// SubscribeChainlinkTWAP subscribes broadly to a supported Chainlink TWAP
// window. Filter the typed messages by Symbol locally when needed; RTDS
// replaces the prior topic filter when a second filtered subscription is sent.
func (c *Client) SubscribeChainlinkTWAP(
	ctx context.Context,
	window ChainlinkTWAPWindowSeconds,
) error {
	topic, err := chainlinkTWAPTopic(window)
	if err != nil {
		return err
	}
	sub := Subscription{Topic: topic, Type: "update"}
	return c.Subscribe(ctx, sub)
}

// SubscribeChainlinkTWAP30Seconds subscribes to the 30-second TWAP feed.
func (c *Client) SubscribeChainlinkTWAP30Seconds(ctx context.Context) error {
	return c.SubscribeChainlinkTWAP(ctx, ChainlinkTWAP30Seconds)
}

// SubscribeChainlinkTWAP60Seconds subscribes to the 60-second TWAP feed.
func (c *Client) SubscribeChainlinkTWAP60Seconds(ctx context.Context) error {
	return c.SubscribeChainlinkTWAP(ctx, ChainlinkTWAP60Seconds)
}

// SubscribeComments subscribes to comment events.
func (c *Client) SubscribeComments(
	ctx context.Context,
	commentType CommentType,
	auth *Credentials,
) error {
	msgType := string(commentType)
	if msgType == "" {
		msgType = "*"
	}
	if auth == nil {
		c.mu.Lock()
		auth = c.creds
		c.mu.Unlock()
	}
	sub := Subscription{
		Topic:    "comments",
		Type:     msgType,
		CLOBAuth: auth,
	}
	return c.Subscribe(ctx, sub)
}

func (c *Client) readLoop(
	ctx context.Context,
	conn *websocket.Conn,
	done chan struct{},
	pongs chan<- time.Time,
) {
	defer close(done)

	for {
		typ, data, err := conn.Read(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			c.logger.Error("read error", "error", err)

			if c.autoReconnect {
				c.scheduleReconnect()
				return
			}

			c.reportError(fmt.Errorf("read: %w", err))
			return
		}

		if typ != websocket.MessageText {
			continue
		}

		if string(data) == "PONG" {
			select {
			case pongs <- time.Now():
			default:
			}
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
		select {
		case c.errs <- fmt.Errorf("rtds: message dropped (channel full): topic=%s type=%s", m.Topic, m.Type):
		default:
		}
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
			if err := conn.Write(ctx, websocket.MessageText, []byte("PING")); err != nil {
				if ctx.Err() != nil {
					return
				}
				c.logger.Error("ping failed", "error", err)
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

// UnsubscribeCryptoPrices unsubscribes from crypto price updates.
func (c *Client) UnsubscribeCryptoPrices(ctx context.Context) error {
	return c.unsubscribe(ctx, "crypto_prices", "update")
}

// UnsubscribeChainlinkPrices unsubscribes from Chainlink price feed updates.
func (c *Client) UnsubscribeChainlinkPrices(ctx context.Context) error {
	return c.unsubscribe(ctx, "crypto_prices_chainlink", "*")
}

// UnsubscribeChainlinkTWAP unsubscribes from one Chainlink TWAP window.
func (c *Client) UnsubscribeChainlinkTWAP(
	ctx context.Context,
	window ChainlinkTWAPWindowSeconds,
) error {
	topic, err := chainlinkTWAPTopic(window)
	if err != nil {
		return err
	}
	return c.unsubscribe(ctx, topic, "update")
}

// UnsubscribeChainlinkTWAP30Seconds unsubscribes from the 30-second TWAP feed.
func (c *Client) UnsubscribeChainlinkTWAP30Seconds(ctx context.Context) error {
	return c.UnsubscribeChainlinkTWAP(ctx, ChainlinkTWAP30Seconds)
}

// UnsubscribeChainlinkTWAP60Seconds unsubscribes from the 60-second TWAP feed.
func (c *Client) UnsubscribeChainlinkTWAP60Seconds(ctx context.Context) error {
	return c.UnsubscribeChainlinkTWAP(ctx, ChainlinkTWAP60Seconds)
}

// UnsubscribeComments unsubscribes from comment events.
func (c *Client) UnsubscribeComments(ctx context.Context, commentType CommentType) error {
	msgType := string(commentType)
	if msgType == "" {
		msgType = "*"
	}
	return c.unsubscribe(ctx, "comments", msgType)
}

// unsubscribe sends an unsubscribe request for the given topic and removes it from the tracked subs.
func (c *Client) unsubscribe(ctx context.Context, topic, msgType string) error {
	c.subsMu.Lock()
	var remaining []Subscription
	var removed []Subscription
	for _, s := range c.subs {
		if s.Topic == topic && (msgType == "" || s.Type == msgType) {
			removed = append(removed, s)
		} else {
			remaining = append(remaining, s)
		}
	}
	c.subs = remaining
	c.subsMu.Unlock()

	if len(removed) == 0 {
		return nil
	}

	req := SubscriptionRequest{
		Action:        ActionUnsubscribe,
		Subscriptions: removed,
	}
	return c.sendJSON(ctx, req)
}

// IsConnected reports whether the client has an active WebSocket connection.
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil && !c.closed
}

func chainlinkTWAPTopic(window ChainlinkTWAPWindowSeconds) (string, error) {
	switch window {
	case ChainlinkTWAP30Seconds:
		return "crypto_prices_twap_thirty", nil
	case ChainlinkTWAP60Seconds:
		return "crypto_prices_twap_sixty", nil
	default:
		return "", fmt.Errorf(
			"rtds: Chainlink TWAP window must be 30 or 60 seconds, got %d",
			window,
		)
	}
}

// SubscriptionCount returns the number of active subscriptions.
func (c *Client) SubscriptionCount() int {
	c.subsMu.RLock()
	defer c.subsMu.RUnlock()
	return len(c.subs)
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

		c.logger.Info("attempting to reconnect", "backoff", backoff)
		ctx, cancel := context.WithTimeout(c.ctx, 10*time.Second)
		err := c.connect(ctx)
		cancel()

		if err == nil {
			c.logger.Info("reconnected successfully")
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
			timer.Reset(backoff)
		}
	}
}
