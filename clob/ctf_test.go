package clob

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestConditionID(t *testing.T) {
	oracle := common.HexToAddress("0x1234567890AbCdEf1234567890aBcDeF12345678")
	questionID := common.HexToHash(
		"0xabcdef000000000000000000000000000000000000000000000000000000000001",
	)
	got := ConditionID(oracle, questionID, 2)
	if got == (common.Hash{}) {
		t.Fatal("ConditionID returned zero hash")
	}
}

func TestCollectionID(t *testing.T) {
	parent := common.Hash{}
	conditionID := common.HexToHash(
		"0xabcdef000000000000000000000000000000000000000000000000000000000001",
	)
	indexSet := big.NewInt(3)

	got := CollectionID(parent, conditionID, indexSet)
	if got == (common.Hash{}) {
		t.Fatal("CollectionID returned zero hash")
	}

	got2 := CollectionID(got, conditionID, indexSet)
	if got2 == got {
		t.Fatal("CollectionID with non-zero parent should produce different result")
	}
}

func TestPositionID(t *testing.T) {
	collateral := common.HexToAddress("0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174")
	collectionID := common.HexToHash(
		"0xabcdef000000000000000000000000000000000000000000000000000000000001",
	)

	got := PositionID(collateral, collectionID)
	if got == nil || got.Sign() == 0 {
		t.Fatal("PositionID returned zero")
	}
}

func TestConditionIDDeterministic(t *testing.T) {
	oracle := common.HexToAddress("0x1234567890AbCdEf1234567890aBcDeF12345678")
	questionID := common.HexToHash(
		"0x0100000000000000000000000000000000000000000000000000000000000000",
	)

	a := ConditionID(oracle, questionID, 2)
	b := ConditionID(oracle, questionID, 2)
	if a != b {
		t.Fatal("ConditionID not deterministic")
	}

	c := ConditionID(oracle, questionID, 3)
	if a == c {
		t.Fatal("ConditionID should differ for different outcomeSlotCount")
	}
}

func TestBinaryPartitionHelpers(t *testing.T) {
	collateral := common.HexToAddress("0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174")
	conditionID := common.HexToHash("0x01")
	amount := big.NewInt(1_000_000)

	split := SplitBinary(collateral, conditionID, amount)
	if split.CollateralToken != collateral {
		t.Error("SplitBinary collateral mismatch")
	}
	if split.ConditionID != conditionID {
		t.Error("SplitBinary conditionID mismatch")
	}
	if split.Amount.Cmp(amount) != 0 {
		t.Error("SplitBinary amount mismatch")
	}
	if split.ParentCollectionID != (common.Hash{}) {
		t.Error("SplitBinary parentCollectionID should be zero")
	}
	if len(split.Partition) != 2 || split.Partition[0].Int64() != 1 ||
		split.Partition[1].Int64() != 2 {
		t.Errorf("SplitBinary partition = %v, want [1, 2]", split.Partition)
	}

	merge := MergeBinary(collateral, conditionID, amount)
	if len(merge.Partition) != 2 || merge.Partition[0].Int64() != 1 ||
		merge.Partition[1].Int64() != 2 {
		t.Errorf("MergeBinary partition = %v, want [1, 2]", merge.Partition)
	}

	redeem := RedeemBinary(collateral, conditionID)
	if len(redeem.IndexSets) != 2 || redeem.IndexSets[0].Int64() != 1 ||
		redeem.IndexSets[1].Int64() != 2 {
		t.Errorf("RedeemBinary indexSets = %v, want [1, 2]", redeem.IndexSets)
	}
}

func TestABIPacking(t *testing.T) {
	collateral := common.HexToAddress("0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174")
	conditionID := common.HexToHash("0x01")
	parent := common.Hash{}
	amount := big.NewInt(1_000_000)
	partition := []*big.Int{big.NewInt(1), big.NewInt(2)}

	splitData, err := ctfABI.Pack(
		"splitPosition",
		collateral,
		parent,
		conditionID,
		partition,
		amount,
	)
	if err != nil {
		t.Fatalf("pack splitPosition: %v", err)
	}
	if len(splitData) == 0 {
		t.Fatal("splitPosition packed data is empty")
	}

	mergeData, err := ctfABI.Pack(
		"mergePositions",
		collateral,
		parent,
		conditionID,
		partition,
		amount,
	)
	if err != nil {
		t.Fatalf("pack mergePositions: %v", err)
	}
	if len(mergeData) == 0 {
		t.Fatal("mergePositions packed data is empty")
	}

	indexSets := []*big.Int{big.NewInt(1), big.NewInt(2)}
	redeemData, err := ctfABI.Pack("redeemPositions", collateral, parent, conditionID, indexSets)
	if err != nil {
		t.Fatalf("pack redeemPositions: %v", err)
	}
	if len(redeemData) == 0 {
		t.Fatal("redeemPositions packed data is empty")
	}
}

func TestNegRiskABIPacking(t *testing.T) {
	conditionID := common.HexToHash("0x01")
	amounts := []*big.Int{big.NewInt(1000), big.NewInt(2000)}

	data, err := negRiskABI.Pack("redeemPositions", conditionID, amounts)
	if err != nil {
		t.Fatalf("pack negRisk redeemPositions: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("negRisk redeemPositions packed data is empty")
	}
}

func TestContractConfigLookup(t *testing.T) {
	cfg, err := getContractConfig(137)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Conditional == "" {
		t.Error("missing Conditional address for mainnet")
	}
	if cfg.NegRiskAdapter == "" {
		t.Error("missing NegRiskAdapter address for mainnet")
	}
	if cfg.Collateral == "" {
		t.Error("missing Collateral address for mainnet")
	}

	cfg2, err := getContractConfig(80002)
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.Conditional == "" {
		t.Error("missing Conditional address for testnet")
	}

	_, err = getContractConfig(1)
	if err == nil {
		t.Error("expected error for unsupported chain ID")
	}
}
