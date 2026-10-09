// Package realtime streams authenticated Polybolt cryptocurrency and equity prices.
// Unlike the public legacy RTDS feed, the live Polybolt endpoint requires CLOB
// API credentials. Prices are exact decimal strings in the symbol's quote currency.
package realtime

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

const DefaultURL = "wss://ws-live-v2.polymarket.com/ws"

type Channel string

const (
	Crypto     Channel = "price.crypto"
	CryptoTWAP Channel = "price.crypto.twap"
	Equity     Channel = "price.equity"
	EquityTWAP Channel = "price.equity.twap"
)

func (c Channel) twap() bool { return c == CryptoTWAP || c == EquityTWAP }
func (c Channel) valid() bool {
	return c == Crypto || c == CryptoTWAP || c == Equity || c == EquityTWAP
}

// Provider is a requested pin; the server can fall back to another source.
type Provider string

const (
	Chainlink Provider = "chainlink"
	Pyth      Provider = "pyth"
)

// Source is actual event provenance, not a requested provider. Unknown values
// are preserved for forward compatibility.
type Source string

const (
	SourceChainlink Source = "chainlink"
	SourcePyth      Source = "pyth"
	SourceMassive   Source = "massive"
)

type Credentials struct {
	APIKey     string `json:"apiKey"`
	Secret     string `json:"secret"`
	Passphrase string `json:"passphrase"`
}

// Config's zero durations use protocol defaults. HTTPClient and Headers apply
// only to the WebSocket upgrade; there is no HTTP streaming transport.
type Config struct {
	URL               string
	Credentials       Credentials
	HTTPClient        *http.Client
	Headers           http.Header
	BufferSize        int           // Per subscription; default 1024. Overflow terminates that subscription.
	ConnectTimeout    time.Duration // Default 10 seconds.
	AckTimeout        time.Duration // Default 10 seconds.
	AcceptanceTimeout time.Duration // Default 30 seconds.
	PingInterval      time.Duration // Application {"op":"ping"}; default 30 seconds.
	StaleTimeout      time.Duration // Any inbound frame proves liveness; default 90 seconds.
	ReconnectMin      time.Duration // Full-jitter exponential backoff; default 1 second.
	ReconnectMax      time.Duration // Default 30 seconds; close code 4003 uses a 10-second ceiling.
}

// Request subscribes to explicit symbols. Crypto symbols must be canonical
// lowercase USD pairs (btcusd, not btc/usd or btcusdt). Equity symbols are
// case-insensitive. TWAP channels have a fixed 60-second window.
type Request struct {
	Channel  Channel
	Symbols  []string
	Provider Provider
	// Types filters price events locally. Empty means snapshots and updates;
	// acceptance confirmations are always delivered.
	Types []EventType
}

type EventType string

const (
	Accepted      EventType = "accepted"
	SnapshotEvent EventType = "snapshot"
	UpdateEvent   EventType = "update"
)

// Point retains the exact full_accuracy_value, never the rounded wire value.
type Point struct {
	Timestamp time.Time
	Value     string
}
type Update struct {
	Point
	ReceivedAt       *time.Time
	IsCarriedForward *bool
}
type Snapshot struct{ Data []Point }

// Confirmation records the actual provider acknowledged by the server. Nil
// means selection was not reported, never inferred from Request.Provider.
type Confirmation struct{ Provider *Source }

// Event has exactly one of Confirmation, Snapshot, or Update. WindowSeconds is
// 60 for TWAP and zero otherwise. Sequence is scoped to a channel on a single
// connection and resets on reconnect; multi-socket subscriptions can interleave.
// ConnectionID identifies the socket generation within this client. Timestamp,
// Sequence, Dropped and Source are price-event metadata; acceptance events
// carry only Channel, Symbol, ConnectionID, WindowSeconds and Confirmation.
type Event struct {
	Type          EventType
	Channel       Channel
	Symbol        string
	Source        Source
	Timestamp     time.Time
	Sequence      uint64
	Dropped       *uint64
	ConnectionID  uint64
	WindowSeconds int
	Confirmation  *Confirmation
	Snapshot      *Snapshot
	Update        *Update
}

var (
	ErrClosed       = errors.New("realtime: closed")
	ErrSlowConsumer = errors.New("realtime: subscription buffer full")
)

// RejectionError preserves known and future server error codes.
type RejectionError struct {
	Code      string
	Channel   Channel
	RequestID string
}

func (e *RejectionError) Error() string {
	return fmt.Sprintf("realtime: request %s rejected (%s, %s)", e.RequestID, e.Channel, e.Code)
}

// ConnectError preserves a failed WebSocket upgrade's HTTP status (zero when
// no response was received) and its underlying transport error.
type ConnectError struct {
	StatusCode int
	Err        error
}

func (e *ConnectError) Error() string {
	return fmt.Sprintf("realtime: connect (HTTP %d): %v", e.StatusCode, e.Err)
}
func (e *ConnectError) Unwrap() error { return e.Err }

// ConnectionError preserves a terminal WebSocket close code and reason.
type ConnectionError struct {
	Code   int
	Reason string
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("realtime: connection closed (%d): %s", e.Code, e.Reason)
}

// TimeoutError identifies a transport or acceptance deadline.
type TimeoutError struct{ Operation string }

func (e *TimeoutError) Error() string { return "realtime: " + e.Operation + " timed out" }
