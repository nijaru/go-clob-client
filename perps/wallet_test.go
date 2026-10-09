package perps

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/nijaru/go-clob-client/clob"
)

type collateralRPC struct {
	requests atomic.Int32
	chain    atomic.Int64
	sent     atomic.Int32
	receipts atomic.Int32
	code     atomic.Bool
	pending  atomic.Bool
	revert   atomic.Bool
	failSend int32
	mu       sync.Mutex
	txs      []*types.Transaction
}

func newCollateralRPC(t *testing.T) (*collateralRPC, string) {
	t.Helper()
	f := &collateralRPC{}
	f.chain.Store(137)
	f.code.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		var req struct {
			ID     json.RawMessage
			Method string
			Params []json.RawMessage
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
			return
		}
		var result any
		switch req.Method {
		case "eth_chainId":
			result = fmt.Sprintf("0x%x", f.chain.Load())
		case "eth_getCode":
			result = "0x"
			if f.code.Load() {
				result = "0x6000"
			}
		case "eth_getTransactionCount":
			result = fmt.Sprintf("0x%x", f.sent.Load())
		case "eth_maxPriorityFeePerGas":
			result = "0x1"
		case "eth_getBlockByNumber":
			result = &types.Header{
				Number:     big.NewInt(1),
				Difficulty: big.NewInt(0),
				GasLimit:   1000000,
				BaseFee:    big.NewInt(1),
			}
		case "eth_estimateGas":
			result = "0x20000"
		case "eth_sendRawTransaction":
			var wire string
			if err := json.Unmarshal(req.Params[0], &wire); err != nil {
				t.Error(err)
				return
			}
			raw, err := hexutil.Decode(wire)
			if err != nil {
				t.Error(err)
				return
			}
			tx := new(types.Transaction)
			if err := tx.UnmarshalBinary(raw); err != nil {
				t.Error(err)
				return
			}
			n := f.sent.Add(1)
			f.mu.Lock()
			f.txs = append(f.txs, tx)
			f.mu.Unlock()
			if n == f.failSend {
				fmt.Fprintf(
					w,
					`{"jsonrpc":"2.0","id":%s,"error":{"code":-32000,"message":"lost submission result"}}`,
					req.ID,
				)
				return
			}
			result = tx.Hash().Hex()
		case "eth_getTransactionReceipt":
			f.receipts.Add(1)
			if !f.pending.Load() {
				var hash common.Hash
				if err := json.Unmarshal(req.Params[0], &hash); err != nil {
					t.Error(err)
					return
				}
				status := uint64(types.ReceiptStatusSuccessful)
				if f.revert.Load() {
					status = types.ReceiptStatusFailed
				}
				result = &types.Receipt{
					Status:      status,
					TxHash:      hash,
					BlockNumber: big.NewInt(1),
					Logs:        []*types.Log{},
				}
			}
		default:
			t.Errorf("unexpected RPC %s", req.Method)
			return
		}
		body, err := json.Marshal(result)
		if err != nil {
			t.Error(err)
			return
		}
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":%s}`, req.ID, body)
	}))
	t.Cleanup(server.Close)
	return f, server.URL
}

