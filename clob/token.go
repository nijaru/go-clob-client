package clob

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/nijaru/go-clob-client/internal/polyrelay"
)

const tokenABIJSON = `[
  {
    "inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],
    "name":"approve","outputs":[{"name":"","type":"bool"}],
    "stateMutability":"nonpayable","type":"function"
  },
  {
    "inputs":[{"name":"recipient","type":"address"},{"name":"amount","type":"uint256"}],
    "name":"transfer","outputs":[{"name":"","type":"bool"}],
    "stateMutability":"nonpayable","type":"function"
  },
  {
    "inputs":[{"name":"operator","type":"address"},{"name":"approved","type":"bool"}],
    "name":"setApprovalForAll","outputs":[],
    "stateMutability":"nonpayable","type":"function"
  },
  {
    "inputs":[{"name":"owner","type":"address"},{"name":"spender","type":"address"}],
    "name":"allowance","outputs":[{"name":"","type":"uint256"}],
    "stateMutability":"view","type":"function"
  },
  {
    "inputs":[{"name":"account","type":"address"},{"name":"operator","type":"address"}],
    "name":"isApprovedForAll","outputs":[{"name":"","type":"bool"}],
    "stateMutability":"view","type":"function"
  }
]`

var tokenABI abi.ABI

var (
	// ErrInvalidTokenAmount indicates a nil, negative, or overflowing uint256 amount.
	ErrInvalidTokenAmount = errors.New("invalid token amount")
	// ErrTokenOperationRequiresEOA indicates that a direct call was attempted for a smart wallet.
	ErrTokenOperationRequiresEOA = errors.New("token operation requires EOA")
	// ErrTradingApprovalConfig indicates that the configured chain lacks the current
	// contract set required for one-call trading approval setup.
	ErrTradingApprovalConfig = errors.New("trading approval configuration unavailable")
)

func init() {
	var err error
	tokenABI, err = abi.JSON(strings.NewReader(tokenABIJSON))
	if err != nil {
		panic("clob: parse token ABI: " + err.Error())
	}
}

// MaxUint256 returns a fresh maximum uint256 value suitable for unlimited
// ERC-20 approvals. The returned value can be mutated by the caller.
func MaxUint256() *big.Int {
	return new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
}

// ERC20ApprovalRequest configures an ERC-20 approve(address,uint256) call.
type ERC20ApprovalRequest struct {
	TokenAddress   common.Address
	SpenderAddress common.Address
	Amount         *big.Int
}

// ERC1155ApprovalForAllRequest configures an ERC-1155 setApprovalForAll call.
type ERC1155ApprovalForAllRequest struct {
	TokenAddress    common.Address
	OperatorAddress common.Address
	Approved        bool
}

// ERC20TransferRequest configures an ERC-20 transfer(address,uint256) call.
type ERC20TransferRequest struct {
	TokenAddress     common.Address
	RecipientAddress common.Address
	Amount           *big.Int
}

// TradingApprovalPlan contains only the approvals that are currently missing
// for the configured wallet. The plan follows the official SDK contract set
// for the selected chain.
type TradingApprovalPlan struct {
	ERC20Approvals   []ERC20ApprovalRequest
	ERC1155Approvals []ERC1155ApprovalForAllRequest
}

// Empty reports whether all required trading approvals are already present.
func (p TradingApprovalPlan) Empty() bool {
	return len(p.ERC20Approvals) == 0 && len(p.ERC1155Approvals) == 0
}

func validateUint256(value *big.Int, name string) error {
	if value == nil {
		return fmt.Errorf("%w: %s is nil", ErrInvalidTokenAmount, name)
	}
	if value.Sign() < 0 {
		return fmt.Errorf("%w: %s must be non-negative", ErrInvalidTokenAmount, name)
	}
	if value.BitLen() > 256 {
		return fmt.Errorf("%w: %s exceeds uint256", ErrInvalidTokenAmount, name)
	}
	return nil
}

