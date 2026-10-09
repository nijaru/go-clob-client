package clob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func assertPublicWalletHeaders(t *testing.T, r *http.Request) {
	t.Helper()
	for name := range r.Header {
		upper := strings.ToUpper(name)
		if strings.HasPrefix(upper, "POLY_") || strings.HasPrefix(upper, "RELAYER_") ||
			upper == "AUTHORIZATION" {
			t.Errorf("credentials leaked to public wallet read: %s", name)
		}
	}
}

func TestPublicWalletDeploymentProbe(t *testing.T) {
	t.Parallel()
	var reads atomic.Int32
	wallet := common.HexToAddress("0x1234")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		assertPublicWalletHeaders(t, r)
		if r.Method != "GET" || r.URL.Path != "/deployed" ||
			r.URL.Query().Get("address") != wallet.Hex() {
			t.Errorf("unexpected probe: %s", r.URL)
		}
		if typ := r.URL.Query().Get("type"); typ != "" && typ != "WALLET" {
			t.Errorf("unexpected type: %s", typ)
		}
		fmt.Fprint(w, `{"deployed":true}`)
	}))
	t.Cleanup(server.Close)
	public, err := NewClient(Config{RelayerHost: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	for _, sig := range []SignatureType{SignatureTypePolyProxy, SignatureTypePolyGnosisSafe, SignatureTypePoly1271} {
		deployed, err := public.IsWalletDeployedAt(testRelayerKeyContext(t), wallet, sig)
		if err != nil || !deployed {
			t.Fatalf("probe %d: %t %v", sig, deployed, err)
		}
	}
	// Authenticated promotion must not attach either CLOB or relayer credentials.
	client := newGaslessClient(t, SignatureTypePoly1271, server.URL)
	client.funderAddress = wallet.Hex()
	client.useServerTime = true
	if deployed, err := client.IsWalletDeployedAt(testRelayerKeyContext(t), client.WalletAddress(), client.signatureType); err != nil ||
		!deployed {
		t.Fatalf("configured wallet: %t %v", deployed, err)
	}
	if reads.Load() != 4 {
		t.Fatal("missing reads")
	}
	if deployed, err := public.IsWalletDeployedAt(t.Context(), wallet, SignatureTypeEOA); err != nil ||
		deployed {
		t.Fatalf("EOA contract status: %t %v", deployed, err)
	}
	for _, sig := range []SignatureType{SignatureTypePoly1271, SignatureType(99)} {
		address := common.Address{}
		if sig == 99 {
			address = wallet
		}
		if _, err := public.IsWalletDeployedAt(t.Context(), address, sig); err == nil {
			t.Fatal("invalid probe accepted")
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := public.IsWalletDeployedAt(ctx, wallet, SignatureTypePoly1271); !errors.Is(
		err,
		context.Canceled,
	) {
		t.Fatal(err)
	}
	if reads.Load() != 4 {
		t.Fatal("local rejection made remote reads")
	}
}

func TestDiscoverDepositWallet(t *testing.T) {
	t.Parallel()
	owner := common.HexToAddress("0x1234")
	legacy, _ := DeriveUUPSDepositWallet(owner, PolygonChainID)
	beacon, _ := DeriveBeaconDepositWallet(owner, PolygonChainID)
	for _, test := range []struct {
		name, response string
		status         int
		want           common.Address
		fail           bool
	}{
		{"legacy", `{"deployed":true}`, 200, legacy, false},
		{"beacon", `{"deployed":false}`, 200, beacon, false},
		{"missing", `{}`, 200, common.Address{}, true},
		{"null", `{"deployed":null}`, 200, common.Address{}, true},
		{"malformed", `{"deployed":"false"}`, 200, common.Address{}, true},
		{"unavailable", `{"error":"unavailable"}`, 503, common.Address{}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var reads atomic.Int32
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					reads.Add(1)
					assertPublicWalletHeaders(t, r)
					if r.Method != "GET" || r.URL.Path != "/deployed" ||
						r.URL.Query().Get("address") != legacy.Hex() ||
						r.URL.Query().Get("type") != "WALLET" {
						t.Errorf("wrong discovery: %s", r.URL)
					}
					w.WriteHeader(test.status)
					fmt.Fprint(w, test.response)
				}),
			)
			t.Cleanup(server.Close)
			client := newGaslessClient(t, SignatureTypeEOA, server.URL)
			if reads.Load() != 0 {
				t.Fatal("constructor performed discovery")
			}
			original := client.WalletAddress()
			wallet, err := client.DiscoverDepositWallet(testRelayerKeyContext(t), owner)
			if (err != nil) != test.fail || wallet != test.want || reads.Load() != 1 {
				t.Fatalf("discovery: %s %v", wallet, err)
			}
			if client.WalletAddress() != original {
				t.Fatal("discovery switched identity")
			}
			client.chainID = AmoyChainID
			if _, err := client.DiscoverDepositWallet(t.Context(), owner); err == nil {
				t.Fatal("unsupported chain accepted")
			}
			if _, err := client.DiscoverDepositWallet(t.Context(), common.Address{}); err == nil {
				t.Fatal("zero owner accepted")
			}
			if reads.Load() != 1 {
				t.Fatal("invalid input made reads")
			}
		})
	}
}

