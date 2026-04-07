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
	ec, err := c.dialRPC(ctx)
	if err != nil {
		return nil, fmt.Errorf("ctf: dial rpc: %w", err)
	}
	defer ec.Close()

	key := c.signer.PrivateKey()
	from := crypto.PubkeyToAddress(key.PublicKey)

	nonce, err := ec.PendingNonceAt(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("ctf: get nonce: %w", err)
	}

	chainID := big.NewInt(c.chainID)
	gasTipCap, err := ec.SuggestGasTipCap(ctx)
	if err != nil {
		return nil, fmt.Errorf("ctf: suggest gas tip: %w", err)
	}

	head, err := ec.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("ctf: get latest header: %w", err)
	}
	gasFeeCap := new(big.Int).Add(
		gasTipCap,
		new(big.Int).Mul(head.BaseFee, big.NewInt(2)),
	)

	msg := ethereum.CallMsg{
		From:      from,
		To:        &to,
		Data:      data,
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
	}
	gasLimit, err := ec.EstimateGas(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("ctf: estimate gas: %w", err)
	}

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
		Gas:       gasLimit,
		To:        &to,
		Value:     big.NewInt(0),
		Data:      data,
	})

	signed, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), key)
	if err != nil {
		return nil, fmt.Errorf("ctf: sign tx: %w", err)
	}

	if err := ec.SendTransaction(ctx, signed); err != nil {
		return nil, fmt.Errorf("ctf: send tx: %w", err)
	}

	receipt, err := waitForReceipt(ctx, ec, signed.Hash())
	if err != nil {
		return nil, err
	}
	if receipt.Status == types.ReceiptStatusFailed {
		return nil, fmt.Errorf("ctf: transaction %s reverted", signed.Hash().Hex())
	}
	return receipt, nil
}

func waitForReceipt(
	ctx context.Context,
	ec *ethclient.Client,
	txHash common.Hash,
) (*types.Receipt, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("ctf: waiting for receipt %s: %w", txHash.Hex(), ctx.Err())
		case <-ticker.C:
			receipt, err := ec.TransactionReceipt(ctx, txHash)
			if err == nil {
				return receipt, nil
			}
			if !errors.Is(err, ethereum.NotFound) {
				return nil, fmt.Errorf("ctf: get receipt %s: %w", txHash.Hex(), err)
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

func (c *SignerClient) SplitPosition(
	ctx context.Context,
	req SplitPositionRequest,
) (*TxReceipt, error) {
	data, err := ctfABI.Pack("splitPosition",
		req.CollateralToken,
		req.ParentCollectionID,
		req.ConditionID,
		req.Partition,
		req.Amount,
	)
	if err != nil {
		return nil, fmt.Errorf("ctf: pack splitPosition: %w", err)
	}

	to, err := c.contractAddr(func(cc contractConfig) string { return cc.Conditional })
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
	data, err := ctfABI.Pack("mergePositions",
		req.CollateralToken,
		req.ParentCollectionID,
		req.ConditionID,
		req.Partition,
		req.Amount,
	)
	if err != nil {
		return nil, fmt.Errorf("ctf: pack mergePositions: %w", err)
	}

	to, err := c.contractAddr(func(cc contractConfig) string { return cc.Conditional })
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
	data, err := ctfABI.Pack("redeemPositions",
		req.CollateralToken,
		req.ParentCollectionID,
		req.ConditionID,
		req.IndexSets,
	)
	if err != nil {
		return nil, fmt.Errorf("ctf: pack redeemPositions: %w", err)
	}

	to, err := c.contractAddr(func(cc contractConfig) string { return cc.Conditional })
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
	data, err := negRiskABI.Pack("redeemPositions",
		req.ConditionID,
		req.Amounts,
	)
	if err != nil {
		return nil, fmt.Errorf("ctf: pack negRisk redeemPositions: %w", err)
	}

	to, err := c.contractAddr(func(cc contractConfig) string { return cc.NegRiskAdapter })
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

func CollectionID(
	parentCollectionID common.Hash,
	conditionID common.Hash,
	indexSet *big.Int,
) common.Hash {
	inner := make([]byte, 64)
	copy(inner[:32], conditionID.Bytes())
	indexSet.FillBytes(inner[32:64])
	h := crypto.Keccak256Hash(inner)

	var result [32]byte
	pb := parentCollectionID.Bytes()
	hb := h.Bytes()
	for i := range result {
		result[i] = pb[i] ^ hb[i]
	}
	return common.BytesToHash(result[:])
}

func PositionID(collateralToken common.Address, collectionID common.Hash) *big.Int {
	buf := make([]byte, 0, 52)
	buf = append(buf, collateralToken.Bytes()...)
	buf = append(buf, collectionID.Bytes()...)
	h := crypto.Keccak256Hash(buf)
	return new(big.Int).SetBytes(h.Bytes())
}
