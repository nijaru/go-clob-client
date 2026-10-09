package clob

import (
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
	"testing"
)

func nativeTestPosition(marker byte, outcome byte) *big.Int {
	var encoded [32]byte
	encoded[0], encoded[30], encoded[31] = 1, marker, outcome
	return new(big.Int).SetBytes(encoded[:])
}

func TestNativePositionReferenceVectors(t *testing.T) {
	t.Parallel()
	// ts-sdk protocol.test.ts and py-sdk test_relayer_positions_helpers.py.
	combo, err := DeriveComboPositions(
		[]*big.Int{nativeTestPosition(2, 1), nativeTestPosition(1, 0)},
	)
	if err != nil {
		t.Fatal(err)
	}
	const golden = "0x032def24bfb0c5c57fb236fac08b94236a0000000000000000000000000000"
	if combo.ConditionID.String() != golden {
		t.Fatalf("combo condition = %s", combo.ConditionID)
	}
	for i, id := range combo.PositionIDs {
		condition, outcome, err := DecodeV2PositionID(id)
		if err != nil || condition != combo.ConditionID || int(outcome) != i {
			t.Fatalf("position decode: %s %d %v", condition, outcome, err)
		}
	}
	// Independent eth-abi 5.2.0 encodings of the stable TS/Python call signatures.
	vectors := []struct {
		method, hex string
		outcome     uint8
	}{
		{
			"split",
			"82d3b9f1032def24bfb0c5c57fb236fac08b94236a0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000007",
			0,
		},
		{
			"merge",
			"5b63685e032def24bfb0c5c57fb236fac08b94236a0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000007",
			0,
		},
		{
			"redeem",
			"d217a3cc032def24bfb0c5c57fb236fac08b94236a00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000007",
			1,
		},
	}
	for _, vector := range vectors {
		data, err := packV2PositionCall(
			vector.method,
			combo.ConditionID,
			big.NewInt(7),
			vector.outcome,
		)
		if err != nil || hex.EncodeToString(data) != vector.hex {
			t.Fatalf("%s wire mismatch: %x %v", vector.method, data, err)
		}
	}
	for _, padded := range []string{golden, golden + "00", golden + "01"} {
		id, err := ParseV2ConditionID(padded)
		if err != nil || id != combo.ConditionID {
			t.Fatalf("condition normalization: %v", err)
		}
	}
}

func TestNativePositionValidation(t *testing.T) {
	t.Parallel()
	for _, legs := range [][]*big.Int{
		nil,
		{nativeTestPosition(1, 0), nativeTestPosition(1, 0)},
		{nativeTestPosition(1, 0), nativeTestPosition(1, 1)},
		{nativeTestPosition(1, 2)},
		{big.NewInt(-1)},
		{new(big.Int).Lsh(big.NewInt(1), 256)},
	} {
		if _, err := DeriveComboPositions(legs); err == nil {
			t.Fatal("invalid combo accepted")
		}
	}
	if _, err := ParseV2ConditionID("0x" + strings.Repeat("01", 31) + "02"); !errors.Is(
		err,
		ErrInvalidPositionOperation,
	) {
		t.Fatal(err)
	}
	legs := []*big.Int{nativeTestPosition(2, 1), nativeTestPosition(1, 0)}
	first := new(big.Int).Set(legs[0])
	combo, err := DeriveComboPositions(legs)
	if err != nil {
		t.Fatal(err)
	}
	combo.Legs[0].SetInt64(0)
	if first.Cmp(legs[0]) != 0 || legs[1].Cmp(nativeTestPosition(1, 0)) != 0 {
		t.Fatal("derivation mutated inputs")
	}
}
