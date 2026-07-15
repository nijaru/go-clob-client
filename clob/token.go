package clob

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

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
  }
]`

var tokenABI abi.ABI

var (
	// ErrInvalidTokenAmount indicates a nil, negative, or overflowing uint256 amount.
	ErrInvalidTokenAmount = errors.New("invalid token amount")
	// ErrTokenOperationRequiresEOA indicates that a direct call was attempted for a smart wallet.
	ErrTokenOperationRequiresEOA = errors.New("token operation requires EOA")
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
