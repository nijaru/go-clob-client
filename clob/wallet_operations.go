package clob

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// WalletOperationsConfig selects the market-discovery endpoint. RPC and HTTP
// ownership remain with the authenticated client; this facade starts no tasks.
type WalletOperationsConfig struct{ GammaHost string }

// WalletOperations resolves market protocol and routes position operations to
// the collateral adapters or native V2 router. The lower-level CTF methods
// remain available for explicit collateral/partition calls (Rust parity).
type WalletOperations struct {
	client  *AuthenticatedClient
	markets *polyhttp.Client
}

func NewWalletOperations(
	client *AuthenticatedClient,
	cfg WalletOperationsConfig,
) (*WalletOperations, error) {
	if client == nil {
		return nil, fmt.Errorf("wallet: nil authenticated client")
	}
	if cfg.GammaHost == "" {
		cfg.GammaHost = "https://gamma-api.polymarket.com"
	}
	return &WalletOperations{
		client: client,
		markets: &polyhttp.Client{
			BaseURL:    cfg.GammaHost,
			HTTPClient: client.http.HTTPClient,
			UserAgent:  client.http.UserAgent,
		},
	}, nil
}

type WalletSplitRequest struct {
	ConditionID string     // existing market; exactly one of ConditionID or Legs
	Legs        []*big.Int // native binary/neg-risk positions
	Amount      *big.Int   // base units
	Metadata    string
}

// WalletMergeRequest identifies a market, existing V2 position, or combo legs.
// Amount=nil means max; a positive amount is checked against both balances.
// MarketID/PositionID additionally support Python's multiple-position workflow.
type WalletMergeRequest struct {
	ConditionID string
	MarketID    string
	PositionID  *big.Int
	Legs        []*big.Int
	Amount      *big.Int
	Metadata    string
}

type WalletRedeemRequest struct {
	ConditionID string // exactly one of ConditionID, MarketID or PositionID
	MarketID    string
	PositionID  *big.Int // native V2 YES/NO ID; redeem its entire balance
	Metadata    string
}

func (w *WalletOperations) SplitPosition(
	ctx context.Context,
	req WalletSplitRequest,
) (*WalletTransactionHandle, error) {
	calls, err := w.PrepareSplitPosition(ctx, req)
	if err != nil {
		return nil, err
	}
	return w.client.ExecuteWalletTransaction(ctx, calls, req.Metadata)
}

func (w *WalletOperations) MergePositions(
	ctx context.Context,
	req WalletMergeRequest,
) (*WalletTransactionHandle, error) {
	calls, _, err := w.prepareMerge(ctx, req)
	if err != nil {
		return nil, err
	}
	return w.client.ExecuteWalletTransaction(ctx, calls, req.Metadata)
}

func (w *WalletOperations) RedeemPositions(
	ctx context.Context,
	req WalletRedeemRequest,
) (*WalletTransactionHandle, error) {
	calls, err := w.PrepareRedeemPositions(ctx, req)
	if err != nil {
		return nil, err
	}
	return w.client.ExecuteWalletTransaction(ctx, calls, req.Metadata)
}

func (w *WalletOperations) PrepareSplitPosition(
	ctx context.Context,
	req WalletSplitRequest,
) ([]TransactionCall, error) {
	if (req.ConditionID != "") == (req.Legs != nil) {
		return nil, fmt.Errorf("%w: provide condition or legs", ErrInvalidPositionOperation)
	}
	if err := validateUint256(req.Amount, "split amount"); err != nil {
		return nil, err
	}
	if req.Legs != nil {
		if req.Amount.Sign() == 0 {
			return nil, fmt.Errorf(
				"%w: combo split amount must be positive",
				ErrInvalidPositionOperation,
			)
		}
		combo, err := DeriveComboPositions(req.Legs)
		if err != nil {
			return nil, err
		}
		prepare, err := w.comboPrepareCall(combo.Legs)
		if err != nil {
			return nil, err
		}
		call, err := w.nativeCall("split", combo.ConditionID, req.Amount, 0)
		if err != nil {
			return nil, err
		}
		return []TransactionCall{prepare, call}, nil
	}
	market, err := w.resolveMarket(ctx, req.ConditionID, "", false)
	if err != nil {
		return nil, err
	}
	call, err := w.marketCall("split", market, req.Amount)
	if err != nil {
		return nil, err
	}
	return []TransactionCall{call}, nil
}

func (w *WalletOperations) PrepareMergePositions(
	ctx context.Context,
	req WalletMergeRequest,
) ([]TransactionCall, error) {
	calls, _, err := w.prepareMerge(ctx, req)
	return calls, err
}

