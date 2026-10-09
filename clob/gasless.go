package clob

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
	"github.com/nijaru/go-clob-client/internal/polyrelay"
)

// Gasless (relayer) operations. These submit on-chain calls through Polymarket's
// relayer without the caller paying gas — required for proxy, Safe, and deposit
// (Poly1271) wallets. EOA wallets broadcast directly via go-ethereum (see ctf.go)
// and do not use the relayer.
//
// The relayer authenticates with builder HMAC or Relayer API-key headers.
// WithRelayerAuth explicitly selects request-scoped auth; otherwise the client's
// BuilderAuth is used. CLOB L2 credentials never authenticate the relayer.
// Nonce fetching, per-scheme signing, retries, and polling live in
// internal/polyrelay; this file supplies the client's chain and wallet identity.

// relayerWalletType maps the client's SignatureType to a relayer transaction
// type. EOA returns an error (EOAs broadcast directly, not via the relayer).
func (s SignatureType) relayerWalletType() (polyrelay.RelayerTransactionType, error) {
	switch s {
	case SignatureTypePolyProxy:
		return polyrelay.TransactionTypeProxy, nil
	case SignatureTypePolyGnosisSafe:
		return polyrelay.TransactionTypeSafe, nil
	case SignatureTypePoly1271:
		return polyrelay.TransactionTypeWallet, nil
	default:
		return "", fmt.Errorf(
			"gasless: signature type %d does not use the relayer (EOAs broadcast directly)",
			s,
		)
	}
}

// RelayerTransport returns a relayer transport backed by a polyhttp client
// pointed at the relayer host with context-selected auth. Each call builds a fresh
// transport; construction is cheap.
func (c *AuthenticatedClient) RelayerTransport() *RelayerTransport {
	return polyrelay.NewTransport(&polyhttp.Client{
		BaseURL:    c.relayerHost,
		HTTPClient: c.http.HTTPClient,
		UserAgent:  c.http.UserAgent,
		Headers:    c.relayerHeaders,
	})
}

// relayerHeaders emits exactly one auth scheme. Builder signatures cover the
// method + bare path + body, excluding query strings. API-key auth requires no
// timestamp or CLOB server-time request.
func (c *AuthenticatedClient) relayerHeaders(
	ctx context.Context,
	method, path string,
	body []byte,
	_ polyhttp.AuthLevel,
	_ *int64,
) (map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	auth := c.relayerAuth(ctx)
	if auth.APIKey != nil {
		return map[string]string{
			"RELAYER_API_KEY":         auth.APIKey.Key,
			"RELAYER_API_KEY_ADDRESS": auth.APIKey.Address.Hex(),
		}, nil
	}
	if auth.BuilderAuth == nil {
		return nil, fmt.Errorf(
			"gasless: relayer auth requires BuilderAuth or RelayerAPIKey, not CLOB API credentials",
		)
	}
	timestamp, err := c.timestamp(ctx)
	if err != nil {
		return nil, err
	}
	headers, err := auth.BuilderAuth.Headers(ctx, BuilderHeaderRequest{
		Method: method, Path: path, Body: body, Timestamp: timestamp,
	})
	if err != nil {
		return nil, err
	}
	// Keep a remote/custom builder resolver from injecting another identity or
	// overriding transport headers. Only the builder wire contract is accepted.
	result := make(map[string]string, 4)
	for _, name := range []string{"POLY_BUILDER_API_KEY", "POLY_BUILDER_PASSPHRASE", "POLY_BUILDER_SIGNATURE", "POLY_BUILDER_TIMESTAMP"} {
		if headers[name] == "" {
			return nil, fmt.Errorf("%w: missing %s", ErrInvalidRelayerAuth, name)
		}
		result[name] = headers[name]
	}
	return result, nil
}

