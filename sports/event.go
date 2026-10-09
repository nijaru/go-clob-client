package sports

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ParseEvent decodes one flat game-result object. Malformed or unrelated frames
// are rejected; unknown fields are preserved in Raw. The stream skips rejected
// frames and increments Stats.MalformedMessages, as the reference SDKs do.
func ParseEvent(data []byte) (Event, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return Event{}, fmt.Errorf("sports: decode event: %w", err)
	}
	for _, key := range []string{"gameId", "leagueAbbreviation", "status", "live", "ended", "score"} {
		value, ok := fields[key]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return Event{}, fmt.Errorf("sports: missing required field %s", key)
		}
	}
	var result GameResult
	if err := json.Unmarshal(data, &result); err != nil {
		return Event{}, fmt.Errorf("sports: decode game result: %w", err)
	}
	// Keep the timestamp representation opaque, but reject shapes not present in
	// either upstream schema. Strings are not interpreted or rewritten here.
	for _, key := range []string{"finishedTimestamp", "finished_timestamp"} {
		value := bytes.TrimSpace(fields[key])
		if len(value) == 0 || bytes.Equal(value, []byte("null")) || value[0] == '"' {
			continue
		}
		var number json.Number
		if err := json.Unmarshal(value, &number); err != nil {
			return Event{}, fmt.Errorf("sports: invalid timestamp %s: %w", key, err)
		}
	}
	return Event{GameResult: result, Raw: bytes.Clone(data)}, nil
}
