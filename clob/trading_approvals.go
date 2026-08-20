package clob

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/nijaru/go-clob-client/internal/polyrelay"
)

// Erc20TradingApproval is one required ERC-20 allowance that is not configured
// for a wallet. It mirrors the official SDK model name.
type Erc20TradingApproval struct {
	TokenAddress   common.Address
	SpenderAddress common.Address
	Amount         *big.Int
}

// Erc1155TradingApproval is one required ERC-1155 operator approval that is not
// configured for a wallet. It mirrors the official SDK model name; the Go
// SDK's existing ERC1155ApprovalForAllRequest is the same shape.
type Erc1155TradingApproval = ERC1155ApprovalForAllRequest

// MissingTradingApprovals is the set of trading approvals a wallet still needs
// to grant before it can trade through the official contract set.
type MissingTradingApprovals struct {
	ERC20Approvals   []Erc20TradingApproval
	ERC1155Approvals []ERC1155ApprovalForAllRequest
}

// Empty reports whether the wallet has all required approvals.
func (m MissingTradingApprovals) Empty() bool {
	return len(m.ERC20Approvals) == 0 && len(m.ERC1155Approvals) == 0
}

// TradingApprovalsState is the current trading approval state for a wallet.
// It is a read-only snapshot: callers read it and, if missing approvals exist,
// submit them through SetupTradingApprovals or SetupTradingApprovalsGasless.
type TradingApprovalsState struct {
	Missing         *MissingTradingApprovals
	IsFullyApproved bool
}

// buildMissingTradingApprovalCalls converts a missing-approvals set into the
// ABI calls required to grant it. It is shared by the EOA and gasless setup
// paths so the two flows cannot drift.
func buildMissingTradingApprovalCalls(missing *MissingTradingApprovals) ([]polyrelay.TransactionCall, error) {
	calls := make(
		[]polyrelay.TransactionCall,
		0,
		len(missing.ERC20Approvals)+len(missing.ERC1155Approvals),
	)
	for _, approval := range missing.ERC20Approvals {
		data, err := packERC20Approval(ERC20ApprovalRequest{
			TokenAddress:   approval.TokenAddress,
			SpenderAddress: approval.SpenderAddress,
			Amount:         approval.Amount,
		})
		if err != nil {
			return nil, err
		}
		calls = append(calls, tokenCall(approval.TokenAddress, data))
	}
	for _, approval := range missing.ERC1155Approvals {
		data, err := packERC1155ApprovalForAll(approval)
		if err != nil {
			return nil, err
		}
		calls = append(calls, tokenCall(approval.TokenAddress, data))
	}
	return calls, nil
}

// GetTradingApprovalsState reads the configured wallet's current on-chain
// approval state and returns a snapshot of what is missing. It does not
// submit any transactions. The wallet defaults to the configured funder
// address; pass an explicit wallet to inspect a different owner.
func (c *SignerClient) GetTradingApprovalsState(
	ctx context.Context,
	wallet string,
) (*TradingApprovalsState, error) {
	config, err := getContractConfig(c.chainID)
	if err != nil {
		return nil, err
	}
	erc20, erc1155, err := requiredTradingApprovals(c.chainID, config)
	if err != nil {
		return nil, err
	}
	ec, err := c.dialRPC(ctx)
	if err != nil {
		return nil, fmt.Errorf("token: dial rpc for trading approvals state: %w", err)
	}
	defer ec.Close()

	owner := common.HexToAddress(c.funderAddress)
	if wallet != "" {
		if !common.IsHexAddress(wallet) {
			return nil, fmt.Errorf("token: invalid wallet address %q", wallet)
		}
		owner = common.HexToAddress(wallet)
	}

	missing := &MissingTradingApprovals{
		ERC20Approvals:   make([]Erc20TradingApproval, 0, len(erc20)),
		ERC1155Approvals: make([]ERC1155ApprovalForAllRequest, 0, len(erc1155)),
	}
	for _, approval := range erc20 {
		allowance, err := readERC20Allowance(ctx, ec, approval.TokenAddress, owner, approval.SpenderAddress)
		if err != nil {
			return nil, fmt.Errorf("token: read ERC20 approval for %s: %w",
				approval.SpenderAddress.Hex(), err)
		}
		if allowance.Cmp(approval.Amount) < 0 {
			missing.ERC20Approvals = append(missing.ERC20Approvals, Erc20TradingApproval{
				TokenAddress:   approval.TokenAddress,
				SpenderAddress: approval.SpenderAddress,
				Amount:         new(big.Int).Set(approval.Amount),
			})
		}
	}
	for _, approval := range erc1155 {
		approved, err := readERC1155ApprovalForAll(ctx, ec, approval.TokenAddress, owner, approval.OperatorAddress)
		if err != nil {
			return nil, fmt.Errorf("token: read ERC1155 approval for %s: %w",
				approval.OperatorAddress.Hex(), err)
		}
		if !approved {
			missing.ERC1155Approvals = append(missing.ERC1155Approvals, approval)
		}
	}

	return &TradingApprovalsState{
		Missing:         missing,
		IsFullyApproved: missing.Empty(),
	}, nil
}

// GetTradingApprovalsState reads the configured wallet's current on-chain
// approval state and returns a snapshot of what is missing. It does not
// submit any transactions. The wallet defaults to the configured funder
// address; pass an explicit wallet to inspect a different owner.
func (c *AuthenticatedClient) GetTradingApprovalsState(
	ctx context.Context,
	wallet string,
) (*TradingApprovalsState, error) {
	return c.SignerClient.GetTradingApprovalsState(ctx, wallet)
}