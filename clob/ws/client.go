package ws

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	json "github.com/go-json-experiment/json"

	"github.com/coder/websocket"
	"github.com/nijaru/go-clob-client/clob"
)

const (
	defaultMarketURL         = "wss://ws-subscriptions-clob.polymarket.com/ws/market"
	defaultUserURL           = "wss://ws-subscriptions-clob.polymarket.com/ws/user"
	defaultHeartbeatInterval = 5 * time.Second
	defaultHeartbeatTimeout  = 15 * time.Second
	// Market initial dumps can contain one full book per subscribed asset. The
	// websocket package defaults to 32 KiB, which is smaller than ordinary
	// multi-asset snapshots and causes an otherwise healthy connection to
	// reconnect forever without delivering any events.
	defaultReadLimit = 4 << 20

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

	eventMu      sync.Mutex // serializes delivery with gap invalidation
	events       chan Event
	errs         chan error
	stop         chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc
	connCancel   context.CancelFunc
	closed       bool
	reconnecting bool

	autoReconnect bool
	subOpMu       sync.Mutex // serializes subscription mutations and replay
	subsMu        sync.RWMutex
	subs          []subscription
	marketRefs    map[string]int
	userRefs      map[string]int
	midpointRefs  map[string]int
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
	target      subscriptionTarget
	channelType string
	key         string
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
		events:            make(chan Event, 256),
		errs:              make(chan error, 10),
		stop:              make(chan struct{}),
		ctx:               ctx,
		cancel:            cancel,
		autoReconnect:     true,
		marketRefs:        make(map[string]int),
		userRefs:          make(map[string]int),
		midpointRefs:      make(map[string]int),
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
	conn.SetReadLimit(defaultReadLimit)

	// Keep replay and ordinary subscribe/unsubscribe commands ordered on the
	// same physical connection, including across reconnects.
	c.subOpMu.Lock()
	defer c.subOpMu.Unlock()

	loopCtx, cancel := context.WithCancel(c.ctx)

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
	go c.readLoop(loopCtx, conn, done, pongs)
	go c.heartbeatLoop(loopCtx, conn, pongs)

	if err := c.replayTrackedSubscriptions(ctx); err != nil {
		c.markDisconnected(conn, GapDisconnected)
		_ = conn.CloseNow()
		return fmt.Errorf("replay subscriptions: %w", err)
	}
	if !c.IsConnected() {
		return fmt.Errorf("websocket disconnected during subscription replay")
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
	if c.closed || !c.autoReconnect || c.reconnecting || c.conn != nil {
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
			c.markDisconnected(conn, GapDisconnected)

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

		// The reader must never block behind a slow consumer: it also
		// processes heartbeat responses and controls reconnect recovery.
		if !c.handleMessage(ctx, data) {
			c.markDisconnected(conn, GapConsumerLagged)
			_ = conn.CloseNow()
			c.scheduleReconnect()
			return
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
			// Polymarket expects plain text "PING"
			if err := conn.Write(ctx, websocket.MessageText, []byte("PING")); err != nil {
				if ctx.Err() != nil {
					return
				}
				c.markDisconnected(conn, GapDisconnected)
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
				c.markDisconnected(conn, GapDisconnected)
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
	conn := c.conn
	closed := c.closed
	c.mu.Unlock()
	if conn == nil || closed {
		return fmt.Errorf("not connected")
	}
	// Do not hold the lifecycle lock across network I/O: Close and reconnect
	// must remain able to interrupt a blocked write.
	return conn.Write(ctx, websocket.MessageText, data)
}

// markDisconnected marks a connection unhealthy once. Every unexpected
// disconnection invalidates local stream state: the server cannot replay gaps.
func (c *Client) markDisconnected(conn *websocket.Conn, reason GapReason) {
	c.mu.Lock()
	if c.conn != conn {
		c.mu.Unlock()
		return
	}
	c.conn = nil
	cancel := c.connCancel
	c.connCancel = nil
	closed := c.closed
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if !closed {
		c.reportGap(reason)
	}
}

// reportGap discards stale queued events and places a gap marker in the
// event stream itself, so consumers need not race an independent error channel.
func (c *Client) reportGap(reason GapReason) {
	c.eventMu.Lock()
	defer c.eventMu.Unlock()

	for {
		select {
		case <-c.events:
		default:
			// With producers serialized, the marker cannot be overtaken.
			c.events <- &GapEvent{Reason: reason}
			c.emitError(fmt.Errorf("clob websocket: continuity lost (%s)", reason))
			return
		}
	}
}

// emitEvent does not block the network reader behind a slow consumer.
func (c *Client) emitEvent(ctx context.Context, event Event) bool {
	c.eventMu.Lock()
	defer c.eventMu.Unlock()

	if ctx.Err() != nil {
		return false
	}
	select {
	case c.events <- event:
		return true
	default:
		return false
	}
}

func (c *Client) emitError(err error) {
	select {
	case c.errs <- err:
	default:
	}
}

func (c *Client) attemptReconnect() {
	defer func() {
		c.mu.Lock()
		c.reconnecting = false
		retry := !c.closed && c.autoReconnect && c.conn == nil
		c.mu.Unlock()
		// If a newly connected socket drops before this goroutine exits,
		// the reader could not schedule another reconnect while reconnecting
		// was still true. Hand off the retry explicitly.
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
			jitter := time.Duration((rand.Float64()*0.5 - 0.25) * float64(backoff))
			nextBackoff := backoff + jitter

			timer.Reset(nextBackoff)
		}
	}
}
