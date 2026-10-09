package clob

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// WalletTransactionSubmission records one EOA send attempt. BroadcastUncertain
// means the RPC send failed, not that the transaction was rejected: reconcile
// its locally computed hash before retrying. ConfirmedReceipt is the valid mined
// receipt observed during execution, including a revert; nil means unconfirmed.
type WalletTransactionHandle struct {
	TransactionID   string
	TransactionHash string
	Submissions     []WalletTransactionSubmission
	RequestedCalls  int
	relayer         *GaslessTransactionHandle
	signer          *SignerClient
}

func (h *WalletTransactionHandle) Wait(ctx context.Context) (*TransactionOutcome, error) {
	if h == nil || h.signer == nil {
		return nil, fmt.Errorf("wallet: no transaction to wait for")
	}
	if h.relayer != nil {
		return h.relayer.Wait(ctx)
	}
	receipt, err := h.WaitReceipt(ctx)
	if err != nil {
		return nil, err
	}
	return &TransactionOutcome{TransactionHash: receipt.TxHash.Hex()}, nil
}

// WaitReceipt waits for all submitted EOA calls or the atomic relayer batch and
// returns the last collected receipt, even alongside an error. Use WaitReceipts
// to reconcile the entire sequence. Neither method waits for CLOB indexing,
// reorg finality, or ERC-20 return-value checks.
func (h *WalletTransactionHandle) WaitReceipt(ctx context.Context) (*types.Receipt, error) {
	receipts, err := h.WaitReceipts(ctx)
	if len(receipts) == 0 {
		return nil, err
	}
	return receipts[len(receipts)-1], err
}

func (h *WalletTransactionHandle) WaitReceipts(ctx context.Context) ([]*types.Receipt, error) {
	if h == nil || h.signer == nil {
		return nil, fmt.Errorf("wallet: no transaction to wait for")
	}
	if h.relayer != nil {
		outcome, err := h.relayer.Wait(ctx)
		if err != nil {
			return nil, err
		}
		receipt, err := h.signer.waitWalletTransactionReceipt(ctx, outcome.TransactionHash)
		if receipt == nil {
			return nil, err
		}
		return []*types.Receipt{receipt}, err
	}
	if len(h.Submissions) == 0 || h.RequestedCalls < len(h.Submissions) {
		return nil, fmt.Errorf("wallet: no valid EOA submissions to wait for")
	}
	receipts := make([]*types.Receipt, 0, len(h.Submissions))
	for _, submission := range h.Submissions {
		receipt, err := h.signer.waitWalletTransactionReceipt(ctx, submission.TransactionHash)
		if receipt != nil {
			receipts = append(receipts, receipt)
		}
		if err != nil {
			return receipts, err
		}
	}
	if len(h.Submissions) != h.RequestedCalls {
		return receipts, ErrWalletTransactionIncomplete
	}
	return receipts, nil
}
func (c *AuthenticatedClient) WaitWalletTransactionReceipt(
	ctx context.Context,
	outcome TransactionOutcome,
) (*types.Receipt, error) {
	return c.SignerClient.waitWalletTransactionReceipt(ctx, outcome.TransactionHash)
}

func (c *SignerClient) waitWalletTransactionReceipt(
	ctx context.Context,
	hash string,
) (*types.Receipt, error) {
	decoded, err := hex.DecodeString(strings.TrimPrefix(hash, "0x"))
	if err != nil || !strings.HasPrefix(hash, "0x") || len(decoded) != common.HashLength ||
		common.BytesToHash(decoded) == (common.Hash{}) {
		return nil, fmt.Errorf("wallet: receipt requires a valid transaction hash")
	}
	ec, err := c.dialRPC(ctx)
	if err != nil {
		return nil, err
	}
	defer ec.Close()
	receipt, err := waitForReceipt(ctx, ec, common.BytesToHash(decoded), "wallet")
	if err != nil {
		return nil, err
	}
	if receipt.Status == types.ReceiptStatusFailed {
		return receipt, fmt.Errorf("%w: %s reverted", ErrWalletTransactionFailed, hash)
	}
	return receipt, nil
}

