package perps

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/nijaru/go-clob-client/internal/polyauth"
)

var defaultSessionChannels = []string{
	"balances",
	"portfolio",
	"orders",
	"fills",
	"funding",
	"deposits",
	"withdrawals",
	"notifications",
	"tpsl",
}

const (
	perpsHeartbeatInterval    = 25 * time.Second
	perpsHeartbeatStale       = 65 * time.Second
	perpsReconnectInitialWait = 100 * time.Millisecond
	perpsReconnectMaxWait     = 5 * time.Second
	perpsReconnectTimeout     = 15 * time.Second
)

var perpsHeartbeatPayload = []byte(`{"id":0,"req":"post","op":{"type":"ping"}}`)

// SessionConfig configures an authenticated Perps WebSocket session.
type SessionConfig struct {
	// WebSocketURL overrides the client's configured WebSocket URL.
	WebSocketURL string
	// Channels replaces the default account update channels when non-empty.
	Channels []string
	// BuilderAddress selects attribution without granting owner consent.
	BuilderAddress string
	// IncludeBuilderFills subscribes to sparse builder receipts.
	IncludeBuilderFills bool
}

// PerpsResyncReason identifies why a session resync event was emitted.
type PerpsResyncReason string

const (
	PerpsResyncReconnect     PerpsResyncReason = "reconnect"
	PerpsResyncSequenceGap   PerpsResyncReason = "sequence_gap"
	PerpsResyncServerRequest PerpsResyncReason = "server"
)

// PerpsSessionResync is emitted when the server asks the notifications stream
// to be backfilled or the session reconnects.
type PerpsSessionResync struct {
	Reason           PerpsResyncReason
	Channel          string
	Timestamp        int64
	Sequence         int64
	PreviousSequence *int64
}

// PerpsSessionEvent is an authenticated account update. Data is retained as
// JSON so callers can decode channel-specific payloads without precision loss
// or an SDK release for every new event field. Known notification frames also
// populate Notification; server resync frames populate Resync.
type PerpsSessionEvent struct {
	Channel      string
	Timestamp    int64
	Sequence     int64
	Type         string
	Data         json.RawMessage
	Notification *PerpsNotification
	Resync       *PerpsSessionResync
	Market       *MarketEvent
}

