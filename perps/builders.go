package perps

import (
	"context"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
)

type PerpsBuilderTerms struct {
	Address string `json:"address"`
	FeeRate string `json:"fee_rate"`
}
type PerpsBuilderStatus struct {
	Address          string `json:"address"`
	Registered       bool   `json:"registered"`
	Enabled          bool   `json:"enabled"`
	AdmissionEnabled bool   `json:"admission_enabled"`
	MaxFeeRate       string `json:"max_fee_rate"`
}
type PerpsBuilderApproval struct {
	Trader          string `json:"trader"`
	Builder         string `json:"builder"`
	MaxFeeRate      string `json:"max_fee_rate"`
	ApprovalVersion int64  `json:"approval_version"`
	Timestamp       int64  `json:"timestamp"`
	Sequence        int64  `json:"sequence"`
}
type PerpsBuilderEarning struct {
	EarningID     int64  `json:"earning_id"`
	TradeID       int64  `json:"trade_id"`
	OrderID       int    `json:"order_id"`
	InstrumentID  int    `json:"instrument_id"`
	Trader        string `json:"trader"`
	Buy           bool   `json:"buy"`
	Price         string `json:"price"`
	Quantity      string `json:"quantity"`
	ClientOrderID string `json:"client_order_id,omitempty"`
	LiquidityRole string `json:"side"`
	Timestamp     int64  `json:"timestamp"`
	Sequence      int64  `json:"sequence"`
	Notional      string `json:"notional"`
	FeeAsset      string `json:"fee_asset"`
	Fee           string `json:"fee"`
	BuilderFee    string `json:"builder_fee"`
	TotalFee      string `json:"total_fee"`
	FeeRate       string `json:"fee_rate"`
}
type BuilderReportingWindow struct {
	Start        int64 `json:"start_timestamp"`
	End          int64 `json:"end_timestamp"`
	AsOfSequence int64 `json:"as_of_sequence"`
}
type BuilderReportingParams struct {
	Start, End, AsOfSequence *int64
	Cursor                   string
}
type BuilderEarningsPage struct {
	BuilderReportingWindow
	Data   []PerpsBuilderEarning `json:"data"`
	More   bool                  `json:"more"`
	Cursor string                `json:"cursor,omitempty"`
}
type BuilderEarningsAsset struct {
	FeeAsset   string `json:"fee_asset"`
	FillCount  int64  `json:"fill_count"`
	Notional   string `json:"notional"`
	BuilderFee string `json:"builder_fee"`
}
type BuilderEarningsSummary struct {
	BuilderReportingWindow
	Data                []BuilderEarningsAsset `json:"data"`
	TraderCount         int64                  `json:"trader_count"`
	ActiveApprovalCount int64                  `json:"active_approval_count"`
}

