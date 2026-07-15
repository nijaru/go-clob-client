package perps

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/coder/websocket"
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

type sessionOp struct {
	Type string         `json:"type"`
	Args map[string]any `json:"args,omitempty"`
}

type sessionAck struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// Session is an authenticated Perps account WebSocket session. It performs
// the official auth and subscription handshake and exposes account updates.
// Signed trading commands are intentionally kept separate until their exact
// MessagePack/EIP-712 contract is implemented and tested.
type Session struct {
	conn      *websocket.Conn
	events    chan PerpsSessionEvent
	errors    chan error
	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}
	closeOnce sync.Once
	closeErr  error
}

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
		conn:   conn,
		events: make(chan PerpsSessionEvent, 128),
		errors: make(chan error, 8),
		ctx:    sessionCtx,
		cancel: cancel,
		done:   make(chan struct{}),
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
				s.reportError(fmt.Errorf("perps: session read: %w", err))
			}
			return
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
		select {
		case s.events <- event:
		case <-s.ctx.Done():
			return
		}
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
		s.closeErr = s.conn.Close(websocket.StatusNormalClosure, "")
		<-s.done
	})
	return s.closeErr
}