func (w *WalletOperations) prepareMerge(
	ctx context.Context,
	req WalletMergeRequest,
) ([]TransactionCall, string, error) {
	identifiers := 0
	if req.ConditionID != "" {
		identifiers++
	}
	if req.MarketID != "" {
		identifiers++
	}
	if req.PositionID != nil {
		identifiers++
	}
	if req.Legs != nil {
		identifiers++
	}
	if identifiers != 1 {
		return nil, "", fmt.Errorf("%w: provide one merge identifier", ErrInvalidPositionOperation)
	}
	if req.Amount != nil {
		if err := validateUint256(req.Amount, "merge amount"); err != nil {
			return nil, "", err
		}
		if req.Amount.Sign() == 0 {
			return nil, "", fmt.Errorf(
				"%w: merge amount must be positive",
				ErrInvalidPositionOperation,
			)
		}
	}
	var market *walletMarketContext
	var condition V2ConditionID
	var prepare *TransactionCall
	var ids [2]*big.Int
	var err error
	if req.Legs != nil {
		combo, deriveErr := DeriveComboPositions(req.Legs)
		if deriveErr != nil {
			return nil, "", deriveErr
		}
		condition, ids = combo.ConditionID, combo.PositionIDs
		call, prepareErr := w.comboPrepareCall(combo.Legs)
		if prepareErr != nil {
			return nil, "", prepareErr
		}
		prepare = &call
	} else if req.PositionID != nil {
		condition, _, err = DecodeV2PositionID(req.PositionID)
		if err != nil {
			return nil, "", err
		}
		ids = condition.PositionIDs()
	} else {
		market, err = w.resolveMarket(ctx, req.ConditionID, req.MarketID, false)
		if err != nil {
			return nil, "", err
		}
		ids = market.outcomes
	}
	cfg, err := getContractConfig(w.client.chainID)
	if err != nil {
		return nil, "", err
	}
	token := common.HexToAddress(cfg.PositionManager)
	conditionKey := condition.String()
	if market != nil {
		token = market.token
		conditionKey = market.condition
	}
	if token == (common.Address{}) {
		return nil, "", fmt.Errorf("%w: position contract unavailable", ErrInvalidPositionOperation)
	}
	balances, err := w.readPositionBalances(ctx, token, ids[:])
	if err != nil {
		return nil, "", err
	}
	maximum := balances[0]
	if balances[1].Cmp(maximum) < 0 {
		maximum = balances[1]
	}
	if maximum.Sign() == 0 {
		return nil, "", fmt.Errorf("%w: no complementary positions", ErrInvalidPositionOperation)
	}
	amount := req.Amount
	if amount == nil {
		amount = maximum
	} else if amount.Cmp(maximum) > 0 {
		return nil, "", fmt.Errorf("%w: amount exceeds mergeable balance %s", ErrInvalidPositionOperation, maximum)
	}
	var call TransactionCall
	if market != nil {
		call, err = w.marketCall("merge", market, amount)
	} else {
		call, err = w.nativeCall("merge", condition, amount, 0)
	}
	if err != nil {
		return nil, "", err
	}
	if prepare != nil {
		return []TransactionCall{*prepare, call}, conditionKey, nil
	}
	return []TransactionCall{call}, conditionKey, nil
}

// MergeMultiplePositions implements the stable Python batch contract: no mixed
// market/position identifier styles and no duplicate conditions. All balances
// and call plans are validated before any transaction is submitted.
func (w *WalletOperations) MergeMultiplePositions(
	ctx context.Context,
	positions []WalletMergeRequest,
	metadata string,
) (*WalletTransactionHandle, error) {
	if len(positions) == 0 {
		return nil, fmt.Errorf("%w: empty merge batch", ErrInvalidPositionOperation)
	}
	positionStyle := positions[0].PositionID != nil
	seen := make(map[string]struct{}, len(positions))
	calls := make([]TransactionCall, 0, len(positions))
	for _, req := range positions {
		if req.Legs != nil || (req.PositionID != nil) != positionStyle {
			return nil, fmt.Errorf("%w: mixed merge identifier styles", ErrInvalidPositionOperation)
		}
		prepared, condition, err := w.prepareMerge(ctx, req)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[condition]; ok {
			return nil, fmt.Errorf("%w: duplicate merge condition", ErrInvalidPositionOperation)
		}
		seen[condition] = struct{}{}
		calls = append(calls, prepared...)
	}
	return w.client.ExecuteWalletTransaction(ctx, calls, metadata)
}