type sessionFrame struct {
	ID   int             `json:"id,omitempty"`
	Op   *sessionOp      `json:"op,omitempty"`
	Req  string          `json:"req,omitempty"`
	Chs  []string        `json:"chs,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

type sessionOp struct {
	Type string         `json:"type"`
	Args map[string]any `json:"args,omitempty"`
}

type sessionAck struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// Session is an authenticated Perps account WebSocket session. It performs
// the official auth and subscription handshake, maintains an application-level
// heartbeat, reconnects and resubscribes after unexpected disconnects, exposes
// account updates, and supports signed trading commands and TP/SL groups.
type Session struct {
	conn           *websocket.Conn
	client         *AuthenticatedClient
	webSocketURL   string
	channels       []string
	events         chan PerpsSessionEvent
	errors         chan error
	ctx            context.Context
	cancel         context.CancelFunc
	done           chan struct{}
	heartbeatDone  chan struct{}
	chainID        int64
	signer         *polyauth.Signer
	lastMessage    atomic.Int64
	connMu         sync.RWMutex
	writeMu        sync.Mutex
	pendingMu      sync.Mutex
	pending        map[int]chan sessionResponse
	nextRequest    int
	orderWaitMu    sync.Mutex
	orderWaiters   map[string]chan orderWaitResponse
	sequenceMu     sync.Mutex
	sequences      map[string]int64
	closeOnce      sync.Once
	closeErr       error
	queuedPayloads []json.RawMessage
	builder        *PerpsBuilderTerms
	builderAddress string
	builderMu      sync.RWMutex
}

// ErrPerpsSigningKeyRequired indicates that a signed command needs the
// delegated proxy private key in PerpsCredentials.
var ErrPerpsSigningKeyRequired = errors.New("perps delegated signing key required")

// OpenSession connects and authenticates a delegated Perps account session.
func (c *AuthenticatedClient) OpenSession(
	ctx context.Context,
	config SessionConfig,
) (*Session, error) {
	builder, err := c.resolveBuilder(ctx, config.BuilderAddress)
	if err != nil {
		return nil, err
	}
	webSocketURL := config.WebSocketURL
	if webSocketURL == "" {
		webSocketURL = c.webSocketHost
	}
	channels := config.Channels
	if len(channels) == 0 {
		channels = append([]string(nil), defaultSessionChannels...)
	} else {
		channels = append([]string(nil), channels...)
	}
	if config.IncludeBuilderFills && !slices.Contains(channels, "builderFills") {
		channels = append(channels, "builderFills")
	}
	conn, _, err := websocket.Dial(
		ctx,
		webSocketURL,
		&websocket.DialOptions{HTTPClient: c.http.HTTPClient},
	)
	if err != nil {
		return nil, fmt.Errorf("perps: dial session: %w", err)
	}
	sessionCtx, cancel := context.WithCancel(ctx)
	session := &Session{
		conn:           conn,
		builder:        builder,
		builderAddress: config.BuilderAddress,
		client:         c,
		events:         make(chan PerpsSessionEvent, 128),
		errors:         make(chan error, 8),
		ctx:            sessionCtx,
		cancel:         cancel,
		done:           make(chan struct{}),
		heartbeatDone:  make(chan struct{}),
		chainID:        c.chainID,
		signer:         c.signer,
		webSocketURL:   webSocketURL,
		channels:       channels,
		pending:        make(map[int]chan sessionResponse),
		nextRequest:    3,
		orderWaiters:   make(map[string]chan orderWaitResponse),
		sequences:      make(map[string]int64),
	}
	session.lastMessage.Store(time.Now().UnixNano())
	if err := session.handshake(ctx, conn, c.credentials, channels); err != nil {
		cancel()
		_ = conn.Close(websocket.StatusPolicyViolation, "authentication failed")
		return nil, err
	}
	go session.readLoop()
	go session.heartbeatLoop()
	return session, nil
}

func (s *Session) handshake(
	ctx context.Context,
	conn *websocket.Conn,
	credentials PerpsCredentials,
	channels []string,
) error {
	if err := s.writeJSONConn(ctx, conn, sessionFrame{
		ID: 1,
		Op: &sessionOp{
			Type: "auth",
			Args: map[string]any{
				"proxy":  credentials.Proxy,
				"secret": credentials.Secret,
			},
		},
		Req: "post",
	}); err != nil {
		return fmt.Errorf("perps: send session auth: %w", err)
	}
	if err := s.readAckConn(ctx, conn, 1); err != nil {
		return fmt.Errorf("perps: session auth rejected: %w", err)
	}
	if err := s.writeJSONConn(
		ctx,
		conn,
		sessionFrame{ID: 2, Req: "sub", Chs: channels},
	); err != nil {
		return fmt.Errorf("perps: send session subscription: %w", err)
	}
	if err := s.readAckConn(ctx, conn, 2); err != nil {
		return fmt.Errorf("perps: session subscription rejected: %w", err)
	}
	return nil
}

func (s *Session) writeJSONConn(
	ctx context.Context,
	conn *websocket.Conn,
	frame sessionFrame,
) error {
	payload, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	return s.writeRawConn(ctx, conn, payload)
}

func (s *Session) writeRaw(ctx context.Context, payload []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	conn := s.currentConn()
	if conn == nil {
		return errors.New("perps: session connection unavailable")
	}
	return conn.Write(ctx, websocket.MessageText, payload)
}

func (s *Session) writeRawConn(
	ctx context.Context,
	conn *websocket.Conn,
	payload []byte,
) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return conn.Write(ctx, websocket.MessageText, payload)
}

func (s *Session) readAckConn(
	ctx context.Context,
	conn *websocket.Conn,
	wantID int,
) error {
	var data json.RawMessage
	for data == nil {
		_, payload, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		messages := []json.RawMessage{payload}
		if isJSONArray(payload) {
			if err := json.Unmarshal(payload, &messages); err != nil {
				return fmt.Errorf("decode handshake batch: %w", err)
			}
		}
		for _, message := range messages {
			var frame sessionFrame
			if err := json.Unmarshal(message, &frame); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
			if frame.ID == wantID {
				if data != nil {
					return fmt.Errorf("duplicate handshake acknowledgement %d", wantID)
				}
				data = frame.Data
				if len(data) == 0 {
					return fmt.Errorf("handshake acknowledgement %d missing data", wantID)
				}
				continue
			}
			// Account updates can arrive before the subscription acknowledgement,
			// including in the same batch. Deliver them once the reader starts.
			if len(s.queuedPayloads) >= cap(s.events) {
				return ErrPerpsSlowConsumer
			}
			s.queuedPayloads = append(s.queuedPayloads, append(json.RawMessage(nil), message...))
		}
	}
	var ack sessionAck
	if err := json.Unmarshal(data, &ack); err == nil && ack.Status != "" {
		if ack.Status != "ok" {
			if ack.Error == "" {
				ack.Error = "request rejected"
			}
			return fmt.Errorf("%s", ack.Error)
		}
		return nil
	}
	var acks []sessionAck
	if err := json.Unmarshal(data, &acks); err != nil {
		return fmt.Errorf("decode acknowledgement: %w", err)
	}
	if len(acks) == 0 {
		return fmt.Errorf("empty acknowledgement")
	}
	for _, ack := range acks {
		if ack.Status != "ok" {
			if ack.Error == "" {
				ack.Error = "request rejected"
			}
			return fmt.Errorf("%s", ack.Error)
		}
	}
	return nil
}

func (s *Session) readLoop() {
	defer close(s.done)
	defer func() {
		s.cancel()
		if s.heartbeatDone != nil {
			<-s.heartbeatDone
		}
	}()
	defer close(s.events)
	defer close(s.errors)
	for {
		queued := s.queuedPayloads
		s.queuedPayloads = nil
		for _, payload := range queued {
			s.handlePayload(payload)
		}
		conn := s.currentConn()
		if conn == nil {
			s.rejectPending(errors.New("perps: session connection unavailable"))
			s.rejectOrderWaiters(errors.New("perps: session connection unavailable"))
			return
		}
		_, payload, err := conn.Read(s.ctx)
		if err != nil {
			if s.ctx.Err() != nil {
				s.rejectPending(errors.New("perps session closed"))
				s.rejectOrderWaiters(errors.New("perps session closed"))
				return
			}
			if s.reconnect(err) {
				s.emitEvent(PerpsSessionEvent{
					Type: "resync",
					Resync: &PerpsSessionResync{
						Reason: PerpsResyncReconnect,
					},
				})
				continue
			}
			s.rejectPending(err)
			s.rejectOrderWaiters(err)
			return
		}
		s.lastMessage.Store(time.Now().UnixNano())
		s.handlePayload(payload)
	}
}

func (s *Session) handlePayload(payload []byte) {
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 {
		return
	}
	if payload[0] == '[' {
		var messages []json.RawMessage
		if err := json.Unmarshal(payload, &messages); err != nil {
			s.reportError(fmt.Errorf("perps: decode session batch: %w", err))
			return
		}
		for _, message := range messages {
			s.handlePayload(message)
		}
		return
	}

	var response struct {
		ID   int             `json:"id"`
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(payload, &response) == nil && response.ID != 0 {
		if s.resolvePending(response.ID, response.Data) {
			return
		}
		return
	}
	var frame struct {
		Channel   string          `json:"ch"`
		Timestamp int64           `json:"ts"`
		Sequence  *int64          `json:"sq"`
		Type      string          `json:"type"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(payload, &frame); err != nil {
		s.reportError(fmt.Errorf("perps: decode session event: %w", err))
		return
	}
	if frame.Channel == "" {
		return
	}
	sequence := int64(0)
	if frame.Sequence != nil {
		sequence = *frame.Sequence
	}
	if frame.Channel == "notifications" && frame.Type == "resync" {
		s.emitEvent(PerpsSessionEvent{
			Channel:   frame.Channel,
			Timestamp: frame.Timestamp,
			Sequence:  sequence,
			Type:      "resync",
			Resync: &PerpsSessionResync{
				Reason:    PerpsResyncServerRequest,
				Channel:   frame.Channel,
				Timestamp: frame.Timestamp,
				Sequence:  sequence,
			},
		})
		return
	}

	if frame.Sequence != nil {
		if resync := s.sequenceResync(frame.Channel, sequence, frame.Timestamp); resync != nil {
			s.emitEvent(PerpsSessionEvent{Type: "resync", Resync: resync})
		}
	}
	event := PerpsSessionEvent{
		Channel:   frame.Channel,
		Timestamp: frame.Timestamp,
		Sequence:  sequence,
		Type:      frame.Type,
		Data:      append(json.RawMessage(nil), frame.Data...),
	}
	if frame.Channel == "notifications" {
		event.Type = "notification"
		var notification PerpsNotification
		if err := json.Unmarshal(frame.Data, &notification); err == nil {
			event.Notification = &notification
		} else if !errors.Is(err, ErrUnknownPerpsNotification) {
			s.reportError(fmt.Errorf("perps: decode notification event: %w", err))
		}
	}
	s.resolveOrderWaiters(event)
	s.emitEvent(event)
}

