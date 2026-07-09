package polyrelay

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

func TestIsRetryableSubmitError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"non-api error", errors.New("boom"), false},
		{"500", &polyhttp.APIError{StatusCode: 500}, false},
		{"429 rate limit", &polyhttp.APIError{StatusCode: 429}, true},
		{"400 unrelated", &polyhttp.APIError{StatusCode: 400, Message: "bad signature"}, false},
		{"400 wallet busy", &polyhttp.APIError{StatusCode: 400, Message: "Wallet busy: active action in flight"}, true},
		{"400 wallet in-flight", &polyhttp.APIError{StatusCode: 400, Message: "wallet has in-flight action"}, true},
		{"400 nonce stale (submitted<onchain)", &polyhttp.APIError{StatusCode: 400, Message: "batch nonce 5 does not match on-chain nonce 8"}, true},
		{"400 nonce ahead (submitted>=onchain)", &polyhttp.APIError{StatusCode: 400, Message: "batch nonce 8 does not match on-chain nonce 5"}, false},
		{"400 nonce equal", &polyhttp.APIError{StatusCode: 400, Message: "batch nonce 5 does not match on-chain nonce 5"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := isRetryableSubmitError(c.err); got != c.want {
				t.Fatalf("isRetryableSubmitError = %v, want %v", got, c.want)
			}
		})
	}
}

func TestPollUntilTerminalConfirmed(t *testing.T) {
	t.Parallel()
	// Sequence: MINED → CONFIRMED (with hash).
	states := []string{"STATE_MINED", "STATE_CONFIRMED"}
	srv := sequenceServer(t, states, "0xhash")
	tr := NewTransport(&polyhttp.Client{BaseURL: srv.URL, HTTPClient: srv.Client()})

	out, err := PollUntilTerminal(context.Background(), tr, "tx-1", "0xfallback", 10, time.Millisecond)
	if err != nil {
		t.Fatalf("PollUntilTerminal: %v", err)
	}
	if out.TransactionHash != "0xhash" {
		t.Fatalf("hash = %s, want 0xhash (prefer polled over fallback)", out.TransactionHash)
	}
}

func TestPollUntilTerminalFallbackHash(t *testing.T) {
	t.Parallel()
	// CONFIRMED but with empty transaction_hash → must use fallback.
	states := []string{"STATE_CONFIRMED"}
	srv := sequenceServer(t, states, "")
	tr := NewTransport(&polyhttp.Client{BaseURL: srv.URL, HTTPClient: srv.Client()})

	out, err := PollUntilTerminal(context.Background(), tr, "tx-1", "0xfallback", 10, time.Millisecond)
	if err != nil {
		t.Fatalf("PollUntilTerminal: %v", err)
	}
	if out.TransactionHash != "0xfallback" {
		t.Fatalf("hash = %s, want fallback 0xfallback", out.TransactionHash)
	}
}

func TestPollUntilTerminalNoHash(t *testing.T) {
	t.Parallel()
	states := []string{"STATE_CONFIRMED"}
	srv := sequenceServer(t, states, "")
	tr := NewTransport(&polyhttp.Client{BaseURL: srv.URL, HTTPClient: srv.Client()})

	_, err := PollUntilTerminal(context.Background(), tr, "tx-1", "", 10, time.Millisecond)
	if !errors.Is(err, ErrNoTransactionHash) {
		t.Fatalf("err = %v, want ErrNoTransactionHash", err)
	}
}

func TestPollUntilTerminalFailure(t *testing.T) {
	t.Parallel()
	states := []string{"STATE_FAILED"}
	srv := sequenceServer(t, states, "")
	tr := NewTransport(&polyhttp.Client{BaseURL: srv.URL, HTTPClient: srv.Client()})

	_, err := PollUntilTerminal(context.Background(), tr, "tx-1", "0xfallback", 10, time.Millisecond)
	if !errors.Is(err, ErrTransactionFailed) {
		t.Fatalf("err = %v, want ErrTransactionFailed", err)
	}
}

func TestPollUntilTerminalTimeout(t *testing.T) {
	t.Parallel()
	// Never terminal.
	states := []string{"STATE_EXECUTED"}
	srv := sequenceServer(t, states, "")
	tr := NewTransport(&polyhttp.Client{BaseURL: srv.URL, HTTPClient: srv.Client()})

	_, err := PollUntilTerminal(context.Background(), tr, "tx-1", "", 2, time.Millisecond)
	if !errors.Is(err, ErrTransactionTimeout) {
		t.Fatalf("err = %v, want ErrTransactionTimeout", err)
	}
}

// sequenceServer returns a test server that replies to GET /v1/account/transactions/{id}
// with a succession of states, cycling on the last.
func sequenceServer(t *testing.T, states []string, hash string) *httptest.Server {
	t.Helper()
	idx := 0
	srv := newJSONServer(func(r *http.Request) any {
		state := states[idx]
		if idx < len(states)-1 {
			idx++
		}
		return map[string]any{
			"state":            state,
			"transaction_hash": hash,
			"transaction_id":   "tx-1",
		}
	})
	t.Cleanup(srv.Close)
	return srv
}
