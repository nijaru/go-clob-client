package rtds

import (
	stdjson "encoding/json" //nolint:depguard // numeric JSON normalization for RTDS payloads
	"fmt"
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
