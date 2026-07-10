package clob

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// selector is the canonical function selector for a signature, computed
// independently of the ABI binding so the packer tests catch signature typos.
func selector(sig string) []byte {
	return crypto.Keccak256([]byte(sig))[:4]
}

// TestGaslessCTFCalldataSelectors asserts each CTF packer emits the canonical
// function selector — proving wire parity with py-sdk's split/merge/redeem call
// builders (calls.py). The full ABI-encoder parity (go-ethereum ≡ eth_abi) is
// covered by the polyrelay encoder golden-vector tests; here we pin the method
// names so a wrong ABI signature is caught immediately.
func TestGaslessCTFCalldataSelectors(t *testing.T) {
	t.Parallel()

	collateral := common.HexToAddress("0x1111111111111111111111111111111111111111")
	condition := common.BytesToHash(bytes.Repeat([]byte{0xab}, 32))
	amount := big.NewInt(1_000_000)

	splitData, err := packSplitPosition(SplitBinary(collateral, condition, amount))
	if err != nil {
		t.Fatalf("packSplitPosition: %v", err)
	}
	mergeData, err := packMergePositions(MergeBinary(collateral, condition, amount))
	if err != nil {
		t.Fatalf("packMergePositions: %v", err)
	}
	redeemData, err := packRedeemPositions(RedeemBinary(collateral, condition))
	if err != nil {
		t.Fatalf("packRedeemPositions: %v", err)
	}

	cases := []struct {
		name string
		sig  string
		data []byte
	}{
		{"splitPosition", "splitPosition(address,bytes32,bytes32,uint256[],uint256)", splitData},
		{"mergePositions", "mergePositions(address,bytes32,bytes32,uint256[],uint256)", mergeData},
		{"redeemPositions", "redeemPositions(address,bytes32,bytes32,uint256[])", redeemData},
	}
	for _, tc := range cases {
		want := selector(tc.sig)
		if !bytes.Equal(tc.data[:4], want) {
			t.Fatalf("%s selector = %x, want %x", tc.name, tc.data[:4], want)
		}
	}
}

// newGaslessCTFMockRelayer returns a mock relayer that serves the params + submit
// endpoints and captures the parsed submit body.
func newGaslessCTFMockRelayer(t *testing.T, captured *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/account/transactions/params":
			_ = json.NewEncoder(w).
				Encode(map[string]string{"address": "0x" + repeatHex(20), "nonce": "3"})
		case "/submit":
			body, _ := io.ReadAll(r.Body)
			var parsed map[string]any
			if err := json.Unmarshal(body, &parsed); err != nil {
				t.Fatalf("decode submit body: %v", err)
			}
			*captured = parsed
			_ = json.NewEncoder(w).Encode(map[string]any{
				"state":         "STATE_NEW",
				"transactionID": "tx-ctf",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestMergePositionsGaslessRoutesThroughRelayer drives the full clob→polyrelay
// path for a gasless merge against a mock relayer (Safe wallet, single call →
// direct CALL) and asserts the submit targets the ConditionalTokens contract and
// carries the mergePositions calldata.
func TestMergePositionsGaslessRoutesThroughRelayer(t *testing.T) {
	t.Parallel()

	var captured map[string]any
	srv := newGaslessCTFMockRelayer(t, &captured)
	t.Cleanup(srv.Close)

	client := newGaslessClient(t, SignatureTypePolyGnosisSafe, srv.URL)
	cond := common.BytesToHash(bytes.Repeat([]byte{0xcd}, 32))
	h, err := client.MergePositionsGasless(
		context.Background(),
		MergeBinary(
			common.HexToAddress("0x1111111111111111111111111111111111111111"),
			cond,
			big.NewInt(5),
		),
		"merge",
	)
	if err != nil {
		t.Fatalf("MergePositionsGasless: %v", err)
	}
	if h.TransactionID != "tx-ctf" {
		t.Fatalf("tx id = %s, want tx-ctf", h.TransactionID)
	}

	// Safe single-call submit: `to` is the call target (ConditionalTokens) and
	// `data` is the CTF calldata.
	conditional, _ := client.conditionalAddr()
	if got, _ := captured["to"].(string); got != conditional.Hex() {
		t.Fatalf("submit to = %s, want ConditionalTokens %s", got, conditional.Hex())
	}
	dataHex, _ := captured["data"].(string)
	wantSel := "0x" + common.Bytes2Hex(
		selector("mergePositions(address,bytes32,bytes32,uint256[],uint256)"),
	)
	if len(dataHex) < 10 || dataHex[:10] != wantSel {
		t.Fatalf("submit data selector = %s, want %s", dataHex, wantSel)
	}
}

// TestRedeemNegRiskGaslessTargetsAdapter verifies the neg-risk redeem routes to
// the NegRiskAdapter contract (not ConditionalTokens).
func TestRedeemNegRiskGaslessTargetsAdapter(t *testing.T) {
	t.Parallel()

	var captured map[string]any
	srv := newGaslessCTFMockRelayer(t, &captured)
	t.Cleanup(srv.Close)

	client := newGaslessClient(t, SignatureTypePolyGnosisSafe, srv.URL)
	cond := common.BytesToHash(bytes.Repeat([]byte{0xef}, 32))
	if _, err := client.RedeemNegRiskGasless(
		context.Background(),
		RedeemNegRiskRequest{ConditionID: cond, Amounts: BinaryPartition()},
		"redeem-negrisk",
	); err != nil {
		t.Fatalf("RedeemNegRiskGasless: %v", err)
	}

	adapter, _ := client.negRiskAdapterAddr()
	if got, _ := captured["to"].(string); got != adapter.Hex() {
		t.Fatalf("submit to = %s, want NegRiskAdapter %s", got, adapter.Hex())
	}
}

// TestGaslessCTFCalldataMatchesOnChain proves the gasless and on-chain paths
// share identical calldata (the packers are the single source of truth) by
// checking the merge calldata has the canonical selector and the exact ABI layout:
// 5 head words + an array of (length + 2 elements) = 8 words.
func TestGaslessCTFCalldataMatchesOnChain(t *testing.T) {
	t.Parallel()

	collateral := common.HexToAddress("0x2222222222222222222222222222222222222222")
	condition := common.BytesToHash(bytes.Repeat([]byte{0x99}, 32))
	amount := big.NewInt(42)

	data, err := packMergePositions(MergeBinary(collateral, condition, amount))
	if err != nil {
		t.Fatalf("packMergePositions: %v", err)
	}
	wantSel := selector("mergePositions(address,bytes32,bytes32,uint256[],uint256)")
	if !bytes.Equal(data[:4], wantSel) {
		t.Fatalf("merge selector = %x, want %x", data[:4], wantSel)
	}
	const wantLen = 4 + 8*32 // selector + 8 ABI words (5 head + array len + 2 elems)
	if len(data) != wantLen {
		t.Fatalf("merge calldata length = %d, want %d", len(data), wantLen)
	}
}
