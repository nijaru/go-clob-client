package clob

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestGetTradingApprovalsStateFullyApproved(t *testing.T) {
	t.Parallel()

	rpc, calls := newTokenReadRPC(t, MaxUint256(), true)
	defer rpc.Close()

	client, err := NewSignerClient(Config{
		ChainID:    PolygonChainID,
		PrivateKey: gaslessTestKey,
		RPCURL:     rpc.URL,
	})
	if err != nil {
		t.Fatalf("new signer client: %v", err)
	}

	state, err := client.GetTradingApprovalsState(t.Context(), "")
	if err != nil {
		t.Fatalf("get trading approvals state: %v", err)
	}
	if !state.IsFullyApproved {
		t.Fatalf("expected fully approved, missing = %+v", state.Missing)
	}
	if state.Missing == nil || !state.Missing.Empty() {
		t.Fatalf("expected empty missing set, got %+v", state.Missing)
	}
	if got := calls.Load(); got != 15 {
		t.Fatalf("eth_call count = %d, want 15", got)
	}
}

func TestGetTradingApprovalsStateMissing(t *testing.T) {
	t.Parallel()

	rpc, calls := newTokenReadRPC(t, big.NewInt(0), false)
	defer rpc.Close()

	client, err := NewSignerClient(Config{
		ChainID:    PolygonChainID,
		PrivateKey: gaslessTestKey,
		RPCURL:     rpc.URL,
	})
	if err != nil {
		t.Fatalf("new signer client: %v", err)
	}

	state, err := client.GetTradingApprovalsState(t.Context(), "")
	if err != nil {
		t.Fatalf("get trading approvals state: %v", err)
	}
	if state.IsFullyApproved {
		t.Fatalf("expected missing approvals, got fully approved")
	}
	if state.Missing == nil {
		t.Fatal("missing set is nil")
	}
	if len(state.Missing.ERC20Approvals) != 7 {
		t.Fatalf("expected 7 missing ERC20 approvals, got %d", len(state.Missing.ERC20Approvals))
	}
	if len(state.Missing.ERC1155Approvals) != 8 {
		t.Fatalf("expected 8 missing ERC1155 approvals, got %d", len(state.Missing.ERC1155Approvals))
	}
	if got := calls.Load(); got != 15 {
		t.Fatalf("eth_call count = %d, want 15", got)
	}
}

func TestGetTradingApprovalsStateExplicitWallet(t *testing.T) {
	t.Parallel()

	rpc, calls := newTokenReadRPC(t, MaxUint256(), true)
	defer rpc.Close()

	client, err := NewSignerClient(Config{
		ChainID:    PolygonChainID,
		PrivateKey: gaslessTestKey,
		RPCURL:     rpc.URL,
	})
	if err != nil {
		t.Fatalf("new signer client: %v", err)
	}

	wallet := common.HexToAddress("0xAbCdEf0123456789AbCdEf0123456789AbCdEf01").Hex()
	state, err := client.GetTradingApprovalsState(t.Context(), wallet)
	if err != nil {
		t.Fatalf("get trading approvals state: %v", err)
	}
	if !state.IsFullyApproved {
		t.Fatalf("expected fully approved for explicit wallet, missing = %+v", state.Missing)
	}
	if got := calls.Load(); got != 15 {
		t.Fatalf("eth_call count = %d, want 15", got)
	}
}

func TestGetTradingApprovalsStateInvalidWallet(t *testing.T) {
	t.Parallel()

	rpc, _ := newTokenReadRPC(t, MaxUint256(), true)
	defer rpc.Close()

	client, err := NewSignerClient(Config{
		ChainID:    PolygonChainID,
		PrivateKey: gaslessTestKey,
		RPCURL:     rpc.URL,
	})
	if err != nil {
		t.Fatalf("new signer client: %v", err)
	}

	if _, err := client.GetTradingApprovalsState(t.Context(), "not-an-address"); err == nil {
		t.Fatal("expected error for invalid wallet address")
	}
}

func TestTradingApprovalsStateMirrorsOfficialModels(t *testing.T) {
	t.Parallel()

	// Erc20TradingApproval mirrors the official SDK model name and shape.
	erc20 := Erc20TradingApproval{
		TokenAddress:   common.HexToAddress("0x1"),
		SpenderAddress: common.HexToAddress("0x2"),
		Amount:         MaxUint256(),
	}
	if erc20.TokenAddress == (common.Address{}) || erc20.SpenderAddress == (common.Address{}) {
		t.Fatal("Erc20TradingApproval fields are zero")
	}
	if erc20.Amount.Cmp(MaxUint256()) != 0 {
		t.Fatal("Erc20TradingApproval amount is not MaxUint256")
	}

	// Erc1155TradingApproval is an alias for the existing request type.
	var erc1155 Erc1155TradingApproval = ERC1155ApprovalForAllRequest{
		TokenAddress:    common.HexToAddress("0x3"),
		OperatorAddress: common.HexToAddress("0x4"),
		Approved:        true,
	}
	if erc1155.TokenAddress == (common.Address{}) || erc1155.OperatorAddress == (common.Address{}) {
		t.Fatal("Erc1155TradingApproval fields are zero")
	}
	if !erc1155.Approved {
		t.Fatal("Erc1155TradingApproval approved is false")
	}
}