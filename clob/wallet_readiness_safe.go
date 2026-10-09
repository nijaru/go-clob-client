package clob

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/nijaru/go-clob-client/internal/polyauth"
	"github.com/nijaru/go-clob-client/internal/polyrelay"
)

var (
	ErrSafeWalletDeploymentIdentity = errors.New(
		"Safe deployment requires the signer's deterministic Safe wallet",
	)
	ErrWalletAlreadyDeployed = errors.New("wallet is already deployed")
)

// DeploySafeWallet explicitly submits a signed, zero-payment SAFE-CREATE for
// the configured owner's deterministic Safe. It rejects arbitrary funders and
// already-deployed wallets, and never switches the client's identity. The
// returned handle waits for relayer confirmation, not an actual chain receipt;
// use WaitWalletTransactionReceipt for independent receipt verification.
// Submissions preserve WithRelayerAuth; the initial deployment probe is public.
func (c *SignerClient) DeploySafeWallet(
	ctx context.Context,
	metadata string,
) (*GaslessTransactionHandle, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.signatureType != SignatureTypePolyGnosisSafe {
		return nil, ErrSafeWalletDeploymentIdentity
	}
	wallet, err := DeriveSafeWallet(c.signer.Address(), c.chainID)
	if err != nil {
		return nil, err
	}
	if wallet == (common.Address{}) || wallet != c.WalletAddress() {
		return nil, ErrSafeWalletDeploymentIdentity
	}
	if len(metadata) > polyrelay.MetadataMaxLength {
		return nil, fmt.Errorf(
			"%w: %d > %d",
			polyrelay.ErrMetadataTooLong,
			len(metadata),
			polyrelay.MetadataMaxLength,
		)
	}
	deployed, err := c.Client.IsWalletDeployedAt(ctx, wallet, SignatureTypePolyGnosisSafe)
	if err != nil {
		return nil, err
	}
	if deployed {
		return nil, ErrWalletAlreadyDeployed
	}
	wc, err := getWalletConfig(c.chainID)
	if err != nil {
		return nil, err
	}
	factory := common.HexToAddress(wc.SafeFactory)
	zero := (common.Address{}).Hex()
	// builder-relayer-client's CreateProxy domain deliberately has no version.
	signature, err := polyauth.SignTypedData(ctx, c.signer, apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": {
				{Name: "name", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
			"CreateProxy": {
				{Name: "paymentToken", Type: "address"},
				{Name: "payment", Type: "uint256"},
				{Name: "paymentReceiver", Type: "address"},
			},
		},
		PrimaryType: "CreateProxy",
		Domain: apitypes.TypedDataDomain{
			Name:              "Polymarket Contract Proxy Factory",
			ChainId:           (*ethmath.HexOrDecimal256)(big.NewInt(c.chainID)),
			VerifyingContract: factory.Hex(),
		},
		Message: apitypes.TypedDataMessage{
			"paymentToken": zero, "payment": "0", "paymentReceiver": zero,
		},
	})
	if err != nil {
		return nil, err
	}
	transport := c.RelayerTransport()
	response, err := transport.Submit(ctx, &RelayerSubmitRequest{
		Type: string(RelayerTransactionSafeCreate), From: c.Address(), To: factory.Hex(),
		ProxyWallet: wallet.Hex(), Data: "0x", Signature: signature, Metadata: metadata,
		SignatureParams: &RelayerSignatureParams{
			PaymentToken: zero, Payment: "0", PaymentReceiver: zero,
		},
	})
	if err != nil {
		return nil, err
	}
	return polyrelay.NewHandle(transport, response), nil
}
