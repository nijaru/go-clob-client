package polyrelay

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// orchServer is a mock relayer that serves nonce endpoints and captures POST /submit
// bodies. submitResp selects the canned /submit behavior (success or an error sequence).
type orchServer struct {
	nonce      string
	relayAddr  string
	t          *testing.T
	submitBody *map[string]any
	submitResp func(attempt int) any
	calls      atomic.Int32
}

func (s *orchServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == executeParamsPath:
		_ = json.NewEncoder(w).Encode(map[string]string{"address": s.relayAddr, "nonce": s.nonce})
	case r.URL.Path == submitPath && r.Method == http.MethodPost:
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		if err := json.Unmarshal(body, &parsed); err != nil {
			s.t.Fatalf("decode submit body: %v", err)
		}
		*s.submitBody = parsed
		attempt := int(s.calls.Add(1))
		resp := s.submitResp(attempt)
		if e, ok := resp.(*cannedError); ok {
			w.WriteHeader(e.status)
			_, _ = w.Write([]byte(e.body))
			return
		}
		_ = json.NewEncoder(w).Encode(resp)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func testGaslessConfig(typ RelayerTransactionType) GaslessConfig {
	return GaslessConfig{
		WalletType:           typ,
		Signer:               addrRepeat(0x01),
		Wallet:               addrRepeat(0x02),
		ChainID:              big.NewInt(137),
		ProxyFactory:         addrRepeat(0x10),
		DepositWalletFactory: addrRepeat(0x11),
		RelayHub:             addrRepeat(0x12),
		SafeMultisend:        addrRepeat(0x13),
		PollInterval:         time.Millisecond,
	}
}

func testTransport(t *testing.T, srv *httptest.Server) *Transport {
	t.Helper()
	return NewTransport(&polyhttp.Client{BaseURL: srv.URL, HTTPClient: srv.Client()})
}

func TestPrepareGaslessDepositWallet(t *testing.T) {
	t.Parallel()
	var captured map[string]any
	srv := httptest.NewServer(&orchServer{
		t: t, nonce: "5", relayAddr: "0xRelay",
		submitBody: &captured,
		submitResp: func(int) any {
			return map[string]any{
				"state":           "STATE_NEW",
				"transactionHash": "",
				"transactionID":   "tx-deposit",
			}
		},
	})
	t.Cleanup(srv.Close)

	calls := []TransactionCall{{To: addrRepeat(0x20), Data: []byte{0xaa}, Value: big.NewInt(0)}}
	tr := testTransport(t, srv)
	h, err := PrepareGasless(
		context.Background(),
		tr,
		testGaslessConfig(TransactionTypeWallet),
		mustKey(t),
		calls,
		"merge",
	)
	if err != nil {
		t.Fatalf("PrepareGasless: %v", err)
	}
	if h.TransactionID != "tx-deposit" {
		t.Fatalf("tx id = %s", h.TransactionID)
	}
	// Verify the assembled body: type, from, nonce, factory target, raw calls carried.
	if captured["type"] != "WALLET" {
		t.Fatalf("type = %v, want WALLET", captured["type"])
	}
	if captured["from"] == nil || captured["nonce"] != "5" {
		t.Fatalf("from/nonce = %v/%v", captured["from"], captured["nonce"])
	}
	if captured["to"] != addrRepeat(0x11).Hex() {
		t.Fatalf("to = %v, want deposit wallet factory", captured["to"])
	}
	dw, _ := captured["depositWalletParams"].(map[string]any)
	if dw == nil || dw["depositWallet"] == nil {
		t.Fatalf("missing depositWalletParams: %v", captured)
	}
}

func TestPrepareGaslessProxy(t *testing.T) {
	t.Parallel()
	var captured map[string]any
	srv := httptest.NewServer(&orchServer{
		t: t, nonce: "9", relayAddr: addrRepeat(0x99).Hex(),
		submitBody: &captured,
		submitResp: func(int) any {
			return map[string]any{"state": "STATE_NEW", "transactionID": "tx-proxy"}
		},
	})
	t.Cleanup(srv.Close)

	calls := []TransactionCall{{To: addrRepeat(0x20), Data: []byte{0xbb}, Value: big.NewInt(0)}}
	tr := testTransport(t, srv)
	h, err := PrepareGasless(
		context.Background(),
		tr,
		testGaslessConfig(TransactionTypeProxy),
		mustKey(t),
		calls,
		"",
	)
	if err != nil {
		t.Fatalf("PrepareGasless: %v", err)
	}
	if h.TransactionID != "tx-proxy" {
		t.Fatalf("tx id = %s", h.TransactionID)
	}
	if captured["type"] != "PROXY" {
		t.Fatalf("type = %v, want PROXY", captured["type"])
	}
	if captured["to"] != addrRepeat(0x10).Hex() {
		t.Fatalf("to = %v, want proxy factory", captured["to"])
	}
	sp, _ := captured["signatureParams"].(map[string]any)
	if sp == nil || sp["relayHub"] == nil || sp["relay"] == nil {
		t.Fatalf("missing proxy signatureParams: %v", captured)
	}
	if sp["relay"] != addrRepeat(0x99).Hex() {
		t.Fatalf("relay = %v, want execute-params relay address", sp["relay"])
	}
}

func TestPrepareGaslessProxyUsesGasEstimate(t *testing.T) {
	t.Parallel()
	// Exercises the previously-untested branch where cfg.GasEstimator returns a
	// real (>0) value: that estimate must reach the submitted payload's
	// signatureParams.gasLimit. Digest↔payload consistency is guaranteed by
	// construction (submitProxy signs and builds the payload from the same
	// *big.Int gasLimit local), so asserting the payload value closes the gap.
	const est uint64 = 123456
	var captured map[string]any
	srv := httptest.NewServer(&orchServer{
		t: t, nonce: "9", relayAddr: addrRepeat(0x99).Hex(),
		submitBody: &captured,
		submitResp: func(int) any {
			return map[string]any{"state": "STATE_NEW", "transactionID": "tx-proxy"}
		},
	})
	t.Cleanup(srv.Close)

	cfg := testGaslessConfig(TransactionTypeProxy)
	cfg.GasEstimator = func(context.Context, common.Address, common.Address, []byte) (uint64, error) {
		return est, nil
	}

	calls := []TransactionCall{{To: addrRepeat(0x20), Data: []byte{0xbb}, Value: big.NewInt(0)}}
	tr := testTransport(t, srv)
	if _, err := PrepareGasless(context.Background(), tr, cfg, mustKey(t), calls, ""); err != nil {
		t.Fatalf("PrepareGasless: %v", err)
	}
	sp, _ := captured["signatureParams"].(map[string]any)
	if sp == nil {
		t.Fatalf("missing signatureParams: %v", captured)
	}
	if sp["gasLimit"] != "123456" {
		t.Fatalf("signatureParams.gasLimit = %v, want 123456 (estimator output)", sp["gasLimit"])
	}
}

func TestPrepareGaslessSafeMultiCallAggregates(t *testing.T) {
	t.Parallel()
	var captured map[string]any
	srv := httptest.NewServer(&orchServer{
		t: t, nonce: "3", relayAddr: "0x",
		submitBody: &captured,
		submitResp: func(int) any {
			return map[string]any{"state": "STATE_NEW", "transactionID": "tx-safe"}
		},
	})
	t.Cleanup(srv.Close)

	calls := []TransactionCall{
		{To: addrRepeat(0x20), Data: []byte{0x01}, Value: big.NewInt(0)},
		{To: addrRepeat(0x21), Data: []byte{0x02}, Value: big.NewInt(0)},
	}
	tr := testTransport(t, srv)
	if _, err := PrepareGasless(context.Background(), tr, testGaslessConfig(TransactionTypeSafe), mustKey(t), calls, ""); err != nil {
		t.Fatalf("PrepareGasless: %v", err)
	}
	// Multi-call → target is safeMultisend, operation is delegatecall (1).
	if captured["to"] != addrRepeat(0x13).Hex() {
		t.Fatalf("to = %v, want safeMultisend for multi-call", captured["to"])
	}
	sp, _ := captured["signatureParams"].(map[string]any)
	if sp["operation"] != "1" {
		t.Fatalf("operation = %v, want 1 (delegatecall) for multi-call", sp["operation"])
	}
}

func TestPrepareGaslessSafeSingleCallDirect(t *testing.T) {
	t.Parallel()
	var captured map[string]any
	srv := httptest.NewServer(&orchServer{
		t: t, nonce: "3", relayAddr: "0x",
		submitBody: &captured,
		submitResp: func(int) any {
			return map[string]any{"state": "STATE_NEW", "transactionID": "tx-safe"}
		},
	})
	t.Cleanup(srv.Close)

	calls := []TransactionCall{{To: addrRepeat(0x20), Data: []byte{0x01}, Value: big.NewInt(0)}}
	tr := testTransport(t, srv)
	if _, err := PrepareGasless(context.Background(), tr, testGaslessConfig(TransactionTypeSafe), mustKey(t), calls, ""); err != nil {
		t.Fatalf("PrepareGasless: %v", err)
	}
	// Single call → target is the call's own target, operation is CALL (0).
	if captured["to"] != addrRepeat(0x20).Hex() {
		t.Fatalf("to = %v, want the single call target", captured["to"])
	}
	sp, _ := captured["signatureParams"].(map[string]any)
	if sp["operation"] != "0" {
		t.Fatalf("operation = %v, want 0 (CALL) for single call", sp["operation"])
	}
}

func TestPrepareGaslessRetriesOnNonceMismatch(t *testing.T) {
	t.Parallel()
	var captured map[string]any
	mock := &orchServer{
		t: t, nonce: "1", relayAddr: "0x",
		submitBody: &captured,
		// First submit: stale nonce (submitted 1 < on-chain 5) → retryable.
		// Second submit: success.
		submitResp: func(attempt int) any {
			if attempt == 1 {
				return &cannedError{
					status: 400,
					body:   `{"error":"batch nonce 1 does not match on-chain nonce 5"}`,
				}
			}
			return map[string]any{"state": "STATE_NEW", "transactionID": "tx-after-retry"}
		},
	}
	srv := httptest.NewServer(mock)
	t.Cleanup(srv.Close)

	calls := []TransactionCall{{To: addrRepeat(0x20), Data: []byte{0x01}, Value: big.NewInt(0)}}
	tr := testTransport(t, srv)
	h, err := PrepareGasless(
		context.Background(),
		tr,
		testGaslessConfig(TransactionTypeWallet),
		mustKey(t),
		calls,
		"",
	)
	if err != nil {
		t.Fatalf("PrepareGasless: %v", err)
	}
	if h.TransactionID != "tx-after-retry" {
		t.Fatalf("tx id = %s, want success after retry", h.TransactionID)
	}
	if got := mock.calls.Load(); got != 2 {
		t.Fatalf("submit attempts = %d, want 2 (retry after stale nonce)", got)
	}
}

func TestPrepareGaslessDoesNotRetryUnrelated400(t *testing.T) {
	t.Parallel()
	var captured map[string]any
	srv := httptest.NewServer(&orchServer{
		t: t, nonce: "1", relayAddr: "0x",
		submitBody: &captured,
		submitResp: func(int) any {
			return &cannedError{status: 400, body: `{"error":"bad signature"}`}
		},
	})
	t.Cleanup(srv.Close)

	calls := []TransactionCall{{To: addrRepeat(0x20), Data: nil, Value: big.NewInt(0)}}
	tr := testTransport(t, srv)
	_, err := PrepareGasless(
		context.Background(),
		tr,
		testGaslessConfig(TransactionTypeWallet),
		mustKey(t),
		calls,
		"",
	)
	if err == nil {
		t.Fatal("expected error for non-retryable 400")
	}
	var apiErr *polyhttp.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 400 {
		t.Fatalf("err = %v, want 400 APIError", err)
	}
}

func TestPrepareGaslessValidation(t *testing.T) {
	t.Parallel()
	tr := testTransport(
		t,
		httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})),
	)
	cfg := testGaslessConfig(TransactionTypeWallet)

	t.Run("nil key", func(t *testing.T) {
		if _, err := PrepareGasless(context.Background(), tr, cfg, nil, []TransactionCall{{To: addrRepeat(1), Value: big.NewInt(0)}}, ""); !errors.Is(
			err,
			ErrNilKey,
		) {
			t.Fatalf("err = %v, want ErrNilKey", err)
		}
	})
	t.Run("empty calls", func(t *testing.T) {
		if _, err := PrepareGasless(context.Background(), tr, cfg, mustKey(t), nil, ""); !errors.Is(
			err,
			ErrEmptyCalls,
		) {
			t.Fatalf("err = %v, want ErrEmptyCalls", err)
		}
	})
	t.Run("metadata too long", func(t *testing.T) {
		long := strings.Repeat("x", MetadataMaxLength+1)
		if _, err := PrepareGasless(context.Background(), tr, cfg, mustKey(t), []TransactionCall{{To: addrRepeat(1), Value: big.NewInt(0)}}, long); !errors.Is(
			err,
			ErrMetadataTooLong,
		) {
			t.Fatalf("err = %v, want ErrMetadataTooLong", err)
		}
	})
}

