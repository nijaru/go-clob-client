package perps

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/nijaru/go-clob-client/internal/polyauth"
	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// OwnerSigner supports local keys, hardware wallets, and external signers.
// SignTypedData must return a hex Ethereum signature with recovery byte 27/28.
type OwnerSigner interface {
	Address() common.Address
	SignTypedData(context.Context, apitypes.TypedData) (string, error)
}

type localOwnerSigner struct{ signer *polyauth.Signer }

func (s localOwnerSigner) Address() common.Address { return s.signer.Address() }

func (s localOwnerSigner) SignTypedData(
	ctx context.Context,
	data apitypes.TypedData,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return polyauth.SignTypedData(ctx, s.signer, data)
}

// NewOwnerSigner adapts a local private key. Never log or persist the key.
func NewOwnerSigner(privateKey string) (OwnerSigner, error) {
	s, err := polyauth.ParsePrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	return localOwnerSigner{s}, nil
}

// OwnerConfig explicitly selects the wallet and collateral contracts; no wallet
// derivation or transaction broadcasting is hidden in this client.
type OwnerConfig struct {
	Config
	Signer          OwnerSigner
	Wallet          string
	CollateralToken string
	DepositContract string
}

// OwnerClient handles owner-only credentials, consent, and collateral requests.
// Submissions are attempted once. Reconcile uncertain outcomes before retrying.
type OwnerClient struct {
	*Client
	signer                 OwnerSigner
	chainID                int64
	wallet, token, deposit common.Address
	config                 Config
	consentMu              sync.Mutex
}

func NewOwner(config OwnerConfig) (*OwnerClient, error) {
	if config.Signer == nil || config.Signer.Address() == (common.Address{}) {
		return nil, fmt.Errorf("perps: owner signer is required")
	}
	if config.Wallet == "" {
		config.Wallet = config.Signer.Address().Hex()
	}
	for name, address := range map[string]string{"wallet": config.Wallet, "collateral token": config.CollateralToken, "deposit contract": config.DepositContract} {
		if address == "" && name != "wallet" {
			continue
		}
		if !common.IsHexAddress(address) || common.HexToAddress(address) == (common.Address{}) {
			return nil, fmt.Errorf("perps: invalid %s address", name)
		}
	}
	cfg := config.Config.normalized()
	return &OwnerClient{
		Client:  New(cfg),
		signer:  config.Signer,
		chainID: cfg.ChainID,
		wallet:  common.HexToAddress(config.Wallet),
		token:   common.HexToAddress(config.CollateralToken),
		deposit: common.HexToAddress(config.DepositContract),
		config:  cfg,
	}, nil
}

func (c *OwnerClient) signedOperation(
	ctx context.Context,
	name string,
	raw []any,
	args any,
) (map[string]any, error) {
	salt, err := randomPerpsSalt()
	if err != nil {
		return nil, err
	}
	ts := time.Now().UnixMilli()
	data, err := operationTypedData(c.chainID, []any{name, raw}, salt, ts)
	if err != nil {
		return nil, err
	}
	sig, err := c.signer.SignTypedData(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("perps: sign %s: %w", name, err)
	}
	return map[string]any{
		"op":   map[string]any{"type": name, "args": args},
		"salt": salt,
		"sig":  sig,
		"ts":   ts,
	}, nil
}

func (c *OwnerClient) submit(ctx context.Context, method, path string, body, out any) error {
	return c.http.DoJSON(ctx, method, path, nil, body, polyhttp.AuthNone, nil, nil, out)
}

// CredentialInfo is the server-authoritative owner and proxy-key listing.
type CredentialInfo struct {
	Address string          `json:"address"`
	Keys    []CredentialKey `json:"keys"`
}
type CredentialKey struct {
	Proxy     string `json:"proxy"`
	ExpiresAt int64  `json:"expiry"`
	Label     string `json:"label,omitempty"`
}

