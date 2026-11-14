package clob

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// WalletNegRiskRequest identifies one protocol-V2 neg-risk condition and the
// explicit positive amount in six-decimal base units. All three operations
// route through the chain's configured V2 Router.
type WalletNegRiskRequest struct {
	ConditionID string
	Amount      *big.Int
	Metadata    string
}

// parseNegRiskEvent verifies the module, arity, reserved bits and synthetic
// Other index as specified by the official TS SDK's NegRiskEventIdSchema.
// Accepts bytes31 or bytes32 zero-padded form, not the NO position suffix.
func parseNegRiskEvent(value string) ([29]byte, uint16, error) {
	var event [29]byte
	if !strings.HasPrefix(value, "0x") ||
		(len(value) != 64 && len(value) != 66) ||
		(len(value) == 66 && !strings.HasSuffix(strings.ToLower(value), "00")) {
		return event, 0, fmt.Errorf(
			"%w: invalid V2 neg-risk condition form",
			ErrInvalidPositionOperation,
		)
	}
	condition, err := ParseV2ConditionID(value)
	if err != nil {
		return event, 0, err
	}
	if condition[0] != 2 {
		return event, 0, fmt.Errorf(
			"%w: condition must use neg-risk module 2",
			ErrInvalidPositionOperation,
		)
	}
	arity := binary.BigEndian.Uint16(condition[17:19])
	index := binary.BigEndian.Uint16(condition[29:31])
	var reserved [8]byte
	if arity < 2 || index > arity || !bytes.Equal(condition[19:27], reserved[:]) {
		return event, 0, fmt.Errorf(
			"%w: invalid neg-risk arity, reserved bits or condition index",
			ErrInvalidPositionOperation,
		)
	}
	copy(event[:], condition[:29])
	return event, index, nil
}

func (w *WalletOperations) prepareNegRisk(
	method string,
	req WalletNegRiskRequest,
) ([]TransactionCall, error) {
	if err := validateUint256(req.Amount, "neg-risk amount"); err != nil {
		return nil, err
	}
	if req.Amount.Sign() <= 0 {
		return nil, fmt.Errorf("%w: neg-risk amount must be positive", ErrInvalidPositionOperation)
	}
	event, index, err := parseNegRiskEvent(req.ConditionID)
	if err != nil {
		return nil, err
	}
	cfg, err := getContractConfig(w.client.chainID)
	if err != nil {
		return nil, err
	}
	if cfg.ProtocolV2Router == "" || !common.IsHexAddress(cfg.ProtocolV2Router) {
		return nil, fmt.Errorf("%w: V2 router is not configured", ErrInvalidPositionOperation)
	}
	amount := new(big.Int).Set(req.Amount)
	var encoded []byte
	switch method {
	case "convert":
		encoded, err = walletABI.Pack(method, event, index, amount)
	case "horizontalSplit", "horizontalMerge":
		encoded, err = walletABI.Pack(method, event, amount)
	default:
		return nil, fmt.Errorf(
			"%w: unknown neg-risk operation %q",
			ErrInvalidPositionOperation,
			method,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("encode %s: %w", method, err)
	}
	return []TransactionCall{
		tokenCall(common.HexToAddress(cfg.ProtocolV2Router), encoded),
	}, nil
}

// PrepareNegRiskConvert builds a Router conversion of NO for the specified
// condition into YES for every other condition, including Other where valid.
// It neither signs nor submits and does not grant operator approvals.
func (w *WalletOperations) PrepareNegRiskConvert(
	req WalletNegRiskRequest,
) ([]TransactionCall, error) {
	return w.prepareNegRisk("convert", req)
}

// PrepareNegRiskHorizontalSplit builds a conversion of pUSD into equal YES
// positions for the event's conditions, including Other.
func (w *WalletOperations) PrepareNegRiskHorizontalSplit(
	req WalletNegRiskRequest,
) ([]TransactionCall, error) {
	return w.prepareNegRisk("horizontalSplit", req)
}

// PrepareNegRiskHorizontalMerge builds a conversion of equal YES positions
// from all conditions, including Other, into pUSD.
func (w *WalletOperations) PrepareNegRiskHorizontalMerge(
	req WalletNegRiskRequest,
) ([]TransactionCall, error) {
	return w.prepareNegRisk("horizontalMerge", req)
}

func (w *WalletOperations) ConvertNegRisk(
	ctx context.Context,
	req WalletNegRiskRequest,
) (*WalletTransactionHandle, error) {
	calls, err := w.PrepareNegRiskConvert(req)
	if err != nil {
		return nil, err
	}
	return w.client.ExecuteWalletTransaction(ctx, calls, req.Metadata)
}

func (w *WalletOperations) HorizontalSplitNegRisk(
	ctx context.Context,
	req WalletNegRiskRequest,
) (*WalletTransactionHandle, error) {
	calls, err := w.PrepareNegRiskHorizontalSplit(req)
	if err != nil {
		return nil, err
	}
	return w.client.ExecuteWalletTransaction(ctx, calls, req.Metadata)
}

func (w *WalletOperations) HorizontalMergeNegRisk(
	ctx context.Context,
	req WalletNegRiskRequest,
) (*WalletTransactionHandle, error) {
	calls, err := w.PrepareNegRiskHorizontalMerge(req)
	if err != nil {
		return nil, err
	}
	return w.client.ExecuteWalletTransaction(ctx, calls, req.Metadata)
}
