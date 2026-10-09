package perps

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
)

// All mutable subscription, acknowledgement and transport state belongs to
// this owner. A reader only dials/reads and reports messages for one generation.
type marketOwner struct {
	stream      *MarketStream
	handles     map[*MarketHandle]struct{}
	wire        map[string]bool
	sequences   map[string]int64
	pending     *marketOperation
	nextID      int
	reader      *marketReader
	writer      *marketWriter
	conn        *websocket.Conn
	lastMessage time.Time
	lastPing    time.Time
	retryAt     time.Time
	backoff     time.Duration
	connected   bool
}

type marketOperation struct {
	id       int
	req      string
	channels []string
	deadline time.Time
}

type marketRead struct {
	conn    *websocket.Conn
	payload []byte
	err     error
}

type marketWriter struct {
	result chan error
	done   chan struct{}
}

type marketReader struct {
	messages chan marketRead
	cancel   context.CancelFunc
	done     chan struct{}
}

func (s *MarketStream) startReader() *marketReader {
	ctx, cancel := context.WithCancel(s.ctx)
	r := &marketReader{
		messages: make(chan marketRead, 128),
		cancel:   cancel,
		done:     make(chan struct{}),
	}
	go func() {
		defer close(r.done)
		send := func(message marketRead) bool {
			select {
			case r.messages <- message:
				return true
			case <-ctx.Done():
				return false
			}
		}
		dialCtx, stop := context.WithTimeout(ctx, perpsReconnectTimeout)
		conn, _, err := websocket.Dial(
			dialCtx,
			s.client.webSocketHost,
			&websocket.DialOptions{HTTPClient: s.client.http.HTTPClient},
		)
		stop()
		if err != nil {
			send(marketRead{err: err})
			return
		}
		defer conn.CloseNow()
		if !send(marketRead{conn: conn}) {
			return
		}
		for {
			_, payload, err := conn.Read(ctx)
			if !send(marketRead{payload: payload, err: err}) || err != nil {
				return
			}
		}
	}()
	return r
}

func (s *MarketStream) run() {
	o := &marketOwner{
		stream:    s,
		handles:   make(map[*MarketHandle]struct{}),
		nextID:    1,
		wire:      make(map[string]bool),
		sequences: make(map[string]int64),
		backoff:   perpsReconnectInitialWait,
	}
	defer close(s.done)
	defer func() {
		o.stopTransport()
		for h := range o.handles {
			o.finish(h, errMarketStreamClosed)
		}
	}()
	tick := time.NewTicker(perpsReconnectInitialWait)
	defer tick.Stop()
	for {
		if s.ctx.Err() != nil {
			return
		}
		o.prune()
		if len(o.handles) == 0 {
			o.stopTransport()
			o.connected = false
			o.retryAt = time.Time{}
			o.backoff = perpsReconnectInitialWait
		} else if o.reader == nil && !time.Now().Before(o.retryAt) {
			o.reader = s.startReader()
		}
		if o.conn != nil {
			o.activateCovered()
			if o.pending == nil && o.writer == nil {
				o.reconcile()
			}
		}
		var messages <-chan marketRead
		if o.reader != nil {
			messages = o.reader.messages
		}
		var written <-chan error
		if o.writer != nil {
			written = o.writer.result
		}
		select {
		case err := <-written:
			<-o.writer.done
			o.writer = nil
			if err != nil {
				o.disconnected(err)
			}
		case <-s.ctx.Done():
			return
		case h := <-s.add:
			o.handles[h] = struct{}{}
			h.stopWake = context.AfterFunc(h.ctx, s.notify)
		case <-s.wake:
		case message := <-messages:
			if message.err != nil {
				o.disconnected(message.err)
				continue
			}
			if message.conn != nil {
				o.conn = message.conn
				o.lastMessage = time.Now()
				o.lastPing = o.lastMessage
				o.backoff = perpsReconnectInitialWait
				if o.connected {
					for h := range o.handles {
						if h.ready {
							h.emit(
								PerpsSessionEvent{
									Type:   "resync",
									Resync: &PerpsSessionResync{Reason: PerpsResyncReconnect},
								},
							)
						}
					}
				}
				o.connected = true
			} else {
				o.lastMessage = time.Now()
				if err := o.payload(message.payload); err != nil {
					o.disconnected(err)
				}
			}
		case now := <-tick.C:
			if o.pending != nil && !now.Before(o.pending.deadline) {
				o.disconnected(fmt.Errorf("perps: public subscription acknowledgement timed out"))
			} else if o.conn != nil && now.Sub(o.lastMessage) > perpsHeartbeatStale {
				o.disconnected(fmt.Errorf("perps: public heartbeat stale"))
			} else if o.conn != nil && o.writer == nil && now.Sub(o.lastPing) >= perpsHeartbeatInterval {
				o.lastPing = now
				o.write(perpsHeartbeatPayload)
			}
		}
	}
}

