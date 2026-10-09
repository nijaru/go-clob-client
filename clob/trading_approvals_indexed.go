package clob

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// TradingApprovalsReader reads indexed approval snapshots without a signer.
// SetupTradingApprovals intentionally rechecks the chain before granting them.
type TradingApprovalsReader struct {
	chainID int64
	http    *polyhttp.Client
}

type TradingApprovalsReaderConfig struct {
	ChainID    int64
	DataHost   string
	HTTPClient *http.Client
}

func NewTradingApprovalsReader(cfg TradingApprovalsReaderConfig) (*TradingApprovalsReader, error) {
	if cfg.ChainID == 0 {
		cfg.ChainID = PolygonChainID
	}
	if _, err := getContractConfig(cfg.ChainID); err != nil {
		return nil, err
	}
	if cfg.DataHost == "" {
		cfg.DataHost = "https://data-api.polymarket.com"
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &TradingApprovalsReader{
		chainID: cfg.ChainID,
		http:    &polyhttp.Client{BaseURL: cfg.DataHost, HTTPClient: cfg.HTTPClient},
	}, nil
}

type indexedApprovalRow struct {
	Token    string          `json:"token"`
	Spender  string          `json:"spender"`
	Standard string          `json:"standard"`
	Amount   json.RawMessage `json:"amount"`
	Approved json.RawMessage `json:"approved"`
}

// FetchTradingApprovalsState validates the account, chain and every required
// row. Unrelated catalog rows are ignored; duplicate/missing required rows and
// malformed finite allowances never silently appear approved.
func (r *TradingApprovalsReader) FetchTradingApprovalsState(
	ctx context.Context,
	wallet common.Address,
) (*TradingApprovalsState, error) {
	if wallet == (common.Address{}) {
		return nil, fmt.Errorf("approvals: zero wallet")
	}
	var envelope struct {
		Data *struct {
			Address   string               `json:"address"`
			ChainID   int64                `json:"chain_id"`
			Contracts []indexedApprovalRow `json:"contracts"`
		} `json:"data"`
	}
	if err := r.http.GetJSON(ctx, "/v2/approvals", url.Values{"user": {wallet.Hex()}}, polyhttp.AuthNone, &envelope); err != nil {
		return nil, err
	}
	snapshot := envelope.Data
	if snapshot == nil || !common.IsHexAddress(snapshot.Address) ||
		common.HexToAddress(snapshot.Address) != wallet ||
		snapshot.ChainID != r.chainID {
		return nil, fmt.Errorf("approvals: snapshot wallet/chain mismatch")
	}
	cfg, err := getContractConfig(r.chainID)
	if err != nil {
		return nil, err
	}
	erc20, erc1155, err := requiredTradingApprovals(r.chainID, cfg)
	if err != nil {
		return nil, err
	}
	find := func(token, spender common.Address, standard string) (*indexedApprovalRow, error) {
		var match *indexedApprovalRow
		for i := range snapshot.Contracts {
			row := &snapshot.Contracts[i]
			if !strings.EqualFold(row.Token, token.Hex()) ||
				!strings.EqualFold(row.Spender, spender.Hex()) {
				continue
			}
			if match != nil {
				return nil, fmt.Errorf("approvals: duplicate required row")
			}
			match = row
		}
		if match == nil || match.Standard != standard ||
			(string(match.Approved) != "true" && string(match.Approved) != "false") {
			return nil, fmt.Errorf("approvals: missing or malformed required row")
		}
		return match, nil
	}
	missing := &MissingTradingApprovals{}
	for _, approval := range erc20 {
		row, err := find(approval.TokenAddress, approval.SpenderAddress, "ERC20")
		if err != nil {
			return nil, err
		}
		var wireAmount string
		if err := json.Unmarshal(row.Amount, &wireAmount); err != nil {
			return nil, fmt.Errorf("approvals: invalid allowance: %w", err)
		}
		amount := MaxUint256()
		if wireAmount != "max" {
			if wireAmount == "" || (len(wireAmount) > 1 && wireAmount[0] == '0') {
				return nil, fmt.Errorf("approvals: invalid finite allowance")
			}
			for _, digit := range wireAmount {
				if digit < '0' || digit > '9' {
					return nil, fmt.Errorf("approvals: invalid finite allowance")
				}
			}
			parsed, ok := new(big.Int).SetString(wireAmount, 10)
			if !ok || validateUint256(parsed, "allowance") != nil {
				return nil, fmt.Errorf("approvals: invalid finite allowance")
			}
			amount = parsed
		}
		if string(row.Approved) != "true" || amount.Cmp(approval.Amount) < 0 {
			missing.ERC20Approvals = append(missing.ERC20Approvals, Erc20TradingApproval(approval))
		}
	}
	for _, approval := range erc1155 {
		row, err := find(approval.TokenAddress, approval.OperatorAddress, "ERC1155")
		if err != nil {
			return nil, err
		}
		if string(row.Approved) != "true" {
			missing.ERC1155Approvals = append(missing.ERC1155Approvals, approval)
		}
	}
	return &TradingApprovalsState{Missing: missing, IsFullyApproved: missing.Empty()}, nil
}
