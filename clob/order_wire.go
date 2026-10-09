package clob

import (
	stdjson "encoding/json"
	"fmt"
	"strconv"

	json "github.com/go-json-experiment/json"
)

// MarshalJSON flattens the signed payload, with numeric salt as required by
// CLOB. V1 and V2/V3 fields are mutually exclusive on the wire.
func (o SignedOrder) MarshalJSON() ([]byte, error) {
	salt, err := strconv.ParseUint(o.Order.Salt, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parse order salt: %w", err)
	}
	type commonWire struct {
		Salt          uint64        `json:"salt"`
		Maker         string        `json:"maker"`
		Signer        string        `json:"signer"`
		TokenID       string        `json:"tokenId"`
		MakerAmount   string        `json:"makerAmount"`
		TakerAmount   string        `json:"takerAmount"`
		Side          Side          `json:"side"`
		SignatureType SignatureType `json:"signatureType"`
		Expiration    string        `json:"expiration"`
		Signature     string        `json:"signature"`
	}
	common := commonWire{
		Salt:          salt,
		Maker:         o.Order.Maker,
		Signer:        o.Order.Signer,
		TokenID:       o.Order.TokenID,
		MakerAmount:   o.Order.MakerAmount,
		TakerAmount:   o.Order.TakerAmount,
		Side:          o.Order.Side,
		SignatureType: o.Order.SignatureType,
		Expiration:    o.Expiration,
		Signature:     o.Signature,
	}
	if o.Legacy != nil {
		return json.Marshal(struct {
			commonWire
			Taker      string `json:"taker"`
			Nonce      string `json:"nonce"`
			FeeRateBps string `json:"feeRateBps"`
		}{common, o.Legacy.Taker, o.Legacy.Nonce, o.Legacy.FeeRateBps})
	}
	return json.Marshal(struct {
		commonWire
		Timestamp string `json:"timestamp"`
		Metadata  string `json:"metadata"`
		Builder   string `json:"builder"`
	}{common, o.Order.Timestamp, o.Order.Metadata, o.Order.Builder})
}

// UnmarshalJSON decodes the stable signed-order wire schema without passing
// integers through float64, including salts above JavaScript's exact range.
func (o *SignedOrder) UnmarshalJSON(data []byte) error {
	var wire struct {
		Order
		Salt       stdjson.RawMessage `json:"salt"`
		Expiration string             `json:"expiration"`
		Signature  string             `json:"signature"`
		Taker      *string            `json:"taker"`
		Nonce      *string            `json:"nonce"`
		FeeRateBps *string            `json:"feeRateBps"`
	}
	if err := stdjson.Unmarshal(data, &wire); err != nil {
		return err
	}
	var salt string
	if err := stdjson.Unmarshal(wire.Salt, &salt); err != nil {
		salt = string(wire.Salt)
	}
	n, err := strconv.ParseUint(salt, 10, 64)
	if err != nil {
		return fmt.Errorf("signed order salt: %w", err)
	}
	wire.Order.Salt = strconv.FormatUint(n, 10)
	if wire.Side != SideBuy && wire.Side != SideSell {
		return fmt.Errorf("signed order side %q is invalid", wire.Side)
	}
	if wire.SignatureType < SignatureTypeEOA || wire.SignatureType > SignatureTypePoly1271 {
		return fmt.Errorf("signed order signature type %d is invalid", wire.SignatureType)
	}
	decoded := SignedOrder{
		Order:      wire.Order,
		Expiration: wire.Expiration,
		Signature:  wire.Signature,
	}
	if wire.Taker != nil || wire.Nonce != nil || wire.FeeRateBps != nil {
		if wire.Taker == nil || wire.Nonce == nil || wire.FeeRateBps == nil {
			return fmt.Errorf("V1 signed order requires taker, nonce, and feeRateBps")
		}
		if wire.Timestamp != "" || wire.Metadata != "" || wire.Builder != "" {
			return fmt.Errorf("signed order mixes V1 and V2 fields")
		}
		decoded.Legacy = &LegacyOrderFields{
			Taker:      *wire.Taker,
			Nonce:      *wire.Nonce,
			FeeRateBps: *wire.FeeRateBps,
		}
	}
	*o = decoded
	return nil
}
