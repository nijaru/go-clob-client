package rtds

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"slices"
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
	ctx          context.Context
	cancel       context.CancelFunc
	connCancel   context.CancelFunc
	closed       bool
	reconnecting bool

	autoReconnect bool
	subsMu        sync.RWMutex
	subs          []registration
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
	c.creds = nil
	if creds != nil {
		snapshot := *creds
		c.creds = &snapshot
	}
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

	// Serialize publishing the connection and replay with registration changes.
	c.subsMu.Lock()
	defer c.subsMu.Unlock()
	c.mu.Lock()
	if c.closed || c.conn != nil {
		closed := c.closed
		c.mu.Unlock()
		cancel()
		_ = conn.CloseNow()
		if closed {
			return fmt.Errorf("client closed")
		}
		return fmt.Errorf("client already connected")
	}
	done := make(chan struct{})
	c.conn = conn
	c.connDone = done
	oldCancel := c.connCancel
	c.connCancel = cancel
	c.mu.Unlock()

	if oldCancel != nil {
		oldCancel()
	}

	pongs := make(chan time.Time, 1)
	go func() {
		defer cancel()
		defer conn.CloseNow()
		c.readLoop(loopCtx, conn, done, pongs)
	}()
	go c.heartbeatLoop(loopCtx, conn, pongs)

	subs := serverSubscriptions(c.subs)

	if len(subs) > 0 {
		c.logger.Debug("resubscribing to topics", "count", len(subs))
		req := SubscriptionRequest{
			Action:        ActionSubscribe,
			Subscriptions: subs,
		}
		if err := c.sendJSON(ctx, req); err != nil {
			return fmt.Errorf("resubscribe: %w", err)
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

	if cancel != nil {
		cancel()
	}
	if connCancel != nil {
		connCancel()
	}
	var err error
	if conn != nil {
		err = conn.Close(websocket.StatusNormalClosure, "")
	}
	if done != nil {
		<-done
	}
	c.subsMu.Lock()
	c.subs = nil
	c.subsMu.Unlock()
	return err
}

// Messages returns a channel of received RTDS messages.
func (c *Client) Messages() <-chan *RtdsMessage {
	return c.msgs
}

// Errors returns a channel of asynchronous errors.
func (c *Client) Errors() <-chan error {
	return c.errs
}

// Subscribe registers a local interest. Each call owns one registration; use
// Unsubscribe with the same subscription to release it. Messages contains the
// union of matching interests, with each incoming message delivered once.
// Filters narrow messages locally; the server receives one broad subscription
// per topic/type. Registrations made while disconnected are replayed on Connect.
// On a write error the registration is retained for reconnect and the error is
// returned to the caller.
func (c *Client) Subscribe(ctx context.Context, sub Subscription) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	entry, err := newRegistration(sub)
	if err != nil {
		return err
	}
	c.subsMu.Lock()
	defer c.subsMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return fmt.Errorf("client closed")
	}
	for _, existing := range c.subs {
		if existing.sub.Topic == entry.sub.Topic && existing.sub.Type == entry.sub.Type &&
			!sameCredentials(existing.sub.CLOBAuth, entry.sub.CLOBAuth) {
			return fmt.Errorf(
				"rtds: conflicting credentials for topic/type %s/%s",
				sub.Topic,
				sub.Type,
			)
		}
	}
	before := serverSubscriptions(c.subs)
	c.subs = append(c.subs, entry)
	return c.syncSubscriptions(
		ctx,
		ActionSubscribe,
		serverDifference(serverSubscriptions(c.subs), before),
	)
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
// window. Use Subscribe with a symbol filter to narrow messages locally.
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
			c.mu.Lock()
			current := c.conn == conn
			if current {
				c.conn = nil
			}
			c.mu.Unlock()
			if !current {
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

		c.handleData(ctx, data)
	}
}

func (c *Client) handleData(ctx context.Context, data []byte) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return
	}
	// RTDS can return a single message or an array of messages.
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
	if m == nil {
		return
	}
	c.subsMu.RLock()
	matched := false
	for _, entry := range c.subs {
		if entry.matches(m) {
			matched = true
			break
		}
	}
	c.subsMu.RUnlock()
	if !matched {
		return
	}
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
				// The reader owns connection loss and reconnect scheduling.
				_ = conn.CloseNow()
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
				return
			}
		}
	}
}

// UnsubscribeCryptoPrices releases one interest for the given symbols.
func (c *Client) UnsubscribeCryptoPrices(ctx context.Context, symbols []string) error {
	sub := Subscription{Topic: "crypto_prices", Type: "update"}
	if len(symbols) > 0 {
		sub.Filters = symbols
	}
	return c.Unsubscribe(ctx, sub)
}

// UnsubscribeChainlinkPrices releases one interest for the given symbol.
func (c *Client) UnsubscribeChainlinkPrices(ctx context.Context, symbol string) error {
	sub := Subscription{Topic: "crypto_prices_chainlink", Type: "*"}
	if symbol != "" {
		sub.Filters = map[string]string{"symbol": symbol}
	}
	return c.Unsubscribe(ctx, sub)
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
	return c.Unsubscribe(ctx, Subscription{Topic: topic, Type: "update"})
}

// UnsubscribeChainlinkTWAP30Seconds unsubscribes from the 30-second TWAP feed.
func (c *Client) UnsubscribeChainlinkTWAP30Seconds(ctx context.Context) error {
	return c.UnsubscribeChainlinkTWAP(ctx, ChainlinkTWAP30Seconds)
}

// UnsubscribeChainlinkTWAP60Seconds unsubscribes from the 60-second TWAP feed.
func (c *Client) UnsubscribeChainlinkTWAP60Seconds(ctx context.Context) error {
	return c.UnsubscribeChainlinkTWAP(ctx, ChainlinkTWAP60Seconds)
}

// UnsubscribeComments releases one comment interest with the given credentials.
// As in SubscribeComments, nil uses the client's configured credentials.
func (c *Client) UnsubscribeComments(
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
	return c.Unsubscribe(ctx, Subscription{Topic: "comments", Type: msgType, CLOBAuth: auth})
}

// Unsubscribe releases one registration equal to sub (including filters and
// credentials). Other registrations, even identical ones, remain active. A
// missing registration is a no-op. On a write error the local release remains
// effective and reconnect replays only the remaining interests.
func (c *Client) Unsubscribe(ctx context.Context, sub Subscription) error {
	entry, err := newRegistration(sub)
	if err != nil {
		return err
	}
	c.subsMu.Lock()
	defer c.subsMu.Unlock()
	before := serverSubscriptions(c.subs)
	for i, existing := range c.subs {
		if existing.identity == entry.identity {
			c.subs = slices.Delete(c.subs, i, i+1)
			break
		}
	}
	return c.syncSubscriptions(
		ctx,
		ActionUnsubscribe,
		serverDifference(before, serverSubscriptions(c.subs)),
	)
}

func (c *Client) syncSubscriptions(ctx context.Context, action Action, subs []Subscription) error {
	if len(subs) == 0 || !c.IsConnected() {
		return nil
	}
	return c.sendJSON(ctx, SubscriptionRequest{Action: action, Subscriptions: subs})
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

// SubscriptionCount returns the number of local registrations, including
// duplicate interests and registrations awaiting connection.
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
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		// A partial write leaves server state uncertain. Reconnect from the
		// authoritative local registrations rather than retrying a delta.
		_ = conn.CloseNow()
		return err
	}
	return nil
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
		retry := !c.closed && c.conn == nil
		c.mu.Unlock()
		if retry {
			c.scheduleReconnect()
		}
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
