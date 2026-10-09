package data

import (
	"encoding/hex"
	"fmt"
	"math"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyjson"
)

type conditionGrammar int

const (
	feedConditions conditionGrammar = iota
	marketConditions
	comboConditions
)

func input(field, message string) error { return &InputError{Field: field, Message: message} }

func requireUser(q url.Values, user string) error {
	if strings.TrimSpace(user) == "" {
		return input("user", "is required")
	}
	q.Set("user", user)
	return nil
}

func optionalUser(q url.Values, user string) error {
	if user == "" {
		return nil
	}
	return requireUser(q, user)
}

func conditionIDs(q url.Values, values []string, grammar conditionGrammar) error {
	if values == nil {
		return nil
	}
	if len(values) == 0 {
		return input("condition_id", "must be nonempty")
	}
	seen := make(map[string]struct{})
	ids := make([]string, 0, len(values))
	for _, value := range values {
		if grammar == comboConditions {
			normalized, err := normalizeComboConditionID(value)
			if err != nil {
				return input("condition_id", "expected a v2 combo identifier")
			}
			value = normalized
		} else {
			if !strings.HasPrefix(value, "0x") {
				return input("condition_id", "must be a 0x-prefixed hex identifier")
			}
			decoded, err := hex.DecodeString(value[2:])
			if err != nil || (len(decoded) != 31 && len(decoded) != 32) {
				return input("condition_id", "must contain 31 or 32 bytes")
			}
			value = strings.ToLower(value)
			if grammar == marketConditions && len(decoded) == 31 {
				if decoded[0] != 1 && decoded[0] != 2 {
					return input("condition_id", "31-byte market IDs must start with 01 or 02")
				}
				value += "00"
			}
		}
		if _, exists := seen[value]; !exists {
			seen[value] = struct{}{}
			ids = append(ids, value)
		}
	}
	if len(ids) > 20 {
		return input("condition_id", "accepts at most 20 distinct identifiers")
	}
	q.Set("condition_id", strings.Join(ids, ","))
	return nil
}

func eventIDs(q url.Values, values []int32) error {
	if values == nil {
		return nil
	}
	if len(values) == 0 {
		return input("event_id", "must be nonempty")
	}
	seen := make(map[int32]struct{})
	ids := make([]string, 0, len(values))
	for _, id := range values {
		if id <= 0 {
			return input("event_id", "must contain positive 32-bit integers")
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			ids = append(ids, strconv.FormatInt(int64(id), 10))
		}
	}
	q.Set("event_id", strings.Join(ids, ","))
	return nil
}

func selectors(q url.Values, p Selectors, grammar conditionGrammar) error {
	if p.ConditionIDs != nil && p.EventIDs != nil {
		return input("selectors", "choose condition IDs or event IDs, not both")
	}
	if err := conditionIDs(q, p.ConditionIDs, grammar); err != nil {
		return err
	}
	return eventIDs(q, p.EventIDs)
}

func enum[T ~string](q url.Values, key string, value T, allowed ...T) error {
	if value == "" {
		return nil
	}
	if !slices.Contains(allowed, value) {
		return input(key, fmt.Sprintf("invalid value %q", value))
	}
	q.Set(key, string(value))
	return nil
}

func boolParam(q url.Values, key string, value *bool) {
	if value != nil {
		q.Set(key, strconv.FormatBool(*value))
	}
}

func amountParam(q url.Values, key string, value *DecimalString) error {
	if value == nil {
		return nil
	}
	// Float parsing checks the service's finite numeric range only. The exact
	// caller spelling, never the rounded float, is sent on the wire.
	n, err := strconv.ParseFloat(string(*value), 64)
	if !polyjson.ValidDecimal(string(*value)) || err != nil || math.IsInf(n, 0) ||
		math.IsNaN(n) ||
		n < 0 {
		return input(key, "must be a finite nonnegative decimal")
	}
	q.Set(key, string(*value))
	return nil
}

func timestampParam(q url.Values, key string, value *time.Time, allowEpoch bool) error {
	if value == nil {
		return nil
	}
	seconds := value.Unix()
	minimum := int64(1)
	if allowEpoch {
		minimum = 0
	}
	if seconds < minimum || seconds > 253402300799 {
		return input(key, "timestamp is outside the supported range")
	}
	q.Set(key, strconv.FormatInt(seconds, 10))
	return nil
}

func window(q url.Values, p TimeWindow, boundedByDefault bool) error {
	if p.FullHistory {
		if p.Start != nil || p.End != nil {
			return input("window", "full history cannot be combined with explicit bounds")
		}
		if boundedByDefault {
			q.Set("start", "1")
		}
		return nil
	}
	if err := timestampParam(q, "start", p.Start, false); err != nil {
		return err
	}
	return timestampParam(q, "end", p.End, false)
}

func direction(q url.Values, value SortDirection) error {
	return enum(q, "sort_direction", value, "ASC", "DESC")
}

func leaderboardWindow(q url.Values, value LeaderboardWindow) error {
	return enum(q, "time_period", value, "day", "week", "month", "all")
}

func category(q url.Values, value string) error {
	if value == "" {
		return nil
	}
	if strings.TrimSpace(value) == "" {
		return input("category", "must be nonempty")
	}
	q.Set("category", value)
	return nil
}
