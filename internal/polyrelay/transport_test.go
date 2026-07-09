package polyrelay

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// relayerTestServer returns a Transport whose HTTP client points at a test
// server. The handler records each request into *seen and dispatches on path
// via the responses map (path → canned JSON body).
type recordedReq struct {
	method string
	path   string
	query  url.Values
	body   string
}

func newTestTransport(t *testing.T, responses map[string]any, seen *[]recordedReq) *Transport {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		*seen = append(
			*seen,
			recordedReq{
				method: r.Method,
				path:   r.URL.Path,
				query:  r.URL.Query(),
				body:   string(body),
			},
		)
		var resp any
		// /v1/account/transactions/{id} is path-based; match by prefix.
		for prefix, v := range responses {
			if strings.HasPrefix(r.URL.Path, prefix) {
				resp = v
				break
			}
		}
		if resp == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// Allow a canned error status.
		if e, ok := resp.(cannedError); ok {
			w.WriteHeader(e.status)
			_, _ = w.Write([]byte(e.body))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	c := &polyhttp.Client{BaseURL: srv.URL, HTTPClient: srv.Client()}
	return NewTransport(c)
}

type cannedError struct {
	status int
	body   string
}

func TestFetchExecuteParams(t *testing.T) {
	t.Parallel()
	var seen []recordedReq
	tr := newTestTransport(t, map[string]any{
		executeParamsPath: map[string]string{"address": "0xAb12345", "nonce": "42"},
	}, &seen)

	got, err := tr.FetchExecuteParams(context.Background(), "0xSigner", TransactionTypeSafe)
	if err != nil {
		t.Fatalf("FetchExecuteParams: %v", err)
	}
	if got.Nonce.Int64() != 42 {
		t.Fatalf("nonce = %d, want 42", got.Nonce.Int64())
	}
	if r := seen[0]; r.method != http.MethodGet || r.path != executeParamsPath ||
		r.query.Get("address") != "0xSigner" || r.query.Get("type") != "SAFE" {
		t.Fatalf("request = %+v, want GET %s?address=0xSigner&type=SAFE", r, executeParamsPath)
	}
}

func TestFetchRelayPayloadUsesSeparatePath(t *testing.T) {
	t.Parallel()
	var seen []recordedReq
	tr := newTestTransport(t, map[string]any{
		relayPayloadPath: map[string]string{"address": "0xRelay", "nonce": "7"},
	}, &seen)

	got, err := tr.FetchRelayPayload(context.Background(), "0xSigner", TransactionTypeProxy)
	if err != nil {
		t.Fatalf("FetchRelayPayload: %v", err)
	}
	if got.Nonce.Int64() != 7 {
		t.Fatalf("nonce = %d, want 7", got.Nonce.Int64())
	}
	if r := seen[0]; r.path != relayPayloadPath {
		t.Fatalf("path = %s, want %s (proxy uses /relay-payload)", r.path, relayPayloadPath)
	}
}

func TestFetchExecuteParamsRejectsBadNonce(t *testing.T) {
	t.Parallel()
	for _, nonce := range []string{"", "not-a-number"} {
		tr := newTestTransport(t, map[string]any{
			executeParamsPath: map[string]string{"address": "0x", "nonce": nonce},
		}, &[]recordedReq{})
		if _, err := tr.FetchExecuteParams(context.Background(), "0x", TransactionTypeWallet); err == nil {
			t.Fatalf("expected error for nonce %q", nonce)
		}
	}
}

func TestSubmitParsesResponse(t *testing.T) {
	t.Parallel()
	var seen []recordedReq
	tr := newTestTransport(t, map[string]any{
		submitPath: map[string]any{
			"state":           "STATE_NEW",
			"transactionHash": "0xabc",
			"transactionID":   "tx-123",
		},
	}, &seen)

	req := BuildWalletCreate(WalletCreateInput{})
	got, err := tr.Submit(context.Background(), req)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if got.State != StateNew || got.TransactionHash != "0xabc" || got.TransactionID != "tx-123" {
		t.Fatalf("response = %+v", got)
	}
	if r := seen[0]; r.method != http.MethodPost || r.path != submitPath {
		t.Fatalf("request = %+v, want POST %s", r, submitPath)
	}
}

func TestGaslessTransactionParsesSnakeCase(t *testing.T) {
	t.Parallel()
	tr := newTestTransport(t, map[string]any{
		"/v1/account/transactions/": map[string]any{
			"state":            "STATE_MINED",
			"transaction_hash": "0xdead",
			"transaction_id":   "tx-9",
			"error_msg":        "",
		},
	}, &[]recordedReq{})

	got, err := tr.GaslessTransaction(context.Background(), "tx-9")
	if err != nil {
		t.Fatalf("GaslessTransaction: %v", err)
	}
	// The load-bearing check: snake_case keys must decode into the right fields.
	if got.TransactionHash != "0xdead" || got.TransactionID != "tx-9" {
		t.Fatalf("gasless tx = %+v (snake_case keys misparsed)", got)
	}
}

func TestIsWalletDeployed(t *testing.T) {
	t.Parallel()
	var seen []recordedReq
	tr := newTestTransport(t, map[string]any{
		deployedPath: map[string]bool{"deployed": true},
	}, &seen)

	deployed, err := tr.IsWalletDeployed(context.Background(), "0xWallet", TransactionTypeWallet)
	if err != nil {
		t.Fatalf("IsWalletDeployed: %v", err)
	}
	if !deployed {
		t.Fatal("deployed = false, want true")
	}
	if q := seen[0].query; q.Get("address") != "0xWallet" || q.Get("type") != "WALLET" {
		t.Fatalf("query = %v", q)
	}
}

func TestIsWalletDeployedOmitsEmptyType(t *testing.T) {
	t.Parallel()
	var seen []recordedReq
	tr := newTestTransport(t, map[string]any{
		deployedPath: map[string]bool{"deployed": false},
	}, &seen)

	if _, err := tr.IsWalletDeployed(context.Background(), "0xWallet", ""); err != nil {
		t.Fatalf("IsWalletDeployed: %v", err)
	}
	if seen[0].query.Has("type") {
		t.Fatalf("type should be omitted for empty txType, got %v", seen[0].query)
	}
}
