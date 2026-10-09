package clob

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

type walletMarketContext struct {
	condition   string
	native      bool
	v2Condition V2ConditionID
	outcomes    [2]*big.Int
	token       common.Address
	adapter     common.Address
}

// Gamma encodes these stable array fields as either arrays or JSON strings.
type walletStringArray []string

func (a *walletStringArray) UnmarshalJSON(data []byte) error {
	var encoded string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &encoded); err != nil {
			return err
		}
		data = []byte(encoded)
	}
	return json.Unmarshal(data, (*[]string)(a))
}

type walletMarketWire struct {
	ID          string            `json:"id"`
	ConditionID string            `json:"conditionId"`
	Version     string            `json:"version"`
	Outcomes    walletStringArray `json:"outcomes"`
	TokenIDs    walletStringArray `json:"clobTokenIds"`
	PositionIDs walletStringArray `json:"positionIds"`
	NegRisk     *bool             `json:"negRisk"`
}

func (w *WalletOperations) resolveMarket(
	ctx context.Context,
	condition, marketID string,
	closed bool,
) (*walletMarketContext, error) {
	query := url.Values{"limit": {"1"}}
	if condition != "" {
		encoded, err := hex.DecodeString(strings.TrimPrefix(condition, "0x"))
		if err != nil || (len(encoded) != 31 && len(encoded) != 32) {
			return nil, fmt.Errorf("%w: invalid market condition", ErrInvalidPositionOperation)
		}
		query.Set("condition_ids", condition)
	} else {
		id, ok := new(big.Int).SetString(marketID, 10)
		if !ok || id.Sign() < 0 {
			return nil, fmt.Errorf("%w: market ID must be an integer", ErrInvalidPositionOperation)
		}
		query.Set("id", id.String())
	}
	if closed {
		query.Set("closed", "true")
	}
	var page struct {
		Markets []walletMarketWire `json:"markets"`
	}
	if err := w.markets.GetJSON(ctx, "/markets/keyset", query, polyhttp.AuthNone, &page); err != nil {
		return nil, err
	}
	markets := page.Markets
	if len(markets) != 1 || len(markets[0].Outcomes) != 2 {
		return nil, fmt.Errorf("%w: binary market not found", ErrInvalidPositionOperation)
	}
	market := markets[0]
	if condition != "" && !strings.EqualFold(condition, market.ConditionID) {
		return nil, fmt.Errorf("wallet: market response condition mismatch")
	}
	if marketID != "" && marketID != market.ID {
		return nil, fmt.Errorf("wallet: market response ID mismatch")
	}
	if market.ConditionID == "" || market.Version == "" {
		return nil, fmt.Errorf("wallet: missing market condition/version")
	}
	cfg, err := getContractConfig(w.client.chainID)
	if err != nil {
		return nil, err
	}
	result := &walletMarketContext{condition: market.ConditionID, native: market.Version == "v2"}
	ids := market.TokenIDs
	if result.native {
		result.v2Condition, err = ParseV2ConditionID(market.ConditionID)
		if err != nil {
			return nil, err
		}
		ids = market.PositionIDs
		result.token = common.HexToAddress(cfg.PositionManager)
	} else {
		data, err := hex.DecodeString(strings.TrimPrefix(market.ConditionID, "0x"))
		if err != nil || len(data) != 32 {
			return nil, fmt.Errorf("wallet: invalid CTF market condition")
		}
		if market.NegRisk == nil {
			return nil, fmt.Errorf("wallet: missing market negative-risk flag")
		}
		result.token = common.HexToAddress(cfg.Conditional)
		result.adapter = common.HexToAddress(cfg.CollateralAdapter)
		if *market.NegRisk {
			result.token = common.HexToAddress(cfg.NegRiskAdapter)
			result.adapter = common.HexToAddress(cfg.NegRiskCollateralAdapter)
		}
		if result.adapter == (common.Address{}) {
			return nil, fmt.Errorf("wallet: collateral adapter unavailable")
		}
	}
	if result.token == (common.Address{}) || len(ids) != 2 {
		return nil, fmt.Errorf("wallet: missing market position IDs or contract")
	}
	for i, id := range ids {
		parsed, ok := new(big.Int).SetString(id, 10)
		if !ok {
			return nil, fmt.Errorf("wallet: invalid market position ID")
		}
		if err := validateUint256(parsed, "market position ID"); err != nil {
			return nil, err
		}
		result.outcomes[i] = parsed
	}
	return result, nil
}
