package clob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/nijaru/go-clob-client/internal/polyrelay"
)

func newSessionOwner(t *testing.T, host string) *AuthenticatedClient {
	t.Helper()
	c := newGaslessClient(t, SignatureTypePoly1271, host)
	wallet, err := DeriveBeaconDepositWallet(c.signer.Address(), PolygonChainID)
	if err != nil {
		t.Fatal(err)
	}
	c.funderAddress = wallet.Hex()
	c.http.BaseURL = host
	return c
}

func TestSessionKeyAuthorizationAndRevocation(t *testing.T) {
	t.Parallel()
	address := common.HexToAddress("0x1234567890123456789012345678901234567890")
	var client *AuthenticatedClient
	var authorizedPayload sessionKeyMutationPayload
	var firstBody []byte
	var submitCount, registryCount atomic.Int32
	var revoked atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/account/transactions/params":
			if r.URL.Query().Get("address") != client.Address() ||
				r.URL.Query().Get("type") != "WALLET" {
				t.Error("wrong execute identity")
			}
			fmt.Fprint(w, `{"nonce":"7"}`)
		case "/v1/session-signers/authorizations":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Error(err)
				return
			}
			if r.Header.Get("Idempotency-Key") != "stable-authorization" ||
				r.Header.Get("POLY_BUILDER_API_KEY") != "builder" {
				t.Error("missing submission identity")
			}
			if submitCount.Add(1) == 1 {
				firstBody = body
				http.Error(w, "temporary", 503)
				return
			}
			if !slices.Equal(body, firstBody) {
				t.Error("retry changed signed payload")
			}
			if err := json.Unmarshal(body, &authorizedPayload); err != nil {
				t.Error(err)
				return
			}
			if authorizedPayload.SessionSignerAddress != address.Hex() ||
				!equalSessionScopes(
					authorizedPayload.Scopes,
					[]SessionKeyScope{SessionKeyScopeCLOB, "FUTURE"},
				) {
				t.Error("wrong scoped authorization")
			}
			verifySessionBatch(t, client, authorizedPayload, true)
			fmt.Fprint(w, `{"status":"SUBMITTED","transactionId":"authorization"}`)
		case "/v1/account/transactions/authorization":
			fmt.Fprint(
				w,
				`{"state":"STATE_CONFIRMED","transaction_id":"authorization","transaction_hash":"0x`+fmt.Sprintf(
					"%064x",
					1,
				)+`"}`,
			)
		case "/v1/session-signers/revocations":
			var payload sessionKeyMutationPayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
				return
			}
			if payload.ValidUntil != "" || payload.Scopes != nil ||
				r.Header.Get("Idempotency-Key") != "stable-revocation" {
				t.Error("unexpected revocation payload")
			}
			verifySessionBatch(t, client, payload, false)
			revoked.Store(true)
			fmt.Fprint(w, `{"status":"FENCED","transactionId":"still-pending"}`)
		case "/v1/user/session-signers":
			if r.Header.Get("POLY_API_KEY") != "k" {
				t.Error("registry missing L2 auth")
			}
			if revoked.Load() {
				fmt.Fprintf(w, `{"wallet":%q,"signers":[]}`, client.funderAddress)
				return
			}
			if registryCount.Add(1) == 1 {
				http.Error(w, "registry not ready", 404)
				return
			}
			fmt.Fprintf(
				w,
				`{"wallet":%q,"signers":[{"address":%q,"scopes":["FUTURE","CLOB","CLOB"],"valid_until":%s}]}`,
				client.funderAddress,
				address.Hex(),
				authorizedPayload.ValidUntil,
			)
		default:
			t.Errorf("unexpected request %s (revocation must not await chain)", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(server.Close)
	client = newSessionOwner(t, server.URL)
	start := time.Now().Unix()
	result, err := client.AuthorizeSessionKey(
		t.Context(),
		AuthorizeSessionKeyRequest{
			Address:        address,
			Scopes:         []SessionKeyScope{SessionKeyScopeCLOB, "FUTURE"},
			IdempotencyKey: "stable-authorization",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.SessionKey.Address != address ||
		result.SessionKey.ValidUntil < start+int64(sessionKeyLifetime/time.Second) ||
		result.SessionKey.ValidUntil > time.Now().Unix()+int64(sessionKeyLifetime/time.Second) ||
		result.Transaction.TransactionID != "authorization" {
		t.Fatalf("wrong authorization result: %+v", result)
	}
	if submitCount.Load() != 2 || registryCount.Load() != 2 {
		t.Fatal("missing submission retry or readiness wait")
	}
	if err := client.RevokeSessionKey(t.Context(), RevokeSessionKeyRequest{Address: address, IdempotencyKey: "stable-revocation"}); err != nil {
		t.Fatal(err)
	}
}

func verifySessionBatch(
	t *testing.T,
	client *AuthenticatedClient,
	payload sessionKeyMutationPayload,
	authorization bool,
) {
	t.Helper()
	var expiry *int64
	if authorization {
		value, err := strconv.ParseInt(payload.ValidUntil, 10, 64)
		if err != nil {
			t.Error(err)
			return
		}
		expiry = &value
	}
	data, err := packSessionKeyCall(common.HexToAddress(payload.SessionSignerAddress), expiry)
	if err != nil {
		t.Error(err)
		return
	}
	nonce, ok := new(big.Int).SetString(payload.Nonce, 10)
	if !ok {
		t.Error("invalid nonce")
		return
	}
	deadline, ok := new(big.Int).SetString(payload.Deadline, 10)
	if !ok || deadline.Int64() < time.Now().Unix() {
		t.Error("invalid deadline")
		return
	}
	signature, err := polyrelay.Sign(
		polyrelay.TransactionTypeWallet,
		client.signer.PrivateKey(),
		polyrelay.RelayRequest{
			Wallet:   client.WalletAddress(),
			ChainID:  big.NewInt(137),
			Nonce:    nonce,
			Deadline: deadline,
			Calls:    []TransactionCall{tokenCall(client.WalletAddress(), data)},
		},
	)
	if err != nil {
		t.Error(err)
		return
	}
	if payload.Signature != "0x"+common.Bytes2Hex(signature) ||
		payload.WalletAddress != client.funderAddress {
		t.Error("authorization signature does not bind exact owner batch")
	}
}

func TestSessionKeyBoundaryValidation(t *testing.T) {
	t.Parallel()
	cases := []string{
		`{"wallet":"0x1111111111111111111111111111111111111111","signers":[]}`,
		`{"wallet":%q}`,
		`{"wallet":%q,"signers":[{"address":"bad","scopes":["CLOB"],"valid_until":1}]}`,
		`{"wallet":%q,"signers":[{"address":"0x1111111111111111111111111111111111111111","scopes":[],"valid_until":1}]}`,
		`{"wallet":%q,"signers":[{"address":"0x1111111111111111111111111111111111111111","scopes":["CLOB"]}]}`,
	}
	for i, response := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			key, _ := crypto.HexToECDSA(gaslessTestKey[2:])
			wallet, _ := DeriveBeaconDepositWallet(crypto.PubkeyToAddress(key.PublicKey), 137)
			if i > 0 {
				response = fmt.Sprintf(response, wallet.Hex())
			}
			server := httptest.NewServer(
				http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, response) },
				),
			)
			t.Cleanup(server.Close)
			if _, err := newSessionOwner(t, server.URL).FetchSessionKeys(t.Context()); !errors.Is(
				err,
				ErrSessionKeyResponse,
			) {
				t.Fatalf("expected response rejection, got %v", err)
			}
		})
	}
	client := newSessionOwner(t, "http://127.0.0.1:1")
	client.builderAuth = nil
	if _, err := client.AuthorizeSessionKey(t.Context(), AuthorizeSessionKeyRequest{}); !errors.Is(
		err,
		ErrSessionKeyBuilderRequired,
	) {
		t.Fatal(err)
	}
	client.funderAddress = "0x1111111111111111111111111111111111111111"
	if _, err := client.FetchSessionKeys(t.Context()); !errors.Is(err, ErrSessionKeyOwnerRequired) {
		t.Fatal(err)
	}
	if err := client.RevokeSessionKey(t.Context(), RevokeSessionKeyRequest{}); !errors.Is(
		err,
		ErrSessionKeyOwnerRequired,
	) {
		t.Fatal(err)
	}
}

func TestSessionKeyFailureAndCancellation(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"FAILED", "SUPERSEDED", "REPAIR_REQUIRED"} {
		if err := validateSessionKeyStatus(sessionKeyMutationResponse{Status: status}, true); !errors.Is(
			err,
			ErrWalletTransactionFailed,
		) {
			t.Fatal(err)
		}
	}
	if err := validateSessionKeyStatus(sessionKeyMutationResponse{Status: "new-unknown", TransactionID: "id"}, true); !errors.Is(
		err,
		ErrSessionKeyResponse,
	) {
		t.Fatal(err)
	}
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) { http.Error(w, "not ready", 404) },
		),
	)
	t.Cleanup(server.Close)
	client := newSessionOwner(t, server.URL)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()
	_, err := client.waitSessionKeyRegistry(
		ctx,
		SessionKey{Address: common.HexToAddress("0x1")},
		false,
	)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("transient registry failure must not prove revocation: %v", err)
	}
}