func packERC20Approval(req ERC20ApprovalRequest) ([]byte, error) {
	if err := validateUint256(req.Amount, "approval amount"); err != nil {
		return nil, err
	}
	return tokenABI.Pack("approve", req.SpenderAddress, req.Amount)
}

func packERC1155ApprovalForAll(req ERC1155ApprovalForAllRequest) ([]byte, error) {
	return tokenABI.Pack("setApprovalForAll", req.OperatorAddress, req.Approved)
}

func packERC20Transfer(req ERC20TransferRequest) ([]byte, error) {
	if err := validateUint256(req.Amount, "transfer amount"); err != nil {
		return nil, err
	}
	return tokenABI.Pack("transfer", req.RecipientAddress, req.Amount)
}

func tokenCall(to common.Address, data []byte) polyrelay.TransactionCall {
	return polyrelay.TransactionCall{To: to, Data: data, Value: new(big.Int)}
}

type contractCaller interface {
	CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error)
}

func readERC20Allowance(
	ctx context.Context,
	ec contractCaller,
	token, owner, spender common.Address,
) (*big.Int, error) {
	data, err := tokenABI.Pack("allowance", owner, spender)
	if err != nil {
		return nil, fmt.Errorf("token: pack allowance: %w", err)
	}
	result, err := ec.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
	if err != nil {
		return nil, fmt.Errorf("token: call allowance: %w", err)
	}
	values, err := tokenABI.Unpack("allowance", result)
	if err != nil {
		return nil, fmt.Errorf("token: unpack allowance: %w", err)
	}
	if len(values) != 1 {
		return nil, fmt.Errorf("token: allowance returned %d values, want 1", len(values))
	}
	allowance, ok := values[0].(*big.Int)
	if !ok || allowance == nil {
		return nil, fmt.Errorf("token: allowance returned %T, want *big.Int", values[0])
	}
	return new(big.Int).Set(allowance), nil
}

func readERC1155ApprovalForAll(
	ctx context.Context,
	ec contractCaller,
	token, owner, operator common.Address,
) (bool, error) {
	data, err := tokenABI.Pack("isApprovedForAll", owner, operator)
	if err != nil {
		return false, fmt.Errorf("token: pack approval-for-all read: %w", err)
	}
	result, err := ec.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
	if err != nil {
		return false, fmt.Errorf("token: call approval-for-all read: %w", err)
	}
	values, err := tokenABI.Unpack("isApprovedForAll", result)
	if err != nil {
		return false, fmt.Errorf("token: unpack approval-for-all read: %w", err)
	}
	if len(values) != 1 {
		return false, fmt.Errorf("token: approval-for-all returned %d values, want 1", len(values))
	}
	approved, ok := values[0].(bool)
	if !ok {
		return false, fmt.Errorf("token: approval-for-all returned %T, want bool", values[0])
	}
	return approved, nil
}

func tradingApprovalAddress(chainID int64, name, value string) (common.Address, error) {
	address := common.HexToAddress(value)
	if !common.IsHexAddress(value) || address == (common.Address{}) {
		return common.Address{}, fmt.Errorf(
			"%w: chain %d is missing %s",
			ErrTradingApprovalConfig,
			chainID,
			name,
		)
	}
	return address, nil
}

