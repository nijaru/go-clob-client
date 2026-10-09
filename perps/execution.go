package perps

import (
	"context"
	"fmt"
	"net/http"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// TWAPRequest configures a server-managed run. Durations are milliseconds.
// Omitted interval is derived by the venue; zero slippage uses its default.
type TWAPRequest struct {
	InstrumentID       int
	Side               PerpsOrderSide
	Quantity           string
	DurationMs         int64
	IntervalMs         *int64
	Randomize          bool
	SlippageBps        int
	MinPrice, MaxPrice string
	ReduceOnly         bool
	ClientOrderID      string
	ExpiresAt          int64
}
type PerpsTWAP struct {
	ID             int64  `json:"twid"`
	InstrumentID   int    `json:"iid"`
	Buy            bool   `json:"buy"`
	Quantity       string `json:"qty"`
	FilledQuantity string `json:"fill"`
	DurationMs     int64  `json:"dur"`
	IntervalMs     int64  `json:"ivl"`
	Randomize      bool   `json:"rnd"`
	SlippageBps    int    `json:"slip_bps"`
	MinPrice       string `json:"min_px"`
	MaxPrice       string `json:"max_px"`
	ReduceOnly     bool   `json:"ro"`
	Status         string `json:"st"`
	Slices         int    `json:"slices"`
	SliceCount     int    `json:"slice_count"`
	StartedAt      int64  `json:"sts"`
	EndsAt         int64  `json:"ets"`
	CreatedAt      int64  `json:"cts"`
	AveragePrice   string `json:"avg_px,omitempty"`
	ClientOrderID  string `json:"coid,omitempty"`
	OrderID        *int   `json:"oid,omitempty"`
}
type TWAPAccepted struct {
	ID        int64 `json:"twid"`
	Timestamp int64 `json:"ts"`
}

type ChaseRequest struct {
	InstrumentID            int
	Side                    PerpsOrderSide
	Quantity                string
	LimitPrice, MaxDistance string
	MaxDistanceBps          *int
	// Defaults to true when nil.
	PostOnly      *bool
	ReduceOnly    bool
	ClientOrderID string
	ExpiresAt     int64
}
type PerpsChase struct {
	ID                 int64  `json:"chid"`
	InstrumentID       int    `json:"iid"`
	Buy                bool   `json:"buy"`
	Quantity           string `json:"qty"`
	FilledQuantity     string `json:"fill"`
	LimitPrice         string `json:"lim"`
	MaxDistance        string `json:"max_dist"`
	MaxDistanceBps     int    `json:"max_dist_bps"`
	PostOnly           bool   `json:"po"`
	ReduceOnly         bool   `json:"ro"`
	ReferencePrice     string `json:"reference_price"`
	Reprices           int    `json:"reprices"`
	PostOnlyRejections int    `json:"post_only_rejections"`
	CreatedAt          int64  `json:"cts"`
	ClientOrderID      string `json:"coid,omitempty"`
	OrderID            *int   `json:"oid,omitempty"`
}
type ChaseAccepted struct {
	ID        int64 `json:"chid"`
	Timestamp int64 `json:"ts"`
}

func (c *AuthenticatedClient) signedREST(
	ctx context.Context,
	method, path, name string,
	raw []any,
	args any,
	expiry int64,
	out any,
) error {
	signer, err := c.delegatedSigner()
	if err != nil {
		return err
	}
	body, err := makePerpsSignedCommand(
		ctx,
		signer,
		c.chainID,
		[]any{name, raw},
		map[string]any{"type": name, "args": args},
		expiry,
	)
	if err != nil {
		return err
	}
	return c.http.DoJSON(
		ctx,
		method,
		path,
		nil,
		body,
		polyhttp.AuthNone,
		nil,
		map[string]string{
			"POLYMARKET-PROXY":  c.credentials.Proxy,
			"POLYMARKET-SECRET": c.credentials.Secret,
		},
		out,
	)
}

func executionIdentity(id int, side PerpsOrderSide, clientID string) error {
	if !validInstrumentID(id) || side != PerpsOrderBuy && side != PerpsOrderSell {
		return fmt.Errorf("perps: invalid execution instrument or side")
	}
	if clientID != "" && !validPerpsClientOrderID(clientID) {
		return fmt.Errorf("perps: invalid client order ID")
	}
	return nil
}

func optionalBound(value string) (*string, error) {
	if value == "" {
		return nil, nil
	}
	v, err := boundedDecimal(value, false)
	return &v, err
}

// CreateTWAP is one submission attempt. An accepted run may not fill all its
// quantity. Reconcile uncertain creates by client ID through GetTWAPs.
func (c *AuthenticatedClient) CreateTWAP(
	ctx context.Context,
	p TWAPRequest,
) (*TWAPAccepted, error) {
	if err := executionIdentity(p.InstrumentID, p.Side, p.ClientOrderID); err != nil {
		return nil, err
	}
	qty, err := boundedDecimal(p.Quantity, true)
	if err != nil {
		return nil, err
	}
	if p.DurationMs < 300000 || p.DurationMs > 86400000 || p.SlippageBps < 0 ||
		p.SlippageBps > 10000 {
		return nil, fmt.Errorf("perps: invalid TWAP duration or slippage")
	}
	if p.IntervalMs != nil && *p.IntervalMs != 0 &&
		(*p.IntervalMs < 30000 || *p.IntervalMs > p.DurationMs || p.DurationMs%*p.IntervalMs != 0) {
		return nil, fmt.Errorf("perps: TWAP interval must be >=30000 and divide duration")
	}
	minimum, err := optionalBound(p.MinPrice)
	if err != nil {
		return nil, err
	}
	maximum, err := optionalBound(p.MaxPrice)
	if err != nil {
		return nil, err
	}
	if minimum != nil && maximum != nil {
		a, _ := parseFixedDecimal(*minimum, false)
		b, _ := parseFixedDecimal(*maximum, false)
		if a.Sign() != 0 && b.Sign() != 0 && a.Cmp(b) >= 0 {
			return nil, fmt.Errorf("perps: TWAP min price must be below max price")
		}
	}
	args := struct {
		IID      int     `json:"iid"`
		Buy      bool    `json:"buy"`
		Qty      string  `json:"qty"`
		Dur      int64   `json:"dur"`
		Interval *int64  `json:"ivl,omitempty"`
		Random   bool    `json:"rnd"`
		Slip     int     `json:"slip_bps"`
		Min      *string `json:"min_px,omitempty"`
		Max      *string `json:"max_px,omitempty"`
		RO       bool    `json:"ro"`
		C        string  `json:"c,omitempty"`
	}{IID: p.InstrumentID, Buy: p.Side == PerpsOrderBuy, Qty: qty, Dur: p.DurationMs, Interval: p.IntervalMs, Random: p.Randomize, Slip: p.SlippageBps, RO: p.ReduceOnly, C: p.ClientOrderID, Min: minimum, Max: maximum}
	// Compact absent raw positions exactly as the official signing grammar does.
	raw := []any{args.IID, args.Buy, qty, args.Dur}
	if p.IntervalMs != nil {
		raw = append(raw, *p.IntervalMs)
	}
	raw = append(raw, p.Randomize, p.SlippageBps)
	if minimum != nil {
		raw = append(raw, *minimum)
	}
	if maximum != nil {
		raw = append(raw, *maximum)
	}
	raw = append(raw, p.ReduceOnly)
	if p.ClientOrderID != "" {
		raw = append(raw, p.ClientOrderID)
	}
	var out struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		TWAPAccepted
	}
	if err := c.signedREST(ctx, http.MethodPost, "/v1/trade/twaps", "createTwap", raw, args, p.ExpiresAt, &out); err != nil {
		return nil, err
	}
	if err := commandRejection("createTwap", out.Status, out.Error); err != nil {
		return nil, err
	}
	if out.ID <= 0 {
		return nil, fmt.Errorf("perps: TWAP identity missing")
	}
	return &out.TWAPAccepted, nil
}

