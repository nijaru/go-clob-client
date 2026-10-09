package clob

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
)

var ErrInvalidPositionOperation = errors.New("invalid position operation")

// V2ConditionID is the native bytes31 condition identifier. It must not be
// converted through common.Hash, which would change fixed-bytes ABI alignment.
type V2ConditionID [31]byte

func (id V2ConditionID) String() string { return "0x" + hex.EncodeToString(id[:]) }

// ParseV2ConditionID accepts bytes31, or bytes32 ending in a YES/NO outcome byte,
// matching the stable wallet router contract.
func ParseV2ConditionID(value string) (V2ConditionID, error) {
	var id V2ConditionID
	value = strings.TrimPrefix(strings.ToLower(value), "0x")
	if len(value) == 64 && (strings.HasSuffix(value, "00") || strings.HasSuffix(value, "01")) {
		value = value[:62]
	}
	data, err := hex.DecodeString(value)
	if err != nil || len(data) != len(id) {
		return id, fmt.Errorf(
			"%w: V2 condition must be bytes31 or bytes32 with binary outcome",
			ErrInvalidPositionOperation,
		)
	}
	copy(id[:], data)
	return id, nil
}

func (id V2ConditionID) PositionIDs() [2]*big.Int {
	var encoded [32]byte
	copy(encoded[:31], id[:])
	yes := new(big.Int).SetBytes(encoded[:])
	encoded[31] = 1
	return [2]*big.Int{yes, new(big.Int).SetBytes(encoded[:])}
}

// DecodeV2PositionID validates the native binary, neg-risk and combo namespaces.
func DecodeV2PositionID(position *big.Int) (V2ConditionID, uint8, error) {
	var condition V2ConditionID
	if err := validateUint256(position, "position ID"); err != nil {
		return condition, 0, err
	}
	var encoded [32]byte
	position.FillBytes(encoded[:])
	if encoded[0] < 1 || encoded[0] > 3 || encoded[31] > 1 {
		return condition, 0, fmt.Errorf(
			"%w: unsupported V2 module or outcome",
			ErrInvalidPositionOperation,
		)
	}
	copy(condition[:], encoded[:31])
	return condition, encoded[31], nil
}

// CanonicalComboLegs validates 1..50 native binary/neg-risk legs, sorts copies
// numerically, and rejects duplicates or both outcomes of one condition.
func CanonicalComboLegs(legs []*big.Int) ([]*big.Int, error) {
	if len(legs) == 0 || len(legs) > 50 {
		return nil, fmt.Errorf("%w: combo requires 1..50 legs", ErrInvalidPositionOperation)
	}
	canonical := make([]*big.Int, len(legs))
	for i, leg := range legs {
		condition, _, err := DecodeV2PositionID(leg)
		if err != nil {
			return nil, err
		}
		if condition[0] == 3 {
			return nil, fmt.Errorf(
				"%w: combo legs must be binary or neg-risk positions",
				ErrInvalidPositionOperation,
			)
		}
		canonical[i] = new(big.Int).Set(leg)
	}
	slices.SortFunc(canonical, func(a, b *big.Int) int { return a.Cmp(b) })
	for i := 1; i < len(canonical); i++ {
		previous := new(big.Int).Rsh(new(big.Int).Set(canonical[i-1]), 8)
		current := new(big.Int).Rsh(new(big.Int).Set(canonical[i]), 8)
		if previous.Cmp(current) == 0 {
			return nil, fmt.Errorf("%w: duplicate leg condition", ErrInvalidPositionOperation)
		}
	}
	return canonical, nil
}

type ComboPositions struct {
	ConditionID V2ConditionID
	PositionIDs [2]*big.Int
	Legs        []*big.Int // canonical, owned copies
}

// DeriveComboPositions computes the native combo condition and complementary
// position IDs from the canonical ABI-encoded leg set, as in TS/Python.
func DeriveComboPositions(legs []*big.Int) (*ComboPositions, error) {
	canonical, err := CanonicalComboLegs(legs)
	if err != nil {
		return nil, err
	}
	encoded, err := walletABI.Methods["prepareCondition"].Inputs.Pack(canonical)
	if err != nil {
		return nil, err
	}
	uintType, err := abi.NewType("uint256", "", nil)
	if err != nil {
		return nil, err
	}
	bytesType, err := abi.NewType("bytes", "", nil)
	if err != nil {
		return nil, err
	}
	preimage, err := (abi.Arguments{{Type: uintType}, {Type: bytesType}}).Pack(
		big.NewInt(3),
		encoded,
	)
	if err != nil {
		return nil, err
	}
	hash := crypto.Keccak256(preimage)
	var condition V2ConditionID
	condition[0] = 3
	copy(condition[1:17], hash[16:])
	return &ComboPositions{
		ConditionID: condition,
		PositionIDs: condition.PositionIDs(),
		Legs:        canonical,
	}, nil
}
