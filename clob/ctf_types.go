package clob

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

func BinaryPartition() []*big.Int {
	return []*big.Int{big.NewInt(1), big.NewInt(2)}
}

// ConditionIDRequest contains the inputs for a provider-backed condition ID read.
type ConditionIDRequest struct {
	Oracle           common.Address
	QuestionID       common.Hash
	OutcomeSlotCount *big.Int
}

// CollectionIDRequest contains the inputs for a provider-backed collection ID read.
type CollectionIDRequest struct {
	ParentCollectionID common.Hash
	ConditionID        common.Hash
	IndexSet           *big.Int
}

// PositionIDRequest contains the inputs for a provider-backed position ID read.
type PositionIDRequest struct {
	CollateralToken common.Address
	CollectionID    common.Hash
}

type SplitPositionRequest struct {
	CollateralToken    common.Address
	ParentCollectionID common.Hash
	ConditionID        common.Hash
	Partition          []*big.Int
	Amount             *big.Int
}

func SplitBinary(
	collateral common.Address,
	conditionID common.Hash,
	amount *big.Int,
) SplitPositionRequest {
	return SplitPositionRequest{
		CollateralToken:    collateral,
		ParentCollectionID: common.Hash{},
		ConditionID:        conditionID,
		Partition:          BinaryPartition(),
		Amount:             amount,
	}
}

type MergePositionsRequest struct {
	CollateralToken    common.Address
	ParentCollectionID common.Hash
	ConditionID        common.Hash
	Partition          []*big.Int
	Amount             *big.Int
}

func MergeBinary(
	collateral common.Address,
	conditionID common.Hash,
	amount *big.Int,
) MergePositionsRequest {
	return MergePositionsRequest{
		CollateralToken:    collateral,
		ParentCollectionID: common.Hash{},
		ConditionID:        conditionID,
		Partition:          BinaryPartition(),
		Amount:             amount,
	}
}

type RedeemPositionsRequest struct {
	CollateralToken    common.Address
	ParentCollectionID common.Hash
	ConditionID        common.Hash
	IndexSets          []*big.Int
}

func RedeemBinary(collateral common.Address, conditionID common.Hash) RedeemPositionsRequest {
	return RedeemPositionsRequest{
		CollateralToken:    collateral,
		ParentCollectionID: common.Hash{},
		ConditionID:        conditionID,
		IndexSets:          BinaryPartition(),
	}
}

type RedeemNegRiskRequest struct {
	ConditionID common.Hash
	Amounts     []*big.Int
}

type TxReceipt struct {
	Hash        common.Hash
	BlockNumber uint64
}