func TestResolveSafeCall(t *testing.T) {
	t.Parallel()
	safeMultisend := addrRepeat(0x13)
	c1 := TransactionCall{To: addrRepeat(0x20), Data: []byte("a"), Value: big.NewInt(7)}
	c2 := TransactionCall{To: addrRepeat(0x21), Data: []byte("b"), Value: big.NewInt(0)}

	t.Run("single direct", func(t *testing.T) {
		t.Parallel()
		target, data, value, op, err := resolveSafeCall(safeMultisend, []TransactionCall{c1})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if target != c1.To || !bytesEq(data, c1.Data) || value.Int64() != 7 ||
			op != safeOperationCall {
			t.Fatalf("single resolve = %s/%v/%d/%d", target, data, value, op)
		}
	})
	t.Run("multi bundles via multisend", func(t *testing.T) {
		t.Parallel()
		target, _, value, op, err := resolveSafeCall(safeMultisend, []TransactionCall{c1, c2})
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if target != safeMultisend || value.Sign() != 0 || op != safeOperationDelegatecall {
			t.Fatalf("multi resolve = %s/%d/%d, want multisend/0/delegatecall", target, value, op)
		}
	})
}

func TestDeployDepositWallet(t *testing.T) {
	t.Parallel()
	var captured map[string]any
	srv := httptest.NewServer(&orchServer{
		t: t, nonce: "0", relayAddr: "0x",
		submitBody: &captured,
		submitResp: func(int) any {
			return map[string]any{"state": "STATE_NEW", "transactionID": "deploy-1"}
		},
	})
	t.Cleanup(srv.Close)

	tr := testTransport(t, srv)
	h, err := DeployDepositWallet(context.Background(), tr, addrRepeat(0x01), addrRepeat(0x11), "")
	if err != nil {
		t.Fatalf("DeployDepositWallet: %v", err)
	}
	if h.TransactionID != "deploy-1" {
		t.Fatalf("tx id = %s", h.TransactionID)
	}
	if captured["type"] != "WALLET-CREATE" {
		t.Fatalf("type = %v, want WALLET-CREATE", captured["type"])
	}
}

func bytesEq(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
