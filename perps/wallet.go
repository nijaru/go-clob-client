package perps

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/nijaru/go-clob-client/clob"
)

// CollateralWalletConfig binds the perps owner to CLOB's existing transaction
// engine. Transactions accepts a signing.Signer or local PrivateKey; CLOB L2
// credentials are not required. Smart wallets require relayer auth, selected
// by context or BuilderAuth. EOA sends use signing.TransactionSigner or optional
// wallet-owned signing.TransactionSender; Safe/proxy relay signing uses
// signing.MessageSigner.
// None of these credentials is perps auth.
// Owner.Wallet may be omitted to use the derived transaction wallet.
// A missing Poly1271 funder selects the current beacon derivation offline;
// use clob.SignerClient.DeriveCurrentDepositWallet explicitly for legacy factories.
type CollateralWalletConfig struct {
	Owner        OwnerConfig
	Transactions clob.Config
}

// CollateralWallet owns no connections or goroutines. Construction performs no
// RPC, deployment, approval or submission. OwnerClient remains independent of
// this optional transaction adapter.
type CollateralWallet struct {
	*OwnerClient
	transactions *clob.SignerClient
	rpcURL       string
	walletType   clob.SignatureType
}

func NewCollateralWallet(config CollateralWalletConfig) (*CollateralWallet, error) {
	owner, err := NewOwner(config.Owner)
	if err != nil {
		return nil, err
	}
	if owner.token == (common.Address{}) || owner.deposit == (common.Address{}) {
		return nil, fmt.Errorf("perps: collateral token and deposit contract required")
	}
	cfg := config.Transactions
	if cfg.ChainID == 0 {
		cfg.ChainID = clob.PolygonChainID
	}
	if cfg.ChainID != config.Owner.Config.normalized().ChainID {
		return nil, fmt.Errorf("perps: owner and transaction chain differ")
	}
	if cfg.RPCURL == "" {
		return nil, fmt.Errorf("perps: explicit collateral RPC URL required")
	}
	// Derivation and transaction-signing identity are owned by clob.
	if cfg.SignatureType == clob.SignatureTypePoly1271 && cfg.FunderAddress == "" {
		probe := cfg
		probe.SignatureType = clob.SignatureTypeEOA
		signer, err := clob.NewSignerClient(probe)
		if err != nil {
			return nil, err
		}
		wallet, err := clob.DeriveBeaconDepositWallet(
			common.HexToAddress(signer.Address()),
			cfg.ChainID,
		)
		if err != nil {
			return nil, err
		}
		cfg.FunderAddress = wallet.Hex()
	}
	client, err := clob.NewSignerClient(cfg)
	if err != nil {
		return nil, err
	}
	if owner.signer.Address() != common.HexToAddress(client.Address()) {
		return nil, fmt.Errorf("perps: transaction signer must match perps owner")
	}
	wallet := client.WalletAddress()
	if wallet == (common.Address{}) {
		return nil, fmt.Errorf("perps: invalid transaction wallet")
	}
	switch cfg.SignatureType {
	case clob.SignatureTypePolyGnosisSafe, clob.SignatureTypePolyProxy:
		var derived common.Address
		if cfg.SignatureType == clob.SignatureTypePolyGnosisSafe {
			derived, err = clob.DeriveSafeWallet(owner.signer.Address(), cfg.ChainID)
		} else {
			derived, err = clob.DeriveProxyWallet(owner.signer.Address(), cfg.ChainID)
		}
		if err != nil {
			return nil, err
		}
		if wallet != derived {
			return nil, fmt.Errorf("perps: wallet does not derive from owner")
		}
	case clob.SignatureTypePoly1271:
		owned, err := client.IsDepositWalletOwner()
		if err != nil {
			return nil, err
		}
		if !owned {
			return nil, fmt.Errorf("perps: deposit wallet session signer is not an owner")
		}
	}
	if config.Owner.Wallet == "" {
		owner.wallet = wallet
	}
	if owner.wallet != wallet {
		return nil, fmt.Errorf("perps: withdrawal and transaction wallet differ")
	}
	return &CollateralWallet{
		OwnerClient:  owner,
		transactions: client,
		rpcURL:       cfg.RPCURL,
		walletType:   cfg.SignatureType,
	}, nil
}

