package clob

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
	"github.com/nijaru/go-clob-client/internal/polyrelay"
)

const (
	collateralReturnPlanEndpoint   = "/v1/collateral-return/plan"
	collateralReturnSubmitEndpoint = "/v1/collateral-return/submit"
	collateralReturnRequestTimeout = 2 * time.Minute
	collateralReturnMetadata       = "Collateral return"
)

// PlanCollateralReturn builds an inspectable plan for returning redundant
// conditional-position value to the authenticated wallet's collateral.
// Planning is server-side and can take up to two minutes for large wallets.
func (c *AuthenticatedClient) PlanCollateralReturn(
	ctx context.Context,
) (*CollateralReturnPlan, error) {
	wallet, err := c.collateralReturnWallet()
	if err != nil {
		return nil, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, collateralReturnRequestTimeout)
	defer cancel()

	var out CollateralReturnPlan
	err = c.collateralReturnHTTP().PostJSON(
		requestCtx,
		collateralReturnPlanEndpoint,
		struct {
			Wallet string `json:"wallet"`
		}{Wallet: wallet.Hex()},
		polyhttp.AuthNone,
		&out,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

type collateralReturnSubmitRequest struct {
	Envelope *polyrelay.SubmitRequest `json:"envelope"`
	PlanHash string                   `json:"plan_hash"`
}

// ExecuteCollateralReturnPlan signs and submits the exact router call carried
// by a previously planned collateral return. It does not recompute the plan
// or submit approvals. Call Handle.Wait to observe the relayer transaction.
func (c *AuthenticatedClient) ExecuteCollateralReturnPlan(
	ctx context.Context,
	plan CollateralReturnPlan,
) (*polyrelay.Handle, error) {
	wallet, err := c.collateralReturnWallet()
	if err != nil {
		return nil, err
	}
	if plan.Wallet != wallet {
		return nil, fmt.Errorf(
			"%w: plan wallet %s does not match client wallet %s",
			ErrCollateralReturnPlanMismatch,
			plan.Wallet.Hex(),
			wallet.Hex(),
		)
	}
	if plan.ChainID != c.chainID {
		return nil, fmt.Errorf(
			"%w: plan chain %d does not match client chain %d",
			ErrCollateralReturnPlanMismatch,
			plan.ChainID,
			c.chainID,
		)
	}
	if strings.TrimSpace(plan.PlanHash) == "" {
		return nil, fmt.Errorf("%w: plan hash is empty", ErrCollateralReturnPlanMismatch)
	}
	data, err := hexutil.Decode(plan.RouterCall.Data)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: invalid router calldata: %v",
			ErrCollateralReturnPlanMismatch,
			err,
		)
	}

	cfg, err := c.gaslessConfig()
	if err != nil {
		return nil, err
	}
	calls := []polyrelay.TransactionCall{{
		To:    plan.RouterCall.To,
		Data:  data,
		Value: big.NewInt(0),
	}}
	requestCtx, cancel := context.WithTimeout(ctx, collateralReturnRequestTimeout)
	defer cancel()

	for attempt := 0; attempt <= polyrelay.SubmitRetryAttempts; attempt++ {
		envelope, buildErr := polyrelay.BuildGaslessSubmit(
			requestCtx,
			c.RelayerTransport(),
			cfg,
			c.signer.PrivateKey(),
			calls,
			collateralReturnMetadata,
		)
		if buildErr == nil {
			var response polyrelay.ExecuteResponse
			buildErr = c.collateralReturnHTTP().PostJSON(
				requestCtx,
				collateralReturnSubmitEndpoint,
				collateralReturnSubmitRequest{Envelope: envelope, PlanHash: plan.PlanHash},
				polyhttp.AuthNone,
				&response,
			)
			if buildErr == nil {
				return polyrelay.NewHandle(c.RelayerTransport(), response), nil
			}
		}

		if attempt == polyrelay.SubmitRetryAttempts || !polyrelay.IsRetryableSubmitError(buildErr) {
			return nil, buildErr
		}
		select {
		case <-requestCtx.Done():
			return nil, requestCtx.Err()
		case <-time.After(polyrelay.DefaultPollInterval):
		}
	}

	return nil, fmt.Errorf("collateral return submit retry loop exhausted")
}

func (c *AuthenticatedClient) collateralReturnWallet() (common.Address, error) {
	switch c.signatureType {
	case SignatureTypePolyProxy, SignatureTypePolyGnosisSafe, SignatureTypePoly1271:
	default:
		return common.Address{}, ErrCollateralReturnUnsupportedWallet
	}
	if c.funderAddress == "" {
		return common.Address{}, ErrCollateralReturnUnsupportedWallet
	}
	return common.HexToAddress(c.funderAddress), nil
}

func (c *AuthenticatedClient) collateralReturnHTTP() *polyhttp.Client {
	httpClient := *c.http.HTTPClient
	if httpClient.Timeout > 0 && httpClient.Timeout < collateralReturnRequestTimeout {
		httpClient.Timeout = collateralReturnRequestTimeout
	}
	return &polyhttp.Client{
		BaseURL:    c.collateralReturnHost,
		HTTPClient: &httpClient,
		UserAgent:  c.http.UserAgent,
		Headers:    c.relayerHeaders,
	}
}
