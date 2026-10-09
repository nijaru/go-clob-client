package realtime

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"sync"
	"time"
)

// Client pools shared filters on provider-isolated WebSocket connections.
// Create one client for concurrent subscriptions and Close it when finished.
// Close waits for all network readers and connection workers to exit.
type Client struct {
	config           Config
	mu               sync.Mutex
	closed           bool
	keys             map[priceKey]*keyState
	subscriptions    map[*Subscription]bool
	sessions         map[*session]bool
	wg               sync.WaitGroup
	done             chan struct{}
	nextConnectionID uint64
}

type keyState struct {
	key          priceKey
	session      *session
	listeners    map[*Subscription]bool
	confirmation *Event
	snapshot     *Event
	accepted     bool // Lifetime acceptance survives reconnect, unlike cached confirmation.
}

// Subscription owns a bounded event channel. Subscribe waits for server
// acceptance of every filter. Its context governs the entire stream lifetime.
// Drain Events, then inspect Err for termination. Buffered events remain
// readable after close. Closing one handle does not close other listeners.
type Subscription struct {
	client      *Client
	events      chan Event
	done        chan struct{}
	ready       chan struct{}
	readyClosed bool
	keys        []*keyState
	types       map[EventType]bool
	err         error
	ended       bool
	stopContext func() bool
}

func (s *Subscription) Events() <-chan Event  { return s.events }
func (s *Subscription) Done() <-chan struct{} { return s.done }

func (s *Subscription) Err() error { s.client.mu.Lock(); defer s.client.mu.Unlock(); return s.err }

func (s *Subscription) Close() error {
	s.client.mu.Lock()
	defer s.client.mu.Unlock()
	s.client.end(s, nil)
	return nil
}

func NewClient(config Config) (*Client, error) {
	if config.URL == "" {
		config.URL = DefaultURL
	}
	u, err := url.Parse(config.URL)
	if err != nil || (u.Scheme != "ws" && u.Scheme != "wss") || u.Host == "" || u.User != nil ||
		u.Fragment != "" {
		return nil, fmt.Errorf("realtime: invalid WebSocket URL")
	}
	if config.Credentials.APIKey == "" || config.Credentials.Secret == "" ||
		config.Credentials.Passphrase == "" {
		return nil, fmt.Errorf("realtime: API key, secret and passphrase are required")
	}
	if config.BufferSize < 0 {
		return nil, fmt.Errorf("realtime: buffer size must be positive")
	}
	if config.BufferSize == 0 {
		config.BufferSize = 1024
	}
	defaults := []struct {
		value    *time.Duration
		fallback time.Duration
	}{
		{&config.ConnectTimeout, 10 * time.Second},
		{&config.AckTimeout, 10 * time.Second},
		{&config.AcceptanceTimeout, 30 * time.Second},
		{&config.PingInterval, 30 * time.Second},
		{&config.StaleTimeout, 90 * time.Second},
		{&config.ReconnectMin, time.Second},
		{&config.ReconnectMax, 30 * time.Second},
	}
	for _, d := range defaults {
		if *d.value < 0 {
			return nil, fmt.Errorf("realtime: durations must be positive")
		}
		if *d.value == 0 {
			*d.value = d.fallback
		}
	}
	if config.ReconnectMin > config.ReconnectMax {
		return nil, fmt.Errorf("realtime: reconnect minimum exceeds maximum")
	}
	config.Headers = config.Headers.Clone()
	return &Client{
		config:        config,
		keys:          make(map[priceKey]*keyState),
		subscriptions: make(map[*Subscription]bool),
		sessions:      make(map[*session]bool),
		done:          make(chan struct{}),
	}, nil
}

