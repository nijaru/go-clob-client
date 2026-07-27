package clob

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func newCollateralReturnClient(t *testing.T, host string, sig SignatureType) *AuthenticatedClient {
	t.Helper()
	cfg := Config{
		ChainID:              PolygonChainID,
		PrivateKey:           gaslessTestKey,
		SignatureType:        sig,
		Credentials:          &Credentials{Key: "k", Secret: "c2VjcmV0", Passphrase: "p"},
		RelayerHost:          host,
		CollateralReturnHost: host,
		RPCURL:               "http://127.0.0.1:1",
		DisableAutoHeartbeat: true,
	}
	if sig != SignatureTypeEOA {
		cfg.FunderAddress = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	}
	client, err := NewAuthenticatedClient(cfg)
	if err != nil {
		t.Fatalf("NewAuthenticatedClient: %v", err)
	}
	return client
}

func collateralReturnPlanJSON(wallet string) map[string]any {
	return map[string]any{
		"plan_hash":              "0xplan",
		"chain_id":               137,
		"wallet":                 wallet,
		"block_number":           "123",
		"starting_pusd":          "10.000000",
		"net_pusd_out":           "1.000000",
		"final_pusd":             "11.000000",
		"operations":             []map[string]any{{"kind": "merge", "amount": "1000000"}},
		"operation_count":        1,
		"truncated":              false,
		"estimated_cost":         0.001,
		"required_pusd_input":    "0",
		"required_positions":     []map[string]any{},
		"position_summary":       map[string]any{"consumed": []any{}, "created": []any{}},
		"candidate_position_ids": []string{"position-1"},
		"router_call": map[string]any{
			"to":   "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			"data": "0x1234",
		},
	}
}

func TestPlanCollateralReturn(t *testing.T) {
	wallet := "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	var sawBuilderHeaders bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != collateralReturnPlanEndpoint || r.Method != http.MethodPost {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("POLY_BUILDER_API_KEY") == "" {
			sawBuilderHeaders = false
		} else {
			sawBuilderHeaders = true
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request: %v", err)
		}
		var request map[string]string
		if err := json.Unmarshal(body, &request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request["wallet"] != common.HexToAddress(wallet).Hex() {
			t.Fatalf("wallet = %q, want %q", request["wallet"], common.HexToAddress(wallet).Hex())
		}
		_ = json.NewEncoder(w).Encode(collateralReturnPlanJSON(wallet))
	}))
	defer server.Close()

	client := newCollateralReturnClient(t, server.URL, SignatureTypePolyGnosisSafe)
	plan, err := client.PlanCollateralReturn(t.Context())
	if err != nil {
		t.Fatalf("PlanCollateralReturn: %v", err)
	}
	if !sawBuilderHeaders {
		t.Fatal("collateral return request missing builder auth headers")
	}
	if plan.PlanHash != "0xplan" || plan.NetPUSDOut != "1.000000" {
		t.Fatalf("plan = %+v", plan)
	}
	if plan.Operations[0].Kind != CollateralReturnMerge {
		t.Fatalf("operation kind = %q, want %q", plan.Operations[0].Kind, CollateralReturnMerge)
	}
}

func TestExecuteCollateralReturnPlanBuildsEnvelope(t *testing.T) {
	wallet := common.HexToAddress("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	router := common.HexToAddress("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/account/transactions/params":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"address": "0xcccccccccccccccccccccccccccccccccccccc",
				"nonce":   "3",
			})
		case collateralReturnSubmitEndpoint:
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read submit body: %v", err)
			}
			if err := json.Unmarshal(body, &captured); err != nil {
				t.Fatalf("decode submit body: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"state":         "STATE_NEW",
				"transactionID": "tx-collateral-return",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newCollateralReturnClient(t, server.URL, SignatureTypePolyGnosisSafe)
	handle, err := client.ExecuteCollateralReturnPlan(t.Context(), CollateralReturnPlan{
		PlanHash: "0xplan",
		ChainID:  PolygonChainID,
		Wallet:   wallet,
		RouterCall: CollateralReturnRouterCall{
			To:   router,
			Data: "0x1234",
		},
	})
	if err != nil {
		t.Fatalf("ExecuteCollateralReturnPlan: %v", err)
	}
	if handle.TransactionID != "tx-collateral-return" {
		t.Fatalf("transaction id = %q", handle.TransactionID)
	}

	if captured["plan_hash"] != "0xplan" {
		t.Fatalf("plan hash = %v", captured["plan_hash"])
	}
	envelope, ok := captured["envelope"].(map[string]any)
	if !ok {
		t.Fatalf("envelope = %T %v", captured["envelope"], captured["envelope"])
	}
	if envelope["type"] != "SAFE" || envelope["to"] != router.Hex() {
		t.Fatalf("envelope type/to = %v/%v", envelope["type"], envelope["to"])
	}
	if envelope["data"] != "0x1234" {
		t.Fatalf("envelope data = %v", envelope["data"])
	}
}

func TestCollateralReturnRejectsEOA(t *testing.T) {
	client := newCollateralReturnClient(t, "http://127.0.0.1:1", SignatureTypeEOA)
	_, err := client.PlanCollateralReturn(t.Context())
	if !errors.Is(err, ErrCollateralReturnUnsupportedWallet) {
		t.Fatalf("error = %v, want ErrCollateralReturnUnsupportedWallet", err)
	}
}

func TestExecuteCollateralReturnPlanRejectsMismatch(t *testing.T) {
	client := newCollateralReturnClient(t, "http://127.0.0.1:1", SignatureTypePolyGnosisSafe)
	_, err := client.ExecuteCollateralReturnPlan(t.Context(), CollateralReturnPlan{
		PlanHash: "0xplan",
		ChainID:  PolygonChainID,
		Wallet:   common.HexToAddress("0xdddddddddddddddddddddddddddddddddddddd"),
	})
	if !errors.Is(err, ErrCollateralReturnPlanMismatch) {
		t.Fatalf("error = %v, want ErrCollateralReturnPlanMismatch", err)
	}
}
