package perps

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// PerpsTPSLFields is the server's trigger and trailing-state record.
type PerpsTPSLFields struct {
	Kind            string `json:"kind"`
	Scope           string `json:"scope"`
	TriggerPrice    string `json:"trp"`
	ParentOrderID   *int   `json:"parent_oid,omitempty"`
	ArmedQuantity   string `json:"armed_qty,omitempty"`
	SlippageBps     *int   `json:"slip_bps,omitempty"`
	TrailingBps     *int   `json:"trail_bps,omitempty"`
	ActivationPrice string `json:"act,omitempty"`
	TrailingAnchor  string `json:"trail_anchor,omitempty"`
	TrailingActive  *bool  `json:"trail_active,omitempty"`
}

// TPSLTrigger is either a fixed trigger (optionally limit) or a trailing market
// stop loss. Quantity is only valid for position exits; omission closes all.
type TPSLTrigger struct {
	TriggerPrice    string
	LimitPrice      string
	TrailingBps     int
	ActivationPrice string
	Quantity        string
}

func ensureClientOrderID(order *PerpsOrderRequest) error {
	if order.ClientOrderID != "" {
		if !validPerpsClientOrderID(order.ClientOrderID) {
			return fmt.Errorf("perps: invalid client order ID")
		}
		return nil
	}
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return err
	}
	order.ClientOrderID = hex.EncodeToString(bytes[:])
	return nil
}

type triggerOrder struct {
	instrumentID   int
	buy            bool
	quantity, kind string
	trigger        TPSLTrigger
}

func (t triggerOrder) wire() ([]any, map[string]any, error) {
	tr := t.trigger
	body := map[string]any{
		"iid": t.instrumentID,
		"buy": t.buy,
		"qty": t.quantity,
		"po":  false,
		"ro":  true,
	}
	raw := []any{t.instrumentID, t.buy}
	triggerRaw := []any{}
	triggerBody := map[string]any{"tpsl": t.kind}
	if tr.TrailingBps != 0 {
		if t.kind != "sl" || tr.TrailingBps < 10 || tr.TrailingBps > 2000 ||
			tr.TriggerPrice != "" ||
			tr.LimitPrice != "" {
			return nil, nil, fmt.Errorf(
				"perps: trailing market stop requires 10-2000 bps and no fixed trigger/limit",
			)
		}
		triggerRaw = []any{true, t.kind, tr.TrailingBps}
		triggerBody["market"] = true
		triggerBody["trail_bps"] = tr.TrailingBps
		if tr.ActivationPrice != "" {
			value, err := boundedDecimal(tr.ActivationPrice, true)
			if err != nil {
				return nil, nil, err
			}
			triggerRaw = append(triggerRaw, value)
			triggerBody["act"] = value
		}
	} else {
		if tr.ActivationPrice != "" {
			return nil, nil, fmt.Errorf("perps: activation price requires trailing stop")
		}
		if _, err := parseFixedDecimal(tr.TriggerPrice, true); err != nil {
			return nil, nil, err
		}
		if tr.LimitPrice == "" {
			triggerRaw = append(triggerRaw, true)
			triggerBody["market"] = true
		} else {
			if _, err := parseFixedDecimal(tr.LimitPrice, true); err != nil {
				return nil, nil, err
			}
			raw = append(raw, tr.LimitPrice)
			body["p"] = tr.LimitPrice
		}
		triggerRaw = append(triggerRaw, tr.TriggerPrice, t.kind)
		triggerBody["trp"] = tr.TriggerPrice
	}
	raw = append(raw, t.quantity, false, true, triggerRaw)
	body["tr"] = triggerBody
	return raw, body, nil
}

func appendBuilder(raw *[]any, body map[string]any, builder *PerpsBuilderTerms) {
	if builder != nil {
		*raw = append(*raw, []any{builder.Address, builder.FeeRate})
		body["builder"] = *builder
	}
}

type OrderWithTPSLRequest struct {
	Order                PerpsOrderRequest
	TakeProfit, StopLoss *TPSLTrigger
	ExpiresAt            int64
	WaitTimeout          time.Duration
}
type OrderWithTPSLResult struct {
	Order            *PerpsOrder
	ClientOrderID    string
	Acknowledgements []PerpsOrderAck
}