func requiredTradingApprovals(
	chainID int64,
	config contractConfig,
) ([]ERC20ApprovalRequest, []ERC1155ApprovalForAllRequest, error) {
	resolve := func(name, value string) (common.Address, error) {
		return tradingApprovalAddress(chainID, name, value)
	}

	collateral, err := resolve("collateral token", config.Collateral)
	if err != nil {
		return nil, nil, err
	}
	conditional, err := resolve("conditional tokens", config.Conditional)
	if err != nil {
		return nil, nil, err
	}
	standardExchange, err := resolve("standard exchange", config.Exchange)
	if err != nil {
		return nil, nil, err
	}
	negRiskExchange, err := resolve("neg-risk exchange", config.NegRiskExchange)
	if err != nil {
		return nil, nil, err
	}
	collateralAdapter, err := resolve("collateral adapter", config.CollateralAdapter)
	if err != nil {
		return nil, nil, err
	}
	negRiskCollateralAdapter, err := resolve(
		"neg-risk collateral adapter",
		config.NegRiskCollateralAdapter,
	)
	if err != nil {
		return nil, nil, err
	}
	protocolV2Router, err := resolve("protocol v2 router", config.ProtocolV2Router)
	if err != nil {
		return nil, nil, err
	}
	exchangeV3, err := resolve("exchange v3", config.ExchangeV3)
	if err != nil {
		return nil, nil, err
	}
	perpsDepositContract, err := resolve("perps deposit contract", config.PerpsDepositContract)
	if err != nil {
		return nil, nil, err
	}
	positionManager, err := resolve("position manager", config.PositionManager)
	if err != nil {
		return nil, nil, err
	}
	autoRedeemOperator, err := resolve("auto-redeem operator", config.AutoRedeemOperator)
	if err != nil {
		return nil, nil, err
	}

	erc20 := []ERC20ApprovalRequest{
		{TokenAddress: collateral, SpenderAddress: standardExchange, Amount: MaxUint256()},
		{TokenAddress: collateral, SpenderAddress: negRiskExchange, Amount: MaxUint256()},
		{TokenAddress: collateral, SpenderAddress: collateralAdapter, Amount: MaxUint256()},
		{TokenAddress: collateral, SpenderAddress: negRiskCollateralAdapter, Amount: MaxUint256()},
		{TokenAddress: collateral, SpenderAddress: protocolV2Router, Amount: MaxUint256()},
		{TokenAddress: collateral, SpenderAddress: exchangeV3, Amount: MaxUint256()},
		{TokenAddress: collateral, SpenderAddress: perpsDepositContract, Amount: MaxUint256()},
	}
	erc1155 := []ERC1155ApprovalForAllRequest{
		{TokenAddress: conditional, OperatorAddress: standardExchange, Approved: true},
		{TokenAddress: conditional, OperatorAddress: negRiskExchange, Approved: true},
		{TokenAddress: conditional, OperatorAddress: collateralAdapter, Approved: true},
		{TokenAddress: conditional, OperatorAddress: negRiskCollateralAdapter, Approved: true},
		{TokenAddress: conditional, OperatorAddress: autoRedeemOperator, Approved: true},
		{TokenAddress: positionManager, OperatorAddress: protocolV2Router, Approved: true},
		{TokenAddress: positionManager, OperatorAddress: exchangeV3, Approved: true},
		{TokenAddress: positionManager, OperatorAddress: autoRedeemOperator, Approved: true},
	}
	return erc20, erc1155, nil
}

// PrepareTradingApprovals reads the configured wallet's current on-chain
// allowances and returns only the missing official trading approvals.
func (c *SignerClient) PrepareTradingApprovals(ctx context.Context) (*TradingApprovalPlan, error) {
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
		return nil, fmt.Errorf("token: dial rpc for trading approvals: %w", err)
	}
	defer ec.Close()

	owner := common.HexToAddress(c.funderAddress)
	plan := &TradingApprovalPlan{
		ERC20Approvals:   make([]ERC20ApprovalRequest, 0, len(erc20)),
		ERC1155Approvals: make([]ERC1155ApprovalForAllRequest, 0, len(erc1155)),
	}
	for _, approval := range erc20 {
		allowance, err := readERC20Allowance(
			ctx,
			ec,
			approval.TokenAddress,
			owner,
			approval.SpenderAddress,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"token: read ERC20 approval for %s: %w",
				approval.SpenderAddress.Hex(),
				err,
			)
		}
		if allowance.Cmp(approval.Amount) < 0 {
			plan.ERC20Approvals = append(plan.ERC20Approvals, approval)
		}
	}
	for _, approval := range erc1155 {
		approved, err := readERC1155ApprovalForAll(
			ctx,
			ec,
			approval.TokenAddress,
			owner,
			approval.OperatorAddress,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"token: read ERC1155 approval for %s: %w",
				approval.OperatorAddress.Hex(),
				err,
			)
		}
		if !approved {
			plan.ERC1155Approvals = append(plan.ERC1155Approvals, approval)
		}
	}
	return plan, nil
}

