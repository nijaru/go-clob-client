package perps

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

const (
	fixtureToken     = "0x0000000000000000000000000000000000000002"
	fixtureRecipient = "0x0000000000000000000000000000000000000003"
	fixtureDeposit   = "0x0000000000000000000000000000000000000004"
)

func fixtureOwner(t *testing.T, host string) *OwnerClient {
	t.Helper()
	signer, err := NewOwnerSigner(fixturePrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := NewOwner(
		OwnerConfig{
			Config:          Config{Host: host, ChainID: 31337},
			Signer:          signer,
			Wallet:          fixtureRecipient,
			CollateralToken: fixtureToken,
			DepositContract: fixtureDeposit,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return owner
}

// The owner CreateProxy primary type and credentials expiry are pinned to
// actions/perps.ts and bindings/perps/account.ts, not account-model guesses.
func TestOwnerCredentialLifecycle(t *testing.T) {
	var proxy string
	var expiry int64
	var created atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method + " " + r.URL.Path {
		case "POST /v1/account/proxy":
			var c fixtureCommand
			if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
				t.Error(err)
				return
			}
			var args struct {
				Owner, Proxy string
				Expiry       int64
			}
			if err := json.Unmarshal(c.Op.Args, &args); err != nil {
				t.Error(err)
				return
			}
			if args.Owner != fixtureProxy || c.Op.Type != "createProxy" ||
				c.Label != "test credential" {
				t.Errorf("create body=%+v", c)
			}
			proxy = args.Proxy
			expiry = args.Expiry
			created.Add(1)
			data := perpsTypedData(
				31337,
				"CreateProxy",
				[]apitypes.Type{
					{Name: "addr", Type: "address"},
					{Name: "exp", Type: "uint64"},
					{Name: "salt", Type: "uint64"},
					{Name: "ts", Type: "uint64"},
				},
				apitypes.TypedDataMessage{
					"addr": proxy,
					"exp":  strconv.FormatInt(expiry, 10),
					"salt": strconv.FormatUint(c.Salt, 10),
					"ts":   strconv.FormatInt(c.Timestamp, 10),
				},
				"",
			)
			assertTypedSignature(t, c.Signature, data, fixtureProxy)
			_, _ = w.Write([]byte(`{"secret":"issued-secret"}`))
		case "GET /v1/account/credentials":
			if r.Header.Get("POLYMARKET-PROXY") != proxy ||
				r.Header.Get("POLYMARKET-SECRET") != "issued-secret" {
				t.Errorf("credential headers=%v", r.Header)
			}
			_ = json.NewEncoder(w).Encode(struct {
				Address string `json:"address"`
				Keys    []struct {
					Proxy  string `json:"proxy"`
					Expiry int64  `json:"expiry"`
				} `json:"keys"`
			}{fixtureProxy, []struct {
				Proxy  string `json:"proxy"`
				Expiry int64  `json:"expiry"`
			}{{proxy, expiry * 1000000}}})
		case "DELETE /v1/account/proxy":
			var c fixtureCommand
			_ = json.NewDecoder(r.Body).Decode(&c)
			assertOperationSignature(t, c, []any{"deleteProxy", []any{proxy}})
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			t.Errorf("request=%s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	owner := fixtureOwner(t, server.URL)
	credentials, err := owner.CreateCredentials(
		t.Context(),
		CreateCredentialsRequest{Lifetime: time.Hour, Label: "test credential"},
	)
	if err != nil {
		t.Fatal(err)
	}
	key, err := crypto.HexToECDSA(credentials.PrivateKey)
	if err != nil || crypto.PubkeyToAddress(key.PublicKey) != common.HexToAddress(proxy) ||
		credentials.ExpiresAt != expiry {
		t.Fatalf("created credential key/expiry mismatch: %v", err)
	}
	client, err := owner.Resume(t.Context(), credentials)
	if err != nil || client.Credentials().ExpiresAt != expiry {
		t.Fatalf("resume: %v", err)
	}
	if err := owner.RevokeCredentials(t.Context(), proxy); err != nil {
		t.Fatal(err)
	}
	if created.Load() != 1 {
		t.Fatalf("created %d keys", created.Load())
	}
}

func TestCreateCredentialsRetainsKeyOnUncertainSubmission(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream unavailable"}`))
	}))
	t.Cleanup(server.Close)
	credentials, err := fixtureOwner(
		t,
		server.URL,
	).CreateCredentials(t.Context(), CreateCredentialsRequest{})
	if err == nil || credentials.PrivateKey == "" || !common.IsHexAddress(credentials.Proxy) ||
		credentials.ExpiresAt <= time.Now().UnixMilli() {
		t.Fatal("uncertain credential submission lost generated key material")
	}
}

func TestResumeRejectsWrongOwnerAndExpiredCredentials(t *testing.T) {
	for _, tc := range []struct {
		name, owner string
		expiry      int64
	}{{"wrong_owner", fixtureRecipient, time.Now().Add(time.Hour).UnixMilli()}, {"expired", fixtureProxy, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, _ = w.Write(
						[]byte(
							`{"address":"` + tc.owner + `","keys":[{"proxy":"` + fixtureProxy + `","expiry":` + strconv.FormatInt(
								tc.expiry,
								10,
							) + `}]}`,
						),
					)
				}),
			)
			defer server.Close()
			_, err := fixtureOwner(
				t,
				server.URL,
			).Resume(t.Context(), PerpsCredentials{Proxy: fixtureProxy, PrivateKey: fixturePrivateKey, Secret: "secret"})
			if err == nil {
				t.Fatal("invalid credentials resumed")
			}
		})
	}
	if _, err := NewAuthenticated(AuthenticatedConfig{Credentials: PerpsCredentials{Proxy: fixtureRecipient, PrivateKey: fixturePrivateKey, Secret: "secret"}}); err == nil {
		t.Fatal("mismatched delegated signing key accepted")
	}
}

