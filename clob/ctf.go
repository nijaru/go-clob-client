package clob

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// SplitTokens converts USDC.e collateral into a full set of outcome tokens (Yes + No).
// Level 2 Auth required.
func (c *AuthenticatedClient) SplitTokens(ctx context.Context, args SplitArgs) error {
	return c.postJSON(ctx, splitPositionsEndpoint, args, polyhttp.AuthL2, nil)
}

// MergeTokens converts a full set of outcome tokens (Yes + No) back into USDC.e collateral.
// Level 2 Auth required.
func (c *AuthenticatedClient) MergeTokens(ctx context.Context, args MergeArgs) error {
	return c.postJSON(ctx, mergePositionsEndpoint, args, polyhttp.AuthL2, nil)
}

// RedeemTokens exchanges winning outcome tokens for USDC.e collateral after market resolution.
// Level 2 Auth required.
func (c *AuthenticatedClient) RedeemTokens(ctx context.Context, args RedeemArgs) error {
	return c.postJSON(ctx, redeemPositionsEndpoint, args, polyhttp.AuthL2, nil)
}

// ConditionID computes the condition ID for a CTF market.
// Equivalent to the Gnosis CTF contract's getConditionId view function.
// oracle is the oracle address, questionID is the 32-byte question identifier,
// outcomeSlotCount is the number of outcome slots.
func ConditionID(oracle common.Address, questionID common.Hash, outcomeSlotCount uint) common.Hash {
	buf := make([]byte, 0, 84) // 20 + 32 + 32
	buf = append(buf, oracle.Bytes()...)
	buf = append(buf, questionID.Bytes()...)
	n := new(big.Int).SetUint64(uint64(outcomeSlotCount))
	slot := make([]byte, 32)
	n.FillBytes(slot)
	buf = append(buf, slot...)
	return crypto.Keccak256Hash(buf)
}

// CollectionID computes the collection ID for a set of positions.
// Equivalent to the Gnosis CTF contract's getCollectionId view function.
func CollectionID(
	parentCollectionID common.Hash,
	conditionID common.Hash,
	indexSet *big.Int,
) common.Hash {
	// ABI encode: each word is 32 bytes
	inner := make([]byte, 64)
	copy(inner[:32], conditionID.Bytes())
	indexSet.FillBytes(inner[32:64])
	h := crypto.Keccak256Hash(inner)

	// XOR with parent
	var result [32]byte
	pb := parentCollectionID.Bytes()
	hb := h.Bytes()
	for i := range result {
		result[i] = pb[i] ^ hb[i]
	}
	return common.BytesToHash(result[:])
}

// PositionID computes the ERC-1155 position ID.
// Equivalent to the Gnosis CTF contract's getPositionId view function.
func PositionID(collateralToken common.Address, collectionID common.Hash) *big.Int {
	buf := make([]byte, 0, 52) // 20 + 32
	buf = append(buf, collateralToken.Bytes()...)
	buf = append(buf, collectionID.Bytes()...)
	h := crypto.Keccak256Hash(buf)
	return new(big.Int).SetBytes(h.Bytes())
}
