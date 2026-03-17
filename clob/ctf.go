package clob

import (
	"context"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// SplitTokens converts USDC.e collateral into a full set of outcome tokens (Yes + No).
// Level 2 Auth required.
func (c *AuthenticatedClient) SplitTokens(ctx context.Context, args SplitArgs) error {
	return c.postJSON(ctx, splitPositionsEndpoint, args, polyhttp.AuthL2, nil)
}

// MergeTokens converts a full set of outcome tokens (Yes + No) back into USDC.e collateral.
// Level 2 Auth required.
func (c *AuthenticatedClient) MergeTokens(ctx context.Context, args MergeArgs) error {
	return c.postJSON(ctx, mergePositionsEndpoint, args, polyhttp.AuthL2, nil)
}

// RedeemTokens exchanges winning outcome tokens for USDC.e collateral after market resolution.
// Level 2 Auth required.
func (c *AuthenticatedClient) RedeemTokens(ctx context.Context, args RedeemArgs) error {
	return c.postJSON(ctx, redeemPositionsEndpoint, args, polyhttp.AuthL2, nil)
}
