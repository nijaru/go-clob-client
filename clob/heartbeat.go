package clob

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// Close stops any background tasks (like heartbeats) and cleans up resources.
// It blocks until the heartbeat loop exits.
func (c *AuthenticatedClient) Close() error {
	return c.closeHeartbeats(context.Background())
}

// Shutdown gracefully stops background tasks with a context deadline. It
// returns ctx.Err() if the heartbeat loop does not stop before the deadline.
func (c *AuthenticatedClient) Shutdown(ctx context.Context) error {
	return c.closeHeartbeats(ctx)
}

var (
	// ErrHeartbeatsActive indicates that an automatic heartbeat loop is already running.
	ErrHeartbeatsActive = errors.New("heartbeats already active")
	// ErrHeartbeatsClosed indicates that the client has been closed and cannot restart heartbeats.
	ErrHeartbeatsClosed = errors.New("authenticated client is closed")
)

// HeartbeatsActive reports whether the automatic heartbeat loop is running.
func (c *AuthenticatedClient) HeartbeatsActive() bool {
	c.heartbeatMu.Lock()
	defer c.heartbeatMu.Unlock()
	return c.heartbeatCancel != nil
}

// StartHeartbeats starts heartbeat posting until ctx is canceled or StopHeartbeats,
// Close, or Shutdown is called. Constructors never start background work.
// It returns ErrHeartbeatsActive when a loop is already running.
func (c *AuthenticatedClient) StartHeartbeats(parent context.Context) error {
	if err := parent.Err(); err != nil {
		return err
	}
	c.heartbeatMu.Lock()
	if c.heartbeatClosed {
		c.heartbeatMu.Unlock()
		return ErrHeartbeatsClosed
	}
	if c.heartbeatCancel != nil {
		c.heartbeatMu.Unlock()
		return ErrHeartbeatsActive
	}
	if c.heartbeatInterval <= 0 {
		c.heartbeatInterval = 5 * time.Second
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	interval := c.heartbeatInterval
	c.heartbeatCancel = cancel
	c.heartbeatDone = done
	c.heartbeatMu.Unlock()

	go c.runHeartbeatLoop(ctx, done, interval)
	return nil
}

// StopHeartbeats stops automatic heartbeat posting and waits for the loop to
// exit or ctx expires. Stopping is reversible until Close or Shutdown is called.
func (c *AuthenticatedClient) StopHeartbeats(ctx context.Context) error {
	c.heartbeatMu.Lock()
	cancel := c.heartbeatCancel
	done := c.heartbeatDone
	c.heartbeatMu.Unlock()
	if cancel == nil || done == nil {
		return nil
	}
	cancel()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// closeHeartbeats permanently closes the heartbeat lifecycle and waits for any
// active loop to terminate.
func (c *AuthenticatedClient) closeHeartbeats(ctx context.Context) error {
	c.heartbeatMu.Lock()
	c.heartbeatClosed = true
	cancel := c.heartbeatCancel
	done := c.heartbeatDone
	c.heartbeatMu.Unlock()
	if cancel == nil || done == nil {
		return nil
	}
	cancel()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *AuthenticatedClient) runHeartbeatLoop(
	ctx context.Context,
	done chan struct{},
	interval time.Duration,
) {
	defer func() {
		c.heartbeatMu.Lock()
		if c.heartbeatDone == done {
			c.heartbeatCancel = nil
			c.heartbeatDone = nil
		}
		close(done)
		c.heartbeatMu.Unlock()
	}()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.heartbeatMu.Lock()
			heartbeatID := c.heartbeatID
			c.heartbeatMu.Unlock()

			resp, err := c.PostHeartbeat(ctx, heartbeatID)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Warn("heartbeat failed", "err", err)
				continue
			}

			c.heartbeatMu.Lock()
			if c.heartbeatDone == done {
				c.heartbeatID = resp.HeartbeatID
			}
			c.heartbeatMu.Unlock()
		}
	}
}
