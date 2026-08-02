package clob

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
)

// CTFProvider is the read-only subset of an Ethereum provider required by
// CTFClient. *ethclient.Client satisfies this interface, and the narrow
// contract also makes provider-backed reads straightforward to test.
type CTFProvider interface {
	CallContract(ctx context.Context, call ethereum.CallMsg, blockNumber *big.Int) ([]byte, error)
}

// CTFClient provides provider-backed reads from the Conditional Tokens
// contract. Transactional CTF operations remain methods on SignerClient.
type CTFClient struct {
	provider    CTFProvider
	conditional common.Address
}

// NewCTFClient creates a read-only CTF client for a supported Polymarket chain.
func NewCTFClient(provider CTFProvider, chainID int64) (*CTFClient, error) {
	if provider == nil {
		return nil, fmt.Errorf("ctf: provider is required")
	}
	config, err := getContractConfig(chainID)
	if err != nil {
		return nil, err
	}
	return &CTFClient{
		provider:    provider,
		conditional: common.HexToAddress(config.Conditional),
	}, nil
}

// Provider returns the underlying provider used for contract reads.
func (c *CTFClient) Provider() CTFProvider {
	return c.provider
}

// ConditionID calls the CTF contract's getConditionId view.
func (c *CTFClient) ConditionID(
	ctx context.Context,
	request ConditionIDRequest,
) (common.Hash, error) {
	if request.OutcomeSlotCount == nil || request.OutcomeSlotCount.Sign() < 0 {
		return common.Hash{}, fmt.Errorf("ctf: outcome slot count must be non-negative")
	}
	data, err := ctfABI.Pack(
		"getConditionId",
		request.Oracle,
		request.QuestionID,
		request.OutcomeSlotCount,
	)
	if err != nil {
		return common.Hash{}, fmt.Errorf("ctf: pack getConditionId: %w", err)
	}
	result, err := c.call(ctx, data)
	if err != nil {
		return common.Hash{}, fmt.Errorf("ctf: get condition ID: %w", err)
	}
	return unpackCTFHash("getConditionId", result)
}

// CollectionID calls the CTF contract's getCollectionId view.
func (c *CTFClient) CollectionID(
	ctx context.Context,
	request CollectionIDRequest,
) (common.Hash, error) {
	if request.IndexSet == nil || request.IndexSet.Sign() < 0 {
		return common.Hash{}, fmt.Errorf("ctf: index set must be non-negative")
	}
	data, err := ctfABI.Pack(
		"getCollectionId",
		request.ParentCollectionID,
		request.ConditionID,
		request.IndexSet,
	)
	if err != nil {
		return common.Hash{}, fmt.Errorf("ctf: pack getCollectionId: %w", err)
	}
	result, err := c.call(ctx, data)
	if err != nil {
		return common.Hash{}, fmt.Errorf("ctf: get collection ID: %w", err)
	}
	return unpackCTFHash("getCollectionId", result)
}

// PositionID calls the CTF contract's getPositionId view.
func (c *CTFClient) PositionID(
	ctx context.Context,
	request PositionIDRequest,
) (*big.Int, error) {
	data, err := ctfABI.Pack(
		"getPositionId",
		request.CollateralToken,
		request.CollectionID,
	)
	if err != nil {
		return nil, fmt.Errorf("ctf: pack getPositionId: %w", err)
	}
	result, err := c.call(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("ctf: get position ID: %w", err)
	}
	outputs, err := ctfABI.Unpack("getPositionId", result)
	if err != nil {
		return nil, fmt.Errorf("ctf: unpack position ID: %w", err)
	}
	if len(outputs) != 1 {
		return nil, fmt.Errorf("ctf: getPositionId returned %d values", len(outputs))
	}
	positionID, ok := outputs[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("ctf: getPositionId returned %T, want *big.Int", outputs[0])
	}
	return new(big.Int).Set(positionID), nil
}

func (c *CTFClient) call(ctx context.Context, data []byte) ([]byte, error) {
	return c.provider.CallContract(ctx, ethereum.CallMsg{
		To:   &c.conditional,
		Data: data,
	}, nil)
}

func unpackCTFHash(method string, data []byte) (common.Hash, error) {
	outputs, err := ctfABI.Unpack(method, data)
	if err != nil {
		return common.Hash{}, fmt.Errorf("ctf: unpack %s: %w", method, err)
	}
	if len(outputs) != 1 {
		return common.Hash{}, fmt.Errorf("ctf: %s returned %d values", method, len(outputs))
	}
	switch value := outputs[0].(type) {
	case [32]byte:
		return common.BytesToHash(value[:]), nil
	case common.Hash:
		return value, nil
	case []byte:
		if len(value) != common.HashLength {
			return common.Hash{}, fmt.Errorf("ctf: %s returned %d bytes", method, len(value))
		}
		return common.BytesToHash(value), nil
	default:
		return common.Hash{}, fmt.Errorf("ctf: %s returned %T, want bytes32", method, outputs[0])
	}
}