func (s *Session) heartbeatLoop() {
	defer close(s.heartbeatDone)
	s.heartbeatLoopWith(perpsHeartbeatInterval, perpsHeartbeatStale)
}

func (s *Session) heartbeatLoopWith(interval, stale time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			last := time.Unix(0, s.lastMessage.Load())
			if time.Since(last) > stale {
				s.closeCurrentConn()
				continue
			}
			if err := s.writeRaw(s.ctx, perpsHeartbeatPayload); err != nil {
				s.closeCurrentConn()
			}
		}
	}
}

func (s *Session) reconnect(cause error) bool {
	s.rejectPending(cause)
	s.rejectOrderWaiters(cause)
	s.reportError(fmt.Errorf("perps: session disconnected: %w", cause))

	wait := perpsReconnectInitialWait
	for {
		select {
		case <-s.ctx.Done():
			return false
		case <-time.After(wait):
		}

		dialCtx, cancel := context.WithTimeout(s.ctx, perpsReconnectTimeout)
		conn, _, err := websocket.Dial(
			dialCtx,
			s.webSocketURL,
			&websocket.DialOptions{HTTPClient: s.client.http.HTTPClient},
		)
		if err == nil {
			err = s.handshake(dialCtx, conn, s.client.credentials, s.channels)
		}
		cancel()
		if err == nil {
			s.connMu.Lock()
			if s.ctx.Err() != nil {
				s.connMu.Unlock()
				_ = conn.Close(websocket.StatusNormalClosure, "session closed")
				return false
			}
			old := s.conn
			s.conn = conn
			s.connMu.Unlock()
			s.sequenceMu.Lock()
			s.sequences = make(map[string]int64)
			s.sequenceMu.Unlock()
			if old != nil {
				_ = old.Close(websocket.StatusNormalClosure, "replaced")
			}
			s.lastMessage.Store(time.Now().UnixNano())
			return true
		}
		if conn != nil {
			_ = conn.Close(websocket.StatusPolicyViolation, "reconnect failed")
		}
		if s.ctx.Err() != nil {
			return false
		}
		wait *= 2
		if wait > perpsReconnectMaxWait {
			wait = perpsReconnectMaxWait
		}
	}
}

