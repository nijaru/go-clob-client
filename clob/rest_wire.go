package clob

import (
	stdjson "encoding/json" //nolint:depguard // retain response number lexemes
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Response normalization never routes JSON numbers through float64. Ordinary
// string fields retain their wire lexeme; numeric asset IDs must be integers.
func responseFields(data []byte) (map[string]stdjson.RawMessage, error) {
	var fields map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, fmt.Errorf("expected response object")
	}
	return fields, nil
}

func decodeStringOrNumber(raw stdjson.RawMessage) (string, error) {
	if isEmptyJSON(raw) {
		return "", nil
	}
	var text string
	if err := stdjson.Unmarshal(raw, &text); err == nil {
		return text, nil
	}
	var number stdjson.Number
	if err := stdjson.Unmarshal(raw, &number); err != nil {
		return "", fmt.Errorf("expected string or number: %w", err)
	}
	if number.String() == "" {
		return "", fmt.Errorf("empty number")
	}
	return number.String(), nil
}

func normalizeResponseStrings(fields map[string]stdjson.RawMessage, keys ...string) error {
	for _, key := range keys {
		if raw, ok := fields[key]; ok && !isEmptyJSON(raw) {
			value, err := decodeStringOrNumber(raw)
			if err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
			fields[key] = mustJSONText(value)
		}
	}
	return nil
}

func decodeAssetID(raw stdjson.RawMessage) (string, error) {
	value, err := decodeStringOrNumber(raw)
	if err != nil || isEmptyJSON(raw) {
		return value, err
	}
	if strings.HasPrefix(strings.TrimSpace(string(raw)), "\"") {
		return value, nil // native position identifiers are opaque strings
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return "", fmt.Errorf("numeric asset ID must be an unsigned integer")
		}
	}
	return value, nil
}

// The current spelling takes precedence unless null or empty, matching the
// established asset-ID fallback contract.
func aliasResponseField(fields map[string]stdjson.RawMessage, key string, aliases ...string) {
	if !isEmptyJSON(fields[key]) && string(fields[key]) != `""` {
		return
	}
	for _, alias := range aliases {
		if raw := fields[alias]; !isEmptyJSON(raw) {
			fields[key] = raw
			return
		}
	}
}

func normalizeResponseAsset(
	fields map[string]stdjson.RawMessage,
	key string,
	aliases ...string,
) error {
	aliasResponseField(fields, key, aliases...)
	if raw, ok := fields[key]; ok {
		value, err := decodeAssetID(raw)
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		fields[key] = mustJSONText(value)
	}
	return nil
}

func decodeResponseFields(fields map[string]stdjson.RawMessage, target any) error {
	data, err := stdjson.Marshal(fields)
	if err != nil {
		return err
	}
	return stdjson.Unmarshal(data, target)
}

func unmarshalResponseStrings(data []byte, target any, keys ...string) error {
	fields, err := responseFields(data)
	if err != nil {
		return err
	}
	if err := normalizeResponseStrings(fields, keys...); err != nil {
		return err
	}
	return decodeResponseFields(fields, target)
}

// Numeric reward dates and builder key dates are explicitly milliseconds;
// trade epochs accept seconds or milliseconds. Calendar strings remain dates.
func normalizeResponseDates(
	fields map[string]stdjson.RawMessage,
	milliseconds bool,
	keys ...string,
) error {
	for _, key := range keys {
		raw, ok := fields[key]
		if !ok || isEmptyJSON(raw) {
			continue
		}
		text, err := decodeStringOrNumber(raw)
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		if text == "" {
			continue
		}
		if epoch, err := strconv.ParseInt(text, 10, 64); err == nil {
			var moment time.Time
			if milliseconds {
				moment = time.UnixMilli(epoch).UTC()
			} else {
				moment, _ = parseUnixMoment(text)
			}
			if moment.Year() < 0 || moment.Year() > 9999 {
				return fmt.Errorf("%s: timestamp outside RFC3339 range", key)
			}
			fields[key] = mustJSONText(moment.Format(time.RFC3339Nano))
			continue
		}
		if _, err := parseDateMoment(text); err != nil {
			return fmt.Errorf("%s: expected epoch, calendar date or RFC3339: %w", key, err)
		}
		fields[key] = mustJSONText(text)
	}
	return nil
}

func parseDateMoment(text string) (time.Time, error) {
	if len(text) == len(time.DateOnly) {
		return time.Parse(time.DateOnly, text)
	}
	return time.Parse(time.RFC3339Nano, text)
}

func unmarshalResponseDates(
	data []byte,
	target any,
	strings []string,
	milliseconds bool,
	dates ...string,
) error {
	fields, err := responseFields(data)
	if err != nil {
		return err
	}
	if err := normalizeResponseStrings(fields, strings...); err != nil {
		return err
	}
	if err := normalizeResponseDates(fields, milliseconds, dates...); err != nil {
		return err
	}
	return decodeResponseFields(fields, target)
}