func collateralConfig(t *testing.T, rpc string, kind clob.SignatureType) CollateralWalletConfig {
	t.Helper()
	signer, err := NewOwnerSigner(fixturePrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	builder, err := clob.NewLocalBuilderAuth(
		clob.Credentials{Key: "builder", Secret: "c2VjcmV0", Passphrase: "p"},
	)
	if err != nil {
		t.Fatal(err)
	}
	return CollateralWalletConfig{
		Owner: OwnerConfig{
			Signer:          signer,
			CollateralToken: fixtureToken,
			DepositContract: fixtureDeposit,
		},
		Transactions: clob.Config{
			PrivateKey:    fixturePrivateKey,
			Credentials:   &clob.Credentials{Key: "clob", Secret: "c2VjcmV0", Passphrase: "p"},
			BuilderAuth:   builder,
			RPCURL:        rpc,
			SignatureType: kind,
			RelayerHost:   "http://127.0.0.1:1",
		},
	}
}

func mustCollateralWallet(t *testing.T, cfg CollateralWalletConfig) *CollateralWallet {
	t.Helper()
	wallet, err := NewCollateralWallet(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return wallet
}

// Golden call words mirror TS perpsDepositCall and Python funds.py at the
// README pins. The transaction recipient is the wallet, the credit recipient
// in calldata is the signer. Approval spender is the perps deposit contract.
func depositGolden() string {
	return "f45346dc" + strings.Repeat(
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
}

func approvalGolden() string {
	return "095ea7b3" + strings.Repeat("0", 63) + "4" + strings.Repeat("0", 56) + "05f5e100"
}

func TestCollateralEOAWorkflow(t *testing.T) {
	t.Parallel()
	f, rpc := newCollateralRPC(t)
	wallet := mustCollateralWallet(t, collateralConfig(t, rpc, clob.SignatureTypeEOA))
	if f.requests.Load() != 0 {
		t.Fatal("constructor performed network requests")
	}
	tx, err := wallet.ApproveAndDeposit(t.Context(), big.NewInt(100000000), "deposit")
	if err != nil {
		t.Fatal(err)
	}
	if len(tx.Submissions) != 2 || tx.Submissions[0].ConfirmedReceipt == nil ||
		tx.Submissions[1].ConfirmedReceipt != nil ||
		f.receipts.Load() != 1 {
		t.Fatal("EOA confirmation prefix missing")
	}
	f.mu.Lock()
	for i, want := range []string{approvalGolden(), depositGolden()} {
		call := f.txs[i]
		to := common.HexToAddress(fixtureToken)
		if i == 1 {
			to = common.HexToAddress(fixtureDeposit)
		}
		owner, err := types.Sender(types.LatestSignerForChainID(big.NewInt(137)), call)
		if err != nil || owner != wallet.signer.Address() || call.To() == nil || *call.To() != to ||
			hex.EncodeToString(call.Data()) != want ||
			call.Value().Sign() != 0 {
			t.Errorf("signed call %d differs from wire contract", i)
		}
	}
	f.mu.Unlock()
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() {
			receipts, err := tx.Wait(t.Context())
			if err != nil || len(receipts) != 2 {
				t.Errorf("concurrent wait: %v %v", receipts, err)
			}
		})
	}
	wg.Wait()
	if f.sent.Load() != 2 {
		t.Fatal("Wait resubmitted calls")
	}
	deposit, err := wallet.Deposit(t.Context(), big.NewInt(1), "")
	if err != nil || len(deposit.Submissions) != 1 || f.sent.Load() != 3 {
		t.Fatalf("standalone deposit implicitly approved: %+v %v", deposit, err)
	}
	revoke, err := wallet.ApproveCollateral(t.Context(), big.NewInt(0), "")
	if err != nil || len(revoke.Submissions) != 1 || f.sent.Load() != 4 {
		t.Fatalf("explicit revocation: %+v %v", revoke, err)
	}
	f.mu.Lock()
	if got := hex.EncodeToString(f.txs[3].Data()); got != "095ea7b3"+strings.Repeat(
		"0",
		63,
	)+"4"+strings.Repeat(
		"0",
		64,
	) {
		t.Errorf("revocation ABI: %s", got)
	}
	f.mu.Unlock()
}

func TestCollateralEOAPartialAndReceiptFailures(t *testing.T) {
	t.Parallel()
	f, rpc := newCollateralRPC(t)
	f.failSend = 2
	wallet := mustCollateralWallet(t, collateralConfig(t, rpc, clob.SignatureTypeEOA))
	tx, err := wallet.ApproveAndDeposit(t.Context(), big.NewInt(100000000), "")
	var submissionErr *CollateralSubmissionError
	if !errors.As(err, &submissionErr) || submissionErr.Operation != "deposit" || tx == nil ||
		len(tx.Submissions) != 1 ||
		tx.Submissions[0].ConfirmedReceipt == nil {
		t.Fatalf("lost confirmed approval prefix: %+v %v", tx, err)
	}
	if f.sent.Load() != 2 {
		t.Fatal("uncertain send retried")
	}
	// Wait only covers the recorded prefix, not the uncertain deposit.
	if receipts, err := tx.Wait(t.Context()); err != nil || len(receipts) != 1 {
		t.Fatalf("prefix wait: %v %v", receipts, err)
	}
	f.revert.Store(true)
	if receipts, err := tx.Wait(t.Context()); !errors.Is(err, ErrCollateralTransactionReverted) ||
		len(receipts) != 1 ||
		receipts[0].Status != 0 {
		t.Fatalf("revert: %v %v", receipts, err)
	}
	f.revert.Store(false)
	f.pending.Store(true)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()
	if _, err := tx.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("pending wait: %v", err)
	}
	if f.sent.Load() != 2 {
		t.Fatal("receipt retry sent another transaction")
	}
}