func TestEOADepositWalletProbe(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, result string
		fail         bool
		beacon       bool
	}{
		{"beacon", `"0x0000000000000000000000000000000000000000000000000000000000000001"`, false, true},
		{"legacy", `"0x"`, false, false},
		{"malformed", `"0x01"`, true, false},
		{"rpc-error", ``, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := newGaslessClient(t, SignatureTypeEOA, "http://127.0.0.1:1")
			original := client.WalletAddress()
			want, _ := DeriveUUPSDepositWallet(client.signer.Address(), PolygonChainID)
			if test.beacon {
				want, _ = DeriveBeaconDepositWallet(client.signer.Address(), PolygonChainID)
			}
			var probes atomic.Int32
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assertPublicWalletHeaders(t, r)
					if r.URL.Path == "/deployed" {
						probes.Add(1)
						if r.Method != "GET" || r.URL.Query().Get("address") != want.Hex() ||
							r.URL.Query().Get("type") != "WALLET" {
							t.Errorf("wrong derived probe: %s", r.URL)
						}
						fmt.Fprint(w, `{"deployed":true}`)
						return
					}
					var req struct {
						ID     json.RawMessage
						Method string
						Params []json.RawMessage
					}
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						t.Error(err)
						return
					}
					var call struct{ To, Input string }
					if len(req.Params) != 2 {
						t.Error("wrong RPC params")
						return
					}
					if err := json.Unmarshal(req.Params[0], &call); err != nil {
						t.Error(err)
					}
					wc, _ := getWalletConfig(PolygonChainID)
					if req.Method != "eth_call" ||
						!strings.EqualFold(call.To, wc.DepositWalletFactory) ||
						call.Input != factoryBeaconSelector {
						t.Errorf("wrong factory RPC: %+v %+v", req, call)
					}
					if test.result == "" {
						fmt.Fprintf(
							w,
							`{"jsonrpc":"2.0","id":%s,"error":{"code":-32000,"message":"upstream unavailable"}}`,
							req.ID,
						)
					} else {
						fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":%s}`, req.ID, test.result)
					}
				}),
			)
			t.Cleanup(server.Close)
			client.rpcURL = server.URL
			client.relayerHost = server.URL
			deployed, err := client.IsDepositWalletDeployed(testRelayerKeyContext(t))
			if (err != nil) != test.fail || deployed == test.fail {
				t.Fatalf("derived probe: %t %v", deployed, err)
			}
			if test.fail && probes.Load() != 0 {
				t.Fatal("RPC failure fell back to a deployment probe")
			}
			if !test.fail && probes.Load() != 1 {
				t.Fatal("missing derived probe")
			}
			if client.WalletAddress() != original {
				t.Fatal("probe switched EOA wallet")
			}
		})
	}
}
