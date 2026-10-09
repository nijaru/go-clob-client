package perps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/nijaru/go-clob-client/clob"
	"github.com/nijaru/go-clob-client/signing"
)

// Only the remote boundary is substituted; commands use real HTTP/RPC and
// signatures are produced and independently recovered with the fixture key.
type externalTypedSigner struct {
	address      common.Address
	addressCalls atomic.Int32
	callback     func(context.Context, apitypes.TypedData) ([]byte, error)
}

func (s *externalTypedSigner) Address() common.Address {
	s.addressCalls.Add(1)
	return s.address
}

func (s *externalTypedSigner) SignTypedData(
	ctx context.Context,
	data apitypes.TypedData,
) ([]byte, error) {
	return s.callback(ctx, data)
}

func externalFixtureSigner(t *testing.T) (*externalTypedSigner, *signing.LocalSigner) {
	t.Helper()
	local, err := signing.NewLocalSigner(fixturePrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	return &externalTypedSigner{address: local.Address(), callback: local.SignTypedData}, local
}

func TestExternalOwnerAndDelegatedHTTP(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.URL.Path {
		case "/v1/account/proxy":
			var c fixtureCommand
			if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
				t.Error(err)
				return
			}
			assertOperationSignature(t, c, []any{"deleteProxy", []any{fixtureRecipient}})
			fmt.Fprint(w, `{"status":"ok"}`)
		case "/v1/trade/orders/all":
			if r.Header.Get("POLYMARKET-PROXY") != fixtureProxy ||
				r.Header.Get("POLYMARKET-SECRET") != "secret" {
				t.Error("delegated headers missing")
			}
			var c fixtureCommand
			if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
				t.Error(err)
				return
			}
			assertOperationSignature(t, c, []any{"cancelAll", []any{}})
			fmt.Fprint(w, `{"status":"ok"}`)
		case "/v1/account/credentials":
			fmt.Fprintf(
				w,
				`{"address":%q,"keys":[{"proxy":%q,"expiry":%d}]}`,
				fixtureProxy,
				fixtureProxy,
				time.Now().Add(time.Hour).UnixMilli(),
			)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(server.Close)
	signer, local := externalFixtureSigner(t)
	type contextKey struct{}
	ctx := context.WithValue(t.Context(), contextKey{}, "caller")
	signer.callback = func(got context.Context, data apitypes.TypedData) ([]byte, error) {
		if got.Value(contextKey{}) != "caller" {
			t.Error("caller context lost")
		}
		return local.SignTypedData(got, data) // V=0/1 must normalize to wire 27/28.
	}
	owner, err := NewOwner(
		OwnerConfig{Config: Config{Host: server.URL, ChainID: 31337}, Signer: signer},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.RevokeCredentials(ctx, fixtureRecipient); err != nil {
		t.Fatal(err)
	}
	account, err := owner.Resume(
		ctx,
		PerpsCredentials{Proxy: fixtureProxy, Secret: "secret"},
		signer,
	)
	if err != nil {
		t.Fatal(err)
	}
	// Both clients retain the EOA pinned at construction, not future Address calls.
	signer.address = common.HexToAddress(fixtureRecipient)
	if err := account.CancelAllOrders(ctx, nil, 0); err != nil {
		t.Fatal(err)
	}
	if err := owner.RevokeCredentials(ctx, fixtureRecipient); err != nil {
		t.Fatal(err)
	}
	if signer.addressCalls.Load() != 2 || requests.Load() != 4 ||
		account.Credentials().PrivateKey != "" {
		t.Fatal("identity not pinned or private key unexpectedly required")
	}
}

func TestExternalSignerValidationAndCancellation(t *testing.T) {
	t.Parallel()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		t.Error("invalid signature reached HTTP")
	}))
	t.Cleanup(server.Close)
	for _, mode := range []string{"wrong_identity", "malformed", "cancel_before", "cancel_during"} {
		t.Run(mode, func(t *testing.T) {
			signer, local := externalFixtureSigner(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			want := signing.ErrInvalidSignature
			switch mode {
			case "wrong_identity":
				signer.address = common.HexToAddress(fixtureRecipient)
			case "malformed":
				signer.callback = func(context.Context, apitypes.TypedData) ([]byte, error) { return []byte{1}, nil }
			case "cancel_before":
				cancel()
				want = context.Canceled
				signer.callback = func(context.Context, apitypes.TypedData) ([]byte, error) {
					t.Error("called signer after cancellation")
					return nil, nil
				}
			case "cancel_during":
				want = context.Canceled
				signer.callback = func(ctx context.Context, data apitypes.TypedData) ([]byte, error) {
					sig, err := local.SignTypedData(ctx, data)
					cancel()
					return sig, err
				}
			}
			owner, err := NewOwner(OwnerConfig{Config: Config{Host: server.URL}, Signer: signer})
			if err != nil {
				t.Fatal(err)
			}
			if err := owner.RevokeCredentials(ctx, fixtureRecipient); !errors.Is(err, want) {
				t.Fatalf("owner: %v", err)
			}
			// Reset cancellation so the delegated path exercises the callback too.
			ctx, cancel = context.WithCancel(t.Context())
			defer cancel()
			if mode == "cancel_before" {
				cancel()
			}
			account, err := NewAuthenticated(
				AuthenticatedConfig{
					Config: Config{Host: server.URL},
					Credentials: PerpsCredentials{
						Proxy:  signer.address.Hex(),
						Secret: "secret",
					},
					DelegatedSigner: signer,
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := account.CancelAllOrders(ctx, nil, 0); !errors.Is(err, want) {
				t.Fatalf("delegated: %v", err)
			}
		})
	}
	if requests.Load() != 0 {
		t.Fatal("invalid signature submitted")
	}
	signer, _ := externalFixtureSigner(t)
	for _, config := range []AuthenticatedConfig{
		{Credentials: PerpsCredentials{Proxy: fixtureProxy, Secret: "secret", PrivateKey: fixturePrivateKey}, DelegatedSigner: signer},
		{Credentials: PerpsCredentials{Proxy: fixtureRecipient, Secret: "secret"}, DelegatedSigner: signer},
	} {
		if _, err := NewAuthenticated(config); err == nil {
			t.Fatal("ambiguous or mismatched delegated signer accepted")
		}
	}
	var nilSigner *externalTypedSigner
	if _, err := NewOwner(OwnerConfig{Signer: nilSigner}); !errors.Is(
		err,
		signing.ErrInvalidSigner,
	) {
		t.Fatalf("typed nil: %v", err)
	}
	if _, err := NewAuthenticated(AuthenticatedConfig{
		Credentials:     PerpsCredentials{Proxy: fixtureProxy, Secret: "secret"},
		DelegatedSigner: nilSigner,
	}); !errors.Is(err, signing.ErrInvalidSigner) {
		t.Fatalf("typed nil delegated signer: %v", err)
	}
}

type externalMessageSigner struct {
	*externalTypedSigner
	signMessage func(context.Context, []byte) ([]byte, error)
}

func (s externalMessageSigner) SignMessage(ctx context.Context, message []byte) ([]byte, error) {
	return s.signMessage(ctx, message)
}

type externalTransactionSigner struct {
	*externalTypedSigner
	signTransaction func(context.Context, *big.Int, *types.Transaction) (*types.Transaction, error)
}

func (s externalTransactionSigner) SignTransaction(
	ctx context.Context,
	chain *big.Int,
	tx *types.Transaction,
) (*types.Transaction, error) {
	return s.signTransaction(ctx, chain, tx)
}

func TestExternalCollateralExecutionAndIncompleteReconciliation(t *testing.T) {
	t.Parallel()
	f, rpc := newCollateralRPC(t)
	f.failSend = 1
	signer, local := externalFixtureSigner(t)
	type contextKey struct{}
	ctx := context.WithValue(t.Context(), contextKey{}, "transaction caller")
	external := externalTransactionSigner{
		signer,
		func(got context.Context, chain *big.Int, tx *types.Transaction) (*types.Transaction, error) {
			if got.Value(contextKey{}) != "transaction caller" {
				t.Error("transaction signer lost caller context")
			}
			return local.SignTransaction(got, chain, tx)
		},
	}
	cfg := collateralConfig(t, rpc, clob.SignatureTypeEOA)
	cfg.Owner.Signer = external
	cfg.Transactions.PrivateKey = ""
	cfg.Transactions.Signer = external
	wallet := mustCollateralWallet(t, cfg)
	if f.requests.Load() != 0 {
		t.Fatal("constructor made RPC requests")
	}
	tx, err := wallet.ApproveAndDeposit(ctx, big.NewInt(1), "")
	if err == nil || tx == nil || len(tx.Submissions) != 1 ||
		!tx.Submissions[0].BroadcastUncertain ||
		tx.RequestedCalls != 2 {
		t.Fatalf("lost uncertain approval: %+v %v", tx, err)
	}
	receipts, err := tx.Wait(t.Context())
	if !errors.Is(err, ErrCollateralTransactionIncomplete) || len(receipts) != 1 ||
		f.sent.Load() != 1 {
		t.Fatalf("missing deposit complete or resubmitted: %v %v", receipts, err)
	}
	f.chain.Store(80002)
	if _, err := tx.Wait(t.Context()); err == nil {
		t.Fatal("wrong receipt RPC chain accepted")
	}
	f.chain.Store(137)
	// Lack of TransactionSigner must not fall back to exporting a private key.
	cfg.Owner.Signer = signer
	cfg.Transactions.Signer = signer
	wallet = mustCollateralWallet(t, cfg)
	if _, err := wallet.Deposit(ctx, big.NewInt(1), ""); !errors.Is(
		err,
		signing.ErrTransactionSigningUnsupported,
	) ||
		f.sent.Load() != 1 {
		t.Fatalf("unsupported external transaction signing: %v", err)
	}
	cfg.Transactions.Credentials = nil
	if _, err := NewCollateralWallet(cfg); err != nil {
		t.Fatalf("unrelated CLOB credentials required by wallet execution: %v", err)
	}
}