func (s *Session) sequenceResync(channel string, sequence, timestamp int64) *PerpsSessionResync {
	if channel == "notifications" || channel == "builderFills" {
		return nil
	}
	s.sequenceMu.Lock()
	defer s.sequenceMu.Unlock()
	if s.sequences == nil {
		s.sequences = make(map[string]int64)
	}
	previous, ok := s.sequences[channel]
	s.sequences[channel] = sequence
	if !ok || sequence == previous+1 {
		return nil
	}
	return &PerpsSessionResync{
		Reason:           PerpsResyncSequenceGap,
		Channel:          channel,
		Timestamp:        timestamp,
		Sequence:         sequence,
		PreviousSequence: &previous,
	}
}

func (s *Session) currentConn() *websocket.Conn {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	return s.conn
}

func (s *Session) closeCurrentConn() {
	conn := s.currentConn()
	if conn != nil {
		_ = conn.Close(websocket.StatusGoingAway, "reconnect")
	}
}

func (s *Session) emitEvent(event PerpsSessionEvent) {
	select {
	case s.events <- event:
	case <-s.ctx.Done():
	default:
		s.reportError(ErrPerpsSlowConsumer)
		s.cancel()
	}
}

// ErrPerpsSlowConsumer means the bounded update queue filled. The account
// session or individual public handle closes rather than dropping data or
// blocking acknowledgements.
var ErrPerpsSlowConsumer = errors.New("perps: event consumer fell behind; session closed")

// AsNotification returns the typed notification payload for a notification
// event, decoding Data when the event was constructed by a caller.
func (e PerpsSessionEvent) AsNotification() (*PerpsNotification, error) {
	if e.Notification != nil {
		return e.Notification, nil
	}
	if e.Channel != "notifications" || e.Resync != nil {
		return nil, fmt.Errorf("perps session event is not a notification")
	}
	var notification PerpsNotification
	if err := json.Unmarshal(e.Data, &notification); err != nil {
		return nil, err
	}
	return &notification, nil
}

func (s *Session) reportError(err error) {
	select {
	case s.errors <- err:
	default:
	}
}

// Events returns authenticated account updates until the session closes.
func (s *Session) Events() <-chan PerpsSessionEvent { return s.events }

// Errors returns asynchronous session transport or decode errors.
func (s *Session) Errors() <-chan error { return s.errors }

// Close closes the authenticated session and its event channels.
func (s *Session) Close() error {
	s.closeOnce.Do(func() {
		s.cancel()
		s.rejectPending(errors.New("perps session closed"))
		s.rejectOrderWaiters(errors.New("perps session closed"))
		s.connMu.Lock()
		conn := s.conn
		s.conn = nil
		s.connMu.Unlock()
		if conn != nil {
			s.closeErr = conn.Close(websocket.StatusNormalClosure, "")
		}
		<-s.done
	})
	return s.closeErr
}
