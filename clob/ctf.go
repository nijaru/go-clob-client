package clob

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func (c *SignerClient) dialRPC(ctx context.Context) (*ethclient.Client, error) {
	return ethclient.DialContext(ctx, c.rpcURL)
}

func (c *SignerClient) sendTxAndWait(
	ctx context.Context,
	to common.Address,
	data []byte,
) (*types.Receipt, error) {
	return c.sendContractTxAndWait(ctx, to, data, "ctf")
}

func (c *SignerClient) sendContractTxAndWait(
	ctx context.Context,
	to common.Address,
	data []byte,
	label string,
) (*types.Receipt, error) {
	call := tokenCall(to, append([]byte(nil), data...))
	hash, err := c.broadcastWalletCall(ctx, call)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	receipt, err := c.waitWalletCallReceipt(ctx, call, hash.Hex())
	if err != nil {
		sendErr := walletSubmissionError(hash, false, err)
		sendErr.Submission.ConfirmedReceipt = receipt
		return receipt, fmt.Errorf("%s: %w", label, sendErr)
	}
	return receipt, nil
}

// waitForReceipt polls for a transaction receipt every 250ms — matching alloy's
// default HTTP polling interval. NotFound is expected until the tx is mined and
// is retried silently; any other error propagates immediately. Only a matching
// mined receipt with an exact success/revert status is trusted. Reverts are
// returned intact so callers can retain the receipt while reporting failure.
func waitForReceipt(
	ctx context.Context,
	ec *ethclient.Client,
	txHash common.Hash,
	label string,
) (*types.Receipt, error) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("%s: waiting for receipt %s: %w", label, txHash.Hex(), ctx.Err())
		case <-ticker.C:
			receipt, err := ec.TransactionReceipt(ctx, txHash)
			if err == nil {
				if receipt.TxHash != txHash || receipt.BlockNumber == nil ||
					receipt.BlockNumber.Sign() < 0 || receipt.BlockHash == (common.Hash{}) {
					return nil, fmt.Errorf(
						"%s: invalid receipt identity or mined block for %s",
						label,
						txHash.Hex(),
					)
				}
				if receipt.Status != types.ReceiptStatusSuccessful &&
					receipt.Status != types.ReceiptStatusFailed {
					return nil, fmt.Errorf(
						"%s: invalid receipt status %d for %s",
						label,
						receipt.Status,
						txHash.Hex(),
					)
				}
				return receipt, nil
			}
			if !errors.Is(err, ethereum.NotFound) {
				return nil, fmt.Errorf("%s: get receipt %s: %w", label, txHash.Hex(), err)
			}
		}
	}
}

func (c *SignerClient) contractAddr(field func(contractConfig) string) (common.Address, error) {
	cfg, err := getContractConfig(c.chainID)
	if err != nil {
		return common.Address{}, err
	}
	return common.HexToAddress(field(cfg)), nil
}

// conditionalAddr resolves the ConditionalTokens (CTF) contract for the chain.
func (c *SignerClient) conditionalAddr() (common.Address, error) {
	return c.contractAddr(func(cc contractConfig) string { return cc.Conditional })
}

// negRiskAdapterAddr resolves the NegRiskAdapter contract for the chain.
func (c *SignerClient) negRiskAdapterAddr() (common.Address, error) {
	return c.contractAddr(func(cc contractConfig) string { return cc.NegRiskAdapter })
}

// packSplitPosition encodes a CTF splitPosition call. Shared by the on-chain EOA
// path and the gasless relayer path so the calldata can never diverge.
func packSplitPosition(req SplitPositionRequest) ([]byte, error) {
	if err := validateCTFPartition(req.Partition, req.Amount); err != nil {
		return nil, err
	}
	return ctfABI.Pack(
		"splitPosition",
		req.CollateralToken,
		req.ParentCollectionID,
		req.ConditionID,
		req.Partition,
		req.Amount,
	)
}

func packMergePositions(req MergePositionsRequest) ([]byte, error) {
	if err := validateCTFPartition(req.Partition, req.Amount); err != nil {
		return nil, err
	}
	return ctfABI.Pack(
		"mergePositions",
		req.CollateralToken,
		req.ParentCollectionID,
		req.ConditionID,
		req.Partition,
		req.Amount,
	)
}

