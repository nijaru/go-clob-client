package perps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"

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
	"tpsl",
}

// SessionConfig configures an authenticated Perps WebSocket session.
type SessionConfig struct {
	// WebSocketURL overrides the client's configured WebSocket URL.
	WebSocketURL string
	// Channels replaces the default account update channels when non-empty.
	Channels []string
}

// PerpsSessionEvent is a raw authenticated account update. Data is retained as
// JSON so callers can decode the channel-specific payload without precision
// loss or an SDK release for every new event field.
type PerpsSessionEvent struct {
	Channel   string
	Timestamp int64
	Sequence  int64
	Data      json.RawMessage
}

type sessionFrame struct {
	ID   int             `json:"id,omitempty"`
	Op   *sessionOp      `json:"op,omitempty"`
	Req  string          `json:"req,omitempty"`
	Chs  []string        `json:"chs,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

type sessionResponse struct {
	data json.RawMessage
	err  error
}

type orderWaitResponse struct {
	update perpsOrderUpdate
	err    error
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
// the official auth and subscription handshake, exposes account updates, and
// supports the stable low-level signed trading commands. TP/SL orchestration
// remains separate from this transport-level API.
type Session struct {
	conn         *websocket.Conn
	client       *AuthenticatedClient
	events       chan PerpsSessionEvent
	errors       chan error
	ctx          context.Context
	cancel       context.CancelFunc
	done         chan struct{}
	chainID      int64
	signer       *polyauth.Signer
	writeMu      sync.Mutex
	pendingMu    sync.Mutex
	pending      map[int]chan sessionResponse
	nextRequest  int
	orderWaitMu  sync.Mutex
	orderWaiters map[int][]chan orderWaitResponse
	orderUpdates []perpsOrderUpdate
	closeOnce    sync.Once
	closeErr     error
}

// ErrPerpsSigningKeyRequired indicates that a signed command needs the
// delegated proxy private key in PerpsCredentials.
var ErrPerpsSigningKeyRequired = errors.New("perps delegated signing key required")

// OpenSession connects and authenticates a delegated Perps account session.
func (c *AuthenticatedClient) OpenSession(
	ctx context.Context,
	config SessionConfig,
) (*Session, error) {
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
	conn, _, err := websocket.Dial(ctx, webSocketURL, nil)
	if err != nil {
		return nil, fmt.Errorf("perps: dial session: %w", err)
	}
	sessionCtx, cancel := context.WithCancel(context.Background())
	session := &Session{
		conn:         conn,
		client:       c,
		events:       make(chan PerpsSessionEvent, 128),
		errors:       make(chan error, 8),
		ctx:          sessionCtx,
		cancel:       cancel,
		done:         make(chan struct{}),
		chainID:      c.chainID,
		pending:      make(map[int]chan sessionResponse),
		nextRequest:  3,
		orderWaiters: make(map[int][]chan orderWaitResponse),
	}
	session.signer, err = c.delegatedSigner()
	if err != nil {
		cancel()
		_ = conn.Close(websocket.StatusPolicyViolation, "invalid signing key")
		return nil, err
	}
	if err := session.handshake(ctx, c.credentials, channels); err != nil {
		cancel()
		_ = conn.Close(websocket.StatusPolicyViolation, "authentication failed")
		return nil, err
	}
	go session.readLoop()
	return session, nil
}

func (s *Session) handshake(
	ctx context.Context,
	credentials PerpsCredentials,
	channels []string,
) error {
	if err := s.writeJSON(ctx, sessionFrame{
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
	if err := s.readAck(ctx, 1); err != nil {
		return fmt.Errorf("perps: session auth rejected: %w", err)
	}
	if err := s.writeJSON(ctx, sessionFrame{ID: 2, Req: "sub", Chs: channels}); err != nil {
		return fmt.Errorf("perps: send session subscription: %w", err)
	}
	if err := s.readAck(ctx, 2); err != nil {
		return fmt.Errorf("perps: session subscription rejected: %w", err)
	}
	return nil
}

func (s *Session) writeJSON(ctx context.Context, frame sessionFrame) error {
	payload, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	return s.writeRaw(ctx, payload)
}

func (s *Session) writeRaw(ctx context.Context, payload []byte) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return s.conn.Write(ctx, websocket.MessageText, payload)
}

func (s *Session) readAck(ctx context.Context, wantID int) error {
	_, payload, err := s.conn.Read(ctx)
	if err != nil {
		return err
	}
	var frame sessionFrame
	if err := json.Unmarshal(payload, &frame); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if frame.ID != wantID {
		return fmt.Errorf("response id %d, want %d", frame.ID, wantID)
	}
	var ack sessionAck
	if err := json.Unmarshal(frame.Data, &ack); err == nil && ack.Status != "" {
		if ack.Status != "ok" {
			if ack.Error == "" {
				ack.Error = "request rejected"
			}
			return fmt.Errorf("%s", ack.Error)
		}
		return nil
	}
	var acks []sessionAck
	if err := json.Unmarshal(frame.Data, &acks); err != nil {
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
	defer close(s.events)
	defer close(s.errors)
	for {
		_, payload, err := s.conn.Read(s.ctx)
		if err != nil {
			if s.ctx.Err() == nil {
				err = fmt.Errorf("perps: session read: %w", err)
				s.reportError(err)
			}
			s.rejectPending(err)
			s.rejectOrderWaiters(err)
			return
		}
		var response struct {
			ID   int             `json:"id"`
			Data json.RawMessage `json:"data"`
		}
		if json.Unmarshal(payload, &response) == nil && response.ID != 0 {
			if s.resolvePending(response.ID, response.Data) {
				continue
			}
		}
		var frame struct {
			Channel   string          `json:"ch"`
			Timestamp int64           `json:"ts"`
			Sequence  int64           `json:"sq"`
			Data      json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(payload, &frame); err != nil {
			s.reportError(fmt.Errorf("perps: decode session event: %w", err))
			continue
		}
		if frame.Channel == "" {
			continue
		}
		event := PerpsSessionEvent{
			Channel:   frame.Channel,
			Timestamp: frame.Timestamp,
			Sequence:  frame.Sequence,
			Data:      append(json.RawMessage(nil), frame.Data...),
		}
		s.resolveOrderWaiters(event)
		select {
		case s.events <- event:
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Session) resolveOrderWaiters(event PerpsSessionEvent) {
	if event.Channel != "orders" {
		return
	}
	var update perpsOrderUpdate
	if err := json.Unmarshal(event.Data, &update); err != nil {
		return
	}
	s.orderWaitMu.Lock()
	waiters := s.orderWaiters[update.ID]
	delete(s.orderWaiters, update.ID)
	if len(waiters) == 0 {
		s.orderUpdates = append(s.orderUpdates, update)
		if len(s.orderUpdates) > 64 {
			s.orderUpdates = s.orderUpdates[len(s.orderUpdates)-64:]
		}
	}
	s.orderWaitMu.Unlock()
	for _, waiter := range waiters {
		waiter <- orderWaitResponse{update: update}
	}
}

func (s *Session) waitForOrderUpdate(
	ctx context.Context,
	orderID int,
) (perpsOrderUpdate, error) {
	response := make(chan orderWaitResponse, 1)
	s.orderWaitMu.Lock()
	for i, update := range s.orderUpdates {
		if update.ID == orderID {
			s.orderUpdates = append(s.orderUpdates[:i], s.orderUpdates[i+1:]...)
			s.orderWaitMu.Unlock()
			return update, nil
		}
	}
	s.orderWaiters[orderID] = append(s.orderWaiters[orderID], response)
	s.orderWaitMu.Unlock()
	select {
	case result := <-response:
		return result.update, result.err
	case <-ctx.Done():
		s.removeOrderWaiter(orderID, response)
		return perpsOrderUpdate{}, ctx.Err()
	case <-s.ctx.Done():
		s.removeOrderWaiter(orderID, response)
		return perpsOrderUpdate{}, errors.New("perps session closed")
	}
}

func (s *Session) removeOrderWaiter(
	orderID int,
	response chan orderWaitResponse,
) {
	s.orderWaitMu.Lock()
	defer s.orderWaitMu.Unlock()
	waiters := s.orderWaiters[orderID]
	for i, waiter := range waiters {
		if waiter == response {
			waiters = append(waiters[:i], waiters[i+1:]...)
			break
		}
	}
	if len(waiters) == 0 {
		delete(s.orderWaiters, orderID)
	} else {
		s.orderWaiters[orderID] = waiters
	}
}

func (s *Session) rejectOrderWaiters(err error) {
	s.orderWaitMu.Lock()
	waiters := slices.Collect(maps.Values(s.orderWaiters))
	s.orderWaiters = make(map[int][]chan orderWaitResponse)
	s.orderUpdates = nil
	s.orderWaitMu.Unlock()
	for _, group := range waiters {
		for _, waiter := range group {
			waiter <- orderWaitResponse{err: err}
		}
	}
}

func (s *Session) resolvePending(id int, data json.RawMessage) bool {
	s.pendingMu.Lock()
	response, ok := s.pending[id]
	if ok {
		delete(s.pending, id)
	}
	s.pendingMu.Unlock()
	if !ok {
		return false
	}
	response <- sessionResponse{data: append(json.RawMessage(nil), data...)}
	return true
}

func (s *Session) rejectPending(err error) {
	s.pendingMu.Lock()
	responses := slices.Collect(maps.Values(s.pending))
	s.pending = make(map[int]chan sessionResponse)
	s.pendingMu.Unlock()
	for _, response := range responses {
		response <- sessionResponse{err: err}
	}
}

func (s *Session) nextID() int {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	id := s.nextRequest
	s.nextRequest++
	return id
}

func (s *Session) sendCommand(ctx context.Context, body map[string]any) (json.RawMessage, error) {
	id := s.nextID()
	body["id"] = id
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("perps: marshal session command: %w", err)
	}
	response := make(chan sessionResponse, 1)
	s.pendingMu.Lock()
	s.pending[id] = response
	s.pendingMu.Unlock()
	if err := s.writeRaw(ctx, payload); err != nil {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return nil, fmt.Errorf("perps: send session command: %w", err)
	}
	select {
	case result := <-response:
		return result.data, result.err
	case <-ctx.Done():
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return nil, ctx.Err()
	case <-s.ctx.Done():
		return nil, errors.New("perps session closed")
	}
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
		s.closeErr = s.conn.Close(websocket.StatusNormalClosure, "")
		<-s.done
	})
	return s.closeErr
}
