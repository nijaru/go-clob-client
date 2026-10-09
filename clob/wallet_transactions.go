package clob

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// WalletTransactionHandle is a broadcast EOA or submitted gasless transaction.
// Wait is explicit and cancellable; cancelling a wait cannot undo a submission.
// It holds no open RPC connection and starts no background goroutine.
type WalletTransactionHandle struct {
	TransactionID   string
	TransactionHash string
	relayer         *GaslessTransactionHandle
	signer          *SignerClient
}

func (h *WalletTransactionHandle) Wait(ctx context.Context) (*TransactionOutcome, error) {
	if h.relayer != nil {
		return h.relayer.Wait(ctx)
	}
	receipt, err := h.signer.waitWalletTransactionReceipt(ctx, h.TransactionHash)
	if err != nil {
		return nil, err
	}
	return &TransactionOutcome{TransactionHash: receipt.TxHash.Hex()}, nil
}

// WaitReceipt first confirms through the existing EOA/relayer handle, then
// waits for the actual on-chain receipt. Neither step waits for CLOB indexing.
// A receipt lookup error cannot undo a transaction already confirmed by Wait.
func (h *WalletTransactionHandle) WaitReceipt(ctx context.Context) (*types.Receipt, error) {
	if h.relayer == nil {
		return h.signer.waitWalletTransactionReceipt(ctx, h.TransactionHash)
	}
	outcome, err := h.Wait(ctx)
	if err != nil {
		return nil, err
	}
	return h.signer.waitWalletTransactionReceipt(ctx, outcome.TransactionHash)
}

// WaitWalletTransactionReceipt waits for an on-chain receipt for an outcome,
// including an explicitly deployed wallet. Unlike a relayer status, the receipt
// includes logs and block metadata. A missing/invalid hash cannot prove success.
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
	if err != nil || !strings.HasPrefix(hash, "0x") || len(decoded) != common.HashLength {
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
		return nil, fmt.Errorf("%w: %s reverted", ErrWalletTransactionFailed, hash)
	}
	return receipt, nil
}

// ExecuteWalletTransaction executes calls in order. Smart wallets relay one
// atomic batch. EOAs confirm each preceding call before broadcasting the last;
// an EOA sequence is not atomic and a failure may leave a completed prefix.
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
			relayer:         h,
			signer:          c.SignerClient,
		}, nil
	}
	var handle *WalletTransactionHandle
	for i, call := range calls {
		tx, err := c.broadcastWalletCall(ctx, call)
		if err != nil {
			return nil, fmt.Errorf("wallet: broadcast call %d of %d: %w", i+1, len(calls), err)
		}
		handle = &WalletTransactionHandle{TransactionHash: tx.Hash().Hex(), signer: c.SignerClient}
		if i+1 < len(calls) {
			if _, err := handle.Wait(ctx); err != nil {
				return nil, fmt.Errorf("wallet: confirm call %d of %d: %w", i+1, len(calls), err)
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
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), c.signer.PrivateKey())
	if err != nil {
		return nil, fmt.Errorf("wallet: sign: %w", err)
	}
	if err := ec.SendTransaction(ctx, signed); err != nil {
		return nil, fmt.Errorf("wallet: broadcast: %w", err)
	}
	return signed, nil
}