func packRedeemPositions(req RedeemPositionsRequest) ([]byte, error) {
	if len(req.IndexSets) == 0 {
		return nil, fmt.Errorf("ctf: empty index sets")
	}
	for _, index := range req.IndexSets {
		if err := validateUint256(index, "index set"); err != nil {
			return nil, err
		}
		if index.Sign() == 0 {
			return nil, fmt.Errorf("ctf: zero index set")
		}
	}
	return ctfABI.Pack(
		"redeemPositions",
		req.CollateralToken,
		req.ParentCollectionID,
		req.ConditionID,
		req.IndexSets,
	)
}

func packRedeemNegRisk(req RedeemNegRiskRequest) ([]byte, error) {
	if len(req.Amounts) != 2 {
		return nil, fmt.Errorf("ctf: neg-risk redemption requires two amounts")
	}
	for _, amount := range req.Amounts {
		if err := validateUint256(amount, "redemption amount"); err != nil {
			return nil, err
		}
	}
	return negRiskABI.Pack("redeemPositions", req.ConditionID, req.Amounts)
}

func (c *SignerClient) SplitPosition(
	ctx context.Context,
	req SplitPositionRequest,
) (*TxReceipt, error) {
	data, err := packSplitPosition(req)
	if err != nil {
		return nil, fmt.Errorf("ctf: pack splitPosition: %w", err)
	}

	to, err := c.conditionalAddr()
	if err != nil {
		return nil, err
	}

	receipt, err := c.sendTxAndWait(ctx, to, data)
	if err != nil {
		return nil, err
	}
	return &TxReceipt{Hash: receipt.TxHash, BlockNumber: receipt.BlockNumber.Uint64()}, nil
}

func (c *SignerClient) MergePositions(
	ctx context.Context,
	req MergePositionsRequest,
) (*TxReceipt, error) {
	data, err := packMergePositions(req)
	if err != nil {
		return nil, fmt.Errorf("ctf: pack mergePositions: %w", err)
	}

	to, err := c.conditionalAddr()
	if err != nil {
		return nil, err
	}

	receipt, err := c.sendTxAndWait(ctx, to, data)
	if err != nil {
		return nil, err
	}
	return &TxReceipt{Hash: receipt.TxHash, BlockNumber: receipt.BlockNumber.Uint64()}, nil
}

func (c *SignerClient) RedeemPositions(
	ctx context.Context,
	req RedeemPositionsRequest,
) (*TxReceipt, error) {
	data, err := packRedeemPositions(req)
	if err != nil {
		return nil, fmt.Errorf("ctf: pack redeemPositions: %w", err)
	}

	to, err := c.conditionalAddr()
	if err != nil {
		return nil, err
	}

	receipt, err := c.sendTxAndWait(ctx, to, data)
	if err != nil {
		return nil, err
	}
	return &TxReceipt{Hash: receipt.TxHash, BlockNumber: receipt.BlockNumber.Uint64()}, nil
}

func (c *SignerClient) RedeemNegRisk(
	ctx context.Context,
	req RedeemNegRiskRequest,
) (*TxReceipt, error) {
	data, err := packRedeemNegRisk(req)
	if err != nil {
		return nil, fmt.Errorf("ctf: pack negRisk redeemPositions: %w", err)
	}

	to, err := c.negRiskAdapterAddr()
	if err != nil {
		return nil, err
	}

	receipt, err := c.sendTxAndWait(ctx, to, data)
	if err != nil {
		return nil, err
	}
	return &TxReceipt{Hash: receipt.TxHash, BlockNumber: receipt.BlockNumber.Uint64()}, nil
}

func ConditionID(oracle common.Address, questionID common.Hash, outcomeSlotCount uint) common.Hash {
	buf := make([]byte, 0, 84)
	buf = append(buf, oracle.Bytes()...)
	buf = append(buf, questionID.Bytes()...)
	n := new(big.Int).SetUint64(uint64(outcomeSlotCount))
	slot := make([]byte, 32)
	n.FillBytes(slot)
	buf = append(buf, slot...)
	return crypto.Keccak256Hash(buf)
}

func PositionID(collateralToken common.Address, collectionID common.Hash) *big.Int {
	buf := make([]byte, 0, 52)
	buf = append(buf, collateralToken.Bytes()...)
	buf = append(buf, collectionID.Bytes()...)
	h := crypto.Keccak256Hash(buf)
	return new(big.Int).SetBytes(h.Bytes())
}
