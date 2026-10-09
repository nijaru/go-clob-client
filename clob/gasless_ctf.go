package clob

import "context"

// SplitPositionGasless executes the same raw CTF calldata as SplitPosition,
// through the configured proxy, Safe or Deposit Wallet.
func (c *AuthenticatedClient) SplitPositionGasless(
	ctx context.Context,
	req SplitPositionRequest,
	metadata string,
) (*GaslessTransactionHandle, error) {
	data, err := packSplitPosition(req)
	if err != nil {
		return nil, err
	}
	to, err := c.conditionalAddr()
	if err != nil {
		return nil, err
	}
	return c.PrepareGaslessTransaction(ctx, []TransactionCall{tokenCall(to, data)}, metadata)
}

// MergePositionsGasless merges raw CTF positions through the smart wallet.
func (c *AuthenticatedClient) MergePositionsGasless(
	ctx context.Context,
	req MergePositionsRequest,
	metadata string,
) (*GaslessTransactionHandle, error) {
	data, err := packMergePositions(req)
	if err != nil {
		return nil, err
	}
	to, err := c.conditionalAddr()
	if err != nil {
		return nil, err
	}
	return c.PrepareGaslessTransaction(ctx, []TransactionCall{tokenCall(to, data)}, metadata)
}

// RedeemPositionsGasless redeems raw CTF positions through the smart wallet.
func (c *AuthenticatedClient) RedeemPositionsGasless(
	ctx context.Context,
	req RedeemPositionsRequest,
	metadata string,
) (*GaslessTransactionHandle, error) {
	data, err := packRedeemPositions(req)
	if err != nil {
		return nil, err
	}
	to, err := c.conditionalAddr()
	if err != nil {
		return nil, err
	}
	return c.PrepareGaslessTransaction(ctx, []TransactionCall{tokenCall(to, data)}, metadata)
}

// RedeemNegRiskGasless redeems the two NegRiskAdapter outcome balances.
func (c *AuthenticatedClient) RedeemNegRiskGasless(
	ctx context.Context,
	req RedeemNegRiskRequest,
	metadata string,
) (*GaslessTransactionHandle, error) {
	data, err := packRedeemNegRisk(req)
	if err != nil {
		return nil, err
	}
	to, err := c.negRiskAdapterAddr()
	if err != nil {
		return nil, err
	}
	return c.PrepareGaslessTransaction(ctx, []TransactionCall{tokenCall(to, data)}, metadata)
}
