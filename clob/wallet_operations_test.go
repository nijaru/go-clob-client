package clob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func walletBalanceRPC(t *testing.T, owner common.Address, balances ...int64) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		if req.Method != "eth_call" || len(req.Params) != 2 {
			t.Errorf("unexpected balance RPC: %s", req.Method)
			return
		}
		var call struct {
			To    common.Address `json:"to"`
			Input string         `json:"input"`
		}
		if err := json.Unmarshal(req.Params[0], &call); err != nil {
			t.Error(err)
			return
		}
		input := common.FromHex(call.Input)
		method := "balanceOfBatch"
		if slices.Equal(input[:4], walletABI.Methods["balanceOf"].ID) {
			method = "balanceOf"
		}
		values, err := walletABI.Methods[method].Inputs.Unpack(input[4:])
		if err != nil {
			t.Error(err)
			return
		}
		var result []byte
		if method == "balanceOf" {
			if values[0].(common.Address) != owner {
				t.Error("balanceOf read wrong wallet")
			}
			result, err = walletABI.Methods[method].Outputs.Pack(big.NewInt(balances[0]))
		} else {
			for _, user := range values[0].([]common.Address) {
				if user != owner {
					t.Error("batch balance read wrong wallet")
				}
			}
			amounts := make([]*big.Int, len(balances))
			for i, value := range balances {
				amounts[i] = big.NewInt(value)
			}
			result, err = walletABI.Methods[method].Outputs.Pack(amounts)
		}
		if err != nil {
			t.Error(err)
			return
		}
		fmt.Fprintf(
			w,
			`{"jsonrpc":"2.0","id":%s,"result":%q}`,
			req.ID,
			"0x"+common.Bytes2Hex(result),
		)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestWalletPositionRoutingAndBalances(t *testing.T) {
	t.Parallel()
	v2, _ := ParseV2ConditionID("0x01" + strings.Repeat("11", 30))
	ctf := "0x" + strings.Repeat("ab", 32)
	for _, test := range []struct {
		name, version, condition string
		negRisk                  bool
		balances                 []int64
	}{
		{"ctf", "v1", ctf, false, []int64{8, 3}},
		{"neg-risk", "v1", ctf, true, []int64{8, 3}},
		{"native", "v2", v2.String(), false, []int64{8, 0}},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := newGaslessClient(t, SignatureTypePolyGnosisSafe, "http://127.0.0.1:1")
			rpc := walletBalanceRPC(t, client.WalletAddress(), test.balances...)
			client.rpcURL = rpc.URL
			market := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/markets/keyset" || r.URL.Query().Get("limit") != "1" ||
						(r.URL.Query().Get("condition_ids") != test.condition && r.URL.Query().Get("id") != "123") {
						t.Errorf("wrong Gamma query: %s", r.URL)
						return
					}
					ids := []string{"1", "2"}
					if test.version == "v2" {
						ids = []string{v2.PositionIDs()[0].String(), v2.PositionIDs()[1].String()}
					}
					fmt.Fprintf(
						w,
						`{"markets":[{"id":"123","version":%q,"conditionId":%q,"outcomes":"[\"YES\",\"NO\"]","negRisk":%t,"clobTokenIds":["1","2"],"positionIds":[%q,%q]}]}`,
						test.version,
						test.condition,
						test.negRisk,
						ids[0],
						ids[1],
					)
				}),
			)
			t.Cleanup(market.Close)
			ops, err := NewWalletOperations(client, WalletOperationsConfig{GammaHost: market.URL})
			if err != nil {
				t.Fatal(err)
			}
			split, err := ops.PrepareSplitPosition(
				t.Context(),
				WalletSplitRequest{ConditionID: test.condition, Amount: big.NewInt(7)},
			)
			if err != nil {
				t.Fatal(err)
			}
			cfg, _ := getContractConfig(137)
			wantTarget := cfg.CollateralAdapter
			wantSelector := "splitPosition"
			if test.negRisk {
				wantTarget = cfg.NegRiskCollateralAdapter
			}
			if test.version == "v2" {
				wantTarget = cfg.ProtocolV2Router
				wantSelector = "split"
			}
			if len(split) != 1 || split[0].To != common.HexToAddress(wantTarget) {
				t.Fatalf("wrong target: %+v", split)
			}
			if wantSelector == "split" {
				if !slices.Equal(split[0].Data[:4], walletABI.Methods[wantSelector].ID) {
					t.Fatal("wrong native selector")
				}
			} else if !slices.Equal(split[0].Data[:4], ctfABI.Methods[wantSelector].ID) {
				t.Fatal("wrong CTF selector")
			}
			merge, err := ops.PrepareMergePositions(
				t.Context(),
				WalletMergeRequest{ConditionID: test.condition},
			)
			if test.balances[1] == 0 {
				if !errors.Is(err, ErrInvalidPositionOperation) {
					t.Fatal(err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				values, err := ctfABI.Methods["mergePositions"].Inputs.Unpack(merge[0].Data[4:])
				if err != nil || values[4].(*big.Int).Int64() != 3 {
					t.Fatal("max merge did not use minimum balance")
				}
				if _, err := ops.PrepareMergePositions(t.Context(), WalletMergeRequest{ConditionID: test.condition, Amount: big.NewInt(4)}); !errors.Is(err, ErrInvalidPositionOperation) {
					t.Fatal("over-balance merge accepted")
				}
			}
			redeem, err := ops.PrepareRedeemPositions(
				t.Context(),
				WalletRedeemRequest{MarketID: "123"},
			)
			if err != nil || len(redeem) != 1 || redeem[0].To != common.HexToAddress(wantTarget) {
				t.Fatalf("redeem routing: %+v %v", redeem, err)
			}
			if test.version == "v2" {
				values, err := walletABI.Methods["redeem"].Inputs.Unpack(redeem[0].Data[4:])
				if err != nil || values[1].(*big.Int).Sign() != 0 ||
					values[2].(*big.Int).Int64() != 8 {
					t.Fatalf("redeem must use actual nonzero balance: %v %v", values, err)
				}
			}
		})
	}
}