// SetupTradingApprovals reads and submits the missing trading approvals from
// an EOA. The returned receipts may contain a completed prefix when a later
// transaction fails. For proxy, Safe, and deposit wallets use the explicit
// SetupTradingApprovalsGasless method.
func (c *SignerClient) SetupTradingApprovals(ctx context.Context) ([]TxReceipt, error) {
	if err := c.requireEOATokenOperation(); err != nil {
		return nil, err
	}
	plan, err := c.PrepareTradingApprovals(ctx)
	if err != nil {
		return nil, err
	}
	receipts := make([]TxReceipt, 0, len(plan.ERC20Approvals)+len(plan.ERC1155Approvals))
	for _, approval := range plan.ERC20Approvals {
		receipt, err := c.ApproveERC20(ctx, approval)
		if err != nil {
			return receipts, fmt.Errorf(
				"token: setup ERC20 approval for %s: %w",
				approval.SpenderAddress.Hex(),
				err,
			)
		}
		receipts = append(receipts, *receipt)
	}
	for _, approval := range plan.ERC1155Approvals {
		receipt, err := c.ApproveERC1155ForAll(ctx, approval)
		if err != nil {
			return receipts, fmt.Errorf(
				"token: setup ERC1155 approval for %s: %w",
				approval.OperatorAddress.Hex(),
				err,
			)
		}
		receipts = append(receipts, *receipt)
	}
	return receipts, nil
}

// SetupTradingApprovalsGasless reads and batches the missing trading approvals
// through the configured proxy, Safe, or deposit wallet. It returns nil,nil
// when all required approvals are already present.
func (c *AuthenticatedClient) SetupTradingApprovalsGasless(
	ctx context.Context,
	metadata string,
) (*polyrelay.Handle, error) {
	if _, err := c.gaslessConfig(); err != nil {
		return nil, err
	}
	plan, err := c.PrepareTradingApprovals(ctx)
	if err != nil {
		return nil, err
	}
	if plan.Empty() {
		return nil, nil
	}
	calls := make(
		[]polyrelay.TransactionCall,
		0,
		len(plan.ERC20Approvals)+len(plan.ERC1155Approvals),
	)
	for _, approval := range plan.ERC20Approvals {
		data, err := packERC20Approval(approval)
		if err != nil {
			return nil, err
		}
		calls = append(calls, tokenCall(approval.TokenAddress, data))
	}
	for _, approval := range plan.ERC1155Approvals {
		data, err := packERC1155ApprovalForAll(approval)
		if err != nil {
			return nil, err
		}
		calls = append(calls, tokenCall(approval.TokenAddress, data))
	}
	if metadata == "" {
		metadata = "Trading setup approvals"
	}
	return c.PrepareGaslessTransaction(ctx, calls, metadata)
}

func (c *SignerClient) requireEOATokenOperation() error {
	if c.signatureType != SignatureTypeEOA {
		return fmt.Errorf(
			"%w: direct transaction requires EOA signature type; use the gasless variant",
			ErrTokenOperationRequiresEOA,
		)
	}
	return nil
}

