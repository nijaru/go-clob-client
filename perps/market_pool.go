package perps

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

var errMarketStreamClosed = errors.New("perps: public market stream closed")

// MarketStream pools public subscriptions on one socket. It is owned by the
// context passed to NewMarketStream, not by any individual subscriber. The
// socket opens lazily and closes when idle. Close waits for all socket IO workers.
type MarketStream struct {
	client *Client
	ctx    context.Context
	cancel context.CancelFunc
	add    chan *MarketHandle
	wake   chan struct{}
	done   chan struct{}
}

// MarketHandle owns a bounded queue and a set of filters on a MarketStream.
// Its context and Close affect only this handle. Consume both Events and Errors.
// Queue overflow closes this handle with ErrPerpsSlowConsumer, not its peers.
type MarketHandle struct {
	stream   *MarketStream
	ctx      context.Context
	cancel   context.CancelFunc
	channels []string
	events   chan PerpsSessionEvent
	errors   chan error
	reply    chan error
	done     chan struct{}
	stopWake func() bool
	ready    bool // owned exclusively by the stream loop
}

// NewMarketStream creates a public-only pool without performing network IO.
func (c *Client) NewMarketStream(ctx context.Context) *MarketStream {
	ctx, cancel := context.WithCancel(ctx)
	s := &MarketStream{
		client: c, ctx: ctx, cancel: cancel,
		add: make(chan *MarketHandle), wake: make(chan struct{}, 1), done: make(chan struct{}),
	}
	go s.run()
	return s
}

// Subscribe waits for subscription acknowledgement. The supplied context owns
// both that wait and the returned handle's entire lifetime. Filters are copied;
// duplicate or overlapping filters deliver each update only once per handle.
func (s *MarketStream) Subscribe(
	ctx context.Context,
	subscriptions []MarketSubscription,
) (*MarketHandle, error) {
	if len(subscriptions) == 0 {
		return nil, fmt.Errorf("perps: at least one public subscription required")
	}
	channels := make([]string, 0, len(subscriptions))
	for _, sub := range subscriptions {
		channel, err := sub.channel()
		if err != nil {
			return nil, err
		}
		channels = append(channels, channel)
	}
	slices.Sort(channels)
	channels = slices.Compact(channels)
	ctx, cancel := context.WithCancel(ctx)
	h := &MarketHandle{
		stream: s, ctx: ctx, cancel: cancel, channels: channels,
		events: make(chan PerpsSessionEvent, 128), errors: make(chan error, 8),
		reply: make(chan error, 1), done: make(chan struct{}),
	}
	select {
	case s.add <- h:
	case <-ctx.Done():
		cancel()
		return nil, ctx.Err()
	case <-s.done:
		cancel()
		return nil, errMarketStreamClosed
	}
	select {
	case err := <-h.reply:
		if err != nil {
			return nil, err
		}
		if ctx.Err() != nil {
			_ = h.Close()
			return nil, ctx.Err()
		}
		return h, nil
	case <-ctx.Done():
		_ = h.Close()
		return nil, ctx.Err()
	case <-s.done:
		return nil, errMarketStreamClosed
	}
}

func (s *MarketStream) notify() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *MarketStream) Close() error {
	s.cancel()
	<-s.done
	return nil
}

func (h *MarketHandle) Events() <-chan PerpsSessionEvent { return h.events }
func (h *MarketHandle) Errors() <-chan error             { return h.errors }

// Close waits for local filter removal and queue closure. The stream owns the
// subsequent server unsubscribe; a failed/uncertain operation resets the socket.
func (h *MarketHandle) Close() error {
	h.cancel()
	h.stream.notify()
	<-h.done
	return nil
}

func (h *MarketHandle) report(err error) {
	select {
	case h.errors <- err:
	default:
	}
}

func (h *MarketHandle) emit(event PerpsSessionEvent) {
	if h.ctx.Err() != nil {
		return
	}
	select {
	case h.events <- event:
	default:
		h.report(ErrPerpsSlowConsumer)
		h.cancel()
	}
}
