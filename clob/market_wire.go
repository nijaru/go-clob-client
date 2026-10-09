package clob

import (
	stdjson "encoding/json" //nolint:depguard // lossless REST number decoding
	"fmt"
)

func (r *MidpointResponse) UnmarshalJSON(data []byte) error {
	type alias MidpointResponse
	return unmarshalResponseStrings(data, (*alias)(r), "mid")
}

func (r *PriceResponse) UnmarshalJSON(data []byte) error {
	type alias PriceResponse
	return unmarshalResponseStrings(data, (*alias)(r), "price")
}

func (r *SpreadResponse) UnmarshalJSON(data []byte) error {
	type alias SpreadResponse
	return unmarshalResponseStrings(data, (*alias)(r), "spread")
}

func (r *LastTradePriceResponse) UnmarshalJSON(data []byte) error {
	type alias LastTradePriceResponse
	return unmarshalResponseStrings(data, (*alias)(r), "price")
}

func decodeResponseMap(data []byte) (map[string]string, error) {
	var fields map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, nil
	}
	out := make(map[string]string, len(fields))
	for key, raw := range fields {
		value, err := decodeStringOrNumber(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		out[key] = value
	}
	return out, nil
}

func (r *MidpointsResponse) UnmarshalJSON(data []byte) error {
	out, err := decodeResponseMap(data)
	if err == nil {
		*r = out
	}
	return err
}

func (r *PricesResponse) UnmarshalJSON(data []byte) error {
	var fields map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(data, &fields); err != nil {
		return err
	}
	if fields == nil {
		*r = nil
		return nil
	}
	out := make(PricesResponse, len(fields))
	for key, raw := range fields {
		values, err := decodeResponseMap(raw)
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		if values == nil {
			out[key] = nil
			continue
		}
		out[key] = make(map[Side]string, len(values))
		for side, value := range values {
			out[key][Side(side)] = value
		}
	}
	*r = out
	return nil
}

// UnmarshalJSON accepts the TS flat map and Rust's nullable spreads envelope.
func (r *SpreadsResponse) UnmarshalJSON(data []byte) error {
	var fields map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(data, &fields); err != nil {
		return err
	}
	if raw, ok := fields["spreads"]; ok {
		data = raw
	}
	out, err := decodeResponseMap(data)
	if err == nil {
		*r = out
	}
	return err
}