// gaslessConfig builds the polyrelay config from the client's chain + wallet
// state, validating the wallet type is supported on this chain.
func (c *AuthenticatedClient) gaslessConfig() (polyrelay.GaslessConfig, error) {
	walletType, err := c.signatureType.relayerWalletType()
	if err != nil {
		return polyrelay.GaslessConfig{}, err
	}
	wc, err := getWalletConfig(c.chainID)
	if err != nil {
		return polyrelay.GaslessConfig{}, err
	}
	switch walletType {
	case polyrelay.TransactionTypeProxy:
		if wc.ProxyFactory == "" || wc.RelayHub == "" {
			return polyrelay.GaslessConfig{}, fmt.Errorf(
				"gasless: proxy wallets unsupported on chain %d",
				c.chainID,
			)
		}
	case polyrelay.TransactionTypeSafe:
		if wc.SafeMultisend == "" {
			return polyrelay.GaslessConfig{}, fmt.Errorf(
				"gasless: safe wallets unsupported on chain %d",
				c.chainID,
			)
		}
	case polyrelay.TransactionTypeWallet:
		if wc.DepositWalletFactory == "" {
			return polyrelay.GaslessConfig{}, fmt.Errorf(
				"gasless: deposit wallets unsupported on chain %d",
				c.chainID,
			)
		}
	}
	sessionSigner := false
	if walletType == polyrelay.TransactionTypeWallet {
		owner, err := c.isDepositWalletOwner()
		if err != nil {
			return polyrelay.GaslessConfig{}, err
		}
		sessionSigner = !owner
	}
	return polyrelay.GaslessConfig{
		SessionSigner:        sessionSigner,
		WalletType:           walletType,
		Signer:               common.HexToAddress(c.Address()),
		Wallet:               common.HexToAddress(c.funderAddress),
		ChainID:              big.NewInt(c.chainID),
		ProxyFactory:         common.HexToAddress(wc.ProxyFactory),
		DepositWalletFactory: common.HexToAddress(wc.DepositWalletFactory),
		RelayHub:             common.HexToAddress(wc.RelayHub),
		SafeMultisend:        common.HexToAddress(wc.SafeMultisend),
		GasEstimator:         c.estimateProxyGas,
	}, nil
}

// estimateProxyGas estimates gas for a proxy submission via eth_estimateGas.
// Errors fall back to the relayer default (200000) inside polyrelay.
func (c *AuthenticatedClient) estimateProxyGas(
	ctx context.Context,
	from, to common.Address,
	data []byte,
) (uint64, error) {
	ec, err := c.dialRPC(ctx)
	if err != nil {
		return 0, err
	}
	defer ec.Close()
	return ec.EstimateGas(ctx, ethereum.CallMsg{From: from, To: &to, Data: data})
}

// PrepareGaslessTransaction signs and submits a batch of calls through the
// relayer for the client's wallet type, retrying transient submit failures.
// The returned Handle is polled (or Wait-ed) for confirmation.
func (c *AuthenticatedClient) PrepareGaslessTransaction(
	ctx context.Context,
	calls []TransactionCall,
	metadata string,
) (*GaslessTransactionHandle, error) {
	cfg, err := c.gaslessConfig()
	if err != nil {
		return nil, err
	}
	return polyrelay.PrepareGasless(
		ctx,
		c.RelayerTransport(),
		cfg,
		c.signer,
		calls,
		metadata,
	)
}

// DeployDepositWallet submits an unsigned WALLET-CREATE for the configured
// owner's beacon Deposit Wallet. It cannot deploy a different wallet from the
// one used by this client. Confirmation is explicit via the returned handle.
func (c *AuthenticatedClient) DeployDepositWallet(
	ctx context.Context,
	metadata string,
) (*GaslessTransactionHandle, error) {
	if err := c.requireDepositWalletDeploymentTarget(); err != nil {
		return nil, err
	}
	wc, err := getWalletConfig(c.chainID)
	if err != nil {
		return nil, err
	}
	return polyrelay.DeployDepositWallet(
		ctx, c.RelayerTransport(),
		common.HexToAddress(c.Address()),
		common.HexToAddress(wc.DepositWalletFactory),
		metadata,
	)
}

// IsWalletDeployed reads the relayer deployment view of the configured wallet
// using context-selected relayer auth. It does not prove chain confirmation or
// trading readiness. Use IsWalletDeployedAt for a public unauthenticated probe,
// or IsDepositWalletDeployed for an EOA-derived deposit wallet probe.
func (c *AuthenticatedClient) IsWalletDeployed(ctx context.Context) (bool, error) {
	cfg, err := c.gaslessConfig()
	if err != nil {
		return false, err
	}
	return c.RelayerTransport().IsWalletDeployed(ctx, c.funderAddress, cfg.WalletType)
}