func (o *marketOwner) stopTransport() {
	if o.reader != nil {
		o.reader.cancel()
		if o.conn != nil {
			_ = o.conn.CloseNow()
		}
		<-o.reader.done
		o.reader = nil
	}
	if o.writer != nil {
		<-o.writer.done
		o.writer = nil
	}
	o.conn = nil
	o.pending = nil
	o.wire = make(map[string]bool)
	o.sequences = make(map[string]int64)
}

func (o *marketOwner) finish(h *MarketHandle, err error) {
	delete(o.handles, h)
	if h.stopWake != nil {
		h.stopWake()
	}
	h.cancel()
	if !h.ready {
		h.reply <- err
	} else if err != nil && err != context.Canceled {
		h.report(err)
	}
	close(h.events)
	close(h.errors)
	close(h.done)
}

func (o *marketOwner) prune() {
	removed := false
	for h := range o.handles {
		if err := h.ctx.Err(); err != nil {
			o.finish(h, err)
			removed = true
		}
	}
	// A canceled in-flight change may already have applied remotely. Rebuild
	// from surviving handles on a clean socket rather than guess server state.
	if removed && o.pending != nil {
		desired := o.desired()
		for _, ch := range o.pending.channels {
			if desired[ch] != (o.pending.req == "sub") {
				o.stopTransport()
				break
			}
		}
	}
}

func (o *marketOwner) disconnected(err error) {
	for h := range o.handles {
		if !h.ready {
			o.finish(h, err)
		} else {
			h.report(fmt.Errorf("perps: public stream disconnected: %w", err))
		}
	}
	o.stopTransport()
	o.retryAt = time.Now().Add(o.backoff)
	o.backoff = min(o.backoff*2, perpsReconnectMaxWait)
}

func (o *marketOwner) desired() map[string]bool {
	channels := make(map[string]bool)
	for h := range o.handles {
		for _, ch := range h.channels {
			channels[ch] = true
		}
	}
	for ch := range channels {
		parts := strings.Split(ch, "::")
		if parts[1] != "all" && channels[parts[0]+"::all"] {
			delete(channels, ch)
		}
	}
	return channels
}

// A handle need not wait for unrelated filter changes or cleanup acks.
func (o *marketOwner) activateCovered() {
	available := func(ch string) bool {
		return o.wire[ch] &&
			(o.pending == nil || o.pending.req != "unsub" || !slices.Contains(o.pending.channels, ch))
	}
	for h := range o.handles {
		if h.ready || h.ctx.Err() != nil {
			continue
		}
		covered := true
		for _, ch := range h.channels {
			topic, _, _ := strings.Cut(ch, "::")
			if !available(ch) && !available(topic+"::all") {
				covered = false
				break
			}
		}
		if covered {
			h.ready = true
			h.reply <- nil
		}
	}
}

func (o *marketOwner) reconcile() {
	desired := o.desired()
	added, removed := []string{}, []string{}
	for ch := range desired {
		if !o.wire[ch] {
			added = append(added, ch)
		}
	}
	for ch := range o.wire {
		if !desired[ch] {
			removed = append(removed, ch)
		}
	}
	// Add replacement coverage before removing old coverage (all <-> iid).
	req, channels := "sub", added
	if len(channels) == 0 {
		req, channels = "unsub", removed
	}
	if len(channels) == 0 {
		return
	}
	slices.Sort(channels)
	op := &marketOperation{
		id:       o.nextID,
		req:      req,
		channels: channels,
		deadline: time.Now().Add(perpsReconnectTimeout),
	}
	o.nextID++
	o.pending = op
	payload, err := json.Marshal(sessionFrame{ID: op.id, Req: req, Chs: channels})
	if err != nil {
		o.disconnected(err)
		return
	}
	o.write(payload)
}

