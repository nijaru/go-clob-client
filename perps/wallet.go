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
// engine. Transactions requires a local key and CLOB credentials; smart wallets
// additionally require BuilderAuth for the relayer. Neither is perps auth.
// Owner.Wallet may be omitted to use the derived transaction wallet.
// A missing Poly1271 funder selects the current beacon derivation offline;
// use clob.SignerClient.DeriveCurrentDepositWallet explicitly for legacy factories.
type CollateralWalletConfig struct {
	Owner        OwnerConfig
	Transactions clob.Config
}

// CollateralWallet owns no connections or goroutines. Construction performs no
// RPC, deployment, approval or submission. OwnerClient remains independent of
// this optional local-key transaction adapter.
type CollateralWallet struct {
	*OwnerClient
	transactions *clob.AuthenticatedClient
	rpcURL       string
	walletType   clob.SignatureType
}

func NewCollateralWallet(config CollateralWalletConfig) (*CollateralWallet, error) {
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
	// Derivation and local-key identity are owned by clob, not duplicated here.
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
	client, err := clob.NewAuthenticatedClient(cfg)
	if err != nil {
		return nil, err
	}
	if config.Owner.Signer == nil ||
		config.Owner.Signer.Address() != common.HexToAddress(client.Address()) {
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
			derived, err = clob.DeriveSafeWallet(config.Owner.Signer.Address(), cfg.ChainID)
		} else {
			derived, err = clob.DeriveProxyWallet(config.Owner.Signer.Address(), cfg.ChainID)
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
		config.Owner.Wallet = wallet.Hex()
	}
	owner, err := NewOwner(config.Owner)
	if err != nil {
		return nil, err
	}
	if owner.wallet != wallet {
		return nil, fmt.Errorf("perps: withdrawal and transaction wallet differ")
	}
	if owner.token == (common.Address{}) || owner.deposit == (common.Address{}) {
		return nil, fmt.Errorf("perps: collateral token and deposit contract required")
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
// Safe/proxy deployment is not supplied by the existing CLOB transaction engine.
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
	return &CollateralTransaction{
		wallet: w,
		Submissions: []CollateralSubmission{
			{
				Operation:       "deploy",
				TransactionID:   h.TransactionID,
				TransactionHash: h.TransactionHash,
				relay:           h,
			},
		},
	}, nil
}
