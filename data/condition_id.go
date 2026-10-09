package data

import (
	"encoding/hex"
	"errors"
	"strings"
)

// A combo condition is structural (31 bytes, protocol 03). Responses and
// filters also accept a 32-byte position identifier with outcome byte 00/01;
// both outcomes refer to the same condition. Ordinary market IDs are not combos.
func normalizeComboConditionID(value string) (string, error) {
	if !strings.HasPrefix(value, "0x") || (len(value) != 64 && len(value) != 66) {
		return "", errors.New("data: expected a v2 combo condition identifier")
	}
	decoded, err := hex.DecodeString(value[2:])
	if err != nil || decoded[0] != 3 ||
		(len(decoded) == 32 && decoded[31] != 0 && decoded[31] != 1) {
		return "", errors.New("data: expected a v2 combo condition identifier")
	}
	return strings.ToLower(value[:64]), nil
}