// PlaceOrderWithTPSL submits an atomic order-scoped group. On rejection or
// uncertain update delivery, the returned acknowledgements retain accepted IDs
// for reconciliation. Submission is never retried.
func (s *Session) PlaceOrderWithTPSL(
	ctx context.Context,
	p OrderWithTPSLRequest,
) (OrderWithTPSLResult, error) {
	if p.TakeProfit == nil && p.StopLoss == nil {
		return OrderWithTPSLResult{}, fmt.Errorf("perps: TP or SL required")
	}
	triggers := []triggerOrder{}
	for _, leg := range []struct {
		kind    string
		trigger *TPSLTrigger
	}{{"tp", p.TakeProfit}, {"sl", p.StopLoss}} {
		if leg.trigger != nil {
			if leg.trigger.Quantity != "" {
				return OrderWithTPSLResult{}, fmt.Errorf("perps: entry exits use entry quantity")
			}
			triggers = append(
				triggers,
				triggerOrder{
					p.Order.InstrumentID,
					p.Order.Side == PerpsOrderSell,
					p.Order.Quantity,
					leg.kind,
					*leg.trigger,
				},
			)
		}
	}
	if p.WaitTimeout < 0 {
		return OrderWithTPSLResult{}, fmt.Errorf("perps: wait timeout must be positive")
	}
	if p.WaitTimeout == 0 {
		p.WaitTimeout = 2 * time.Second
	}
	if err := ensureClientOrderID(&p.Order); err != nil {
		return OrderWithTPSLResult{}, err
	}
	watch, err := s.watchOrder(p.Order.ClientOrderID)
	if err != nil {
		return OrderWithTPSLResult{}, err
	}
	defer s.unwatchOrder(watch)
	acks, err := s.postOrderGroup(ctx, []PerpsOrderRequest{p.Order}, triggers, "order", p.ExpiresAt)
	result := OrderWithTPSLResult{ClientOrderID: p.Order.ClientOrderID, Acknowledgements: acks}
	if err != nil {
		return result, err
	}
	for _, ack := range acks {
		if err := validatePerpsPostOrderAck(ack); err != nil {
			return result, err
		}
	}
	waitCtx, cancel := context.WithTimeout(ctx, p.WaitTimeout)
	defer cancel()
	update, err := s.waitWatchedOrder(waitCtx, watch, acks[0].OrderID)
	if err != nil {
		return result, err
	}
	order := update.order()
	result.Order = &order
	return result, nil
}

type PositionTPSLRequest struct {
	InstrumentID         int
	TakeProfit, StopLoss *TPSLTrigger
	ExpiresAt            int64
}

// PostPositionTPSL reads the position to determine exit direction. Fixed partial
// quantities are clamped by the engine at trigger time; omission sends zero to
// close the whole live position. The read does not reserve the position.
func (s *Session) PostPositionTPSL(
	ctx context.Context,
	p PositionTPSLRequest,
) ([]PerpsOrderAck, error) {
	if !validInstrumentID(p.InstrumentID) || p.TakeProfit == nil && p.StopLoss == nil {
		return nil, fmt.Errorf("perps: instrument and TP or SL required")
	}
	portfolio, err := s.client.GetPortfolio(ctx)
	if err != nil {
		return nil, err
	}
	found := false
	buy := false
	for _, position := range portfolio.Positions {
		if position.InstrumentID == p.InstrumentID {
			size, err := parseFixedDecimal(position.Size, false)
			if err != nil {
				return nil, err
			}
			if size.Sign() == 0 {
				break
			}
			buy = size.Sign() < 0
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("perps: no open position for instrument")
	}
	triggers := []triggerOrder{}
	for _, leg := range []struct {
		kind    string
		trigger *TPSLTrigger
	}{{"tp", p.TakeProfit}, {"sl", p.StopLoss}} {
		if leg.trigger != nil {
			if leg.trigger.LimitPrice != "" {
				return nil, fmt.Errorf("perps: position exits are market triggers")
			}
			quantity := "0"
			if leg.trigger.Quantity != "" {
				quantity, err = boundedDecimal(leg.trigger.Quantity, true)
				if err != nil {
					return nil, err
				}
			}
			triggers = append(
				triggers,
				triggerOrder{p.InstrumentID, buy, quantity, leg.kind, *leg.trigger},
			)
		}
	}
	acks, err := s.postOrderGroup(ctx, nil, triggers, "position", p.ExpiresAt)
	if err != nil {
		return acks, err
	}
	for _, ack := range acks {
		if err := validatePerpsPostOrderAck(ack); err != nil {
			return acks, err
		}
	}
	return acks, nil
}

type LeverageUpdate struct {
	InstrumentID int
	Leverage     int
	Cross        bool
}

// UpdateLeverages returns one result per input, including individual rejections.
func (s *Session) UpdateLeverages(
	ctx context.Context,
	updates []LeverageUpdate,
) ([]PerpsLeverageResult, error) {
	if len(updates) == 0 || len(updates) > 100 {
		return nil, fmt.Errorf("perps: leverage batch must contain 1-100 updates")
	}
	seen := map[int]bool{}
	raw := []any{}
	args := []any{}
	for _, u := range updates {
		if !validInstrumentID(u.InstrumentID) || u.Leverage <= 0 ||
			uint64(u.Leverage) > 4294967295 ||
			seen[u.InstrumentID] {
			return nil, fmt.Errorf("perps: invalid or duplicate leverage update")
		}
		seen[u.InstrumentID] = true
		raw = append(raw, []any{u.InstrumentID, u.Leverage, u.Cross})
		args = append(args, struct {
			IID      int  `json:"iid"`
			Leverage int  `json:"lev"`
			Cross    bool `json:"cross"`
		}{u.InstrumentID, u.Leverage, u.Cross})
	}
	data, err := s.sendSignedCommand(
		ctx,
		[]any{"updateLeverages", raw},
		map[string]any{"type": "updateLeverages", "args": args},
	)
	if err != nil {
		return nil, err
	}
	var out []PerpsLeverageResult
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if len(out) != len(updates) {
		return nil, fmt.Errorf("perps: leverage result count mismatch")
	}
	for i, result := range out {
		if result.InstrumentID != updates[i].InstrumentID ||
			result.Status != "ok" && result.Status != "err" {
			return nil, fmt.Errorf("perps: invalid leverage result")
		}
	}
	return out, nil
}
