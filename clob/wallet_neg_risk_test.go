package clob

import (
	"bytes"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

// Independent reference vectors from Polymarket/ts-sdk
// packages/client/src/abis.test.ts (merged default branch).
func TestNegRiskRouterOperationVectors(t *testing.T) {
	t.Parallel()
	const eventID = "0x0211111111111111111111111111111111000300000000000000000000"
	const router = "0x12121212006e4CD160D18e3f00711DA5c3372600"
	w := &WalletOperations{client: &SignerClient{Client: &Client{chainID: PolygonChainID}}}
	amount := big.NewInt(1_000_000)
	req := WalletNegRiskRequest{ConditionID: eventID + "0003", Amount: amount}
	for _, tc := range []struct {
		method string
		build  func(WalletNegRiskRequest) ([]TransactionCall, error)
	}{
		{"convert", w.PrepareNegRiskConvert},
		{"horizontalSplit", w.PrepareNegRiskHorizontalSplit},
		{"horizontalMerge", w.PrepareNegRiskHorizontalMerge},
	} {
		t.Run(tc.method, func(t *testing.T) {
			calls, err := tc.build(req)
			if err != nil {
				t.Fatal(err)
			}
			if len(calls) != 1 || calls[0].To != common.HexToAddress(router) {
				t.Fatalf("invalid router target: %+v", calls)
			}
			method := walletABI.Methods[tc.method]
			if !bytes.Equal(calls[0].Data[:4], method.ID) {
				t.Fatalf("invalid method selector: %x", calls[0].Data[:4])
			}
			decoded, err := method.Inputs.Unpack(calls[0].Data[4:])
			if err != nil {
				t.Fatal(err)
			}
			if len(decoded) < 2 || decoded[len(decoded)-1].(*big.Int).Cmp(amount) != 0 {
				t.Fatalf("amount changed in call: %+v", decoded)
			}
			if tc.method == "convert" && decoded[1].(uint16) != 3 {
				t.Fatalf("condition index is not the Other index: %+v", decoded)
			}
			// A zero-padded bytes32 condition must encode identically.
			padded := req
			padded.ConditionID += "00"
			again, err := tc.build(padded)
			if err != nil || !bytes.Equal(again[0].Data, calls[0].Data) {
				t.Fatalf("non-canonical padded condition: %v", err)
			}
		})
	}
}

func TestNegRiskRouterRejectsInvalidContracts(t *testing.T) {
	t.Parallel()
	const eventID = "0x0211111111111111111111111111111111000300000000000000000000"
	w := &WalletOperations{client: &SignerClient{Client: &Client{chainID: PolygonChainID}}}
	valid := eventID + "0001"
	for _, id := range []string{
		"123", "0x01" + valid[4:],
		eventID + "0004",
		eventID + "0001" + "01",
		strings.Replace(eventID, "00030000", "00010000", 1) + "0001",
		strings.Replace(eventID, "0000000000", "0000000001", 1) + "0001",
	} {
		if _, err := w.PrepareNegRiskConvert(WalletNegRiskRequest{
			ConditionID: id, Amount: big.NewInt(1),
		}); err == nil {
			t.Fatalf("accepted invalid neg-risk condition %s", id)
		}
	}
	for _, amount := range []*big.Int{
		nil, big.NewInt(0), big.NewInt(-1), new(big.Int).Lsh(big.NewInt(1), 256),
	} {
		if _, err := w.PrepareNegRiskHorizontalSplit(WalletNegRiskRequest{
			ConditionID: valid, Amount: amount,
		}); err == nil {
			t.Fatalf("accepted invalid amount %v", amount)
		}
	}
	w.client.chainID = 80002 // no supported V2 router on Amoy
	if _, err := w.PrepareNegRiskConvert(WalletNegRiskRequest{
		ConditionID: valid, Amount: big.NewInt(1),
	}); err == nil {
		t.Fatal("accepted neg-risk operation without a configured router")
	}
}
