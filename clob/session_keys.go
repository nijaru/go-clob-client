package clob

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/nijaru/go-clob-client/internal/polyhttp"
	"github.com/nijaru/go-clob-client/internal/polyrelay"
)

// SessionKeyScope is open to new server-defined scopes.
type SessionKeyScope string

const (
	SessionKeyScopeAll       SessionKeyScope = "ALL"
	SessionKeyScopeCLOB      SessionKeyScope = "CLOB"
	SessionKeyScopeCombosRFQ SessionKeyScope = "COMBOSRFQ"
	// The stable SDKs use 4,315 hours, not a rounded 180-day duration.
	sessionKeyLifetime          = 4315 * time.Hour
	sessionKeySubmissionTimeout = 5 * time.Minute
)

var (
	ErrSessionKeyOwnerRequired   = errors.New("session keys require a Deposit Wallet owner")
	ErrSessionKeyBuilderRequired = errors.New(
		"session-key authorization requires builder API-key authentication",
	)
	ErrInvalidSessionKey  = errors.New("invalid session key")
	ErrSessionKeyResponse = errors.New("invalid session-key response")
)

type SessionKey struct {
	Address    common.Address
	Scopes     []SessionKeyScope
	ValidUntil int64 // whole Unix seconds
}

type AuthorizeSessionKeyRequest struct {
	Address        common.Address
	Scopes         []SessionKeyScope // nil defaults to ALL; empty is invalid
	IdempotencyKey string            // optional; reused for every submission retry
}

type RevokeSessionKeyRequest struct {
	Address        common.Address
	IdempotencyKey string
}

type AuthorizeSessionKeyResult struct {
	SessionKey  SessionKey
	Transaction TransactionOutcome
}

func (c *AuthenticatedClient) requireSessionKeyOwner() error {
	owner, err := c.isDepositWalletOwner()
	if err != nil {
		return err
	}
	if !owner {
		return ErrSessionKeyOwnerRequired
	}
	return nil
}

// FetchSessionKeys returns the active registry, validating the registry wallet
// against this authenticated account. Only the Deposit Wallet owner may list it.
func (c *AuthenticatedClient) FetchSessionKeys(ctx context.Context) ([]SessionKey, error) {
	if err := c.requireSessionKeyOwner(); err != nil {
		return nil, err
	}
	var response struct {
		Wallet  string `json:"wallet"`
		Signers *[]struct {
			Address    string            `json:"address"`
			Scopes     []SessionKeyScope `json:"scopes"`
			ValidUntil *int64            `json:"valid_until"`
		} `json:"signers"`
	}
	if err := c.http.GetJSON(ctx, "/v1/user/session-signers", nil, polyhttp.AuthL2, &response); err != nil {
		return nil, err
	}
	if !common.IsHexAddress(response.Wallet) ||
		common.HexToAddress(response.Wallet) != c.WalletAddress() ||
		response.Signers == nil {
		return nil, fmt.Errorf("%w: registry wallet or signers", ErrSessionKeyResponse)
	}
	keys := make([]SessionKey, 0, len(*response.Signers))
	for _, signer := range *response.Signers {
		if !common.IsHexAddress(signer.Address) || signer.ValidUntil == nil ||
			*signer.ValidUntil < 0 ||
			len(signer.Scopes) == 0 {
			return nil, fmt.Errorf("%w: signer metadata", ErrSessionKeyResponse)
		}
		for _, scope := range signer.Scopes {
			if scope == "" {
				return nil, fmt.Errorf("%w: empty scope", ErrSessionKeyResponse)
			}
		}
		keys = append(
			keys,
			SessionKey{
				Address:    common.HexToAddress(signer.Address),
				Scopes:     signer.Scopes,
				ValidUntil: *signer.ValidUntil,
			},
		)
	}
	return keys, nil
}

type sessionKeyMutationPayload struct {
	Deadline             string            `json:"deadline"`
	Nonce                string            `json:"nonce"`
	Scopes               []SessionKeyScope `json:"scopes,omitempty"`
	SessionSignerAddress string            `json:"sessionSignerAddress"`
	Signature            string            `json:"signature"`
	ValidUntil           string            `json:"validUntil,omitempty"`
	WalletAddress        string            `json:"walletAddress"`
}

type sessionKeyMutationResponse struct {
	Status          string `json:"status"`
	TransactionHash string `json:"transactionHash"`
	TransactionID   string `json:"transactionId"`
}

