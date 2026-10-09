package clob

import (
	"context"
	"fmt"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// IsWalletDeployedAt reads the public relayer deployment view for an explicit
// wallet and signature type. No signer, CLOB credentials, or relayer auth is
// required or sent. This does not prove chain confirmation, approvals, or
// trading readiness. An EOA is not a deployed contract wallet and returns false.
func (c *Client) IsWalletDeployedAt(
	ctx context.Context,
	wallet common.Address,
	walletType SignatureType,
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if wallet == (common.Address{}) {
		return false, fmt.Errorf("wallet: deployment probe requires a nonzero address")
	}
	query := url.Values{"address": {wallet.Hex()}}
	switch walletType {
	case SignatureTypeEOA:
		return false, nil
	case SignatureTypePolyProxy, SignatureTypePolyGnosisSafe:
		// The public relayer infers legacy proxy/Safe types from the address.
	case SignatureTypePoly1271:
		query.Set("type", string(RelayerTransactionWallet))
	default:
		return false, fmt.Errorf("wallet: unsupported signature type %d", walletType)
	}
	transport := &polyhttp.Client{
		BaseURL: c.relayerHost, HTTPClient: c.http.HTTPClient, UserAgent: c.http.UserAgent,
	}
	var response struct {
		Deployed *bool `json:"deployed"`
	}
	if err := transport.GetJSON(ctx, "/deployed", query, polyhttp.AuthNone, &response); err != nil {
		return false, err
	}
	if response.Deployed == nil {
		return false, fmt.Errorf("wallet: deployment response is missing deployed")
	}
	return *response.Deployed, nil
}

// DiscoverDepositWallet explicitly selects the owner's deployed legacy UUPS
// wallet, or otherwise its deterministic beacon wallet, as in the unified SDKs'
// default wallet flow. It performs one public relayer read, not a deployment or
// factory RPC call. The returned beacon wallet need not be deployed. Errors never
// select a fallback wallet. No configured client identity is changed; pass the
// returned address explicitly to a subsequent offline constructor if desired.
func (c *Client) DiscoverDepositWallet(
	ctx context.Context,
	owner common.Address,
) (common.Address, error) {
	if owner == (common.Address{}) {
		return common.Address{}, fmt.Errorf("wallet: discovery requires a nonzero owner")
	}
	legacy, err := DeriveUUPSDepositWallet(owner, c.chainID)
	if err != nil {
		return common.Address{}, err
	}
	if legacy == (common.Address{}) {
		return common.Address{}, fmt.Errorf(
			"wallet: deposit wallet unsupported on chain %d",
			c.chainID,
		)
	}
	deployed, err := c.IsWalletDeployedAt(ctx, legacy, SignatureTypePoly1271)
	if err != nil {
		return common.Address{}, err
	}
	if deployed {
		return legacy, nil
	}
	return DeriveBeaconDepositWallet(owner, c.chainID)
}

// IsDepositWalletDeployed explicitly probes the signer's factory-current
// deposit wallet, including when this client is configured as an EOA. Factory
// selection uses DeriveCurrentDepositWallet's read-only RPC; deployment uses
// the public relayer view. This does not change the client's configured wallet.
// Use DiscoverDepositWallet instead for deployed-legacy-first selection.
func (c *SignerClient) IsDepositWalletDeployed(ctx context.Context) (bool, error) {
	wallet, err := c.DeriveCurrentDepositWallet(ctx)
	if err != nil {
		return false, err
	}
	return c.Client.IsWalletDeployedAt(ctx, wallet, SignatureTypePoly1271)
}
