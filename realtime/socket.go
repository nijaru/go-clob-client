package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/coder/websocket"
)

const operationInterval = 110 * time.Millisecond

type session struct {
	client      *Client
	ctx         context.Context
	cancel      context.CancelFunc
	provider    Provider
	keys        map[priceKey]*keyState // client.mu
	wake        chan struct{}
	target      int       // client.mu; pressure lowers placement capacity, not existing filters.
	lastDrop    time.Time // client.mu
	rid         uint64    // worker-owned, never reset across reconnects.
	lastSent    time.Time
	generation  uint64
	established bool
}

func (s *session) wakeUp() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *session) states() []*keyState {
	s.client.mu.Lock()
	defer s.client.mu.Unlock()
	states := make([]*keyState, 0, len(s.keys))
	for _, state := range s.keys {
		states = append(states, state)
	}
	return states
}

func (s *session) reset() {
	s.client.mu.Lock()
	defer s.client.mu.Unlock()
	for _, state := range s.keys {
		state.confirmation = nil
		state.snapshot = nil
	}
}

func (s *session) run() {
	attempt := 0
	for s.ctx.Err() == nil {
		err := s.connected()
		if s.ctx.Err() != nil {
			return
		}
		_, authFailed := errors.AsType[*authFailure](err)
		rejected, isRejection := errors.AsType[*RejectionError](err)
		closed, isClose := errors.AsType[*ConnectionError](err)
		if (authFailed && isRejection && rejected.Code != "auth_unavailable") ||
			(isClose && (closed.Code == 4001 || closed.Code == 4008)) {
			s.terminate(err)
			return
		}
		s.reset()
		if s.established {
			attempt = 0
		}
		ceiling := s.client.config.ReconnectMin
		for i := 0; i < attempt && ceiling < s.client.config.ReconnectMax; i++ {
			if ceiling > s.client.config.ReconnectMax/2 {
				ceiling = s.client.config.ReconnectMax
				break
			}
			ceiling *= 2
		}
		if ceiling > s.client.config.ReconnectMax {
			ceiling = s.client.config.ReconnectMax
		}
		if closed != nil && closed.Code == 4003 {
			ceiling = 10 * time.Second
		}
		delay := time.Duration(rand.Int64N(int64(ceiling)))
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-s.ctx.Done():
			timer.Stop()
			return
		}
		if attempt < 30 {
			attempt++
		}
	}
}

type authFailure struct{ error }

func (e *authFailure) Unwrap() error { return e.error }

type readResult struct {
	data []byte
	err  error
}
type transport struct {
	session    *session
	conn       *websocket.Conn
	input      chan readResult
	readerDone chan struct{}
	cancel     context.CancelFunc
	ping       *time.Ticker
	stale      *time.Timer
	filters    map[priceKey]*keyState // Worker-owned wire incarnations, including pending subscribes.
}