// AuthorizeSessionKey signs an owner batch, submits its scoped authorization,
// waits for chain confirmation, then verifies exact expiry and scopes in the
// active registry. The application owns the session key's private key.
func (c *AuthenticatedClient) AuthorizeSessionKey(
	ctx context.Context,
	req AuthorizeSessionKeyRequest,
) (*AuthorizeSessionKeyResult, error) {
	if err := c.requireSessionKeyOwner(); err != nil {
		return nil, err
	}
	if c.relayerAuth(ctx).BuilderAuth == nil {
		return nil, ErrSessionKeyBuilderRequired
	}
	if req.Address == (common.Address{}) {
		return nil, ErrInvalidSessionKey
	}
	scopes := slices.Clone(req.Scopes)
	if scopes == nil {
		scopes = []SessionKeyScope{SessionKeyScopeAll}
	}
	if len(scopes) == 0 {
		return nil, ErrInvalidSessionKey
	}
	for _, scope := range scopes {
		if scope == "" ||
			(scope != SessionKeyScopeAll && slices.Contains(scopes, SessionKeyScopeAll)) {
			return nil, fmt.Errorf("%w: scopes", ErrInvalidSessionKey)
		}
	}
	idempotency, err := sessionKeyIdempotencyKey(req.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	validUntil := time.Now().Add(sessionKeyLifetime).Unix()
	payload, err := c.signSessionKeyMutation(ctx, req.Address, &validUntil)
	if err != nil {
		return nil, err
	}
	payload.Scopes = scopes
	var response sessionKeyMutationResponse
	if err := c.submitSessionKeyMutation(ctx, "/v1/session-signers/authorizations", idempotency, payload, &response); err != nil {
		return nil, err
	}
	if err := validateSessionKeyStatus(response, true); err != nil {
		return nil, err
	}
	transaction, err := polyrelay.NewHandle(c.RelayerTransport(), polyrelay.ExecuteResponse{TransactionID: response.TransactionID, TransactionHash: response.TransactionHash}).
		Wait(ctx)
	if err != nil {
		return nil, err
	}
	expected := SessionKey{Address: req.Address, Scopes: scopes, ValidUntil: validUntil}
	key, err := c.waitSessionKeyRegistry(ctx, expected, true)
	if err != nil {
		return nil, err
	}
	return &AuthorizeSessionKeyResult{SessionKey: *key, Transaction: *transaction}, nil
}

// RevokeSessionKey returns after the key leaves the active registry. Its chain
// transaction may still be pending. A failed registry read never proves removal.
func (c *AuthenticatedClient) RevokeSessionKey(
	ctx context.Context,
	req RevokeSessionKeyRequest,
) error {
	if err := c.requireSessionKeyOwner(); err != nil {
		return err
	}
	if req.Address == (common.Address{}) {
		return ErrInvalidSessionKey
	}
	idempotency, err := sessionKeyIdempotencyKey(req.IdempotencyKey)
	if err != nil {
		return err
	}
	payload, err := c.signSessionKeyMutation(ctx, req.Address, nil)
	if err != nil {
		return err
	}
	var response sessionKeyMutationResponse
	if err := c.submitSessionKeyMutation(ctx, "/v1/session-signers/revocations", idempotency, payload, &response); err != nil {
		return err
	}
	if err := validateSessionKeyStatus(response, false); err != nil {
		return err
	}
	_, err = c.waitSessionKeyRegistry(ctx, SessionKey{Address: req.Address}, false)
	return err
}

func (c *AuthenticatedClient) signSessionKeyMutation(
	ctx context.Context,
	address common.Address,
	expiry *int64,
) (sessionKeyMutationPayload, error) {
	data, err := packSessionKeyCall(address, expiry)
	if err != nil {
		return sessionKeyMutationPayload{}, err
	}
	cfg, err := c.gaslessConfig()
	if err != nil {
		return sessionKeyMutationPayload{}, err
	}
	envelope, err := polyrelay.BuildGaslessSubmit(
		ctx,
		c.RelayerTransport(),
		cfg,
		c.signer.PrivateKey(),
		[]TransactionCall{tokenCall(c.WalletAddress(), data)},
		"",
	)
	if err != nil {
		return sessionKeyMutationPayload{}, err
	}
	payload := sessionKeyMutationPayload{
		Deadline:             envelope.DepositWallet.Deadline,
		Nonce:                envelope.Nonce,
		SessionSignerAddress: strings.ToLower(address.Hex()),
		Signature:            envelope.Signature,
		WalletAddress:        c.WalletAddress().Hex(),
	}
	if expiry != nil {
		payload.ValidUntil = fmt.Sprint(*expiry)
	}
	return payload, nil
}

func sessionKeyIdempotencyKey(key string) (string, error) {
	if key != "" {
		key = strings.TrimSpace(key)
		if key == "" {
			return "", fmt.Errorf("%w: empty idempotency key", ErrInvalidSessionKey)
		}
		return key, nil
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", id[:4], id[4:6], id[6:8], id[8:10], id[10:]), nil
}

func validateSessionKeyStatus(response sessionKeyMutationResponse, authorization bool) error {
	accepted := []string{"PENDING", "FENCED", "SWEPT", "CHAIN_SUBMITTED", "CONFIRMED"}
	failed := []string{"FAILED"}
	if authorization {
		accepted = []string{"SUBMITTED", "REGISTRY_PENDING", "REGISTERED"}
		failed = append(failed, "SUPERSEDED", "REPAIR_REQUIRED")
	}
	if slices.Contains(failed, response.Status) {
		return fmt.Errorf("%w: session-key status %s", ErrWalletTransactionFailed, response.Status)
	}
	if !slices.Contains(accepted, response.Status) || response.TransactionID == "" {
		return fmt.Errorf("%w: operation status or transaction ID", ErrSessionKeyResponse)
	}
	if response.TransactionHash != "" {
		if len(response.TransactionHash) != 66 ||
			!strings.HasPrefix(response.TransactionHash, "0x") ||
			len(common.FromHex(response.TransactionHash)) != 32 {
			return fmt.Errorf("%w: transaction hash", ErrSessionKeyResponse)
		}
	}
	return nil
}

func (c *AuthenticatedClient) submitSessionKeyMutation(
	ctx context.Context,
	path, idempotency string,
	payload sessionKeyMutationPayload,
	out *sessionKeyMutationResponse,
) error {
	requestCtx, cancel := context.WithTimeout(ctx, sessionKeySubmissionTimeout)
	defer cancel()
	httpClient := *c.http.HTTPClient
	if httpClient.Timeout > 0 && httpClient.Timeout < sessionKeySubmissionTimeout {
		httpClient.Timeout = sessionKeySubmissionTimeout
	}
	transport := &polyhttp.Client{
		BaseURL:    c.relayerHost,
		HTTPClient: &httpClient,
		UserAgent:  c.http.UserAgent,
		Headers:    c.relayerHeaders,
	}
	// Retry the identical signed body and key, never rebuild the nonce/expiry.
	for attempt := 0; ; attempt++ {
		err := transport.DoJSON(
			requestCtx,
			http.MethodPost,
			path,
			nil,
			payload,
			polyhttp.AuthNone,
			nil,
			map[string]string{"Idempotency-Key": idempotency},
			out,
		)
		if err == nil || attempt == 2 || !retryWalletError(err, false) {
			return err
		}
		if err := walletSleep(requestCtx, polyrelay.DefaultPollInterval); err != nil {
			return err
		}
	}
}

func (c *AuthenticatedClient) waitSessionKeyRegistry(
	ctx context.Context,
	expected SessionKey,
	authorization bool,
) (*SessionKey, error) {
	for attempt := 0; attempt < polyrelay.DefaultPollMaxAttempts; attempt++ {
		keys, err := c.FetchSessionKeys(ctx)
		if err != nil && !retryWalletError(err, true) {
			return nil, err
		}
		if err == nil {
			active := false
			for _, key := range keys {
				if key.Address != expected.Address {
					continue
				}
				active = true
				if authorization && key.ValidUntil == expected.ValidUntil &&
					equalSessionScopes(key.Scopes, expected.Scopes) {
					return &key, nil
				}
			}
			if !authorization && !active {
				return nil, nil
			}
		}
		if attempt+1 < polyrelay.DefaultPollMaxAttempts {
			if err := walletSleep(ctx, polyrelay.DefaultPollInterval); err != nil {
				return nil, err
			}
		}
	}
	return nil, fmt.Errorf(
		"%w: session key %s registry readiness",
		ErrWalletTransactionTimeout,
		expected.Address,
	)
}

func equalSessionScopes(left, right []SessionKeyScope) bool {
	for _, scope := range left {
		if !slices.Contains(right, scope) {
			return false
		}
	}
	for _, scope := range right {
		if !slices.Contains(left, scope) {
			return false
		}
	}
	return true
}
