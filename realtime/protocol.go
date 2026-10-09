package realtime

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	cryptoSymbol = regexp.MustCompile(`^[a-z0-9]+usd$`)
	equitySymbol = regexp.MustCompile(`^[a-zA-Z0-9._:/-]+$`)
	decimal      = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)
)

type priceKey struct {
	channel  Channel
	symbol   string
	provider Provider
}

func requestKeys(r Request) ([]priceKey, error) {
	if !r.Channel.valid() {
		return nil, fmt.Errorf("realtime: invalid channel %q", r.Channel)
	}
	if r.Provider != "" && r.Provider != Chainlink && r.Provider != Pyth {
		return nil, fmt.Errorf("realtime: invalid provider %q", r.Provider)
	}
	if len(r.Symbols) == 0 {
		return nil, fmt.Errorf("realtime: symbols are required")
	}
	for _, t := range r.Types {
		if t != SnapshotEvent && t != UpdateEvent {
			return nil, fmt.Errorf("realtime: invalid event type %q", t)
		}
	}
	seen := make(map[priceKey]bool)
	var keys []priceKey
	for _, symbol := range r.Symbols {
		if r.Channel == Crypto || r.Channel == CryptoTWAP {
			if len(symbol) > 64 || !cryptoSymbol.MatchString(symbol) {
				return nil, fmt.Errorf(
					"realtime: crypto symbol %q must be a canonical lowercase USD pair",
					symbol,
				)
			}
		} else {
			symbol = strings.ToLower(strings.TrimSpace(symbol))
			if len(symbol) == 0 || len(symbol) > 64 || !equitySymbol.MatchString(symbol) {
				return nil, fmt.Errorf("realtime: invalid equity symbol %q", symbol)
			}
		}
		key := priceKey{r.Channel, symbol, r.Provider}
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	return keys, nil
}

type wireFilter struct {
	Symbol        string   `json:"symbol"`
	WindowSeconds int      `json:"window_seconds,omitempty"`
	Provider      Provider `json:"provider,omitempty"`
}
type wireSubscription struct {
	Channel Channel    `json:"channel"`
	Filter  wireFilter `json:"filter"`
}

func (k priceKey) wire() wireSubscription {
	f := wireFilter{Symbol: k.symbol, Provider: k.provider}
	if k.channel.twap() {
		f.WindowSeconds = 60
	}
	return wireSubscription{k.channel, f}
}

type operation struct {
	Op            string             `json:"op"`
	RID           string             `json:"rid,omitempty"`
	Auth          *Credentials       `json:"auth,omitempty"`
	Subscriptions []wireSubscription `json:"subscriptions,omitempty"`
}
type ack struct {
	Op       string  `json:"op"`
	RID      string  `json:"rid"`
	Channel  Channel `json:"channel"`
	Provider *Source `json:"provider"`
	Code     string  `json:"code"`
}

type wirePoint struct {
	Timestamp *int64          `json:"timestamp"`
	Value     json.RawMessage `json:"value"`
	Exact     *string         `json:"full_accuracy_value"`
}

func (p wirePoint) point() (Point, bool) {
	if p.Timestamp == nil || p.Exact == nil || !decimal.MatchString(*p.Exact) {
		return Point{}, false
	}
	var approximate string
	if json.Unmarshal(p.Value, &approximate) != nil {
		approximate = string(p.Value)
	}
	if !decimal.MatchString(approximate) {
		return Point{}, false
	}
	return Point{time.UnixMilli(*p.Timestamp).UTC(), *p.Exact}, true
}

// Malformed and future envelopes are ignored, matching both reference clients.
func parseEvent(data []byte) (Event, bool) {
	var w struct {
		V        *int            `json:"v"`
		Channel  Channel         `json:"channel"`
		Seq      *uint64         `json:"seq"`
		TS       *int64          `json:"ts"`
		Snapshot bool            `json:"snapshot"`
		Dropped  *uint64         `json:"dropped"`
		Payload  json.RawMessage `json:"payload"`
	}
	if json.Unmarshal(data, &w) != nil || w.V == nil || *w.V != 1 || !w.Channel.valid() ||
		w.Seq == nil ||
		w.TS == nil ||
		hasNull(data, "snapshot", "dropped") {
		return Event{}, false
	}
	var p struct {
		wirePoint
		Symbol     *string      `json:"symbol"`
		Source     *Source      `json:"source"`
		Data       *[]wirePoint `json:"data"`
		Window     int          `json:"window_seconds"`
		ReceivedAt *int64       `json:"received_at"`
		Carried    *bool        `json:"is_carried_forward"`
	}
	if json.Unmarshal(w.Payload, &p) != nil || p.Symbol == nil || p.Source == nil ||
		(w.Channel.twap() && p.Window != 60) {
		return Event{}, false
	}
	e := Event{
		Channel:   w.Channel,
		Symbol:    strings.ToLower(*p.Symbol),
		Source:    *p.Source,
		Timestamp: time.UnixMilli(*w.TS).UTC(),
		Sequence:  *w.Seq,
		Dropped:   w.Dropped,
	}
	if w.Channel.twap() {
		e.WindowSeconds = 60
	}
	if w.Snapshot {
		if p.Data == nil {
			return Event{}, false
		}
		points := make([]Point, 0, len(*p.Data))
		for _, raw := range *p.Data {
			point, ok := raw.point()
			if !ok {
				return Event{}, false
			}
			points = append(points, point)
		}
		e.Type = SnapshotEvent
		e.Snapshot = &Snapshot{points}
	} else {
		point, ok := p.wirePoint.point()
		if !ok || hasNull(w.Payload, "received_at", "is_carried_forward") {
			return Event{}, false
		}
		e.Type = UpdateEvent
		e.Update = &Update{Point: point}
		if !w.Channel.twap() {
			e.Update.IsCarriedForward = p.Carried
			if p.ReceivedAt != nil {
				t := time.UnixMilli(*p.ReceivedAt).UTC()
				e.Update.ReceivedAt = &t
			}
		}
	}
	return e, true
}

func hasNull(data []byte, fields ...string) bool {
	var obj map[string]json.RawMessage
	if json.Unmarshal(data, &obj) != nil {
		return true
	}
	for _, field := range fields {
		if string(obj[field]) == "null" {
			return true
		}
	}
	return false
}

// Joining a shared filter gets current two-minute history, not a stale initial
// snapshot. Copy before delivery so subscribers cannot mutate one another's data.
func refreshSnapshot(previous *Event, event Event) Event {
	if event.Snapshot != nil {
		return event
	}
	history := []Point(nil)
	if previous != nil && previous.Source == event.Source {
		history = previous.Snapshot.Data
	}
	point := event.Update.Point
	if len(history) > 0 && history[len(history)-1].Timestamp.After(point.Timestamp) {
		return *previous
	}
	points := make([]Point, 0, len(history)+1)
	for _, p := range history {
		if p.Timestamp.After(point.Timestamp.Add(-2*time.Minute)) &&
			p.Timestamp.Before(point.Timestamp) {
			points = append(points, p)
		}
	}
	points = append(points, point)
	event.Type = SnapshotEvent
	event.Update = nil
	event.Snapshot = &Snapshot{points}
	return event
}

func cloneEvent(e Event) Event {
	if e.Snapshot != nil {
		e.Snapshot = &Snapshot{append([]Point(nil), e.Snapshot.Data...)}
	}
	if e.Update != nil {
		u := *e.Update
		if u.ReceivedAt != nil {
			v := *u.ReceivedAt
			u.ReceivedAt = &v
		}
		if u.IsCarriedForward != nil {
			v := *u.IsCarriedForward
			u.IsCarriedForward = &v
		}
		e.Update = &u
	}
	if e.Dropped != nil {
		v := *e.Dropped
		e.Dropped = &v
	}
	if e.Confirmation != nil {
		c := *e.Confirmation
		if c.Provider != nil {
			v := *c.Provider
			c.Provider = &v
		}
		e.Confirmation = &c
	}
	return e
}