// UnmarshalJSON normalizes the credential endpoint's nanosecond expiry exactly.
func (k *CredentialKey) UnmarshalJSON(data []byte) error {
	var wire struct {
		Proxy  string `json:"proxy"`
		Expiry uint64 `json:"expiry"`
		Label  string `json:"label"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if wire.Expiry == 0 {
		return fmt.Errorf("perps: invalid credential expiry")
	}
	if wire.Expiry > 9007199254740991 {
		wire.Expiry /= 1000000
	}
	*k = CredentialKey{Proxy: wire.Proxy, ExpiresAt: int64(wire.Expiry), Label: wire.Label}
	return nil
}

func (c *AuthenticatedClient) GetCredentials(ctx context.Context) (*CredentialInfo, error) {
	var out CredentialInfo
	if err := c.getAuthenticatedJSON(ctx, "/v1/account/credentials", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Resume validates key identity, owner, and expiration against the server.
func (c *OwnerClient) Resume(
	ctx context.Context,
	credentials PerpsCredentials,
) (*AuthenticatedClient, error) {
	client, err := NewAuthenticated(AuthenticatedConfig{Config: c.config, Credentials: credentials})
	if err != nil {
		return nil, err
	}
	signer, err := client.delegatedSigner()
	if err != nil {
		return nil, err
	}
	if signer == nil {
		return nil, ErrPerpsSigningKeyRequired
	}
	info, err := client.GetCredentials(ctx)
	if err != nil {
		return nil, err
	}
	if !common.IsHexAddress(info.Address) ||
		common.HexToAddress(info.Address) != c.signer.Address() {
		return nil, fmt.Errorf("perps: credentials belong to a different owner")
	}
	for _, key := range info.Keys {
		if common.IsHexAddress(key.Proxy) && common.HexToAddress(key.Proxy) == signer.Address() {
			if key.ExpiresAt <= time.Now().UnixMilli() {
				return nil, fmt.Errorf("perps: credentials expired")
			}
			client.credentials.ExpiresAt = key.ExpiresAt
			return client, nil
		}
	}
	return nil, fmt.Errorf("perps: proxy absent from credential listing")
}

type CreateCredentialsRequest struct {
	// Lifetime defaults to seven days.
	Lifetime time.Duration
	Label    string
}

// CreateCredentials returns generated key material even when submission or
// subsequent validation has an uncertain outcome. Keep it private; reconcile
// or revoke the returned proxy rather than blindly creating another.
func (c *OwnerClient) CreateCredentials(
	ctx context.Context,
	request CreateCredentialsRequest,
) (PerpsCredentials, error) {
	lifetime := request.Lifetime
	if lifetime == 0 {
		lifetime = 7 * 24 * time.Hour
	}
	if lifetime < time.Millisecond {
		return PerpsCredentials{}, fmt.Errorf("perps: credential lifetime must be positive")
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		return PerpsCredentials{}, err
	}
	proxy := crypto.PubkeyToAddress(key.PublicKey).Hex()
	salt, err := randomPerpsSalt()
	if err != nil {
		return PerpsCredentials{}, err
	}
	ts := time.Now().UnixMilli()
	expiry := ts + lifetime.Milliseconds()
	data := perpsTypedData(
		c.chainID,
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
			"salt": strconv.FormatUint(salt, 10),
			"ts":   strconv.FormatInt(ts, 10),
		},
		"",
	)
	sig, err := c.signer.SignTypedData(ctx, data)
	if err != nil {
		return PerpsCredentials{}, err
	}
	body := map[string]any{
		"op": map[string]any{
			"type": "createProxy",
			"args": map[string]any{
				"owner":  c.signer.Address().Hex(),
				"proxy":  proxy,
				"expiry": expiry,
			},
		},
		"salt": salt,
		"ts":   ts,
		"sig":  sig,
	}
	if request.Label != "" {
		body["label"] = request.Label
	}
	var out struct {
		Secret string `json:"secret"`
	}
	credentials := PerpsCredentials{
		Proxy:      proxy,
		PrivateKey: hex.EncodeToString(crypto.FromECDSA(key)),
		ExpiresAt:  expiry,
	}
	if err := c.submit(ctx, http.MethodPost, "/v1/account/proxy", body, &out); err != nil {
		return credentials, err
	}
	credentials.Secret = out.Secret
	if out.Secret == "" {
		return credentials, fmt.Errorf("perps: credential response missing secret")
	}
	client, err := c.Resume(ctx, credentials)
	if err != nil {
		return credentials, err
	}
	return client.Credentials(), nil
}

func (c *OwnerClient) RevokeCredentials(ctx context.Context, proxy string) error {
	if !common.IsHexAddress(proxy) {
		return fmt.Errorf("perps: invalid proxy")
	}
	body, err := c.signedOperation(ctx, "deleteProxy", []any{proxy}, struct {
		Proxy string `json:"proxy"`
	}{proxy})
	if err != nil {
		return err
	}
	var ack sessionAck
	if err := c.submit(ctx, http.MethodDelete, "/v1/account/proxy", body, &ack); err != nil {
		return err
	}
	return commandRejection("deleteProxy", ack.Status, ack.Error)
}