func (c *Client) Subscribe(ctx context.Context, request Request) (*Subscription, error) {
	keys, err := requestKeys(request)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s := &Subscription{
		client: c,
		events: make(chan Event, c.config.BufferSize),
		done:   make(chan struct{}),
		ready:  make(chan struct{}),
		types:  make(map[EventType]bool),
	}
	for _, t := range request.Types {
		s.types[t] = true
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, ErrClosed
	}
	c.subscriptions[s] = true
	for _, key := range keys {
		state := c.keys[key]
		if state == nil {
			socket := c.place(key.provider)
			state = &keyState{key: key, session: socket, listeners: make(map[*Subscription]bool)}
			c.keys[key] = state
			socket.keys[key] = state
		}
		state.listeners[s] = true
		s.keys = append(s.keys, state)
	}
	s.stopContext = context.AfterFunc(
		ctx,
		func() { c.mu.Lock(); defer c.mu.Unlock(); c.end(s, ctx.Err()) },
	)
	for _, state := range s.keys {
		if state.confirmation != nil {
			c.deliver(s, *state.confirmation)
		}
		if state.snapshot != nil {
			c.deliver(s, *state.snapshot)
		}
		state.session.wakeUp()
	}
	c.markReady(s)
	c.mu.Unlock()
	timer := time.NewTimer(c.config.AcceptanceTimeout)
	defer timer.Stop()
	select {
	case <-s.ready:
	case <-ctx.Done():
		c.mu.Lock()
		c.end(s, ctx.Err())
		c.mu.Unlock()
	case <-timer.C:
		c.mu.Lock()
		c.end(s, &TimeoutError{Operation: "subscription acceptance"})
		c.mu.Unlock()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if s.ended {
		if s.err != nil {
			return nil, s.err
		}
		return nil, ErrClosed
	}
	return s, nil
}

// Close is concurrent-safe and idempotent. The client cannot be reused afterward.
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		<-c.done
		return nil
	}
	c.closed = true
	for s := range c.subscriptions {
		c.end(s, ErrClosed)
	}
	for socket := range c.sessions {
		socket.cancel()
	}
	c.mu.Unlock()
	c.wg.Wait()
	close(c.done)
	return nil
}

// All subscription, key and pool state is owned by c.mu. Network effects never
// run under it; each socket has one worker that reconciles desired filters with
// acknowledged server state, including removals during in-flight operations.
func (c *Client) place(provider Provider) *session {
	now := time.Now()
	for s := range c.sessions {
		if s.ctx.Err() != nil || s.provider != provider {
			continue
		}
		target := s.target
		if now.Sub(s.lastDrop) >= time.Minute {
			target = 64
		}
		if len(s.keys) < target {
			return s
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &session{
		client:   c,
		ctx:      ctx,
		cancel:   cancel,
		provider: provider,
		keys:     make(map[priceKey]*keyState),
		wake:     make(chan struct{}, 1),
		target:   64,
	}
	c.sessions[s] = true
	c.wg.Go(func() {
		s.run()
		c.mu.Lock()
		delete(c.sessions, s)
		c.mu.Unlock()
	})
	return s
}

func (c *Client) deliver(s *Subscription, event Event) {
	if s.ended || (event.Type != Accepted && len(s.types) > 0 && !s.types[event.Type]) {
		return
	}
	select {
	case s.events <- cloneEvent(event):
	default:
		c.end(s, ErrSlowConsumer)
	}
}

func (c *Client) markReady(s *Subscription) {
	if s.readyClosed {
		return
	}
	for _, key := range s.keys {
		if key.confirmation == nil {
			return
		}
	}
	s.readyClosed = true
	close(s.ready)
}

func (c *Client) end(s *Subscription, err error) {
	if s.ended {
		return
	}
	s.ended = true
	s.err = err
	if s.stopContext != nil {
		s.stopContext()
	}
	delete(c.subscriptions, s)
	for _, state := range s.keys {
		delete(state.listeners, s)
		if len(state.listeners) != 0 {
			continue
		}
		if c.keys[state.key] == state {
			delete(c.keys, state.key)
		}
		delete(state.session.keys, state.key)
		if len(state.session.keys) == 0 {
			state.session.cancel()
		} else {
			state.session.wakeUp()
		}
	}
	if !s.readyClosed {
		s.readyClosed = true
		close(s.ready)
	}
	close(s.events)
	close(s.done)
}

// Initial dial failures reject never-accepted keys rather than hiding the
// transport/HTTP cause behind an acceptance timeout. Accepted streams retain
// ownership and keep reconnecting; joining callers have their own deadline.
func (s *session) rejectUnaccepted(err error) {
	c := s.client
	c.mu.Lock()
	defer c.mu.Unlock()
	var states []*keyState
	for _, state := range s.keys {
		if !state.accepted {
			states = append(states, state)
		}
	}
	c.reject(states, err)
}

// Terminal failure must exclude new registrations and end every listener in
// the same critical section; taking a keys snapshot first can strand a racing
// subscription on a worker that is about to exit.
func (s *session) terminate(err error) {
	c := s.client
	c.mu.Lock()
	defer c.mu.Unlock()
	s.cancel()
	c.reject(slices.Collect(maps.Values(s.keys)), err)
}

func (s *session) fail(states []*keyState, err error) {
	c := s.client
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reject(states, err)
}

// Capture the entire fanout first: ending one multi-key handle removes its
// listeners from other keys. Caller holds c.mu.
func (c *Client) reject(states []*keyState, err error) {
	listeners := make(map[*Subscription]bool)
	for _, state := range states {
		for listener := range state.listeners {
			listeners[listener] = true
		}
	}
	for listener := range listeners {
		c.end(listener, err)
	}
}