func (c *Client) GetBuilderStatus(
	ctx context.Context,
	address string,
) (*PerpsBuilderStatus, error) {
	if !common.IsHexAddress(address) {
		return nil, fmt.Errorf("perps: invalid builder address")
	}
	var out PerpsBuilderStatus
	if err := c.getJSON(ctx, "/v1/info/builder", url.Values{"address": {address}}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *AuthenticatedClient) GetBuilderApprovals(
	ctx context.Context,
	builder string,
) ([]PerpsBuilderApproval, error) {
	q := url.Values{}
	if builder != "" {
		if !common.IsHexAddress(builder) {
			return nil, fmt.Errorf("perps: invalid builder address")
		}
		q.Set("builder", builder)
	}
	var out struct {
		Data []PerpsBuilderApproval `json:"data"`
	}
	if err := c.getAuthenticatedJSON(ctx, "/v1/account/builder-approvals", q, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// ApproveBuilderFee grants durable owner consent. It does not open a session or
// enable attribution. Revocation is a replacement approval with maxFeeRate "0".
func (c *OwnerClient) ApproveBuilderFee(
	ctx context.Context,
	client *AuthenticatedClient,
	builder, maxFeeRate string,
) (*PerpsBuilderApproval, error) {
	c.consentMu.Lock()
	defer c.consentMu.Unlock()
	if client == nil || !common.IsHexAddress(builder) {
		return nil, fmt.Errorf("perps: client and builder required")
	}
	if err := validateBuilderFeeRate(maxFeeRate); err != nil {
		return nil, err
	}
	rate := maxFeeRate
	info, err := client.GetCredentials(ctx)
	if err != nil {
		return nil, err
	}
	if !common.IsHexAddress(info.Address) ||
		common.HexToAddress(info.Address) != c.signer.Address() {
		return nil, fmt.Errorf("perps: builder consent owner mismatch")
	}
	approvals, err := client.GetBuilderApprovals(ctx, builder)
	if err != nil {
		return nil, err
	}
	version := int64(1)
	for _, approval := range approvals {
		if common.HexToAddress(approval.Builder) == common.HexToAddress(builder) {
			version = approval.ApprovalVersion + 1
		}
	}
	if version <= 0 {
		return nil, fmt.Errorf("perps: invalid builder approval version")
	}
	body, err := c.signedOperation(ctx, "approveBuilder", []any{builder, rate, version}, struct {
		Builder    string `json:"builder"`
		MaxFeeRate string `json:"max_fee_rate"`
		Version    int64  `json:"approval_version"`
	}{builder, rate, version})
	if err != nil {
		return nil, err
	}
	var out PerpsBuilderApproval
	if err := c.submit(ctx, http.MethodPost, "/v1/account/builder-approvals", body, &out); err != nil {
		return nil, err
	}
	if out.ApprovalVersion != version || !common.IsHexAddress(out.Builder) ||
		common.HexToAddress(out.Builder) != common.HexToAddress(builder) {
		return nil, fmt.Errorf("perps: builder approval response mismatch")
	}
	return &out, nil
}

// resolveBuilder never grants consent and clamps to the saved owner-approved maximum.
func (c *AuthenticatedClient) resolveBuilder(
	ctx context.Context,
	address string,
) (*PerpsBuilderTerms, error) {
	if address == "" {
		return nil, nil
	}
	status, err := c.GetBuilderStatus(ctx, address)
	if err != nil {
		return nil, err
	}
	if !status.Registered || !status.Enabled || !status.AdmissionEnabled {
		return nil, fmt.Errorf("perps: builder is unavailable")
	}
	approvals, err := c.GetBuilderApprovals(ctx, address)
	if err != nil {
		return nil, err
	}
	approved := "0"
	for _, a := range approvals {
		if common.HexToAddress(a.Builder) == common.HexToAddress(address) {
			approved = a.MaxFeeRate
		}
	}
	if err := validateBuilderFeeRate(status.MaxFeeRate); err != nil {
		return nil, err
	}
	if err := validateBuilderFeeRate(approved); err != nil {
		return nil, err
	}
	cap, err := parseFixedDecimal(status.MaxFeeRate, false)
	if err != nil || cap.Sign() < 0 {
		return nil, fmt.Errorf("perps: invalid builder fee cap")
	}
	maximum, err := parseFixedDecimal(approved, false)
	if err != nil || maximum.Sign() < 0 {
		return nil, fmt.Errorf("perps: invalid builder approval rate")
	}
	rate := approved
	if cap.Cmp(maximum) < 0 {
		rate = status.MaxFeeRate
	}
	r, _ := parseFixedDecimal(rate, false)
	if r.Sign() == 0 {
		return nil, nil
	}
	return &PerpsBuilderTerms{Address: address, FeeRate: rate}, nil
}

// builderTerms captures immutable attribution once for an entire signed batch.
func (s *Session) builderTerms() *PerpsBuilderTerms {
	s.builderMu.RLock()
	defer s.builderMu.RUnlock()
	if s.builder == nil {
		return nil
	}
	copy := *s.builder
	return &copy
}

// RefreshBuilder rechecks availability and saved consent. Empty address disables
// attribution locally without revoking durable consent.
func (s *Session) RefreshBuilder(ctx context.Context, address string) error {
	terms, err := s.client.resolveBuilder(ctx, address)
	if err != nil {
		return err
	}
	s.builderMu.Lock()
	s.builder = terms
	s.builderAddress = address
	s.builderMu.Unlock()
	return nil
}

type BuilderConsentRequest struct{ BuilderAddress, MaxFeeRate string }

// ApproveBuilderFee grants consent with an explicit owner, then refreshes this
// session's attribution. Other sessions must refresh their own captured terms.
func (s *Session) ApproveBuilderFee(
	ctx context.Context,
	owner *OwnerClient,
	p BuilderConsentRequest,
) (*PerpsBuilderApproval, error) {
	if owner == nil {
		return nil, fmt.Errorf("perps: owner client required")
	}
	address := p.BuilderAddress
	if address == "" {
		s.builderMu.RLock()
		address = s.builderAddress
		s.builderMu.RUnlock()
	}
	if address == "" {
		return nil, fmt.Errorf("perps: builder address required")
	}
	approval, err := owner.ApproveBuilderFee(ctx, s.client, address, p.MaxFeeRate)
	if err != nil {
		return nil, err
	}
	rate, err := parseFixedDecimal(approval.MaxFeeRate, false)
	if err != nil {
		return approval, err
	}
	if rate.Sign() == 0 {
		s.builderMu.Lock()
		if common.HexToAddress(s.builderAddress) == common.HexToAddress(address) {
			s.builder = nil
			s.builderAddress = ""
		}
		s.builderMu.Unlock()
		return approval, nil
	}
	return approval, s.RefreshBuilder(ctx, address)
}

func (s *Session) RevokeBuilderFee(
	ctx context.Context,
	owner *OwnerClient,
	address string,
) (*PerpsBuilderApproval, error) {
	return s.ApproveBuilderFee(
		ctx,
		owner,
		BuilderConsentRequest{BuilderAddress: address, MaxFeeRate: "0"},
	)
}

func builderReportingQuery(p BuilderReportingParams) (url.Values, error) {
	q := url.Values{}
	if p.Cursor != "" {
		q.Set("cursor", p.Cursor)
		return q, nil
	}
	for key, value := range map[string]*int64{"start_timestamp": p.Start, "end_timestamp": p.End, "as_of_sequence": p.AsOfSequence} {
		if value != nil {
			if *value < 0 {
				return nil, fmt.Errorf("perps: reporting bounds must be nonnegative")
			}
			q.Set(key, strconv.FormatInt(*value, 10))
		}
	}
	if p.Start != nil && p.End != nil && (*p.End < *p.Start || *p.End-*p.Start > 90*24*60*60*1000) {
		return nil, fmt.Errorf("perps: invalid reporting window")
	}
	return q, nil
}

func (c *AuthenticatedClient) GetBuilderEarningsPage(
	ctx context.Context,
	p BuilderReportingParams,
) (BuilderEarningsPage, error) {
	q, err := builderReportingQuery(p)
	if err != nil {
		return BuilderEarningsPage{}, err
	}
	var out BuilderEarningsPage
	if err := c.getAuthenticatedJSON(ctx, "/v1/account/builder-earnings", q, &out); err != nil {
		return BuilderEarningsPage{}, err
	}
	if out.More && (out.Cursor == "" || out.Cursor == p.Cursor) {
		return BuilderEarningsPage{}, ErrPaginationNonProgress
	}
	return out, nil
}

func (c *AuthenticatedClient) IterBuilderEarnings(
	ctx context.Context,
	p BuilderReportingParams,
) iter.Seq2[[]PerpsBuilderEarning, error] {
	return func(yield func([]PerpsBuilderEarning, error) bool) {
		seen := map[string]bool{}
		for {
			out, err := c.GetBuilderEarningsPage(ctx, p)
			if err != nil {
				yield(nil, err)
				return
			}
			if !yield(out.Data, nil) || !out.More {
				return
			}
			if seen[out.Cursor] {
				yield(nil, ErrPaginationNonProgress)
				return
			}
			seen[out.Cursor] = true
			p.Cursor = out.Cursor
		}
	}
}

func (c *AuthenticatedClient) GetBuilderEarningsSummary(
	ctx context.Context,
	p BuilderReportingParams,
) (*BuilderEarningsSummary, error) {
	if p.Cursor != "" {
		return nil, fmt.Errorf("perps: summary cannot use a cursor")
	}
	q, err := builderReportingQuery(p)
	if err != nil {
		return nil, err
	}
	var out BuilderEarningsSummary
	if err := c.getAuthenticatedJSON(ctx, "/v1/account/builder-earnings-summary", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