// A bounded write must not prevent the owner from processing cancellations.
// Only one writer exists per socket, and stopTransport closes its socket and joins
// both IO workers before a new generation is started.
func (o *marketOwner) write(payload []byte) {
	w := &marketWriter{result: make(chan error, 1), done: make(chan struct{})}
	o.writer = w
	conn := o.conn
	go func() {
		defer close(w.done)
		ctx, cancel := context.WithTimeout(o.stream.ctx, perpsReconnectTimeout)
		defer cancel()
		w.result <- conn.Write(ctx, websocket.MessageText, payload)
	}()
}

func (o *marketOwner) payload(payload []byte) error {
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 {
		return nil
	}
	if payload[0] == '[' {
		var batch []json.RawMessage
		if err := json.Unmarshal(payload, &batch); err != nil {
			return err
		}
		for _, message := range batch {
			if err := o.payload(message); err != nil {
				return err
			}
		}
		return nil
	}
	var frame struct {
		ID        int             `json:"id"`
		Channel   string          `json:"ch"`
		Timestamp int64           `json:"ts"`
		Sequence  *int64          `json:"sq"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(payload, &frame); err != nil {
		return err
	}
	if frame.ID != 0 {
		if o.pending == nil || frame.ID != o.pending.id {
			return nil
		}
		if err := marketAck(frame.Data); err != nil {
			return err
		}
		for _, ch := range o.pending.channels {
			if o.pending.req == "sub" {
				o.wire[ch] = true
			} else {
				delete(o.wire, ch)
				delete(o.sequences, ch)
			}
		}
		o.pending = nil
		return nil
	}
	if frame.Channel == "" {
		return nil
	} // heartbeat response
	event := PerpsSessionEvent{Channel: frame.Channel, Timestamp: frame.Timestamp, Data: frame.Data}
	market, err := event.AsMarket()
	if err != nil {
		return fmt.Errorf("perps: decode public market event: %w", err)
	}
	var resync *PerpsSessionResync
	if frame.Sequence != nil {
		event.Sequence = *frame.Sequence
		previous, exists := o.sequences[frame.Channel]
		if exists && *frame.Sequence != previous+1 {
			resync = &PerpsSessionResync{
				Reason: PerpsResyncSequenceGap, Channel: frame.Channel,
				Timestamp: frame.Timestamp, Sequence: *frame.Sequence, PreviousSequence: &previous,
			}
		}
		o.sequences[frame.Channel] = *frame.Sequence
	}
	for h := range o.handles {
		if !h.matches(market) {
			continue
		}
		// Each consumer owns its payload, including nested typed slices/pointers.
		copy := event
		copy.Data = append(json.RawMessage(nil), event.Data...)
		copy.Market, err = copy.AsMarket()
		if err != nil {
			return err
		}
		if resync != nil {
			r := *resync
			p := *resync.PreviousSequence
			r.PreviousSequence = &p
			h.emit(PerpsSessionEvent{Type: "resync", Resync: &r})
		}
		h.emit(copy)
	}
	return nil
}

func (h *MarketHandle) matches(event *MarketEvent) bool {
	channel := string(event.Topic) + "::" + strconv.Itoa(event.InstrumentID)
	if event.Interval != "" {
		channel += "::" + string(event.Interval)
	}
	return slices.Contains(h.channels, channel) ||
		slices.Contains(h.channels, string(event.Topic)+"::all")
}

func marketAck(data json.RawMessage) error {
	var ack sessionAck
	if err := json.Unmarshal(data, &ack); err == nil && ack.Status != "" {
		if ack.Status == "ok" {
			return nil
		}
		return fmt.Errorf("perps: public subscription rejected: %s", ack.Error)
	}
	var acks []sessionAck
	if err := json.Unmarshal(data, &acks); err != nil || len(acks) == 0 {
		return fmt.Errorf("perps: invalid public subscription acknowledgement")
	}
	for _, ack := range acks {
		if ack.Status != "ok" {
			return fmt.Errorf("perps: public subscription rejected: %s", ack.Error)
		}
	}
	return nil
}
