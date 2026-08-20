package polyrelay

import (
	"context"
	"errors"
	"net/url"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// Relayer HTTP endpoints. The relayer is a separate service from the CLOB API;
// paths mirror py-sdk's transport layer exactly.
const (
	executeParamsPath = "/v1/account/transactions/params"
	submitPath        = "/submit"
	deployedPath      = "/deployed"
)

func gaslessTxPath(id string) string { return "/v1/account/transactions/" + url.PathEscape(id) }

// errInvalidNonce is returned when the relayer returns a missing or non-numeric
// nonce, which would produce an invalid signature.
var errInvalidNonce = errors.New("polyrelay: relayer returned a missing or non-numeric nonce")

// Transport issues the raw relayer HTTP calls. It wraps a *polyhttp.Client whose
// Headers func supplies relayer auth (POLY_BUILDER_* or RELAYER_*); the auth is
// the caller's concern — all methods here are issued at AuthNone and rely on the
// client's header resolver.
type Transport struct {
	http *polyhttp.Client
}

// NewTransport wraps a pre-configured relayer HTTP client.
func NewTransport(c *polyhttp.Client) *Transport {
	return &Transport{http: c}
}

// Client returns the underlying polyhttp client.
func (t *Transport) Client() *polyhttp.Client { return t.http }

// FetchExecuteParams gets the relayer execute nonce and relay address for
// address+type via /v1/account/transactions/params, the unified endpoint used by
// all wallet types (proxy, safe, deposit). py-sdk additionally exposes a legacy
// /relay-payload alias for proxy; both return the same {address, nonce} and the
// unified path is canonical (matches ts-sdk).
func (t *Transport) FetchExecuteParams(
	ctx context.Context,
	address string,
	txType RelayerTransactionType,
) (ExecuteParams, error) {
	return t.fetchParams(ctx, executeParamsPath, address, txType)
}

func (t *Transport) fetchParams(
	ctx context.Context,
	path, address string,
	txType RelayerTransactionType,
) (ExecuteParams, error) {
	q := url.Values{"address": {address}, "type": {string(txType)}}
	var w executeParamsWire
	if err := t.http.GetJSON(ctx, path, q, polyhttp.AuthNone, &w); err != nil {
		return ExecuteParams{}, err
	}
	return parseExecuteParams(w)
}

// Submit posts a gasless submission body and returns the immediate response.
// Retry on transient (rate-limit / nonce-mismatch / wallet-busy) errors is the
// caller's responsibility — see PrepareGasless.
func (t *Transport) Submit(ctx context.Context, req *SubmitRequest) (ExecuteResponse, error) {
	var w executeResponseWire
	if err := t.http.PostJSON(ctx, submitPath, req, polyhttp.AuthNone, &w); err != nil {
		return ExecuteResponse{}, err
	}
	return ExecuteResponse{
		State:           RelayerTransactionState(w.State),
		TransactionHash: w.TransactionHash,
		TransactionID:   w.TransactionID,
	}, nil
}

// GaslessTransaction fetches the current status of a submitted transaction.
func (t *Transport) GaslessTransaction(
	ctx context.Context,
	transactionID string,
) (GaslessTransaction, error) {
	var w gaslessTransactionWire
	if err := t.http.GetJSON(
		ctx,
		gaslessTxPath(transactionID),
		nil,
		polyhttp.AuthNone,
		&w,
	); err != nil {
		return GaslessTransaction{}, err
	}
	return GaslessTransaction{
		State:           RelayerTransactionState(w.State),
		TransactionHash: w.TransactionHash,
		TransactionID:   w.TransactionID,
		ErrorMsg:        w.ErrorMsg,
	}, nil
}

// IsWalletDeployed reports whether a wallet of the given type is deployed
// on-chain for the address. The type is optional (relayer infers if absent).
func (t *Transport) IsWalletDeployed(
	ctx context.Context,
	address string,
	txType RelayerTransactionType,
) (bool, error) {
	q := url.Values{"address": {address}}
	if txType != "" {
		q.Set("type", string(txType))
	}
	var w deployedWire
	if err := t.http.GetJSON(ctx, deployedPath, q, polyhttp.AuthNone, &w); err != nil {
		return false, err
	}
	return w.Deployed, nil
}
