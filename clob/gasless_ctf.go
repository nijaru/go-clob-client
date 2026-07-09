package clob

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/nijaru/go-clob-client/internal/polyrelay"
)

// gaslessCTFCall packs a CTF calldata payload targeted at a contract as a single
// zero-value relayer TransactionCall. The relayer (proxy/Safe/deposit wallet)
// executes it; the calldata is identical to the on-chain EOA path, so the two are
// wire-equivalent — only the execution transport differs.
func gaslessCTFCall(to common.Address, data []byte) polyrelay.TransactionCall {
	return polyrelay.TransactionCall{To: to, Data: data, Value: big.NewInt(0)}
}

// SplitPositionGasless splits collateral into a complementary outcome pair via the
// gasless relayer. It is the non-EOA (proxy/Safe/deposit) equivalent of
// SignerClient.SplitPosition; the calldata and target contract are identical.
func (c *AuthenticatedClient) SplitPositionGasless(
	ctx context.Context,
	req SplitPositionRequest,
	metadata string,
) (*polyrelay.Handle, error) {
	data, err := packSplitPosition(req)
	if err != nil {
		return nil, fmt.Errorf("gasless split: pack: %w", err)
	}
	to, err := c.conditionalAddr()
	if err != nil {
		return nil, err
	}
	calls := []polyrelay.TransactionCall{gaslessCTFCall(to, data)}
	return c.PrepareGaslessTransaction(ctx, calls, metadata)
}

// MergePositionsGasless merges a complementary outcome pair back into collateral
// via the gasless relayer. The non-EOA equivalent of SignerClient.MergePositions;
// the calldata and target contract are identical.
func (c *AuthenticatedClient) MergePositionsGasless(
	ctx context.Context,
	req MergePositionsRequest,
	metadata string,
) (*polyrelay.Handle, error) {
	data, err := packMergePositions(req)
	if err != nil {
		return nil, fmt.Errorf("gasless merge: pack: %w", err)
	}
	to, err := c.conditionalAddr()
	if err != nil {
		return nil, err
	}
	calls := []polyrelay.TransactionCall{gaslessCTFCall(to, data)}
	return c.PrepareGaslessTransaction(ctx, calls, metadata)
}

// RedeemPositionsGasless redeems a resolved condition's outcome positions for
// collateral via the gasless relayer. The non-EOA equivalent of
// SignerClient.RedeemPositions; the calldata and target contract are identical.
func (c *AuthenticatedClient) RedeemPositionsGasless(
	ctx context.Context,
	req RedeemPositionsRequest,
	metadata string,
) (*polyrelay.Handle, error) {
	data, err := packRedeemPositions(req)
	if err != nil {
		return nil, fmt.Errorf("gasless redeem: pack: %w", err)
	}
	to, err := c.conditionalAddr()
	if err != nil {
		return nil, err
	}
	calls := []polyrelay.TransactionCall{gaslessCTFCall(to, data)}
	return c.PrepareGaslessTransaction(ctx, calls, metadata)
}

// RedeemNegRiskGasless redeems a resolved negative-risk condition's positions via
// the gasless relayer. The non-EOA equivalent of SignerClient.RedeemNegRisk; the
// calldata and target contract (NegRiskAdapter) are identical.
func (c *AuthenticatedClient) RedeemNegRiskGasless(
	ctx context.Context,
	req RedeemNegRiskRequest,
	metadata string,
) (*polyrelay.Handle, error) {
	data, err := packRedeemNegRisk(req)
	if err != nil {
		return nil, fmt.Errorf("gasless neg-risk redeem: pack: %w", err)
	}
	to, err := c.negRiskAdapterAddr()
	if err != nil {
		return nil, err
	}
	calls := []polyrelay.TransactionCall{gaslessCTFCall(to, data)}
	return c.PrepareGaslessTransaction(ctx, calls, metadata)
}
