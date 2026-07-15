package perps

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strconv"
	"time"

	ethmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"

	"github.com/nijaru/go-clob-client/internal/polyauth"
)

func randomPerpsSalt() (uint64, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return 0, fmt.Errorf("perps: generate command salt: %w", err)
	}
	return uint64(binary.BigEndian.Uint64(raw[:])), nil
}

func signPerpsOperation(
	signer *polyauth.Signer,
	chainID int64,
	op []any,
	salt uint64,
	timestamp int64,
) (string, error) {
	encoded, err := encodePerpsMsgpack(op)
	if err != nil {
		return "", err
	}
	dataHash := crypto.Keccak256(encoded)
	typedData := apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": {
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
			},
			"Op": {
				{Name: "data", Type: "bytes32"},
				{Name: "salt", Type: "uint64"},
				{Name: "ts", Type: "uint64"},
			},
		},
		PrimaryType: "Op",
		Domain: apitypes.TypedDataDomain{
			Name:    "Polymarket",
			Version: "1",
			ChainId: ethmath.NewHexOrDecimal256(chainID),
		},
		Message: apitypes.TypedDataMessage{
			"data": "0x" + fmt.Sprintf("%x", dataHash),
			"salt": strconv.FormatUint(salt, 10),
			"ts":   strconv.FormatInt(timestamp, 10),
		},
	}
	return polyauth.SignTypedData(signer, typedData)
}

func makePerpsSignedCommand(
	signer *polyauth.Signer,
	chainID int64,
	op []any,
	bodyOp map[string]any,
	expiresAt int64,
) (map[string]any, error) {
	if signer == nil {
		return nil, ErrPerpsSigningKeyRequired
	}
	if expiresAt < 0 {
		return nil, fmt.Errorf("perps: expiration must be positive when supplied")
	}
	salt, err := randomPerpsSalt()
	if err != nil {
		return nil, err
	}
	timestamp := time.Now().UnixMilli()
	signature, err := signPerpsOperation(signer, chainID, op, salt, timestamp)
	if err != nil {
		return nil, fmt.Errorf("perps: sign session command: %w", err)
	}
	body := map[string]any{
		"op":   bodyOp,
		"salt": salt,
		"sig":  signature,
		"ts":   timestamp,
	}
	if expiresAt != 0 {
		body["exp"] = expiresAt
	}
	return body, nil
}

func encodePerpsMsgpack(value any) ([]byte, error) {
	var out bytes.Buffer
	if err := appendPerpsMsgpack(&out, value); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// appendPerpsMsgpack implements the compact MessagePack subset used by the
// official perps signed operation format. Undefined JS array entries are
// omitted by callers before encoding, matching compactSignableValue.
func appendPerpsMsgpack(out *bytes.Buffer, value any) error {
	switch value := value.(type) {
	case nil:
		return out.WriteByte(0xc0)
	case bool:
		if value {
			return out.WriteByte(0xc3)
		}
		return out.WriteByte(0xc2)
	case string:
		return appendPerpsMsgpackString(out, value)
	case int:
		if value < 0 {
			return fmt.Errorf("perps: unsupported negative MessagePack integer %d", value)
		}
		return appendPerpsMsgpackUint(out, uint64(value))
	case int64:
		if value < 0 {
			return fmt.Errorf("perps: unsupported negative MessagePack integer %d", value)
		}
		return appendPerpsMsgpackUint(out, uint64(value))
	case uint64:
		return appendPerpsMsgpackUint(out, value)
	case []any:
		if err := appendPerpsMsgpackArrayHeader(out, len(value)); err != nil {
			return err
		}
		for _, item := range value {
			if err := appendPerpsMsgpack(out, item); err != nil {
				return err
			}
		}
		return nil
	case []int:
		items := make([]any, len(value))
		for i, item := range value {
			items[i] = item
		}
		return appendPerpsMsgpack(out, items)
	case []string:
		items := make([]any, len(value))
		for i, item := range value {
			items[i] = item
		}
		return appendPerpsMsgpack(out, items)
	default:
		return fmt.Errorf("perps: unsupported MessagePack value %T", value)
	}
}

func appendPerpsMsgpackString(out *bytes.Buffer, value string) error {
	length := len(value)
	switch {
	case length < 32:
		if err := out.WriteByte(0xa0 | byte(length)); err != nil {
			return err
		}
	case length <= 255:
		if err := out.WriteByte(0xd9); err != nil {
			return err
		}
		if err := out.WriteByte(byte(length)); err != nil {
			return err
		}
	case length <= 65535:
		if err := out.WriteByte(0xda); err != nil {
			return err
		}
		var raw [2]byte
		binary.BigEndian.PutUint16(raw[:], uint16(length))
		if _, err := out.Write(raw[:]); err != nil {
			return err
		}
	default:
		if err := out.WriteByte(0xdb); err != nil {
			return err
		}
		var raw [4]byte
		binary.BigEndian.PutUint32(raw[:], uint32(length))
		if _, err := out.Write(raw[:]); err != nil {
			return err
		}
	}
	_, err := out.WriteString(value)
	return err
}

func appendPerpsMsgpackArrayHeader(out *bytes.Buffer, length int) error {
	switch {
	case length < 16:
		return out.WriteByte(0x90 | byte(length))
	case length <= 65535:
		if err := out.WriteByte(0xdc); err != nil {
			return err
		}
		var raw [2]byte
		binary.BigEndian.PutUint16(raw[:], uint16(length))
		_, err := out.Write(raw[:])
		return err
	default:
		if err := out.WriteByte(0xdd); err != nil {
			return err
		}
		var raw [4]byte
		binary.BigEndian.PutUint32(raw[:], uint32(length))
		_, err := out.Write(raw[:])
		return err
	}
}

func appendPerpsMsgpackUint(out *bytes.Buffer, value uint64) error {
	switch {
	case value <= 0x7f:
		return out.WriteByte(byte(value))
	case value <= 0xff:
		if err := out.WriteByte(0xcc); err != nil {
			return err
		}
		return out.WriteByte(byte(value))
	case value <= 0xffff:
		if err := out.WriteByte(0xcd); err != nil {
			return err
		}
		var raw [2]byte
		binary.BigEndian.PutUint16(raw[:], uint16(value))
		_, err := out.Write(raw[:])
		return err
	case value <= 0xffffffff:
		if err := out.WriteByte(0xce); err != nil {
			return err
		}
		var raw [4]byte
		binary.BigEndian.PutUint32(raw[:], uint32(value))
		_, err := out.Write(raw[:])
		return err
	default:
		if err := out.WriteByte(0xcf); err != nil {
			return err
		}
		var raw [8]byte
		binary.BigEndian.PutUint64(raw[:], value)
		_, err := out.Write(raw[:])
		return err
	}
}