func (s *session) connected() error {
	s.established = false
	cfg := s.client.config
	dialCtx, cancel := context.WithTimeout(s.ctx, cfg.ConnectTimeout)
	conn, response, err := websocket.Dial(
		dialCtx,
		cfg.URL,
		&websocket.DialOptions{HTTPClient: cfg.HTTPClient, HTTPHeader: cfg.Headers},
	)
	cancel()
	if err != nil {
		failure := &ConnectError{Err: err}
		if response != nil {
			failure.StatusCode = response.StatusCode
			if response.Body != nil {
				response.Body.Close()
			}
		}
		s.rejectUnaccepted(failure)
		return failure
	}
	conn.SetReadLimit(2 * 1024 * 1024)
	readerCtx, stop := context.WithCancel(s.ctx)
	t := &transport{
		session:    s,
		conn:       conn,
		input:      make(chan readResult),
		readerDone: make(chan struct{}),
		cancel:     stop,
		ping:       time.NewTicker(cfg.PingInterval),
		stale:      time.NewTimer(cfg.StaleTimeout),
		filters:    make(map[priceKey]*keyState),
	}
	s.client.mu.Lock()
	s.client.nextConnectionID++
	s.generation = s.client.nextConnectionID
	s.client.mu.Unlock()
	go func() {
		defer close(t.readerDone)
		for {
			_, data, err := conn.Read(readerCtx)
			select {
			case t.input <- readResult{data, err}:
			case <-readerCtx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	defer func() {
		t.cancel()
		t.conn.CloseNow()
		<-t.readerDone
		t.ping.Stop()
		t.stale.Stop()
	}()
	if _, err := t.request(operation{Op: "auth", Auth: &cfg.Credentials}, nil); err != nil {
		return &authFailure{err}
	}
	for s.ctx.Err() == nil {
		desired := s.states()
		wanted := make(map[priceKey]*keyState, len(desired))
		for _, state := range desired {
			wanted[state.key] = state
		}
		var removes, adds []*keyState
		for key, state := range t.filters {
			if wanted[key] != state {
				removes = append(removes, state)
			}
		}
		for key, state := range wanted {
			if t.filters[key] != state {
				adds = append(adds, state)
			}
		}
		if len(removes) > 0 {
			if _, err := t.change("unsubscribe", removes); err != nil {
				return err
			}
			for _, state := range removes {
				delete(t.filters, state.key)
			}
			continue
		}
		if len(adds) > 0 {
			accepted, err := t.change("subscribe", adds)
			if err != nil {
				if _, ok := errors.AsType[*RejectionError](err); ok {
					s.fail(adds, err)
				}
				return err
			}
			for i, state := range adds {
				if accepted[i] {
					s.established = true
				} else {
					delete(t.filters, state.key)
				}
			}
			continue
		}
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-s.wake:
		case frame := <-t.input:
			if err := t.handle(frame); err != nil {
				return err
			}
		case <-t.ping.C:
			if err := t.send(operation{Op: "ping"}); err != nil {
				return err
			}
		case <-t.stale.C:
			return &TimeoutError{Operation: "heartbeat"}
		}
	}
	return s.ctx.Err()
}

func (t *transport) send(frame operation) error {
	ctx, cancel := context.WithTimeout(t.session.ctx, t.session.client.config.AckTimeout)
	defer cancel()
	data, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	if err := t.conn.Write(ctx, websocket.MessageText, data); err != nil {
		return websocketError("write", err)
	}
	return nil
}

func websocketError(operation string, err error) error {
	if closed, ok := errors.AsType[websocket.CloseError](err); ok {
		return &ConnectionError{int(closed.Code), closed.Reason}
	}
	return fmt.Errorf("realtime: %s: %w", operation, err)
}

func (t *transport) handle(frame readResult) error {
	if frame.err != nil {
		return websocketError("read", frame.err)
	}
	t.stale.Reset(t.session.client.config.StaleTimeout)
	event, ok := parseEvent(frame.data)
	if !ok {
		return nil
	}
	s, c := t.session, t.session.client
	event.ConnectionID = s.generation
	c.mu.Lock()
	defer c.mu.Unlock()
	if event.Dropped != nil && *event.Dropped > 0 && time.Since(s.lastDrop) >= 5*time.Second {
		if time.Since(s.lastDrop) >= time.Minute {
			s.target = 64
		}
		s.target = max(1, s.target/2)
		s.lastDrop = time.Now()
	}
	key := priceKey{event.Channel, event.Symbol, s.provider}
	state := t.filters[key]
	// An old upstream filter may emit while removal is in flight. Its frames
	// must not reach a replacement that has not sent its own subscribe yet.
	if state == nil || s.keys[key] != state {
		return nil
	}
	snapshot := refreshSnapshot(state.snapshot, event)
	state.snapshot = &snapshot
	for listener := range state.listeners {
		c.deliver(listener, event)
	}
	return nil
}

func (t *transport) change(op string, states []*keyState) ([]bool, error) {
	filters := make([]wireSubscription, 0, len(states))
	for _, state := range states {
		filters = append(filters, state.key.wire())
	}
	// At most 64 filters of <=64-byte symbols fit well below the 60KB frame
	// ceiling. Reconciliation never combines filters from different sockets.
	return t.request(operation{Op: op, Subscriptions: filters}, states)
}

func (t *transport) request(frame operation, states []*keyState) ([]bool, error) {
	s := t.session
	s.rid++
	frame.RID = strconv.FormatUint(s.rid, 10)
	// Serialize and pace authentication and operations while still handling
	// events and heartbeat frames. Only this worker writes the connection.
	pace := time.NewTimer(max(0, operationInterval-time.Since(s.lastSent)))
	for waiting := true; waiting; {
		select {
		case <-pace.C:
			waiting = false
		case <-s.ctx.Done():
			pace.Stop()
			return nil, s.ctx.Err()
		case result := <-t.input:
			if err := t.handle(result); err != nil {
				pace.Stop()
				return nil, err
			}
		case <-t.ping.C:
			if err := t.send(operation{Op: "ping"}); err != nil {
				pace.Stop()
				return nil, err
			}
		case <-t.stale.C:
			pace.Stop()
			return nil, &TimeoutError{Operation: "heartbeat"}
		}
	}
	s.lastSent = time.Now()
	if err := t.send(frame); err != nil {
		return nil, err
	}
	if frame.Op == "subscribe" {
		// Track the sent incarnation before reading any replies. This also
		// permits price frames arriving before their acceptance acknowledgment.
		for _, state := range states {
			t.filters[state.key] = state
		}
	}
	expected := map[string]string{"auth": "authed", "subscribe": "subscribed", "unsubscribe": "unsubscribed"}[frame.Op]
	timer := time.NewTimer(s.client.config.AckTimeout)
	defer timer.Stop()
	accepted := make([]bool, len(states))
	index := 0
	for {
		select {
		case <-s.ctx.Done():
			return nil, s.ctx.Err()
		case <-timer.C:
			return nil, &TimeoutError{Operation: frame.Op + " acknowledgement"}
		case <-t.stale.C:
			return nil, &TimeoutError{Operation: "heartbeat"}
		case <-t.ping.C:
			if err := t.send(operation{Op: "ping"}); err != nil {
				return nil, err
			}
		case result := <-t.input:
			if err := t.handle(result); err != nil {
				return nil, err
			}
			var a ack
			if json.Unmarshal(result.data, &a) != nil || a.RID != frame.RID ||
				hasNull(result.data, "rid", "channel", "provider", "code") {
				continue
			}
			if a.Op != expected && a.Op != "error" {
				continue
			}
			if a.Op == "error" && a.Code == "" {
				continue
			}
			if a.Op == "error" && (a.Channel == "" || len(states) == 0) {
				return nil, &RejectionError{a.Code, a.Channel, a.RID}
			}
			if len(states) == 0 {
				return accepted, nil
			}
			if index >= len(states) || a.Channel != states[index].key.channel {
				continue
			}
			state := states[index]
			if a.Op == "error" {
				rejection := &RejectionError{a.Code, a.Channel, a.RID}
				if frame.Op == "unsubscribe" {
					return nil, rejection
				}
				s.fail([]*keyState{state}, rejection)
			} else {
				accepted[index] = true
				if frame.Op == "subscribe" {
					t.confirm(state, a.Provider)
				}
			}
			index++
			if index == len(states) {
				return accepted, nil
			}
		}
	}
}

func (t *transport) confirm(state *keyState, provider *Source) {
	c, s := t.session.client, t.session
	c.mu.Lock()
	defer c.mu.Unlock()
	if s.keys[state.key] != state {
		return
	}
	event := Event{
		Type:         Accepted,
		Channel:      state.key.channel,
		Symbol:       state.key.symbol,
		ConnectionID: s.generation,
		Confirmation: &Confirmation{provider},
	}
	if state.key.channel.twap() {
		event.WindowSeconds = 60
	}
	state.confirmation = &event
	state.accepted = true
	for listener := range state.listeners {
		c.deliver(listener, event)
		c.markReady(listener)
	}
}