// ApproveERC20 sends an ERC-20 approval directly from an EOA.
//
// For proxy, Safe, and deposit-wallet accounts use AuthenticatedClient's
// ApproveERC20Gasless method so the transaction is executed by the configured
// wallet rather than the signing EOA.
func (c *SignerClient) ApproveERC20(
	ctx context.Context,
	req ERC20ApprovalRequest,
) (*TxReceipt, error) {
	if err := c.requireEOATokenOperation(); err != nil {
		return nil, err
	}
	data, err := packERC20Approval(req)
	if err != nil {
		return nil, err
	}
	receipt, err := c.sendContractTxAndWait(ctx, req.TokenAddress, data, "token approve")
	if err != nil {
		return nil, err
	}
	return &TxReceipt{Hash: receipt.TxHash, BlockNumber: receipt.BlockNumber.Uint64()}, nil
}

// ApproveERC1155ForAll sends an ERC-1155 operator approval directly from an EOA.
func (c *SignerClient) ApproveERC1155ForAll(
	ctx context.Context,
	req ERC1155ApprovalForAllRequest,
) (*TxReceipt, error) {
	if err := c.requireEOATokenOperation(); err != nil {
		return nil, err
	}
	data, err := packERC1155ApprovalForAll(req)
	if err != nil {
		return nil, err
	}
	receipt, err := c.sendContractTxAndWait(ctx, req.TokenAddress, data, "token approval-for-all")
	if err != nil {
		return nil, err
	}
	return &TxReceipt{Hash: receipt.TxHash, BlockNumber: receipt.BlockNumber.Uint64()}, nil
}

// TransferERC20 sends an ERC-20 transfer directly from an EOA.
func (c *SignerClient) TransferERC20(
	ctx context.Context,
	req ERC20TransferRequest,
) (*TxReceipt, error) {
	if err := c.requireEOATokenOperation(); err != nil {
		return nil, err
	}
	data, err := packERC20Transfer(req)
	if err != nil {
		return nil, err
	}
	receipt, err := c.sendContractTxAndWait(ctx, req.TokenAddress, data, "token transfer")
	if err != nil {
		return nil, err
	}
	return &TxReceipt{Hash: receipt.TxHash, BlockNumber: receipt.BlockNumber.Uint64()}, nil
}

// ApproveERC20Gasless submits an ERC-20 approval through the configured
// proxy, Safe, or deposit wallet.
func (c *AuthenticatedClient) ApproveERC20Gasless(
	ctx context.Context,
	req ERC20ApprovalRequest,
	metadata string,
) (*polyrelay.Handle, error) {
	data, err := packERC20Approval(req)
	if err != nil {
		return nil, err
	}
	return c.PrepareGaslessTransaction(
		ctx,
		[]polyrelay.TransactionCall{tokenCall(req.TokenAddress, data)},
		metadata,
	)
}

// ApproveERC1155ForAllGasless submits an ERC-1155 operator approval through
// the configured proxy, Safe, or deposit wallet.
func (c *AuthenticatedClient) ApproveERC1155ForAllGasless(
	ctx context.Context,
	req ERC1155ApprovalForAllRequest,
	metadata string,
) (*polyrelay.Handle, error) {
	data, err := packERC1155ApprovalForAll(req)
	if err != nil {
		return nil, err
	}
	return c.PrepareGaslessTransaction(
		ctx,
		[]polyrelay.TransactionCall{tokenCall(req.TokenAddress, data)},
		metadata,
	)
}

// TransferERC20Gasless submits an ERC-20 transfer through the configured
// proxy, Safe, or deposit wallet.
func (c *AuthenticatedClient) TransferERC20Gasless(
	ctx context.Context,
	req ERC20TransferRequest,
	metadata string,
) (*polyrelay.Handle, error) {
	data, err := packERC20Transfer(req)
	if err != nil {
		return nil, err
	}
	return c.PrepareGaslessTransaction(
		ctx,
		[]polyrelay.TransactionCall{tokenCall(req.TokenAddress, data)},
		metadata,
	)
}