func (w *WalletOperations) PrepareRedeemPositions(
	ctx context.Context,
	req WalletRedeemRequest,
) ([]TransactionCall, error) {
	identifiers := 0
	if req.ConditionID != "" {
		identifiers++
	}
	if req.MarketID != "" {
		identifiers++
	}
	if req.PositionID != nil {
		identifiers++
	}
	if identifiers != 1 {
		return nil, fmt.Errorf("%w: provide one redemption identifier", ErrInvalidPositionOperation)
	}
	if req.PositionID != nil {
		condition, outcome, err := DecodeV2PositionID(req.PositionID)
		if err != nil {
			return nil, err
		}
		cfg, err := getContractConfig(w.client.chainID)
		if err != nil {
			return nil, err
		}
		balances, err := w.readPositionBalances(
			ctx,
			common.HexToAddress(cfg.PositionManager),
			[]*big.Int{req.PositionID},
		)
		if err != nil {
			return nil, err
		}
		if balances[0].Sign() == 0 {
			return nil, fmt.Errorf("%w: no position balance to redeem", ErrInvalidPositionOperation)
		}
		call, err := w.nativeCall("redeem", condition, balances[0], outcome)
		if err != nil {
			return nil, err
		}
		return []TransactionCall{call}, nil
	}
	market, err := w.resolveMarket(ctx, req.ConditionID, req.MarketID, true)
	if err != nil {
		return nil, err
	}
	if !market.native {
		call, err := w.marketCall("redeem", market, nil)
		if err != nil {
			return nil, err
		}
		return []TransactionCall{call}, nil
	}
	balances, err := w.readPositionBalances(ctx, market.token, market.outcomes[:])
	if err != nil {
		return nil, err
	}
	calls := make([]TransactionCall, 0, 2)
	for outcome, balance := range balances {
		if balance.Sign() == 0 {
			continue
		}
		call, err := w.nativeCall("redeem", market.v2Condition, balance, uint8(outcome))
		if err != nil {
			return nil, err
		}
		calls = append(calls, call)
	}
	if len(calls) == 0 {
		return nil, fmt.Errorf("%w: no market balances to redeem", ErrInvalidPositionOperation)
	}
	return calls, nil
}

func (w *WalletOperations) nativeCall(
	method string,
	condition V2ConditionID,
	amount *big.Int,
	outcome uint8,
) (TransactionCall, error) {
	cfg, err := getContractConfig(w.client.chainID)
	if err != nil {
		return TransactionCall{}, err
	}
	if cfg.ProtocolV2Router == "" {
		return TransactionCall{}, fmt.Errorf(
			"%w: V2 router unavailable",
			ErrInvalidPositionOperation,
		)
	}
	data, err := packV2PositionCall(method, condition, amount, outcome)
	return tokenCall(common.HexToAddress(cfg.ProtocolV2Router), data), err
}

func (w *WalletOperations) comboPrepareCall(legs []*big.Int) (TransactionCall, error) {
	cfg, err := getContractConfig(w.client.chainID)
	if err != nil {
		return TransactionCall{}, err
	}
	if cfg.CombinatorialModule == "" {
		return TransactionCall{}, fmt.Errorf(
			"%w: combo module unavailable",
			ErrInvalidPositionOperation,
		)
	}
	data, err := walletABI.Pack("prepareCondition", legs)
	return tokenCall(common.HexToAddress(cfg.CombinatorialModule), data), err
}

func (w *WalletOperations) marketCall(
	method string,
	market *walletMarketContext,
	amount *big.Int,
) (TransactionCall, error) {
	if market.native {
		return w.nativeCall(method, market.v2Condition, amount, 0)
	}
	cfg, err := getContractConfig(w.client.chainID)
	if err != nil {
		return TransactionCall{}, err
	}
	collateral := common.HexToAddress(cfg.Collateral)
	condition := common.HexToHash(market.condition)
	var data []byte
	switch method {
	case "split":
		data, err = packSplitPosition(SplitBinary(collateral, condition, amount))
	case "merge":
		data, err = packMergePositions(MergeBinary(collateral, condition, amount))
	case "redeem":
		data, err = packRedeemPositions(RedeemBinary(collateral, condition))
	default:
		return TransactionCall{}, fmt.Errorf(
			"%w: unsupported method %s",
			ErrInvalidPositionOperation,
			method,
		)
	}
	return tokenCall(market.adapter, data), err
}

func (w *WalletOperations) readPositionBalances(
	ctx context.Context,
	token common.Address,
	ids []*big.Int,
) ([]*big.Int, error) {
	if token == (common.Address{}) {
		return nil, fmt.Errorf("%w: position contract unavailable", ErrInvalidPositionOperation)
	}
	for _, id := range ids {
		if err := validateUint256(id, "position ID"); err != nil {
			return nil, err
		}
	}
	method := "balanceOfBatch"
	var data []byte
	var err error
	if len(ids) == 1 {
		method = "balanceOf"
		data, err = walletABI.Pack(method, w.client.WalletAddress(), ids[0])
	} else {
		owners := make([]common.Address, len(ids))
		for i := range owners {
			owners[i] = w.client.WalletAddress()
		}
		data, err = walletABI.Pack(method, owners, ids)
	}
	if err != nil {
		return nil, err
	}
	ec, err := w.client.dialRPC(ctx)
	if err != nil {
		return nil, err
	}
	defer ec.Close()
	response, err := ec.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
	if err != nil {
		return nil, err
	}
	values, err := walletABI.Unpack(method, response)
	if err != nil {
		return nil, err
	}
	if len(values) != 1 {
		return nil, fmt.Errorf("wallet: invalid balance response")
	}
	if method == "balanceOf" {
		return []*big.Int{values[0].(*big.Int)}, nil
	}
	balances := values[0].([]*big.Int)
	if len(balances) != len(ids) {
		return nil, fmt.Errorf("wallet: expected %d balances, got %d", len(ids), len(balances))
	}
	return balances, nil
}