func (c *AuthenticatedClient) GetTWAPs(ctx context.Context) ([]PerpsTWAP, error) {
	var out []PerpsTWAP
	if err := c.getAuthenticatedJSON(ctx, "/v1/account/twaps", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *AuthenticatedClient) ControlTWAP(
	ctx context.Context,
	id int64,
	action string,
	expiresAt int64,
) error {
	if id <= 0 || action != "pause" && action != "resume" && action != "cancel" {
		return fmt.Errorf("perps: invalid TWAP control")
	}
	name, method := "controlTwap", http.MethodPatch
	raw := []any{id, action}
	args := map[string]any{"twid": id, "act": action}
	if action == "cancel" {
		name = "cancelTwap"
		method = http.MethodDelete
		raw = []any{id}
		args = map[string]any{"twid": id}
	}
	var ack sessionAck
	if err := c.signedREST(ctx, method, "/v1/trade/twaps", name, raw, args, expiresAt, &ack); err != nil {
		return err
	}
	return commandRejection(name, ack.Status, ack.Error)
}

func (c *AuthenticatedClient) CreateChase(
	ctx context.Context,
	p ChaseRequest,
) (*ChaseAccepted, error) {
	if err := executionIdentity(p.InstrumentID, p.Side, p.ClientOrderID); err != nil {
		return nil, err
	}
	qty, err := boundedDecimal(p.Quantity, true)
	if err != nil {
		return nil, err
	}
	limit, err := optionalBound(p.LimitPrice)
	if err != nil {
		return nil, err
	}
	distance, err := optionalBound(p.MaxDistance)
	if err != nil {
		return nil, err
	}
	if p.MaxDistanceBps != nil {
		if *p.MaxDistanceBps < 1 || *p.MaxDistanceBps > 1000 {
			return nil, fmt.Errorf("perps: chase distance must be 1-1000 bps")
		}
		if distance != nil && *distance != "0" {
			return nil, fmt.Errorf("perps: choose absolute or bps chase distance")
		}
	}
	po := true
	if p.PostOnly != nil {
		po = *p.PostOnly
	}
	raw := []any{p.InstrumentID, p.Side == PerpsOrderBuy, qty}
	args := struct {
		IID      int     `json:"iid"`
		Buy      bool    `json:"buy"`
		Qty      string  `json:"qty"`
		Limit    *string `json:"lim,omitempty"`
		Distance *string `json:"max_dist,omitempty"`
		Bps      *int    `json:"max_dist_bps,omitempty"`
		PO       bool    `json:"po"`
		RO       bool    `json:"ro"`
		C        string  `json:"c,omitempty"`
	}{p.InstrumentID, p.Side == PerpsOrderBuy, qty, limit, distance, p.MaxDistanceBps, po, p.ReduceOnly, p.ClientOrderID}
	if limit != nil {
		raw = append(raw, *limit)
	}
	if distance != nil {
		raw = append(raw, *distance)
	}
	if p.MaxDistanceBps != nil {
		raw = append(raw, *p.MaxDistanceBps)
	}
	raw = append(raw, po, p.ReduceOnly)
	if p.ClientOrderID != "" {
		raw = append(raw, p.ClientOrderID)
	}
	var out struct {
		Status string `json:"status"`
		Error  string `json:"error"`
		ChaseAccepted
	}
	if err := c.signedREST(ctx, http.MethodPost, "/v1/trade/chases", "createChase", raw, args, p.ExpiresAt, &out); err != nil {
		return nil, err
	}
	if err := commandRejection("createChase", out.Status, out.Error); err != nil {
		return nil, err
	}
	if out.ID <= 0 {
		return nil, fmt.Errorf("perps: chase identity missing")
	}
	return &out.ChaseAccepted, nil
}

func (c *AuthenticatedClient) GetChases(ctx context.Context) ([]PerpsChase, error) {
	var out []PerpsChase
	if err := c.getAuthenticatedJSON(ctx, "/v1/account/chases", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *AuthenticatedClient) CancelChase(ctx context.Context, id int64, expiresAt int64) error {
	if id <= 0 {
		return fmt.Errorf("perps: invalid chase ID")
	}
	var ack sessionAck
	if err := c.signedREST(ctx, http.MethodDelete, "/v1/trade/chases", "cancelChase", []any{id}, struct {
		ID int64 `json:"chid"`
	}{id}, expiresAt, &ack); err != nil {
		return err
	}
	return commandRejection("cancelChase", ack.Status, ack.Error)
}
