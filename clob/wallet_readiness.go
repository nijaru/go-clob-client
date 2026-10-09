package clob

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/nijaru/go-clob-client/internal/polyrelay"
)

var (
	ErrWalletDeploymentRequired = errors.New("wallet must already be deployed")
	ErrWalletDeploymentIdentity = errors.New(
		"deposit wallet deployment requires the signer's beacon wallet",
	)
)

// WalletReadiness records deployment visibility separately from confirmation.
// Deployment is the submitted handle; Transaction is set only after relayer
// confirmation, not receipt verification. Both are nil for an already-deployed
// wallet or EOA. Partial progress is returned on wait errors: resume
// Deployment.Wait or WaitWalletDeployed rather than submitting again.
type WalletReadiness struct {
	Wallet      common.Address
	Deployed    bool // relayer deployment view; not CLOB/indexer readiness
	Deployment  *GaslessTransactionHandle
	Transaction *TransactionOutcome
}

func (c *AuthenticatedClient) requireDepositWalletDeploymentTarget() error {
	if c.signatureType != SignatureTypePoly1271 {
		return ErrWalletDeploymentIdentity
	}
	wallet, err := DeriveBeaconDepositWallet(c.signer.Address(), c.chainID)
	if err != nil {
		return err
	}
	if wallet == (common.Address{}) || c.WalletAddress() != wallet {
		return ErrWalletDeploymentIdentity
	}
	return nil
}

// EnsureWalletReady explicitly checks readiness and, only for an undeployed
// owner beacon Deposit Wallet or deterministic owner Safe, submits deployment
// and waits for relayer confirmation and deployment visibility. EOA readiness
// requires no remote request. Undeployed proxy, legacy UUPS, and session-signer
// wallets are not created. It does not verify chain receipts, grant approvals,
// or wait for CLOB balances/session-key indexing.
func (c *AuthenticatedClient) EnsureWalletReady(
	ctx context.Context,
	metadata string,
) (*WalletReadiness, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := &WalletReadiness{Wallet: c.WalletAddress()}
	if c.signatureType == SignatureTypeEOA {
		result.Deployed = true
		return result, nil
	}
	deployed, err := c.IsWalletDeployed(ctx)
	if err != nil {
		return nil, err
	}
	if deployed {
		result.Deployed = true
		return result, nil
	}
	var handle *GaslessTransactionHandle
	switch c.signatureType {
	case SignatureTypePoly1271:
		handle, err = c.DeployDepositWallet(ctx, metadata)
	case SignatureTypePolyGnosisSafe:
		handle, err = c.DeploySafeWallet(ctx, metadata)
	default:
		return nil, fmt.Errorf("%w: %s", ErrWalletDeploymentRequired, result.Wallet)
	}
	if err != nil {
		return nil, err
	}
	// Retain the handle even if confirmation fails or is cancelled.
	result.Deployment = handle
	outcome, err := handle.Wait(ctx)
	if err != nil {
		return result, err
	}
	result.Transaction = outcome
	if err := c.WaitWalletDeployed(ctx); err != nil {
		return result, err
	}
	result.Deployed = true
	return result, nil
}

// WaitWalletDeployed waits for the relayer's deployment view of this wallet.
// It does not prove transaction confirmation or CLOB/indexer readiness. An EOA
// is immediately ready. Failed reads never count as a deployed wallet.
func (c *AuthenticatedClient) WaitWalletDeployed(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.signatureType == SignatureTypeEOA {
		return nil
	}
	for attempt := 0; attempt < polyrelay.DefaultPollMaxAttempts; attempt++ {
		deployed, err := c.IsWalletDeployed(ctx)
		if err != nil && !retryWalletError(err, false) {
			return err
		}
		if err == nil && deployed {
			return nil
		}
		if attempt+1 < polyrelay.DefaultPollMaxAttempts {
			if err := walletSleep(ctx, polyrelay.DefaultPollInterval); err != nil {
				return err
			}
		}
	}
	return fmt.Errorf(
		"%w: wallet %s deployment visibility",
		ErrWalletTransactionTimeout,
		c.WalletAddress(),
	)
}
