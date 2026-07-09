package clob

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"

	"github.com/nijaru/go-clob-client/internal/polyauth"
	"github.com/nijaru/go-clob-client/internal/polyhttp"
	"github.com/nijaru/go-clob-client/internal/polyrelay"
)

// Gasless (relayer) operations. These submit on-chain calls through Polymarket's
// relayer without the caller paying gas — required for proxy, Safe, and deposit
// (Poly1271) wallets. EOA wallets broadcast directly via go-ethereum (see ctf.go)
// and do not use the relayer.
//
// The relayer authenticates with POLY_BUILDER_* headers (the same builder-key
// HMAC). This client prefers an explicit BuilderAuth and otherwise signs with the
// L2 API credentials, which serve the same role. The heavy lifting (nonce fetch,
// per-scheme signing, retry, poll) lives in internal/polyrelay; this file wires
// it onto the AuthenticatedClient using the chain's contract addresses.

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
// pointed at the relayer host with builder-key auth. Each call builds a fresh
// transport; construction is cheap.
func (c *AuthenticatedClient) RelayerTransport() *polyrelay.Transport {
	return polyrelay.NewTransport(&polyhttp.Client{
		BaseURL:    c.relayerHost,
		HTTPClient: c.http.HTTPClient,
		UserAgent:  c.http.UserAgent,
		Headers:    c.relayerHeaders,
	})
}

// relayerHeaders always emits POLY_BUILDER_* auth — the relayer requires it on
// every call regardless of auth level. The signature covers method + bare path +
// body (query string excluded), matching py-sdk's transport and polyhttp's
// header-invocation contract.
func (c *AuthenticatedClient) relayerHeaders(
	ctx context.Context,
	method, path string,
	body []byte,
	_ polyhttp.AuthLevel,
	_ *int64,
) (map[string]string, error) {
	timestamp, err := c.timestamp(ctx)
	if err != nil {
		return nil, err
	}
	if ba := c.getBuilderAuth(); ba != nil {
		return ba.Headers(ctx, BuilderHeaderRequest{
			Method: method, Path: path, Body: body, Timestamp: timestamp,
		})
	}
	creds := c.credentials()
	if creds == nil {
		return nil, fmt.Errorf("gasless: relayer auth requires API credentials or BuilderAuth")
	}
	return polyauth.BuilderHeaders(
		creds.Key,
		c.decodedSecret,
		creds.Passphrase,
		timestamp,
		method,
		path,
		body,
	)
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
	return polyrelay.GaslessConfig{
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
	calls []polyrelay.TransactionCall,
	metadata string,
) (*polyrelay.Handle, error) {
	cfg, err := c.gaslessConfig()
	if err != nil {
		return nil, err
	}
	return polyrelay.PrepareGasless(
		ctx,
		c.RelayerTransport(),
		cfg,
		c.signer.PrivateKey(),
		calls,
		metadata,
	)
}

// DeployDepositWallet submits an unsigned WALLET-CREATE to deploy a new deposit
// wallet for the signer via the relayer.
func (c *AuthenticatedClient) DeployDepositWallet(
	ctx context.Context,
	metadata string,
) (*polyrelay.Handle, error) {
	wc, err := getWalletConfig(c.chainID)
	if err != nil {
		return nil, err
	}
	if wc.DepositWalletFactory == "" {
		return nil, fmt.Errorf("gasless: deposit wallets unsupported on chain %d", c.chainID)
	}
	return polyrelay.DeployDepositWallet(
		ctx, c.RelayerTransport(),
		common.HexToAddress(c.Address()),
		common.HexToAddress(wc.DepositWalletFactory),
		metadata,
	)
}

// IsWalletDeployed reports whether the client's wallet is deployed on-chain.
func (c *AuthenticatedClient) IsWalletDeployed(ctx context.Context) (bool, error) {
	cfg, err := c.gaslessConfig()
	if err != nil {
		return false, err
	}
	return c.RelayerTransport().IsWalletDeployed(ctx, c.funderAddress, cfg.WalletType)
}