func TestWalletComboSubmissionAndPositionRedemption(t *testing.T) {
	t.Parallel()
	bodies := make(chan RelayerSubmitRequest, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/account/transactions/params":
			fmt.Fprint(w, `{"nonce":"1"}`)
		case "/submit":
			var body RelayerSubmitRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				return
			}
			bodies <- body
			fmt.Fprint(w, `{"state":"STATE_NEW","transactionID":"position-tx"}`)
		default:
			t.Error(r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(server.Close)
	client := newSessionOwner(t, server.URL)
	rpc := walletBalanceRPC(t, client.WalletAddress(), 6, 4)
	client.rpcURL = rpc.URL
	ops, _ := NewWalletOperations(client, WalletOperationsConfig{})
	legs := []*big.Int{nativeTestPosition(2, 1), nativeTestPosition(1, 0)}
	combo, err := DeriveComboPositions(legs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ops.SplitPosition(t.Context(), WalletSplitRequest{Legs: legs, Amount: big.NewInt(7)}); err != nil {
		t.Fatal(err)
	}
	split := <-bodies
	cfg, _ := getContractConfig(137)
	if split.DepositWallet == nil || len(split.DepositWallet.Calls) != 2 ||
		split.DepositWallet.Calls[0].Target != common.HexToAddress(cfg.CombinatorialModule).Hex() ||
		split.DepositWallet.Calls[1].Target != common.HexToAddress(cfg.ProtocolV2Router).Hex() {
		t.Fatalf("wrong atomic combo calls: %+v", split)
	}
	if _, err := ops.MergePositions(t.Context(), WalletMergeRequest{Legs: legs}); err != nil {
		t.Fatal(err)
	}
	merge := <-bodies
	call := common.FromHex(merge.DepositWallet.Calls[1].Data)
	if new(big.Int).SetBytes(call[len(call)-32:]).Int64() != 4 {
		t.Fatal("combo max merge amount")
	}
	if _, err := ops.RedeemPositions(t.Context(), WalletRedeemRequest{PositionID: combo.PositionIDs[1]}); err != nil {
		t.Fatal(err)
	}
	redeem := <-bodies
	data := common.FromHex(redeem.DepositWallet.Calls[0].Data)
	values, err := walletABI.Methods["redeem"].Inputs.Unpack(data[4:])
	if err != nil || values[1].(*big.Int).Int64() != 1 || values[2].(*big.Int).Int64() != 6 {
		t.Fatalf("single native position redemption: %v %v", values, err)
	}
	if _, err := ops.MergeMultiplePositions(t.Context(), []WalletMergeRequest{{PositionID: combo.PositionIDs[0]}, {PositionID: combo.PositionIDs[1]}}, ""); !errors.Is(
		err,
		ErrInvalidPositionOperation,
	) {
		t.Fatal("duplicate conditions accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := ops.PrepareMergePositions(ctx, WalletMergeRequest{Legs: legs}); !errors.Is(
		err,
		context.Canceled,
	) {
		t.Fatalf("RPC cancellation: %v", err)
	}
}