// ExecuteWalletTransaction executes calls in order. Smart wallets relay one
// atomic batch. EOAs confirm each preceding call before broadcasting the last;
// an EOA sequence is not atomic. On error a nonnil handle retains any confirmed
// prefix and every attempted send hash, including uncertain broadcasts. Never
// blindly retry the whole sequence; inspect the handle and reconcile first.
func (c *AuthenticatedClient) ExecuteWalletTransaction(
	ctx context.Context,
	calls []TransactionCall,
	metadata string,
) (*WalletTransactionHandle, error) {
	if len(calls) == 0 {
		return nil, fmt.Errorf("%w: no calls", ErrInvalidPositionOperation)
	}
	for _, call := range calls {
		if err := validateUint256(call.Value, "call value"); err != nil {
			return nil, err
		}
	}
	if c.signatureType != SignatureTypeEOA {
		h, err := c.PrepareGaslessTransaction(ctx, calls, metadata)
		if err != nil {
			return nil, err
		}
		return &WalletTransactionHandle{
			TransactionID:   h.TransactionID,
			TransactionHash: h.TransactionHash,
			RequestedCalls:  len(calls),
			relayer:         h,
			signer:          c.SignerClient,
		}, nil
	}
	handle := &WalletTransactionHandle{RequestedCalls: len(calls), signer: c.SignerClient}
	for i, call := range calls {
		tx, err := c.broadcastWalletCall(ctx, call)
		if tx != nil {
			handle.TransactionHash = tx.Hash().Hex()
			handle.Submissions = append(handle.Submissions, WalletTransactionSubmission{
				TransactionHash:    handle.TransactionHash,
				BroadcastUncertain: err != nil,
			})
		}
		if err != nil {
			if len(handle.Submissions) == 0 {
				handle = nil
			}
			return handle, fmt.Errorf("wallet: broadcast call %d of %d: %w", i+1, len(calls), err)
		}
		if i+1 < len(calls) {
			receipt, err := c.waitWalletTransactionReceipt(ctx, handle.TransactionHash)
			handle.Submissions[i].ConfirmedReceipt = receipt
			if err != nil {
				return handle, fmt.Errorf("wallet: confirm call %d of %d: %w", i+1, len(calls), err)
			}
		}
	}
	return handle, nil
}

func (c *SignerClient) broadcastWalletCall(
	ctx context.Context,
	call TransactionCall,
) (*types.Transaction, error) {
	if err := c.requireEOATokenOperation(); err != nil {
		return nil, err
	}
	if err := validateUint256(call.Value, "call value"); err != nil {
		return nil, err
	}
	ec, err := c.dialRPC(ctx)
	if err != nil {
		return nil, fmt.Errorf("wallet: dial rpc: %w", err)
	}
	defer ec.Close()
	from := c.signer.Address()
	nonce, err := ec.PendingNonceAt(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("wallet: nonce: %w", err)
	}
	tip, err := ec.SuggestGasTipCap(ctx)
	if err != nil {
		return nil, fmt.Errorf("wallet: gas tip: %w", err)
	}
	head, err := ec.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("wallet: latest header: %w", err)
	}
	if head.BaseFee == nil {
		return nil, fmt.Errorf("wallet: chain does not support EIP-1559")
	}
	fee := new(big.Int).Add(tip, new(big.Int).Mul(head.BaseFee, big.NewInt(2)))
	gas, err := ec.EstimateGas(
		ctx,
		ethereum.CallMsg{
			From:      from,
			To:        &call.To,
			Value:     call.Value,
			Data:      call.Data,
			GasFeeCap: fee,
			GasTipCap: tip,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("wallet: estimate gas: %w", err)
	}
	chainID := big.NewInt(c.chainID)
	tx := types.NewTx(
		&types.DynamicFeeTx{
			ChainID:   chainID,
			Nonce:     nonce,
			GasTipCap: tip,
			GasFeeCap: fee,
			Gas:       gas,
			To:        &call.To,
			Value:     call.Value,
			Data:      call.Data,
		},
	)
	signed, err := c.signer.SignTransaction(ctx, chainID, tx)
	if err != nil {
		return nil, fmt.Errorf("wallet: sign: %w", err)
	}
	if err := ec.SendTransaction(ctx, signed); err != nil {
		return signed, fmt.Errorf("wallet: broadcast: %w", err)
	}
	return signed, nil
}

type WalletTransactionSubmission struct {
	TransactionHash    string
	BroadcastUncertain bool
	ConfirmedReceipt   *types.Receipt
}

var ErrWalletTransactionIncomplete = errors.New("wallet: not all requested calls were submitted")
