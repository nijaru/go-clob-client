package clob

import (
	"bytes"
	stdjson "encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

// NotificationCommentID preserves a comment identity whether encoded as an
// integer or an opaque string. It is not the account-scoped notification ID.
type NotificationCommentID string

func (id *NotificationCommentID) UnmarshalJSON(raw []byte) error {
	var text string
	if err := stdjson.Unmarshal(raw, &text); err == nil {
		*id = NotificationCommentID(text)
		return nil
	}
	var number stdjson.Number
	if err := stdjson.Unmarshal(raw, &number); err != nil {
		return fmt.Errorf("notification comment id: %w", err)
	}
	if _, ok := new(big.Int).SetString(number.String(), 10); !ok {
		return fmt.Errorf("notification comment id: expected integer")
	}
	*id = NotificationCommentID(number.String())
	return nil
}

func (p *MarketNotificationPayload) UnmarshalJSON(raw []byte) error {
	type plain MarketNotificationPayload
	value := plain{}
	wire := struct {
		*plain
		AcceptingOrdersTimestamp stdjson.RawMessage `json:"accepting_order_timestamp"`
		EndDate                  stdjson.RawMessage `json:"end_date_iso"`
		GameStartTime            stdjson.RawMessage `json:"game_start_time"`
	}{plain: &value}
	if err := stdjson.Unmarshal(raw, &wire); err != nil {
		return err
	}
	for _, date := range []struct {
		raw  stdjson.RawMessage
		dest **time.Time
	}{
		{wire.AcceptingOrdersTimestamp, &value.AcceptingOrdersTimestamp},
		{wire.EndDate, &value.EndDate},
		{wire.GameStartTime, &value.GameStartTime},
	} {
		instant, err := decodeNotificationDate(date.raw)
		if err != nil {
			return err
		}
		*date.dest = instant
	}
	*p = MarketNotificationPayload(value)
	return nil
}

func (p *ChildCommentNotificationPayload) UnmarshalJSON(raw []byte) error {
	type plain ChildCommentNotificationPayload
	value := plain{}
	wire := struct {
		*plain
		CreatedAt stdjson.RawMessage `json:"createdAt"`
	}{plain: &value}
	if err := stdjson.Unmarshal(raw, &wire); err != nil {
		return err
	}
	instant, err := decodeNotificationDate(wire.CreatedAt)
	if err != nil {
		return err
	}
	value.CreatedAt = instant
	*p = ChildCommentNotificationPayload(value)
	return nil
}

// TS DateLikeToIsoDateTimeStringSchema accepts epoch milliseconds or date text.
// Parsing integers directly avoids float64 rounding of financial/event metadata.
func decodeNotificationDate(raw []byte) (*time.Time, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) || bytes.Equal(raw, []byte(`""`)) {
		return nil, nil
	}
	var text string
	if err := stdjson.Unmarshal(raw, &text); err != nil {
		var number stdjson.Number
		if err := stdjson.Unmarshal(raw, &number); err != nil {
			return nil, fmt.Errorf("notification date: %w", err)
		}
		text = number.String()
	}
	if ms, err := strconv.ParseInt(text, 10, 64); err == nil {
		instant := time.UnixMilli(ms).UTC()
		return &instant, nil
	}
	text = strings.Replace(text, " ", "T", 1)
	if strings.HasSuffix(text, "+00") {
		text += ":00"
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05.999999999", time.DateOnly} {
		if instant, err := time.Parse(layout, text); err == nil {
			instant = instant.UTC()
			return &instant, nil
		}
	}
	return nil, fmt.Errorf("notification date: invalid value %q", text)
}
