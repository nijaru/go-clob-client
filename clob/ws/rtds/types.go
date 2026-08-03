package rtds

import (
	stdjson "encoding/json" //nolint:depguard // numeric JSON normalization for RTDS payloads
	"fmt"
	"math/big"
	"strings"
	"time"

	json "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
)

// Action represents an RTDS subscription action.
type Action string

const (
	ActionSubscribe   Action = "subscribe"
	ActionUnsubscribe Action = "unsubscribe"
)

// SubscriptionRequest is the top-level message sent to RTDS.
type SubscriptionRequest struct {
	Action        Action         `json:"action"`
	Subscriptions []Subscription `json:"subscriptions"`
}

// Credentials matches the structure required for CLOB authentication.
type Credentials struct {
	Key        string `json:"key"`
	Secret     string `json:"secret"`
	Passphrase string `json:"passphrase"`
}

// Subscription represents an individual topic subscription.
type Subscription struct {
	Topic    string       `json:"topic"`
	Type     string       `json:"type"`
	Filters  any          `json:"filters,omitempty"`
	CLOBAuth *Credentials `json:"clob_auth,omitempty"`
}

// MarshalJSON implements custom serialization for Subscription to handle
// topic-specific filter formatting (escaped string vs raw JSON).
func (s Subscription) MarshalJSON() ([]byte, error) {
	type Alias Subscription
	aux := struct {
		Alias
	}{
		Alias: Alias(s),
	}

	// Chainlink requires filters as an escaped JSON string.
	// Other topics (like Binance crypto_prices) expect raw JSON.
	if s.Topic == "crypto_prices_chainlink" && s.Filters != nil {
		filtersJSON, err := json.Marshal(s.Filters)
		if err != nil {
			return nil, err
		}
		aux.Filters = string(filtersJSON)
	}

	return json.Marshal(aux)
}

// RtdsMessage is the top-level message received from RTDS.
type RtdsMessage struct {
	Topic     string         `json:"topic"`
	Type      string         `json:"type"`
	Timestamp int64          `json:"timestamp"`
	Payload   jsontext.Value `json:"payload"`
}

