// Package sports streams public live game results from Polymarket's dedicated
// sports WebSocket. Connecting subscribes to all games; there is no authentication,
// subscription frame, acknowledgement, or server-side filter.
package sports

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const DefaultURL = "wss://sports-api.polymarket.com/ws"

// Config is the connection request. Zero values select the documented defaults.
// Headers apply only to the HTTP upgrade; no credentials are required.
// The caller owns HTTPClient and must not mutate it while a stream is active.
type Config struct {
	URL             string
	HTTPClient      *http.Client
	Headers         http.Header
	BufferSize      int           // Default 1024 events; overflow terminates the stream.
	MaxMessageBytes int64         // Default 1 MiB; bounds each incoming message.
	ConnectTimeout  time.Duration // Default 10 seconds, including each reconnect attempt.
	WriteTimeout    time.Duration // Default 5 seconds for a heartbeat response.
	StaleTimeout    time.Duration // Default 30 seconds without a server text ping.
	ReconnectMin    time.Duration // Full-jitter exponential backoff base; default 250 ms.
	ReconnectMax    time.Duration // Backoff ceiling; default 30 seconds.
}

// GameResult retains the flat wire names and values. Status, score, period,
// elapsed and turn are opaque strings, not enums or parsed scores.
// Optional string pointers distinguish a value (including empty) from null/absent.
// FinishedTimestamp and FinishedTimestampSnake retain both timestamp spellings,
// independently, as raw JSON: upstream accepts numbers and strings. No timestamp
// unit inference, alias coalescing, or score normalization is performed.
type GameResult struct {
	GameID                 int64           `json:"gameId"`
	SportradarGameID       *string         `json:"sportradarGameId,omitempty"`
	Slug                   *string         `json:"slug,omitempty"`
	LeagueAbbreviation     string          `json:"leagueAbbreviation"`
	HomeTeam               *string         `json:"homeTeam,omitempty"`
	AwayTeam               *string         `json:"awayTeam,omitempty"`
	Status                 string          `json:"status"`
	Live                   bool            `json:"live"`
	Ended                  bool            `json:"ended"`
	Score                  string          `json:"score"`
	Period                 *string         `json:"period,omitempty"`
	Elapsed                *string         `json:"elapsed,omitempty"`
	FinishedTimestamp      json.RawMessage `json:"finishedTimestamp,omitempty"`
	FinishedTimestampSnake json.RawMessage `json:"finished_timestamp,omitempty"`
	Turn                   *string         `json:"turn,omitempty"`
}

// Event is a game result, not the synthetic topic/type envelope used by the
// unified SDKs. Raw owns a copy of the complete incoming JSON, including unknown
// metadata and null/absent distinctions. The consumer may retain or mutate it.
type Event struct {
	GameResult
	Raw json.RawMessage `json:"-"`
}

var (
	ErrSlowConsumer = errors.New("sports: event buffer full")
	ErrStale        = errors.New("sports: server ping overdue")
	// errClosed distinguishes explicit Close from parent-context cancellation.
	errClosed = errors.New("sports: closed")
)

// ConnectError preserves a failed upgrade's HTTP status (zero if unavailable)
// and the underlying error, including context cancellation or timeout.
type ConnectError struct {
	StatusCode int
	Err        error
}

func (e *ConnectError) Error() string {
	return fmt.Sprintf("sports: connect (HTTP %d): %v", e.StatusCode, e.Err)
}
func (e *ConnectError) Unwrap() error { return e.Err }

// Stats reports malformed frames skipped and successful replacement connections.
// LastConnectionError is the most recent transport/heartbeat failure; it remains
// available after recovery. These are local diagnostics, not server metadata.
type Stats struct {
	MalformedMessages   uint64
	Reconnects          uint64
	LastConnectionError error
}
