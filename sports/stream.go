package sports

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/url"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Stream owns one connection lifecycle, including replacement sockets. It has
// one bounded event queue; multiple channel readers compete, rather than fan out.
// Each Dial creates an independent stream. All methods are concurrency-safe.
type Stream struct {
	config Config
	ctx    context.Context
	cancel context.CancelCauseFunc
	events chan Event
	done   chan struct{}

	mu    sync.Mutex
	err   error
	stats Stats
}

// Dial opens a public, all-games stream bound to ctx. The initial upgrade must
// succeed; subsequent disconnects and stale heartbeats reconnect until ctx ends
// or Close is called. Reconnects have no replay/resume guarantee.
func Dial(ctx context.Context, config Config) (*Stream, error) {
	config, err := configure(config)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancelCause(ctx)
	conn, err := connect(ctx, config)
	if err != nil {
		cancel(err)
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		conn.CloseNow()
		cancel(err)
		return nil, err
	}
	s := &Stream{
		config: config, ctx: ctx, cancel: cancel,
		events: make(chan Event, config.BufferSize), done: make(chan struct{}),
	}
	go s.run(conn)
	return s, nil
}

// Events returns the bounded queue. It closes on termination; already-buffered
// events remain readable. A full queue terminates with ErrSlowConsumer rather
// than silently dropping results or blocking the heartbeat reader.
func (s *Stream) Events() <-chan Event { return s.events }

// Done closes after all lifecycle work and socket cleanup have completed.
func (s *Stream) Done() <-chan struct{} { return s.done }

// Err returns the terminal error, or nil while active or explicitly closed.
// Wait for Done or for Events to close before relying on the final result.
func (s *Stream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *Stream) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats
}

// Close cancels and joins the lifecycle, including any read, write, upgrade or
// reconnect delay in progress. It is idempotent and safe for concurrent callers.
// It does not wait for the peer's close handshake or drain the consumer queue.
func (s *Stream) Close() error {
	s.cancel(errClosed)
	<-s.done
	return s.Err()
}

func (s *Stream) run(conn *websocket.Conn) {
	var terminal error
	defer func() {
		s.cancel(terminal)
		s.mu.Lock()
		s.err = terminal
		s.mu.Unlock()
		close(s.events)
		close(s.done)
	}()
	for {
		err := s.read(conn)
		conn.CloseNow()
		if s.ctx.Err() != nil {
			if !errors.Is(context.Cause(s.ctx), errClosed) {
				terminal = s.ctx.Err()
			}
			return
		}
		if errors.Is(err, ErrSlowConsumer) {
			terminal = err
			return
		}
		s.connectionFailed(err)
		// Reset the backoff after a successful connection, as both reference
		// managers do. Failed replacement upgrades continue increasing it.
		capDelay := s.config.ReconnectMin
		for {
			if err := wait(s.ctx, time.Duration(rand.Int64N(int64(capDelay)))); err != nil {
				if !errors.Is(context.Cause(s.ctx), errClosed) {
					terminal = s.ctx.Err()
				}
				return
			}
			conn, err = connect(s.ctx, s.config)
			if err == nil {
				s.mu.Lock()
				s.stats.Reconnects++
				s.mu.Unlock()
				break
			}
			s.connectionFailed(err)
			if capDelay > s.config.ReconnectMax/2 {
				capDelay = s.config.ReconnectMax
			} else {
				capDelay *= 2
			}
		}
	}
}

func (s *Stream) connectionFailed(err error) {
	s.mu.Lock()
	s.stats.LastConnectionError = err
	s.mu.Unlock()
}

func (s *Stream) read(conn *websocket.Conn) error {
	deadline := time.Now().Add(s.config.StaleTimeout)
	for {
		// Only an application ping refreshes this deadline, not game events or
		// RFC 6455 control frames. Read cancellation also closes the connection.
		ctx, cancel := context.WithDeadline(s.ctx, deadline)
		_, data, err := conn.Read(ctx)
		cancel()
		if err != nil {
			if s.ctx.Err() == nil && !time.Now().Before(deadline) {
				return ErrStale
			}
			return fmt.Errorf("sports: read: %w", err)
		}
		if string(data) == "ping" {
			deadline = time.Now().Add(s.config.StaleTimeout)
			ctx, cancel := context.WithTimeout(
				s.ctx,
				min(s.config.WriteTimeout, s.config.StaleTimeout),
			)
			err := conn.Write(ctx, websocket.MessageText, []byte("pong"))
			cancel()
			if err != nil {
				return fmt.Errorf("sports: pong: %w", err)
			}
			continue
		}
		event, err := ParseEvent(data)
		if err != nil {
			s.mu.Lock()
			s.stats.MalformedMessages++
			s.mu.Unlock()
			continue
		}
		if err := s.ctx.Err(); err != nil {
			return err
		}
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case s.events <- event:
		default:
			return ErrSlowConsumer
		}
	}
}

func connect(ctx context.Context, config Config) (*websocket.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, config.ConnectTimeout)
	defer cancel()
	conn, response, err := websocket.Dial(ctx, config.URL, &websocket.DialOptions{
		HTTPClient: config.HTTPClient, HTTPHeader: config.Headers,
	})
	if err != nil {
		status := 0
		if response != nil {
			status = response.StatusCode
		}
		return nil, &ConnectError{StatusCode: status, Err: err}
	}
	conn.SetReadLimit(config.MaxMessageBytes)
	return conn, nil
}

func wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ctx.Err()
	}
}

func configure(c Config) (Config, error) {
	if c.URL == "" {
		c.URL = DefaultURL
	}
	u, err := url.Parse(c.URL)
	if err != nil || (u.Scheme != "ws" && u.Scheme != "wss") || u.Host == "" || u.User != nil ||
		u.Fragment != "" {
		return Config{}, errors.New(
			"sports: URL must be ws/wss with a host, without userinfo or fragment",
		)
	}
	if c.BufferSize < 0 || c.MaxMessageBytes < 0 {
		return Config{}, errors.New("sports: buffer and message limits must be positive")
	}
	if c.BufferSize == 0 {
		c.BufferSize = 1024
	}
	if c.MaxMessageBytes == 0 {
		c.MaxMessageBytes = 1 << 20
	}
	for _, item := range []struct {
		value *time.Duration
		def   time.Duration
	}{
		{&c.ConnectTimeout, 10 * time.Second},
		{&c.WriteTimeout, 5 * time.Second},
		{&c.StaleTimeout, 30 * time.Second},
		{&c.ReconnectMin, 250 * time.Millisecond},
		{&c.ReconnectMax, 30 * time.Second},
	} {
		if *item.value < 0 {
			return Config{}, errors.New("sports: timeouts and backoff must be positive")
		}
		if *item.value == 0 {
			*item.value = item.def
		}
	}
	if c.ReconnectMax < c.ReconnectMin {
		return Config{}, errors.New("sports: ReconnectMax must be >= ReconnectMin")
	}
	c.Headers = c.Headers.Clone()
	return c, nil
}
