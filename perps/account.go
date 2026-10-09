package perps

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

const (
	accountBalancesEndpoint   = "/v1/account/balances"
	accountPortfolioEndpoint  = "/v1/account/portfolio"
	accountStatsEndpoint      = "/v1/account/stats"
	accountAutoCancelEndpoint = "/v1/account/auto-cancel"
	accountConfigEndpoint     = "/v1/account/config"
	accountOpenOrdersEndpoint = "/v1/account/open-orders"
	accountOrdersEndpoint     = "/v1/account/orders"
)

// AccountConfigParams filters account configuration by instrument.
type AccountConfigParams struct {
	InstrumentID *int
}

// OpenOrdersParams filters open orders by instrument.
type OpenOrdersParams struct {
	InstrumentID *int
}

// AccountOrdersParams filters authenticated order history.
type AccountOrdersParams struct {
	OrderID       *int
	ClientOrderID string
	InstrumentID  *int
	Start         int64
	End           int64
}

// AccountHistoryParams filters descending authenticated history pages.
type AccountHistoryParams struct {
	Start            int64
	End              int64
	InstrumentID     *int
	DepositStatus    PerpsDepositStatus
	WithdrawalStatus PerpsWithdrawalStatus
	Hash             string
	// Fills use a server trade-ID cursor; other histories use opaque SDK cursors.
	// Sort applies only to fills.
	Sort   PerpsSortDirection
	Cursor string
}

// AccountIntervalHistoryParams filters an equity or PnL history page.
type AccountIntervalHistoryParams struct {
	Start    int64
	End      int64
	Interval PerpsPnlInterval
	Cursor   string
}

