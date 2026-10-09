package clob

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestIndexedTradingApprovalsContract(t *testing.T) {
	t.Parallel()
	wallet := common.HexToAddress("0x1111111111111111111111111111111111111111")
	config, _ := getContractConfig(137)
	erc20, erc1155, err := requiredTradingApprovals(137, config)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"complete", "finite", "revoked", "duplicate", "missing", "wrong-standard", "malformed", "wrong-chain", "wrong-wallet"} {
		t.Run(scenario, func(t *testing.T) {
			rows := make([]indexedApprovalRow, 0, 16)
			for _, approval := range erc20 {
				rows = append(
					rows,
					indexedApprovalRow{
						Token:    approval.TokenAddress.Hex(),
						Spender:  approval.SpenderAddress.Hex(),
						Standard: "ERC20",
						Amount:   json.RawMessage(`"max"`),
						Approved: json.RawMessage(`true`),
					},
				)
			}
			for _, approval := range erc1155 {
				rows = append(
					rows,
					indexedApprovalRow{
						Token:    approval.TokenAddress.Hex(),
						Spender:  approval.OperatorAddress.Hex(),
						Standard: "ERC1155",
						Approved: json.RawMessage(`true`),
					},
				)
			}
			// The catalog is not SDK-owned: even malformed unrelated row fields
			// must not invalidate otherwise valid required approvals.
			rows = append(
				rows,
				indexedApprovalRow{
					Token:    "future-token",
					Spender:  "future-spender",
					Standard: "FUTURE",
					Amount:   json.RawMessage(`{}`),
					Approved: json.RawMessage(`"unknown"`),
				},
			)
			chain := 137
			address := wallet.Hex()
			switch scenario {
			case "finite":
				rows[0].Amount = json.RawMessage(`"1"`)
			case "revoked":
				rows[0].Approved = json.RawMessage(`false`)
			case "duplicate":
				rows = append(rows, rows[0])
			case "missing":
				rows = rows[1:]
			case "wrong-standard":
				rows[0].Standard = "ERC1155"
			case "malformed":
				rows[0].Amount = json.RawMessage(`"01"`)
			case "wrong-chain":
				chain = 80002
			case "wrong-wallet":
				address = common.HexToAddress("0x2").Hex()
			}
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/v2/approvals" || r.URL.Query().Get("user") != wallet.Hex() {
						t.Errorf("wrong approval query: %s", r.URL)
					}
					encoded, err := json.Marshal(rows)
					if err != nil {
						t.Error(err)
						return
					}
					fmt.Fprintf(
						w,
						`{"data":{"address":%q,"chain_id":%d,"contracts":%s}}`,
						address,
						chain,
						encoded,
					)
				}),
			)
			t.Cleanup(server.Close)
			reader, err := NewTradingApprovalsReader(
				TradingApprovalsReaderConfig{DataHost: server.URL},
			)
			if err != nil {
				t.Fatal(err)
			}
			state, err := reader.FetchTradingApprovalsState(t.Context(), wallet)
			if scenario == "complete" {
				if err != nil || !state.IsFullyApproved {
					t.Fatalf("complete snapshot: %+v %v", state, err)
				}
			} else if scenario == "finite" || scenario == "revoked" {
				if err != nil || state.IsFullyApproved || len(state.Missing.ERC20Approvals) != 1 {
					t.Fatalf("missing allowance: %+v %v", state, err)
				}
			} else if err == nil {
				t.Fatalf("%s snapshot accepted", scenario)
			}
		})
	}
}

func TestEOATradingApprovalsReadSigner(t *testing.T) {
	t.Parallel()
	client := newGaslessClient(t, SignatureTypeEOA, "http://127.0.0.1:1")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     json.RawMessage   `json:"id"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		var call struct {
			Input string `json:"input"`
		}
		if err := json.Unmarshal(request.Params[0], &call); err != nil {
			t.Error(err)
			return
		}
		input := common.FromHex(call.Input)
		if common.BytesToAddress(input[4:36]) != client.signer.Address() {
			t.Error("EOA approvals read zero/funder instead of signer")
		}
		result, err := tokenABI.Methods["allowance"].Outputs.Pack(MaxUint256())
		if string(input[:4]) == string(tokenABI.Methods["isApprovedForAll"].ID) {
			result, err = tokenABI.Methods["isApprovedForAll"].Outputs.Pack(true)
		}
		if err != nil {
			t.Error(err)
			return
		}
		fmt.Fprintf(
			w,
			`{"jsonrpc":"2.0","id":%s,"result":%q}`,
			request.ID,
			"0x"+common.Bytes2Hex(result),
		)
	}))
	t.Cleanup(server.Close)
	client.rpcURL = server.URL
	plan, err := client.PrepareTradingApprovals(t.Context())
	if err != nil || !plan.Empty() {
		t.Fatalf("EOA approvals: %+v %v", plan, err)
	}
}
