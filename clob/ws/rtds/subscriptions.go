package rtds

import (
	"bytes"
	stdjson "encoding/json"
	"fmt"
	"reflect"
	"slices"

	json "github.com/go-json-experiment/json"
)

// A registration is the sole owner of a local interest. Server state is derived
// from these interests, never maintained as a second reference-count registry.
type registration struct {
	sub      Subscription
	identity string
	matches  func(*RtdsMessage) bool
}

func newRegistration(sub Subscription) (registration, error) {
	if sub.Topic == "" || sub.Type == "" {
		return registration{}, fmt.Errorf("rtds: topic and type are required (use * for all types)")
	}
	// Snapshot caller-owned filters and credentials. In particular, changing a
	// caller's map after Subscribe must not change matching or replay state.
	type plain Subscription
	data, err := json.Marshal(plain(sub), json.Deterministic(true))
	if err != nil {
		return registration{}, fmt.Errorf("rtds: subscription: %w", err)
	}
	decoder := stdjson.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var snapshot plain
	if err := decoder.Decode(&snapshot); err != nil {
		return registration{}, fmt.Errorf("rtds: subscription: %w", err)
	}
	sub = Subscription(snapshot)
	var symbols []string
	var fields map[string]any
	switch filters := sub.Filters.(type) {
	case nil:
	case []any:
		for _, value := range filters {
			symbol, ok := value.(string)
			if !ok {
				return registration{}, fmt.Errorf("rtds: symbol filters must contain only strings")
			}
			symbols = append(symbols, symbol)
		}
	case map[string]any:
		fields = filters
	default:
		return registration{}, fmt.Errorf("rtds: filters must be a symbol array or payload-field object")
	}
	return registration{
		sub:      sub,
		identity: string(data),
		matches: func(m *RtdsMessage) bool {
			if m.Topic != sub.Topic || (sub.Type != "*" && m.Type != sub.Type) {
				return false
			}
			if len(symbols) == 0 && len(fields) == 0 {
				return true
			}
			var payload map[string]any
			decoder := stdjson.NewDecoder(bytes.NewReader(m.Payload))
			decoder.UseNumber()
			if err := decoder.Decode(&payload); err != nil {
				return false
			}
			if len(symbols) > 0 {
				symbol, ok := payload["symbol"].(string)
				return ok && slices.Contains(symbols, symbol)
			}
			for key, want := range fields {
				got, ok := payload[key]
				if !ok || !reflect.DeepEqual(got, want) {
					return false
				}
			}
			return true
		},
	}, nil
}

func sameCredentials(a, b *Credentials) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func serverSubscriptions(entries []registration) []Subscription {
	var subs []Subscription
	for _, entry := range entries {
		sub := entry.sub
		if slices.ContainsFunc(subs, func(existing Subscription) bool {
			return existing.Topic == sub.Topic && existing.Type == sub.Type
		}) {
			continue
		}
		sub.Filters = nil
		subs = append(subs, sub)
	}
	return subs
}

func serverDifference(left, right []Subscription) []Subscription {
	var delta []Subscription
	for _, sub := range left {
		if !slices.ContainsFunc(right, func(other Subscription) bool {
			return other.Topic == sub.Topic && other.Type == sub.Type
		}) {
			delta = append(delta, sub)
		}
	}
	return delta
}
