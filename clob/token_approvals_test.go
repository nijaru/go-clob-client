package clob

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestRequiredTradingApprovalsMatchesOfficialContractSet(t *testing.T) {
	t.Parallel()

	config, err := getContractConfig(PolygonChainID)
	if err != nil {
		t.Fatal(err)
	}
	erc20, erc1155, err := requiredTradingApprovals(PolygonChainID, config)
	if err != nil {
		t.Fatal(err)
	}
	if len(erc20) != 8 || len(erc1155) != 9 {
		t.Fatalf("approval requirements = %d ERC20, %d ERC1155; want 8, 9", len(erc20), len(erc1155))
	}

	wantERC20Spenders := []string{
		config.Exchange,
		config.NegRiskExchange,
		config.NegRiskAdapter,
		config.CollateralAdapter,
		config.NegRiskCollateralAdapter,
		config.ProtocolV2Router,
		config.ExchangeV3,
		config.PerpsDepositContract,
	}
	for i, want := range wantERC20Spenders {
		if got := erc20[i].SpenderAddress; got != common.HexToAddress(want) {
			t.Fatalf("ERC20 spender[%d] = %s, want %s", i, got, want)
		}
		if erc20[i].Amount.Cmp(MaxUint256()) != 0 {
			t.Fatalf("ERC20 amount[%d] = %s, want MaxUint256", i, erc20[i].Amount)
		}
	}
	wantERC1155 := [][2]string{
		{config.Conditional, config.Exchange},
		{config.Conditional, config.NegRiskExchange},
		{config.Conditional, config.NegRiskAdapter},
		{config.Conditional, config.CollateralAdapter},
		{config.Conditional, config.NegRiskCollateralAdapter},
		{config.Conditional, config.AutoRedeemOperator},
		{config.PositionManager, config.ProtocolV2Router},
		{config.PositionManager, config.ExchangeV3},
		{config.PositionManager, config.AutoRedeemOperator},
	}
	for i, want := range wantERC1155 {
		if got := erc1155[i]; got.TokenAddress != common.HexToAddress(want[0]) ||
			got.OperatorAddress != common.HexToAddress(want[1]) || !got.Approved {
			t.Fatalf("ERC1155 approval[%d] = %+v, want token=%s operator=%s approved=true", i, got, want[0], want[1])
		}
	}

	erc20[0].Amount.SetInt64(0)
	if erc20[1].Amount.Sign() == 0 {
		t.Fatal("approval amounts share mutable state")
	}

	amoyConfig, err := getContractConfig(AmoyChainID)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = requiredTradingApprovals(AmoyChainID, amoyConfig)
	if err == nil {
		t.Fatal("expected incomplete Amoy approval configuration")
	}
	if !strings.Contains(err.Error(), "trading approval configuration unavailable") {
		t.Fatalf("Amoy error = %v", err)
	}
}

