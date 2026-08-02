package clob

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
)

type fakeCTFProvider struct {
	result []byte
	to     common.Address
	data   []byte
}

func (p *fakeCTFProvider) CallContract(
	_ context.Context,
	call ethereum.CallMsg,
	_ *big.Int,
) ([]byte, error) {
	p.to = *call.To
	p.data = append([]byte(nil), call.Data...)
	return p.result, nil
}

func packCTFOutput(t *testing.T, method string, values ...any) []byte {
	t.Helper()
	output, err := ctfABI.Methods[method].Outputs.Pack(values...)
	if err != nil {
		t.Fatalf("pack %s output: %v", method, err)
	}
	return output
}

func TestCTFClientProviderBackedReads(t *testing.T) {
	t.Parallel()

	provider := &fakeCTFProvider{
		result: packCTFOutput(t, "getConditionId", [32]byte{1}),
	}
	client, err := NewCTFClient(provider, PolygonChainID)
	if err != nil {
		t.Fatalf("new CTF client: %v", err)
	}
	if client.Provider() != provider {
		t.Fatal("Provider did not return the configured provider")
	}

	conditionID, err := client.ConditionID(t.Context(), ConditionIDRequest{
		Oracle:           common.HexToAddress("0x0000000000000000000000000000000000000001"),
		QuestionID:       common.Hash{2},
		OutcomeSlotCount: big.NewInt(2),
	})
	if err != nil {
		t.Fatalf("condition ID: %v", err)
	}
	if conditionID != (common.Hash{1}) {
		t.Fatalf("condition ID = %s, want 0x01", conditionID)
	}
	if provider.to != common.HexToAddress(contractConfigs[PolygonChainID].Conditional) {
		t.Fatalf("call target = %s, want CTF contract", provider.to)
	}
	if len(provider.data) != 4+32*3 {
		t.Fatalf("calldata length = %d, want %d", len(provider.data), 4+32*3)
	}

	provider.result = packCTFOutput(t, "getCollectionId", [32]byte{3})
	collectionID, err := client.CollectionID(t.Context(), CollectionIDRequest{
		ConditionID: common.Hash{4},
		IndexSet:    big.NewInt(1),
	})
	if err != nil {
		t.Fatalf("collection ID: %v", err)
	}
	if collectionID != (common.Hash{3}) {
		t.Fatalf("collection ID = %s, want 0x03", collectionID)
	}

	provider.result = packCTFOutput(t, "getPositionId", big.NewInt(5))
	positionID, err := client.PositionID(t.Context(), PositionIDRequest{
		CollateralToken: common.Address{1},
		CollectionID:    common.Hash{2},
	})
	if err != nil {
		t.Fatalf("position ID: %v", err)
	}
	if positionID.Cmp(big.NewInt(5)) != 0 {
		t.Fatalf("position ID = %s, want 5", positionID)
	}
}

func TestCTFClientRejectsInvalidReadInputs(t *testing.T) {
	t.Parallel()

	provider := &fakeCTFProvider{}
	client, err := NewCTFClient(provider, PolygonChainID)
	if err != nil {
		t.Fatalf("new CTF client: %v", err)
	}
	if _, err := client.ConditionID(t.Context(), ConditionIDRequest{}); err == nil {
		t.Fatal("expected missing outcome slot count error")
	}
	if _, err := client.CollectionID(t.Context(), CollectionIDRequest{IndexSet: big.NewInt(-1)}); err == nil {
		t.Fatal("expected negative index set error")
	}
	if _, err := NewCTFClient(provider, 1); err == nil {
		t.Fatal("expected unsupported chain error")
	}
}

var _ CTFProvider = (*fakeCTFProvider)(nil)