func TestCollateralOwnerSigningAndExactTransfer(t *testing.T) {
	var transfers atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var c fixtureCommand
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			t.Error(err)
			return
		}
		if r.Header.Get("POLYMARKET-SECRET") != "" {
			t.Error("owner request unexpectedly used delegated credentials")
		}
		switch r.URL.Path {
		case "/v1/account/internal-transfer":
			transfers.Add(1)
			var args struct{ Account, Token, Amount, To string }
			_ = json.Unmarshal(c.Op.Args, &args)
			if c.Label != "treasury-42" || args.Amount != "100.00" ||
				args.Account != fixtureProxy ||
				args.Token != fixtureToken ||
				args.To != fixtureRecipient {
				t.Errorf("transfer args=%+v label=%s", args, c.Label)
			}
			assertOperationSignature(
				t,
				c,
				[]any{
					"internalTransfer",
					[]any{fixtureProxy, fixtureToken, "100.00", fixtureRecipient},
				},
			)
			_, _ = w.Write([]byte(`{"status":"ok","transfer_id":42}`))
		case "/v1/account/withdraw":
			var args struct{ Account, Token, Amount, To string }
			_ = json.Unmarshal(c.Op.Args, &args)
			if args.Amount != "100000000" || args.To != fixtureRecipient {
				t.Errorf("withdraw args=%+v", args)
			}
			data := perpsTypedData(
				31337,
				"Withdraw",
				[]apitypes.Type{
					{Name: "account", Type: "address"},
					{Name: "token", Type: "address"},
					{Name: "amount", Type: "uint256"},
					{Name: "fee", Type: "uint256"},
					{Name: "to", Type: "address"},
					{Name: "salt", Type: "uint64"},
					{Name: "ts", Type: "uint64"},
				},
				apitypes.TypedDataMessage{
					"account": fixtureProxy,
					"token":   fixtureToken,
					"amount":  "100000000",
					"fee":     "0",
					"to":      fixtureRecipient,
					"salt":    strconv.FormatUint(c.Salt, 10),
					"ts":      strconv.FormatInt(c.Timestamp, 10),
				},
				fixtureDeposit,
			)
			if c.Timestamp > time.Now().Unix()+1 || c.Timestamp < time.Now().Unix()-1 {
				t.Error("withdraw used milliseconds instead of seconds")
			}
			assertTypedSignature(t, c.Signature, data, fixtureProxy)
			_, _ = w.Write([]byte(`{"status":"ok","withdraw_id":8}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	owner := fixtureOwner(t, server.URL)
	id, err := owner.TransferCollateral(
		t.Context(),
		TransferCollateralRequest{
			Recipient: fixtureRecipient,
			Amount:    "100.00",
			Label:     "treasury-42",
		},
	)
	if err != nil || id != 42 {
		t.Fatalf("transfer=%d %v", id, err)
	}
	id, err = owner.Withdraw(t.Context(), big.NewInt(100000000))
	if err != nil || id != 8 {
		t.Fatalf("withdraw=%d %v", id, err)
	}
	if _, err := owner.TransferCollateral(t.Context(), TransferCollateralRequest{Recipient: fixtureRecipient, Amount: "1", Label: strings.Repeat("é", 33)}); err == nil ||
		transfers.Load() != 1 {
		t.Fatal("oversized UTF-8 label submitted")
	}
	call, err := owner.PrepareDeposit(big.NewInt(100000000))
	if err != nil {
		t.Fatal(err)
	}
	// ABI declaration in client/src/abis.ts: deposit(address,uint256,address).
	want := "f45346dc" + strings.Repeat(
		"0",
		63,
	) + "2" + strings.Repeat(
		"0",
		56,
	) + "05f5e100" + strings.Repeat(
		"0",
		24,
	) + strings.ToLower(
		strings.TrimPrefix(fixtureProxy, "0x"),
	)
	if call.To.Hex() != common.HexToAddress(fixtureDeposit).Hex() || call.Value.Sign() != 0 ||
		hex.EncodeToString(call.Data) != want {
		t.Fatalf("deposit ABI=%x", call.Data)
	}
}

func TestTransfersNeverAutomaticallyRetryUnknownOutcome(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"unknown outcome"}`))
	}))
	t.Cleanup(server.Close)
	_, err := fixtureOwner(
		t,
		server.URL,
	).TransferCollateral(t.Context(), TransferCollateralRequest{Recipient: fixtureRecipient, Amount: "1", Label: "reconcile-me"})
	if err == nil || attempts.Load() != 1 {
		t.Fatalf("error=%v attempts=%d", err, attempts.Load())
	}
}