// GetBalances returns the authenticated account's collateral balances.
func (c *AuthenticatedClient) GetBalances(ctx context.Context) ([]PerpsBalance, error) {
	var out []PerpsBalance
	if err := c.getAuthenticatedJSON(ctx, accountBalancesEndpoint, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetPortfolio returns the authenticated account's current portfolio.
func (c *AuthenticatedClient) GetPortfolio(ctx context.Context) (*PerpsPortfolio, error) {
	var out PerpsPortfolio
	if err := c.getAuthenticatedJSON(ctx, accountPortfolioEndpoint, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAccountStats returns the authenticated account's seven-day statistics.
func (c *AuthenticatedClient) GetAccountStats(ctx context.Context) (*PerpsAccountStats, error) {
	var out PerpsAccountStats
	if err := c.getAuthenticatedJSON(ctx, accountStatsEndpoint, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAutoCancelStatus returns the authenticated account's auto-cancel schedule
// and daily trigger counters. A zero deadline means that it is disarmed.
func (c *AuthenticatedClient) GetAutoCancelStatus(
	ctx context.Context,
) (*PerpsAutoCancelStatus, error) {
	var out PerpsAutoCancelStatus
	if err := c.getAuthenticatedJSON(ctx, accountAutoCancelEndpoint, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAccountConfig returns leverage and margin mode, optionally filtered by
// instrument.
func (c *AuthenticatedClient) GetAccountConfig(
	ctx context.Context,
	p AccountConfigParams,
) ([]PerpsAccountConfig, error) {
	query := url.Values{}
	if p.InstrumentID != nil {
		query.Set("instrument_id", strconv.Itoa(*p.InstrumentID))
	}
	var out []PerpsAccountConfig
	if err := c.getAuthenticatedJSON(ctx, accountConfigEndpoint, query, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetOpenOrders returns currently open orders, optionally filtered by
// instrument.
func (c *AuthenticatedClient) GetOpenOrders(
	ctx context.Context,
	p OpenOrdersParams,
) ([]PerpsOrder, error) {
	query := url.Values{}
	if p.InstrumentID != nil {
		query.Set("instrument_id", strconv.Itoa(*p.InstrumentID))
	}
	var out []PerpsOrder
	if err := c.getAuthenticatedJSON(ctx, accountOpenOrdersEndpoint, query, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetOrders returns authenticated order history with optional filters.
func (c *AuthenticatedClient) GetOrders(
	ctx context.Context,
	p AccountOrdersParams,
) ([]PerpsOrder, error) {
	query := url.Values{}
	if p.OrderID != nil {
		query.Set("order_id", strconv.Itoa(*p.OrderID))
	}
	if p.ClientOrderID != "" {
		query.Set("client_order_id", p.ClientOrderID)
	}
	if p.InstrumentID != nil {
		query.Set("instrument_id", strconv.Itoa(*p.InstrumentID))
	}
	if p.Start != 0 {
		query.Set("start_timestamp", strconv.FormatInt(p.Start, 10))
	}
	if p.End != 0 {
		query.Set("end_timestamp", strconv.FormatInt(p.End, 10))
	}
	var out []PerpsOrder
	if err := c.getAuthenticatedJSON(ctx, accountOrdersEndpoint, query, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetFillsPage returns one page of authenticated fills.
func (c *AuthenticatedClient) GetFillsPage(
	ctx context.Context,
	p AccountHistoryParams,
) (PerpsPage[PerpsAccountFill], error) {
	if err := validateSortDirection(p.Sort); err != nil {
		return PerpsPage[PerpsAccountFill]{}, err
	}
	var out PerpsPage[PerpsAccountFill]
	if err := c.getAuthenticatedJSON(ctx, "/v1/account/fills", fillsQuery(p), &out); err != nil {
		return PerpsPage[PerpsAccountFill]{}, err
	}
	if out.More {
		if len(out.Data) == 0 {
			return PerpsPage[PerpsAccountFill]{}, ErrPaginationNonProgress
		}
		out.NextCursor = strconv.FormatInt(out.Data[len(out.Data)-1].TradeID, 10)
		if out.NextCursor == p.Cursor {
			return PerpsPage[PerpsAccountFill]{}, ErrPaginationNonProgress
		}
	}
	return out, nil
}

// GetFundingPaymentsPage returns one page of authenticated funding payments.
func (c *AuthenticatedClient) GetFundingPaymentsPage(
	ctx context.Context,
	p AccountHistoryParams,
) (PerpsPage[PerpsAccountFundingPayment], error) {
	return descendingHistory(
		ctx,
		c,
		"/v1/account/funding",
		p,
		24*time.Hour,
		func(t PerpsAccountFundingPayment) string { return strconv.FormatInt(t.ID, 10) },
		func(t PerpsAccountFundingPayment) int64 { return t.Timestamp },
		false,
	)
}

// GetDepositsPage returns one page of authenticated collateral deposits.
func (c *AuthenticatedClient) GetDepositsPage(
	ctx context.Context,
	p AccountHistoryParams,
) (PerpsPage[PerpsDeposit], error) {
	return descendingHistory(
		ctx,
		c,
		"/v1/account/deposits",
		p,
		90*24*time.Hour,
		func(t PerpsDeposit) string { return t.Hash },
		func(t PerpsDeposit) int64 {
			return func() int64 {
				if t.ConfirmedTimestamp != 0 {
					return t.ConfirmedTimestamp
				}
				return t.CreatedTimestamp
			}()
		},
		false,
	)
}

// GetWithdrawalsPage returns one page of authenticated collateral withdrawals.
func (c *AuthenticatedClient) GetWithdrawalsPage(
	ctx context.Context,
	p AccountHistoryParams,
) (PerpsPage[PerpsWithdrawal], error) {
	return descendingHistory(
		ctx,
		c,
		"/v1/account/withdrawals",
		p,
		90*24*time.Hour,
		func(t PerpsWithdrawal) string { return strconv.Itoa(t.WithdrawalID) },
		func(t PerpsWithdrawal) int64 {
			return func() int64 {
				if t.ConfirmedTimestamp != 0 {
					return t.ConfirmedTimestamp
				}
				return t.CreatedTimestamp
			}()
		},
		false,
	)
}

// GetEquityHistoryPage returns one page of timestamped account equity.
func (c *AuthenticatedClient) GetEquityHistoryPage(
	ctx context.Context,
	p AccountIntervalHistoryParams,
) (PerpsPage[PerpsEquityPoint], error) {
	return intervalHistory(
		ctx,
		c,
		"/v1/account/equity",
		p,
		func(t PerpsEquityPoint) int64 { return t.Timestamp },
	)
}

// GetPnlHistoryPage returns one page of timestamped account PnL.
func (c *AuthenticatedClient) GetPnlHistoryPage(
	ctx context.Context,
	p AccountIntervalHistoryParams,
) (PerpsPage[PerpsPnlPoint], error) {
	return intervalHistory(
		ctx,
		c,
		"/v1/account/pnl",
		p,
		func(t PerpsPnlPoint) int64 { return t.Timestamp },
	)
}

func fillsQuery(p AccountHistoryParams) url.Values {
	query := historyQuery(p)
	if p.Sort != "" {
		query.Set("sort", string(p.Sort))
	}
	if p.Cursor != "" {
		query.Set("cursor", p.Cursor)
	}
	return query
}

func historyQuery(p AccountHistoryParams) url.Values {
	query := url.Values{}
	if p.Start != 0 {
		query.Set("start_timestamp", strconv.FormatInt(p.Start, 10))
	}
	if p.End != 0 {
		query.Set("end_timestamp", strconv.FormatInt(p.End, 10))
	}
	if p.InstrumentID != nil {
		query.Set("instrument_id", strconv.Itoa(*p.InstrumentID))
	}
	if p.DepositStatus != "" {
		query.Set("deposit_status", string(p.DepositStatus))
	}
	if p.WithdrawalStatus != "" {
		query.Set("withdrawal_status", string(p.WithdrawalStatus))
	}
	if p.Hash != "" {
		query.Set("hash", p.Hash)
	}
	return query
}

func validateSortDirection(sort PerpsSortDirection) error {
	if sort == "" || sort == PerpsSortDescending || sort == PerpsSortAscending {
		return nil
	}
	return fmt.Errorf("perps: invalid sort direction %q", sort)
}

func intervalHistoryQuery(p AccountIntervalHistoryParams) url.Values {
	query := url.Values{}
	if p.Start != 0 {
		query.Set("start_timestamp", strconv.FormatInt(p.Start, 10))
	}
	if p.End != 0 {
		query.Set("end_timestamp", strconv.FormatInt(p.End, 10))
	}
	if p.Interval != "" {
		query.Set("interval", string(p.Interval))
	}
	return query
}