func TestCollateralEOAApprovalMustConfirmBeforeDeposit(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(map[bool]string{false: "reverted", true: "pending"}[pending], func(t *testing.T) {
			t.Parallel()
			f, rpc := newCollateralRPC(t)
			f.pending.Store(pending)
			f.revert.Store(!pending)
			wallet := mustCollateralWallet(t, collateralConfig(t, rpc, clob.SignatureTypeEOA))
			ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
			defer cancel()
			tx, err := wallet.ApproveAndDeposit(ctx, big.NewInt(1), "")
			want := ErrCollateralTransactionReverted
			if pending {
				want = context.DeadlineExceeded
			}
			if !errors.Is(err, want) || tx == nil || len(tx.Submissions) != 1 ||
				f.sent.Load() != 1 {
				t.Fatalf("deposit sent before successful approval receipt: %+v %v", tx, err)
			}
		})
	}
}

func TestCollateralValidationBeforeEffects(t *testing.T) {
	t.Parallel()
	f, rpc := newCollateralRPC(t)
	cfg := collateralConfig(t, rpc, clob.SignatureTypeEOA)
	wallet := mustCollateralWallet(t, cfg)
	for _, amount := range []*big.Int{nil, big.NewInt(0), big.NewInt(-1), new(big.Int).Lsh(big.NewInt(1), 256)} {
		if _, err := wallet.ApproveAndDeposit(t.Context(), amount, ""); err == nil {
			t.Fatal("invalid amount accepted")
		}
	}
	for _, change := range []func(*CollateralWalletConfig){
		func(c *CollateralWalletConfig) { c.Owner.Config.ChainID = 80002 },
		func(c *CollateralWalletConfig) { c.Owner.Wallet = fixtureRecipient },
		func(c *CollateralWalletConfig) { c.Owner.Signer = nil },
		func(c *CollateralWalletConfig) { c.Transactions.RPCURL = "" },
		func(c *CollateralWalletConfig) {
			c.Transactions.SignatureType = clob.SignatureTypePoly1271
			c.Transactions.FunderAddress = fixtureRecipient
		},
	} {
		copy := cfg
		change(&copy)
		if _, err := NewCollateralWallet(copy); err == nil {
			t.Fatal("invalid identity accepted")
		}
	}
	f.chain.Store(80002)
	if _, err := wallet.Deposit(t.Context(), big.NewInt(1), ""); err == nil {
		t.Fatal("wrong RPC chain accepted")
	}
	if f.sent.Load() != 0 {
		t.Fatal("invalid input caused a broadcast")
	}
}

