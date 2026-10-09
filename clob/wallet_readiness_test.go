package clob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

func TestEnsureWalletReadyDeployment(t *testing.T) {
	t.Parallel()
	for _, visible := range []bool{true, false} {
		t.Run(fmt.Sprint(visible), func(t *testing.T) {
			var client *AuthenticatedClient
			var submitted, confirmed, deploymentVisible atomic.Bool
			deploymentVisible.Store(visible)
			var deployChecks atomic.Int32
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assertRelayerKeyHeaders(t, r)
					switch r.URL.Path {
					case "/deployed":
						deployChecks.Add(1)
						if r.URL.Query().Get("address") != client.WalletAddress().Hex() ||
							r.URL.Query().Get("type") != "WALLET" {
							t.Error("deployment identity mismatch")
						}
						fmt.Fprintf(
							w,
							`{"deployed":%t}`,
							deploymentVisible.Load() && confirmed.Load(),
						)
					case "/submit":
						if submitted.Swap(true) {
							t.Error("duplicate deployment")
						}
						var body map[string]any
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Error(err)
							return
						}
						cfg, _ := getWalletConfig(PolygonChainID)
						if body["type"] != "WALLET-CREATE" || body["from"] != client.Address() ||
							body["to"] != common.HexToAddress(cfg.DepositWalletFactory).Hex() ||
							body["metadata"] != "deploy-explicitly" ||
							len(body) != 4 {
							t.Errorf("wrong unsigned deployment: %v", body)
						}
						fmt.Fprint(w, `{"state":"STATE_NEW","transactionID":"deploy"}`)
					case "/v1/account/transactions/deploy":
						confirmed.Store(true)
						fmt.Fprint(
							w,
							`{"state":"STATE_CONFIRMED","transaction_id":"deploy","transaction_hash":"0x0000000000000000000000000000000000000000000000000000000000000001"}`,
						)
					default:
						t.Errorf("unexpected endpoint %s", r.URL.Path)
						http.Error(w, "unexpected", 404)
					}
				}),
			)
			t.Cleanup(server.Close)
			client = newSessionOwner(t, server.URL)
			if submitted.Load() || deployChecks.Load() != 0 {
				t.Fatal("constructor made remote writes")
			}
			ctx, cancel := context.WithTimeout(testRelayerKeyContext(t), 250*time.Millisecond)
			defer cancel()
			result, err := client.EnsureWalletReady(ctx, "deploy-explicitly")
			if result == nil || result.Wallet != client.WalletAddress() ||
				result.Transaction == nil ||
				result.Transaction.TransactionID != "deploy" ||
				!confirmed.Load() ||
				deployChecks.Load() != 2 {
				t.Fatalf("readiness: %+v %v", result, err)
			}
			if visible {
				if err != nil || !result.Deployed {
					t.Fatalf("ready: %+v %v", result, err)
				}
				again, err := client.EnsureWalletReady(ctx, "must-not-deploy")
				if err != nil || again.Transaction != nil || !again.Deployed {
					t.Fatalf("already deployed: %+v %v", again, err)
				}
			} else {
				if !errors.Is(err, context.DeadlineExceeded) || result.Deployed || result.Transaction.TransactionHash == "" {
					t.Fatalf("confirmation must not prove visibility: %+v %v", result, err)
				}
				deploymentVisible.Store(true)
				if err := client.WaitWalletDeployed(testRelayerKeyContext(t)); err != nil {
					t.Fatalf("resume visibility wait: %v", err)
				}
			}
		})
	}
}

func TestWalletDeploymentOwnerBoundaries(t *testing.T) {
	t.Parallel()
	var writes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/deployed" {
			writes.Add(1)
			t.Error("unexpected mutation")
		}
		fmt.Fprint(w, `{"deployed":false}`)
	}))
	t.Cleanup(server.Close)
	for _, sig := range []SignatureType{SignatureTypeEOA, SignatureTypePolyProxy, SignatureTypePolyGnosisSafe, SignatureTypePoly1271} {
		client := newGaslessClient(t, sig, server.URL)
		if sig == SignatureTypePoly1271 {
			client.funderAddress = common.HexToAddress("0x123").Hex()
		}
		if _, err := client.DeployDepositWallet(t.Context(), ""); !errors.Is(
			err,
			ErrWalletDeploymentIdentity,
		) {
			t.Fatalf("unsafe target %d: %v", sig, err)
		}
		result, err := client.EnsureWalletReady(t.Context(), "")
		switch sig {
		case SignatureTypeEOA:
			if err != nil || !result.Deployed || result.Transaction != nil {
				t.Fatalf("EOA readiness: %+v %v", result, err)
			}
		case SignatureTypePoly1271:
			if !errors.Is(err, ErrWalletDeploymentIdentity) {
				t.Fatalf("session target: %v", err)
			}
		case SignatureTypePolyGnosisSafe:
			if !errors.Is(err, ErrSafeWalletDeploymentIdentity) {
				t.Fatalf("non-owner Safe target: %v", err)
			}
		default:
			if !errors.Is(err, ErrWalletDeploymentRequired) {
				t.Fatalf("unsupported deployment: %v", err)
			}
		}
	}
	client := newSessionOwner(t, server.URL)
	legacy, err := DeriveUUPSDepositWallet(client.signer.Address(), PolygonChainID)
	if err != nil {
		t.Fatal(err)
	}
	client.funderAddress = legacy.Hex()
	if _, err := client.EnsureWalletReady(t.Context(), ""); !errors.Is(
		err,
		ErrWalletDeploymentIdentity,
	) {
		t.Fatalf("must not create legacy UUPS: %v", err)
	}
	if writes.Load() != 0 {
		t.Fatal("rejected targets submitted mutations")
	}
}