func (w *CollateralWallet) WalletAddress() common.Address { return w.wallet }

func (w *CollateralWallet) dial(ctx context.Context) (*ethclient.Client, error) {
	ec, err := ethclient.DialContext(ctx, w.rpcURL)
	if err != nil {
		return nil, err
	}
	chain, err := ec.ChainID(ctx)
	if err != nil {
		ec.Close()
		return nil, err
	}
	if chain.Cmp(big.NewInt(w.chainID)) != 0 {
		ec.Close()
		return nil, fmt.Errorf(
			"perps: RPC chain %s differs from configured chain %d",
			chain,
			w.chainID,
		)
	}
	return ec, nil
}

// WalletReadiness distinguishes on-chain code from the relayer's registry.
// Neither is a transaction receipt or evidence that collateral was credited.
type WalletReadiness struct {
	OnChain    bool
	Registered bool
}

func (w *CollateralWallet) Readiness(ctx context.Context) (WalletReadiness, error) {
	ec, err := w.dial(ctx)
	if err != nil {
		return WalletReadiness{}, err
	}
	defer ec.Close()
	if w.walletType == clob.SignatureTypeEOA {
		return WalletReadiness{OnChain: true}, nil
	}
	code, err := ec.CodeAt(ctx, w.wallet, nil)
	if err != nil {
		return WalletReadiness{}, err
	}
	state := WalletReadiness{OnChain: len(code) != 0}
	state.Registered, err = w.transactions.IsWalletDeployed(ctx)
	return state, err
}

// DeployDepositWallet explicitly submits creation of the current beacon wallet.
// It does not wait or infer deployment from relayer registry readiness.
// Safe creation is separate; proxy deployment is not supplied.
func (w *CollateralWallet) DeployDepositWallet(
	ctx context.Context,
	metadata string,
) (*CollateralTransaction, error) {
	if w.walletType != clob.SignatureTypePoly1271 {
		return nil, fmt.Errorf("perps: deployment requires a deposit wallet")
	}
	current, err := clob.DeriveBeaconDepositWallet(w.signer.Address(), w.chainID)
	if err != nil {
		return nil, err
	}
	if current != w.wallet {
		return nil, fmt.Errorf("perps: cannot deploy a legacy wallet with current factory creation")
	}
	ec, err := w.dial(ctx)
	if err != nil {
		return nil, err
	}
	ec.Close()
	h, err := w.transactions.DeployDepositWallet(ctx, metadata)
	if err != nil {
		return nil, err
	}
	return w.deploymentTransaction(h), nil
}

// DeploySafeWallet explicitly submits creation of the deterministic owner Safe.
// Wait on the returned transaction for independent RPC receipt verification.
func (w *CollateralWallet) DeploySafeWallet(
	ctx context.Context,
	metadata string,
) (*CollateralTransaction, error) {
	if w.walletType != clob.SignatureTypePolyGnosisSafe {
		return nil, fmt.Errorf("perps: deployment requires a Safe wallet")
	}
	ec, err := w.dial(ctx)
	if err != nil {
		return nil, err
	}
	ec.Close()
	h, err := w.transactions.DeploySafeWallet(ctx, metadata)
	if err != nil {
		return nil, err
	}
	return w.deploymentTransaction(h), nil
}

func (w *CollateralWallet) deploymentTransaction(
	h *clob.GaslessTransactionHandle,
) *CollateralTransaction {
	return &CollateralTransaction{
		wallet:         w,
		RequestedCalls: 1,
		relay:          h,
		Submissions: []CollateralSubmission{{
			Operation: "deploy", TransactionID: h.TransactionID,
			TransactionHash: h.TransactionHash,
		}},
	}
}