func TestPrepareTradingApprovalsSkipsSatisfiedState(t *testing.T) {
	t.Parallel()

	rpc, calls := newTokenReadRPC(t, MaxUint256(), true)
	client, err := NewSignerClient(Config{
		ChainID:    PolygonChainID,
		PrivateKey: gaslessTestKey,
		RPCURL:     rpc.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	plan, err := client.PrepareTradingApprovals(t.Context())
	if err != nil {
		t.Fatalf("PrepareTradingApprovals: %v", err)
	}
	if !plan.Empty() {
		t.Fatalf("plan = %+v, want empty", plan)
	}
	if got := calls.Load(); got != 17 {
		t.Fatalf("eth_call count = %d, want 17", got)
	}
}

func TestPrepareTradingApprovalsReturnsMissingState(t *testing.T) {
	t.Parallel()

	rpc, calls := newTokenReadRPC(t, big.NewInt(0), false)
	client, err := NewSignerClient(Config{
		ChainID:    PolygonChainID,
		PrivateKey: gaslessTestKey,
		RPCURL:     rpc.URL,
	})
	if err != nil {
		t.Fatal(err)
	}

	plan, err := client.PrepareTradingApprovals(t.Context())
	if err != nil {
		t.Fatalf("PrepareTradingApprovals: %v", err)
	}
	if len(plan.ERC20Approvals) != 8 || len(plan.ERC1155Approvals) != 9 {
		t.Fatalf("plan = %d ERC20, %d ERC1155; want 8, 9", len(plan.ERC20Approvals), len(plan.ERC1155Approvals))
	}
	if got := calls.Load(); got != 17 {
		t.Fatalf("eth_call count = %d, want 17", got)
	}
}

func TestSetupTradingApprovalsGaslessBatchesMissingState(t *testing.T) {
	t.Parallel()

	rpc, calls := newTokenReadRPC(t, big.NewInt(0), false)
	var captured map[string]any
	relayer := newGaslessCTFMockRelayer(t, &captured)
	client := newGaslessClient(t, SignatureTypePolyGnosisSafe, relayer.URL)
	client.rpcURL = rpc.URL

	handle, err := client.SetupTradingApprovalsGasless(t.Context(), "")
	if err != nil {
		t.Fatalf("SetupTradingApprovalsGasless: %v", err)
	}
	if handle == nil || handle.TransactionID != "tx-ctf" {
		t.Fatalf("handle = %+v, want tx-ctf", handle)
	}
	if got := calls.Load(); got != 17 {
		t.Fatalf("eth_call count = %d, want 17", got)
	}
	if got, _ := captured["metadata"].(string); got != "Trading setup approvals" {
		t.Fatalf("metadata = %q, want default setup metadata", got)
	}
	data, _ := captured["data"].(string)
	want := "0x" + common.Bytes2Hex(selector("multiSend(bytes)"))
	if !strings.HasPrefix(data, want) {
		t.Fatalf("batch selector = %q, want prefix %q", data, want)
	}
}

func TestSetupTradingApprovalsGaslessReturnsNilWhenComplete(t *testing.T) {
	t.Parallel()

	rpc, calls := newTokenReadRPC(t, MaxUint256(), true)
	var relayerCalls atomic.Int32
	relayer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		relayerCalls.Add(1)
		http.Error(w, "unexpected relayer request", http.StatusInternalServerError)
	}))
	t.Cleanup(relayer.Close)
	client := newGaslessClient(t, SignatureTypePolyGnosisSafe, relayer.URL)
	client.rpcURL = rpc.URL

	handle, err := client.SetupTradingApprovalsGasless(t.Context(), "")
	if err != nil {
		t.Fatalf("SetupTradingApprovalsGasless: %v", err)
	}
	if handle != nil {
		t.Fatalf("handle = %+v, want nil when approvals are complete", handle)
	}
	if got := calls.Load(); got != 17 {
		t.Fatalf("eth_call count = %d, want 17", got)
	}
	if got := relayerCalls.Load(); got != 0 {
		t.Fatalf("relayer request count = %d, want 0", got)
	}
}

func newTokenReadRPC(t *testing.T, allowance *big.Int, approved bool) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	allowanceResult, err := tokenABI.Methods["allowance"].Outputs.Pack(allowance)
	if err != nil {
		t.Fatalf("pack allowance result: %v", err)
	}
	approvedResult, err := tokenABI.Methods["isApprovedForAll"].Outputs.Pack(approved)
	if err != nil {
		t.Fatalf("pack approval result: %v", err)
	}
	allowanceHex := "0x" + common.Bytes2Hex(allowanceResult)
	approvedHex := "0x" + common.Bytes2Hex(approvedResult)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode JSON-RPC request: %v", err)
			return
		}
		if request.Method != "eth_call" || len(request.Params) == 0 {
			t.Errorf("unexpected JSON-RPC request: %+v", request)
			return
		}
		var call struct {
			Input string `json:"input"`
		}
		if err := json.Unmarshal(request.Params[0], &call); err != nil {
			t.Errorf("decode eth_call: %v", err)
			return
		}
		calls.Add(1)
		result := allowanceHex
		approvalSelector := "0x" + common.Bytes2Hex(tokenABI.Methods["isApprovedForAll"].ID)
		if strings.HasPrefix(strings.ToLower(call.Input), strings.ToLower(approvalSelector)) {
			result = approvedHex
		}
		if len(request.ID) == 0 {
			request.ID = json.RawMessage("1")
		}
		_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":%q}`, request.ID, result)
	}))
	t.Cleanup(server.Close)
	return server, &calls
}