func TestSessionBuilderConsentClampsAndRevokes(t *testing.T) {
	approved := "0.0002"
	var version int64 = 1
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/account/credentials":
			_, _ = w.Write([]byte(`{"address":"` + fixtureProxy + `","keys":[]}`))
		case "/v1/info/builder":
			_, _ = w.Write(
				[]byte(
					`{"address":"` + fixtureRecipient + `","registered":true,"enabled":true,"admission_enabled":true,"max_fee_rate":"0.0005"}`,
				),
			)
		case "/v1/account/builder-approvals":
			if r.Method == http.MethodPost {
				var c fixtureCommand
				_ = json.NewDecoder(r.Body).Decode(&c)
				var args struct {
					Builder string `json:"builder"`
					Rate    string `json:"max_fee_rate"`
					Version int64  `json:"approval_version"`
				}
				_ = json.Unmarshal(c.Op.Args, &args)
				if args.Version != version+1 {
					t.Errorf("version=%d want %d", args.Version, version+1)
				}
				assertOperationSignature(
					t,
					c,
					[]any{"approveBuilder", []any{fixtureRecipient, args.Rate, args.Version}},
				)
				approved = args.Rate
				version = args.Version
				_ = json.NewEncoder(w).
					Encode(PerpsBuilderApproval{Trader: fixtureProxy, Builder: fixtureRecipient, MaxFeeRate: approved, ApprovalVersion: version, Timestamp: 1, Sequence: 1})
			} else {
				_ = json.NewEncoder(w).Encode(struct {
					Data []PerpsBuilderApproval `json:"data"`
				}{[]PerpsBuilderApproval{{Trader: fixtureProxy, Builder: fixtureRecipient, MaxFeeRate: approved, ApprovalVersion: version, Timestamp: 1, Sequence: 1}}})
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	client := fixtureClient(t, Config{Host: server.URL})
	s := &Session{client: client}
	if err := s.RefreshBuilder(t.Context(), fixtureRecipient); err != nil {
		t.Fatal(err)
	}
	if s.builderTerms().FeeRate != "0.0002" {
		t.Fatal("builder fee exceeded saved consent")
	}
	owner := fixtureOwner(t, server.URL)
	approval, err := s.ApproveBuilderFee(
		t.Context(),
		owner,
		BuilderConsentRequest{MaxFeeRate: "0.0010"},
	)
	if err != nil || approval.ApprovalVersion != 2 || s.builderTerms().FeeRate != "0.0005" {
		t.Fatalf("approval=%+v terms=%+v err=%v", approval, s.builderTerms(), err)
	}
	if _, err := s.RevokeBuilderFee(t.Context(), owner, ""); err != nil {
		t.Fatal(err)
	}
	if s.builderTerms() != nil {
		t.Fatal("revoked builder attribution remained enabled")
	}
}