// AsCryptoPrice attempts to unmarshal the payload as a CryptoPrice.
func (m *RtdsMessage) AsCryptoPrice() (*CryptoPrice, error) {
	if m.Topic != "crypto_prices" {
		return nil, fmt.Errorf("message topic is %s, not crypto_prices", m.Topic)
	}
	var p CryptoPrice
	if err := json.Unmarshal(m.Payload, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// AsChainlinkPrice attempts to unmarshal the payload as a ChainlinkPrice.
func (m *RtdsMessage) AsChainlinkPrice() (*ChainlinkPrice, error) {
	if m.Topic != "crypto_prices_chainlink" {
		return nil, fmt.Errorf("message topic is %s, not crypto_prices_chainlink", m.Topic)
	}
	var p ChainlinkPrice
	if err := json.Unmarshal(m.Payload, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// AsChainlinkTWAPPrice attempts to unmarshal a Chainlink TWAP payload.
// The exact signed E18 value is preferred over the rounded display value.
func (m *RtdsMessage) AsChainlinkTWAPPrice() (*ChainlinkTWAPPrice, error) {
	window, expected, err := chainlinkTWAPMessageWindow(m.Topic)
	if err != nil {
		return nil, err
	}
	if m.Type != "update" {
		return nil, fmt.Errorf("message type is %s, not update", m.Type)
	}

	var wire struct {
		Symbol            string             `json:"symbol"`
		Value             stdjson.RawMessage `json:"value"`
		FullAccuracyValue stdjson.RawMessage `json:"full_accuracy_value"`
		Timestamp         int64              `json:"timestamp"`
		WindowSeconds     int                `json:"window_s"`
	}
	if err := stdjson.Unmarshal(m.Payload, &wire); err != nil {
		return nil, fmt.Errorf("decode Chainlink TWAP payload: %w", err)
	}
	displayValue, err := decodeDecimalValue(wire.Value)
	if err != nil || displayValue == "" {
		if err == nil {
			err = fmt.Errorf("value is empty")
		}
		return nil, fmt.Errorf("Chainlink TWAP display value: %w", err)
	}
	if len(wire.FullAccuracyValue) == 0 {
		return nil, fmt.Errorf("Chainlink TWAP full_accuracy_value is required")
	}
	var fullAccuracyValue string
	if err := stdjson.Unmarshal(wire.FullAccuracyValue, &fullAccuracyValue); err != nil {
		return nil, fmt.Errorf("Chainlink TWAP full_accuracy_value: expected signed integer string: %w", err)
	}
	value, err := chainlinkE18ToDecimalString(fullAccuracyValue)
	if err != nil {
		return nil, fmt.Errorf("Chainlink TWAP full_accuracy_value: %w", err)
	}
	if wire.WindowSeconds != int(window) {
		return nil, fmt.Errorf(
			"Chainlink TWAP topic %q requires window_s=%d, got %d",
			m.Topic,
			expected,
			wire.WindowSeconds,
		)
	}
	return &ChainlinkTWAPPrice{
		Symbol:        wire.Symbol,
		Timestamp:     wire.Timestamp,
		Value:         value,
		WindowSeconds: window,
	}, nil
}

// AsComment attempts to unmarshal the payload as a Comment.
func (m *RtdsMessage) AsComment() (*Comment, error) {
	if m.Topic != "comments" {
		return nil, fmt.Errorf("message topic is %s, not comments", m.Topic)
	}
	var p Comment
	if err := json.Unmarshal(m.Payload, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// CryptoPrice represents a Binance crypto price update. Value preserves the
// exact decimal spelling while accepting the numeric JSON emitted by RTDS.
type CryptoPrice struct {
	Symbol    string `json:"symbol"`
	Timestamp int64  `json:"timestamp"`
	Value     string `json:"value"`
}

// UnmarshalJSON accepts both numeric and quoted decimal values from RTDS.
func (p *CryptoPrice) UnmarshalJSON(data []byte) error {
	var wire struct {
		Symbol    string             `json:"symbol"`
		Timestamp int64              `json:"timestamp"`
		Value     stdjson.RawMessage `json:"value"`
	}
	if err := stdjson.Unmarshal(data, &wire); err != nil {
		return err
	}
	value, err := decodeDecimalValue(wire.Value)
	if err != nil {
		return fmt.Errorf("crypto price value: %w", err)
	}
	*p = CryptoPrice{Symbol: wire.Symbol, Timestamp: wire.Timestamp, Value: value}
	return nil
}

// ChainlinkTWAPWindowSeconds identifies a supported Chainlink TWAP averaging window.
type ChainlinkTWAPWindowSeconds int

const (
	ChainlinkTWAP30Seconds ChainlinkTWAPWindowSeconds = 30
	ChainlinkTWAP60Seconds ChainlinkTWAPWindowSeconds = 60

	// Short aliases for callers that prefer window-oriented names.
	ChainlinkTWAPWindow30 = ChainlinkTWAP30Seconds
	ChainlinkTWAPWindow60 = ChainlinkTWAP60Seconds
)

// ChainlinkTWAPPrice represents a Chainlink time-weighted average price update.
// Value is normalized from the signed E18 full_accuracy_value field.
type ChainlinkTWAPPrice struct {
	Symbol        string                     `json:"symbol"`
	Timestamp     int64                      `json:"timestamp"`
	Value         string                     `json:"value"`
	WindowSeconds ChainlinkTWAPWindowSeconds `json:"window_s"`
}

func chainlinkTWAPMessageWindow(topic string) (ChainlinkTWAPWindowSeconds, int, error) {
	switch topic {
	case "crypto_prices_twap_thirty":
		return ChainlinkTWAP30Seconds, int(ChainlinkTWAP30Seconds), nil
	case "crypto_prices_twap_sixty":
		return ChainlinkTWAP60Seconds, int(ChainlinkTWAP60Seconds), nil
	case "prices.crypto.chainlink.twap":
		return 0, 0, fmt.Errorf("logical Chainlink TWAP topic requires a raw window topic")
	default:
		return 0, 0, fmt.Errorf("message topic is %s, not a Chainlink TWAP topic", topic)
	}
}

func chainlinkE18ToDecimalString(value string) (string, error) {
	if value == "" || (value[0] != '-' && (value[0] < '0' || value[0] > '9')) {
		return "", fmt.Errorf("expected signed integer string")
	}
	for i, r := range value {
		if i == 0 && r == '-' {
			if len(value) == 1 {
				return "", fmt.Errorf("expected signed integer string")
			}
			continue
		}
		if r < '0' || r > '9' {
			return "", fmt.Errorf("expected signed integer string")
		}
	}

	n := new(big.Int)
	if _, ok := n.SetString(value, 10); !ok {
		return "", fmt.Errorf("expected signed integer string")
	}
	negative := n.Sign() < 0
	n.Abs(n)
	digits := n.String()
	const scale = 18
	whole := "0"
	fraction := ""
	if len(digits) > scale {
		whole = digits[:len(digits)-scale]
		fraction = digits[len(digits)-scale:]
	} else {
		fraction = strings.Repeat("0", scale-len(digits)) + digits
	}
	fraction = strings.TrimRight(fraction, "0")
	result := whole
	if fraction != "" {
		result += "." + fraction
	}
	if negative && result != "0" {
		result = "-" + result
	}
	return result, nil
}

// ChainlinkPrice represents a Chainlink price feed update. Value preserves
// the exact decimal spelling while accepting the numeric JSON emitted by RTDS.
type ChainlinkPrice struct {
	Symbol    string `json:"symbol"`
	Timestamp int64  `json:"timestamp"`
	Value     string `json:"value"`
}

// UnmarshalJSON accepts both numeric and quoted decimal values from RTDS.
func (p *ChainlinkPrice) UnmarshalJSON(data []byte) error {
	var wire struct {
		Symbol    string             `json:"symbol"`
		Timestamp int64              `json:"timestamp"`
		Value     stdjson.RawMessage `json:"value"`
	}
	if err := stdjson.Unmarshal(data, &wire); err != nil {
		return err
	}
	value, err := decodeDecimalValue(wire.Value)
	if err != nil {
		return fmt.Errorf("chainlink price value: %w", err)
	}
	*p = ChainlinkPrice{Symbol: wire.Symbol, Timestamp: wire.Timestamp, Value: value}
	return nil
}

func decodeDecimalValue(raw stdjson.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("missing value")
	}
	var value string
	if err := stdjson.Unmarshal(raw, &value); err == nil {
		return value, nil
	}
	var number stdjson.Number
	if err := stdjson.Unmarshal(raw, &number); err != nil || number.String() == "" {
		if err == nil {
			err = fmt.Errorf("empty number")
		}
		return "", fmt.Errorf("expected decimal string or number: %w", err)
	}
	return number.String(), nil
}

// Comment represents a comment event payload.
type Comment struct {
	ID               string         `json:"id"`
	Body             string         `json:"body"`
	CreatedAt        time.Time      `json:"createdAt"`
	ParentCommentID  *string        `json:"parentCommentID,omitzero"`
	ParentEntityID   int64          `json:"parentEntityID"`
	ParentEntityType string         `json:"parentEntityType"`
	Profile          CommentProfile `json:"profile"`
	ReactionCount    int64          `json:"reactionCount"`
	ReplyAddress     *string        `json:"replyAddress,omitzero"`
	ReportCount      int64          `json:"reportCount"`
	UserAddress      string         `json:"userAddress"`
}

// CommentProfile contains author information for a comment.
type CommentProfile struct {
	BaseAddress           string  `json:"baseAddress"`
	DisplayUsernamePublic bool    `json:"displayUsernamePublic"`
	Name                  string  `json:"name"`
	ProxyWallet           *string `json:"proxyWallet,omitzero"`
	Pseudonym             *string `json:"pseudonym,omitzero"`
}

// CommentType defines the types of comment events.
type CommentType string

const (
	CommentCreated  CommentType = "comment_created"
	CommentRemoved  CommentType = "comment_removed"
	ReactionCreated CommentType = "reaction_created"
	ReactionRemoved CommentType = "reaction_removed"
)
