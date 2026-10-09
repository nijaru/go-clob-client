package clob

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

	"github.com/ethereum/go-ethereum/common"
)

func testRelayerKeyContext(t *testing.T) context.Context {
	t.Helper()
	ctx, err := WithRelayerAuth(
		t.Context(),
		RelayerAuthConfig{
			APIKey: &RelayerAPIKey{Key: "relayer-key", Address: common.HexToAddress("0x1234")},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

func assertRelayerKeyHeaders(t *testing.T, r *http.Request) {
	t.Helper()
	if r.Header.Get("RELAYER_API_KEY") != "relayer-key" ||
		r.Header.Get("RELAYER_API_KEY_ADDRESS") != common.HexToAddress("0x1234").Hex() {
		t.Errorf("wrong relayer key headers")
	}
	for _, name := range []string{"POLY_API_KEY", "POLY_SIGNATURE", "POLY_BUILDER_API_KEY", "POLY_BUILDER_SIGNATURE", "POLY_BUILDER_TIMESTAMP", "POLY_BUILDER_PASSPHRASE"} {
		if r.Header.Get(name) != "" {
			t.Errorf("leaked %s", name)
		}
	}
}

type relayerTestBuilderAuth struct{ BuilderAuth }

func (a relayerTestBuilderAuth) Headers(
	ctx context.Context,
	req BuilderHeaderRequest,
) (map[string]string, error) {
	headers, err := a.BuilderAuth.Headers(ctx, req)
	if err == nil {
		headers["RELAYER_API_KEY"] = "injected"
		headers["POLY_API_KEY"] = "injected"
		headers["Authorization"] = "injected"
	}
	return headers, err
}

func TestRelayerAuthSelection(t *testing.T) {
	t.Parallel()
	builder, err := NewLocalBuilderAuth(
		Credentials{Key: "builder", Secret: "c2VjcmV0", Passphrase: "p"},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, cfg := range []RelayerAuthConfig{
		{},
		{APIKey: &RelayerAPIKey{Key: "key", Address: common.HexToAddress("0x1")}, BuilderAuth: builder},
		{APIKey: &RelayerAPIKey{Key: "", Address: common.HexToAddress("0x1")}},
		{APIKey: &RelayerAPIKey{Key: "key"}},
		{APIKey: &RelayerAPIKey{Key: "injected\r\nheader", Address: common.HexToAddress("0x1")}},
	} {
		if _, err := WithRelayerAuth(t.Context(), cfg); !errors.Is(err, ErrInvalidRelayerAuth) {
			t.Fatalf("invalid config: %v", err)
		}
	}
	key := &RelayerAPIKey{Key: "initial", Address: common.HexToAddress("0x1")}
	ctx, err := WithRelayerAuth(t.Context(), RelayerAuthConfig{APIKey: key})
	if err != nil {
		t.Fatal(err)
	}
	key.Key = "changed"
	c := newGaslessClient(t, SignatureTypePolyGnosisSafe, "http://127.0.0.1:1")
	c.useServerTime = true // API-key auth must not request CLOB server time
	headers, err := c.relayerHeaders(ctx, "POST", "/submit", nil, 0, nil)
	if err != nil || headers["RELAYER_API_KEY"] != "initial" || len(headers) != 2 {
		t.Fatalf("key snapshot: %v %v", headers, err)
	}
	c.useServerTime = false
	builderCtx, err := WithRelayerAuth(
		t.Context(),
		RelayerAuthConfig{BuilderAuth: relayerTestBuilderAuth{builder}},
	)
	if err != nil {
		t.Fatal(err)
	}
	headers, err = c.relayerHeaders(builderCtx, "POST", "/submit", []byte(`{}`), 0, nil)
	if err != nil || headers["POLY_BUILDER_API_KEY"] != "builder" || len(headers) != 4 {
		t.Fatalf("builder selection: %v %v", headers, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := c.relayerHeaders(cancelled, "GET", "/deployed", nil, 0, nil); !errors.Is(
		err,
		context.Canceled,
	) {
		t.Fatal(err)
	}
}

func TestRelayerAPIKeyGaslessAndRevocation(t *testing.T) {
	t.Parallel()
	var client *AuthenticatedClient
	var submissions atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/user/session-signers" {
			if r.Header.Get("POLY_API_KEY") != "k" || r.Header.Get("RELAYER_API_KEY") != "" {
				t.Error("registry auth must remain L2")
			}
			fmt.Fprintf(w, `{"wallet":%q,"signers":[]}`, client.funderAddress)
			return
		}
		assertRelayerKeyHeaders(t, r)
		switch r.URL.Path {
		case "/v1/account/transactions/params":
			if r.URL.Query().Get("address") != client.Address() ||
				r.URL.Query().Get("type") != "WALLET" {
				t.Error("wrong signer identity")
			}
			fmt.Fprint(w, `{"nonce":"7"}`)
		case "/submit":
			submissions.Add(1)
			fmt.Fprint(w, `{"state":"STATE_NEW","transactionID":"gasless-key"}`)
		case "/v1/account/transactions/gasless-key":
			fmt.Fprint(
				w,
				`{"state":"STATE_CONFIRMED","transaction_id":"gasless-key","transaction_hash":"0x0000000000000000000000000000000000000000000000000000000000000001"}`,
			)
		case "/v1/session-signers/revocations":
			var payload sessionKeyMutationPayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
				return
			}
			if r.Header.Get("Idempotency-Key") != "revocation-key" {
				t.Error("missing idempotency")
			}
			verifySessionBatch(t, client, payload, false)
			fmt.Fprint(w, `{"status":"FENCED","transactionId":"revocation-pending"}`)
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			http.Error(w, "unexpected", 404)
		}
	}))
	t.Cleanup(server.Close)
	client = newSessionOwner(t, server.URL) // builder configured; explicit key wins
	ctx := testRelayerKeyContext(t)
	handle, err := client.ExecuteWalletTransaction(
		ctx,
		[]TransactionCall{{To: common.HexToAddress("0x1"), Value: big.NewInt(0)}},
		"key-auth",
	)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := handle.Wait(ctx)
	if err != nil || outcome.TransactionID != "gasless-key" || submissions.Load() != 1 {
		t.Fatalf("gasless: %+v %v", outcome, err)
	}
	if _, err := client.AuthorizeSessionKey(ctx, AuthorizeSessionKeyRequest{Address: common.HexToAddress("0x9")}); !errors.Is(
		err,
		ErrSessionKeyBuilderRequired,
	) {
		t.Fatalf("API key must not authorize: %v", err)
	}
	if err := client.RevokeSessionKey(ctx, RevokeSessionKeyRequest{Address: common.HexToAddress("0x9"), IdempotencyKey: "revocation-key"}); err != nil {
		t.Fatal(err)
	}
	client.funderAddress = common.HexToAddress("0x123").Hex()
	if err := client.RevokeSessionKey(ctx, RevokeSessionKeyRequest{Address: common.HexToAddress("0x9")}); !errors.Is(
		err,
		ErrSessionKeyOwnerRequired,
	) {
		t.Fatalf("session signer must not revoke: %v", err)
	}
}