func TestCollateralGaslessAndDeployment(t *testing.T) {
	for _, kind := range []clob.SignatureType{clob.SignatureTypePolyGnosisSafe, clob.SignatureTypePoly1271} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			t.Parallel()
			f, rpc := newCollateralRPC(t)
			var submits atomic.Int32
			var missingHash atomic.Bool
			var payloadMu sync.Mutex
			var payload string
			registry := atomic.Bool{}
			registry.Store(true)
			relay := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("POLY_BUILDER_API_KEY") != "builder" {
						t.Error("relayer auth is not builder auth")
					}
					switch r.URL.Path {
					case "/deployed":
						fmt.Fprintf(w, `{"deployed":%t}`, registry.Load())
					case "/v1/account/transactions/params":
						if r.URL.Query().Get("type") == "" {
							t.Error("missing wallet type")
						}
						fmt.Fprint(
							w,
							`{"address":"0x0000000000000000000000000000000000000001","nonce":"3"}`,
						)
					case "/submit":
						var raw json.RawMessage
						if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
							t.Error(err)
							return
						}
						payloadMu.Lock()
						payload = strings.ToLower(string(raw))
						payloadMu.Unlock()
						submits.Add(1)
						fmt.Fprint(w, `{"state":"STATE_NEW","transactionID":"tx-test"}`)
					case "/v1/account/transactions/tx-test":
						hash := common.HexToHash("0x1234").Hex()
						if missingHash.Load() {
							hash = ""
						}
						fmt.Fprintf(
							w,
							`{"state":"STATE_CONFIRMED","transaction_id":"tx-test","transaction_hash":"%s"}`,
							hash,
						)
					default:
						t.Errorf("unexpected relayer path %s", r.URL.Path)
						w.WriteHeader(404)
					}
				}),
			)
			t.Cleanup(relay.Close)
			cfg := collateralConfig(t, rpc, kind)
			cfg.Transactions.RelayerHost = relay.URL
			wallet := mustCollateralWallet(t, cfg)
			f.code.Store(false)
			state, err := wallet.Readiness(t.Context())
			if err != nil || state.OnChain || !state.Registered {
				t.Fatalf("registry confused with deployment: %+v %v", state, err)
			}
			if _, err := wallet.Deposit(t.Context(), big.NewInt(1), ""); err == nil ||
				submits.Load() != 0 {
				t.Fatal("deposit to undeployed wallet")
			}
			f.code.Store(true)
			registry.Store(false)
			tx, err := wallet.ApproveAndDeposit(t.Context(), big.NewInt(100000000), "test metadata")
			if err != nil || len(tx.Submissions) != 1 || submits.Load() != 1 || f.sent.Load() != 0 {
				t.Fatalf("atomic relay: %+v %v", tx, err)
			}
			payloadMu.Lock()
			if !strings.Contains(payload, approvalGolden()) ||
				!strings.Contains(payload, depositGolden()) ||
				strings.Index(
					payload,
					approvalGolden(),
				) > strings.Index(
					payload,
					depositGolden(),
				) ||
				!strings.Contains(payload, "test metadata") {
				t.Errorf("relayed ABI differs: %s", payload)
			}
			payloadMu.Unlock()
			missingHash.Store(true)
			if _, err := tx.Wait(t.Context()); err == nil {
				t.Fatal("hashless relay confirmation mistaken for receipt")
			}
			missingHash.Store(false)
			f.pending.Store(true)
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
			_, err = tx.Wait(ctx)
			cancel()
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("relayer confirmation mistaken for receipt: %v", err)
			}
			f.pending.Store(false)
			if receipts, err := tx.Wait(t.Context()); err != nil || len(receipts) != 1 {
				t.Fatalf("relay receipt: %v %v", receipts, err)
			}
			if kind == clob.SignatureTypePoly1271 {
				h, err := wallet.DeployDepositWallet(t.Context(), "explicit deploy")
				if err != nil || h.Submissions[0].TransactionID != "tx-test" {
					t.Fatalf("deploy: %v %v", h, err)
				}
				payloadMu.Lock()
				if !strings.Contains(payload, "wallet-create") ||
					strings.Contains(payload, "\"signature\"") {
					t.Errorf("deploy wire: %s", payload)
				}
				payloadMu.Unlock()
				if _, err := h.Wait(t.Context()); err != nil {
					t.Fatal(err)
				}
			} else if _, err := wallet.DeployDepositWallet(t.Context(), ""); err == nil {
				t.Fatal("safe deploy unsupported, must not infer success")
			}
		})
	}
}
