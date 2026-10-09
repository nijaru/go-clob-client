package gamma

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Timestamp is a Gamma timestamp normalized to UTC RFC3339 with nanosecond
// precision. Its empty value represents absent, null or empty wire timestamps.
// Time converts a present timestamp to time.Time.
type Timestamp string

func (t *Timestamp) UnmarshalJSON(data []byte) error {
	var text *string
	if err := json.Unmarshal(data, &text); err != nil {
		return fmt.Errorf("gamma timestamp: %w", err)
	}
	if text == nil || *text == "" {
		*t = ""
		return nil
	}
	instant, err := parseTimestamp(*text)
	if err != nil {
		return err
	}
	*t = Timestamp(instant.UTC().Format(time.RFC3339Nano))
	return nil
}

func (t Timestamp) Time() (time.Time, error) {
	if t == "" {
		return time.Time{}, fmt.Errorf("gamma timestamp: value is absent")
	}
	return parseTimestamp(string(t))
}

func parseTimestamp(text string) (time.Time, error) {
	// Python models/gamma/common.py::parse_optional_datetime accepts Gamma's
	// PostgreSQL space separator and reduced +00 offset as well as ISO dates.
	text = strings.Replace(text, " ", "T", 1)
	// Normalize ISO offsets written as +HH or +HHMM. Offsets belong to a
	// datetime, not to the '-' separators in a calendar date.
	if split := strings.IndexByte(text, 'T'); split >= 0 {
		for i := len(text) - 1; i > split; i-- {
			if text[i] != '+' && text[i] != '-' {
				continue
			}
			switch len(text) - i {
			case 3:
				text += ":00"
			case 5:
				text = text[:i+3] + ":" + text[i+3:]
			}
			break
		}
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		"2006-01-02T15:04Z07:00",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04",
		time.DateOnly,
	} {
		if value, err := time.Parse(layout, text); err == nil {
			return value, nil
		}
	}
	return time.Time{}, fmt.Errorf("gamma timestamp: invalid datetime %q", text)
}
