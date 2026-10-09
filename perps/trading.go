package perps

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// PerpsOrderSide is the direction of an authenticated order.
type PerpsOrderSide string

const (
	PerpsOrderBuy  PerpsOrderSide = "buy"
	PerpsOrderSell PerpsOrderSide = "sell"
)

// PerpsOrderRequest describes an entry order. Order expiry is distinct from
// the signed command deadline. Builder attribution comes from SessionConfig.
type PerpsOrderRequest struct {
	InstrumentID  int
	Side          PerpsOrderSide
	Price         string
	Quantity      string
	TimeInForce   PerpsTimeInForce
	PostOnly      bool
	ReduceOnly    bool
	ClientOrderID string
	GTDExpiry     int64
}

// PerpsOrderAck is the acknowledgement returned by createOrders.
type PerpsOrderAck struct {
	Status         string `json:"status"`
	OrderID        int    `json:"oid,omitempty"`
	ClientOrderID  string `json:"coid,omitempty"`
	Error          string `json:"error,omitempty"`
	orderIDPresent bool
	Builder        *PerpsBuilderTerms `json:"builder,omitempty"`
}

func (a *PerpsOrderAck) UnmarshalJSON(data []byte) error {
	type alias PerpsOrderAck
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*a = PerpsOrderAck(value)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	orderID, ok := fields["oid"]
	a.orderIDPresent = ok && !bytes.Equal(bytes.TrimSpace(orderID), []byte("null"))
	return nil
}

// PerpsCancelResult is the acknowledgement returned by a cancel command.
type PerpsCancelResult struct {
	Status          string `json:"status"`
	OrderID         int    `json:"oid,omitempty"`
	ClientOrderID   string `json:"coid,omitempty"`
	Error           string `json:"error,omitempty"`
	orderIDPresent  bool
	clientIDPresent bool
}

// PerpsLeverageResult is the acknowledgement returned by updateLeverage.
type PerpsLeverageResult struct {
	Status       string `json:"status"`
	InstrumentID int    `json:"instrument_id"`
	Leverage     int    `json:"leverage"`
	Cross        bool   `json:"cross"`
	Error        string `json:"error,omitempty"`
}

type perpsOrderUpdate struct {
	ID               int                `json:"oid"`
	InstrumentID     int                `json:"iid"`
	Buy              bool               `json:"buy"`
	Price            string             `json:"p"`
	Quantity         string             `json:"qty"`
	TimeInForce      PerpsTimeInForce   `json:"tif"`
	PostOnly         bool               `json:"po"`
	ReduceOnly       bool               `json:"ro"`
	Status           PerpsOrderStatus   `json:"status"`
	RestingQuantity  string             `json:"rest"`
	FilledQuantity   string             `json:"fill"`
	CreatedTimestamp int64              `json:"cts"`
	UpdatedTimestamp int64              `json:"uts"`
	ClientOrderID    string             `json:"coid,omitempty"`
	TPSL             *PerpsTPSLFields   `json:"tpsl,omitempty"`
	ChaseID          *int64             `json:"chid,omitempty"`
	Builder          *PerpsBuilderTerms `json:"builder,omitempty"`
}

func (u perpsOrderUpdate) order() PerpsOrder {
	return PerpsOrder{
		ID:               u.ID,
		InstrumentID:     u.InstrumentID,
		Buy:              u.Buy,
		Price:            u.Price,
		Quantity:         u.Quantity,
		TimeInForce:      u.TimeInForce,
		PostOnly:         u.PostOnly,
		ReduceOnly:       u.ReduceOnly,
		Status:           u.Status,
		RestingQuantity:  u.RestingQuantity,
		FilledQuantity:   u.FilledQuantity,
		CreatedTimestamp: u.CreatedTimestamp,
		UpdatedTimestamp: u.UpdatedTimestamp,
		ClientOrderID:    u.ClientOrderID,
		TPSL:             u.TPSL, ChaseID: u.ChaseID, Builder: u.Builder,
	}
}

// OrderPlacementError retains submission identity when acknowledgement or update
// delivery fails. The outcome may be unknown; reconcile with GetOrders before
// resubmitting, using ClientOrderID or the accepted order IDs.
type OrderPlacementError struct {
	Cause            error
	ClientOrderID    string
	Acknowledgements []PerpsOrderAck
}

func (e *OrderPlacementError) Error() string {
	return fmt.Sprintf("perps: place order %s: %v", e.ClientOrderID, e.Cause)
}
func (e *OrderPlacementError) Unwrap() error { return e.Cause }

// PlaceOrder submits one entry order and waits for its first authenticated
// orders update. Use PostOrders when the acknowledgement is sufficient or when
// submitting a batch. TP/SL order groups remain a separate API.
func (s *Session) PlaceOrder(
	ctx context.Context,
	order PerpsOrderRequest,
	expiresAt int64,
) (*PerpsOrder, error) {
	if err := ensureClientOrderID(&order); err != nil {
		return nil, err
	}
	watch, err := s.watchOrder(order.ClientOrderID)
	if err != nil {
		return nil, err
	}
	defer s.unwatchOrder(watch)
	acknowledgements, err := s.PostOrders(ctx, []PerpsOrderRequest{order}, expiresAt)
	placementError := func(cause error) error {
		return &OrderPlacementError{
			Cause:            cause,
			ClientOrderID:    order.ClientOrderID,
			Acknowledgements: acknowledgements,
		}
	}
	if err != nil {
		return nil, placementError(err)
	}
	if len(acknowledgements) != 1 {
		return nil, placementError(
			fmt.Errorf("expected one order acknowledgement, got %d", len(acknowledgements)),
		)
	}
	acknowledgement := acknowledgements[0]
	if err := validatePerpsPostOrderAck(acknowledgement); err != nil {
		return nil, fmt.Errorf("perps: %w", err)
	}
	waitContext, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	update, err := s.waitWatchedOrder(waitContext, watch, acknowledgement.OrderID)
	if err != nil {
		return nil, placementError(fmt.Errorf("wait for order update: %w", err))
	}
	orderResult := update.order()
	return &orderResult, nil
}

// PostOrders signs and submits one or more entry orders over the authenticated
// Perps WebSocket session.
func (s *Session) PostOrders(
	ctx context.Context,
	orders []PerpsOrderRequest,
	expiresAt int64,
) ([]PerpsOrderAck, error) {
	if len(orders) == 0 {
		return nil, fmt.Errorf("perps: at least one order is required")
	}
	return s.postOrderGroup(ctx, orders, nil, "", expiresAt)
}

func (s *Session) postOrderGroup(
	ctx context.Context,
	orders []PerpsOrderRequest,
	triggers []triggerOrder,
	group string,
	expiresAt int64,
) ([]PerpsOrderAck, error) {
	builder := s.builderTerms()
	rawOrders := make([]any, len(orders))
	bodyOrders := make([]any, len(orders))
	for i, order := range orders {
		raw, body, err := perpsOrderWire(order)
		if err != nil {
			return nil, err
		}
		appendBuilder(&raw, body, builder)
		rawOrders[i] = raw
		bodyOrders[i] = body
	}
	for _, trigger := range triggers {
		raw, body, err := trigger.wire()
		if err != nil {
			return nil, err
		}
		appendBuilder(&raw, body, builder)
		rawOrders = append(rawOrders, raw)
		bodyOrders = append(bodyOrders, body)
	}
	op := []any{"createOrders", rawOrders}
	bodyOp := map[string]any{"type": "createOrders", "args": bodyOrders}
	if group != "" {
		op = append(op, group)
		bodyOp["grp"] = group
	}
	data, err := s.sendSignedCommand(ctx, op, bodyOp, expiresAt)
	if err != nil {
		return nil, err
	}
	var acknowledgements []PerpsOrderAck
	if err := json.Unmarshal(data, &acknowledgements); err != nil {
		return nil, fmt.Errorf("perps: decode order acknowledgement: %w", err)
	}
	if len(acknowledgements) != len(rawOrders) {
		return acknowledgements, fmt.Errorf("perps: order acknowledgement count mismatch")
	}
	for _, ack := range acknowledgements {
		if ack.Status == "ok" && !ack.orderIDPresent {
			return acknowledgements, fmt.Errorf(
				"perps: successful acknowledgement missing order ID",
			)
		}
		if ack.Status != "ok" && ack.Status != "err" {
			return acknowledgements, fmt.Errorf("perps: invalid order acknowledgement status")
		}
	}
	return acknowledgements, nil
}

// CancelOrders cancels orders by numeric order ID.
func (s *Session) CancelOrders(
	ctx context.Context,
	orderIDs []int,
	expiresAt int64,
) ([]PerpsCancelResult, error) {
	if len(orderIDs) == 0 {
		return nil, fmt.Errorf("perps: at least one order ID is required")
	}
	for _, orderID := range orderIDs {
		if orderID < 0 {
			return nil, fmt.Errorf("perps: order ID must be non-negative")
		}
	}
	return s.CancelOrdersWithRetry(
		ctx,
		CancelOrdersRequest{OrderIDs: orderIDs, ExpiresAt: expiresAt},
	)
}

// CancelOrdersByClientID cancels orders by caller-supplied client ID.
func (s *Session) CancelOrdersByClientID(
	ctx context.Context,
	clientOrderIDs []string,
	expiresAt int64,
) ([]PerpsCancelResult, error) {
	if len(clientOrderIDs) == 0 {
		return nil, fmt.Errorf("perps: at least one client order ID is required")
	}
	for _, clientOrderID := range clientOrderIDs {
		if !validPerpsClientOrderID(clientOrderID) {
			return nil, fmt.Errorf(
				"perps: client order ID must be 32 lowercase hexadecimal characters",
			)
		}
	}
	return s.CancelOrdersWithRetry(
		ctx,
		CancelOrdersRequest{ClientOrderIDs: clientOrderIDs, ExpiresAt: expiresAt},
	)
}

// UpdateLeverage signs and submits a leverage/margin-mode update.
func (s *Session) UpdateLeverage(
	ctx context.Context,
	instrumentID, leverage int,
	cross bool,
) (*PerpsLeverageResult, error) {
	if !validInstrumentID(instrumentID) {
		return nil, fmt.Errorf("perps: invalid instrument ID")
	}
	if leverage <= 0 || uint64(leverage) > 4294967295 {
		return nil, fmt.Errorf("perps: leverage must be positive")
	}
	op := []any{"updateLeverage", []any{instrumentID, leverage, cross}}
	data, err := s.sendSignedCommand(ctx, op, map[string]any{
		"type": "updateLeverage",
		"args": map[string]any{"iid": instrumentID, "lev": leverage, "cross": cross},
	})
	if err != nil {
		return nil, err
	}
	var result PerpsLeverageResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("perps: decode leverage acknowledgement: %w", err)
	}
	if err := validatePerpsLeverageResult(result); err != nil {
		return nil, fmt.Errorf("perps: %w", err)
	}
	return &result, nil
}

// UpdateMargin adjusts isolated margin for an instrument. Positive amounts add
// margin and negative amounts remove it.
func (s *Session) UpdateMargin(
	ctx context.Context,
	instrumentID int,
	amount string,
) error {
	if !validInstrumentID(instrumentID) {
		return fmt.Errorf("perps: invalid instrument ID")
	}
	normalizedAmount, err := normalizePerpsDecimal(amount)
	if err != nil {
		return err
	}
	op := []any{"updateMargin", []any{instrumentID, normalizedAmount}}
	data, err := s.sendSignedCommand(ctx, op, map[string]any{
		"type": "updateMargin",
		"args": map[string]any{"iid": instrumentID, "amt": normalizedAmount},
	})
	if err != nil {
		return err
	}
	var ack sessionAck
	if err := json.Unmarshal(data, &ack); err != nil {
		return fmt.Errorf("perps: decode margin acknowledgement: %w", err)
	}
	if ack.Status != "ok" {
		if ack.Error == "" {
			ack.Error = "margin update rejected"
		}
		return fmt.Errorf("perps: %s", ack.Error)
	}
	return nil
}

func normalizePerpsDecimal(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("perps: margin amount is required")
	}
	decimal, err := parseFixedDecimal(value, false)
	if err != nil {
		return "", fmt.Errorf("perps: invalid margin amount: %w", err)
	}
	if decimal.Sign() == 0 {
		return "0", nil
	}
	negative := strings.HasPrefix(value, "-")
	value = strings.TrimPrefix(value, "-")
	whole, fraction, _ := strings.Cut(value, ".")
	whole = strings.TrimLeft(whole, "0")
	if whole == "" {
		whole = "0"
	}
	fraction = strings.TrimRight(fraction, "0")
	if fraction != "" {
		whole += "." + fraction
	}
	if negative {
		whole = "-" + whole
	}
	return whole, nil
}

func validatePerpsPostOrderAck(ack PerpsOrderAck) error {
	if ack.Status != "ok" {
		if ack.Error == "" {
			ack.Error = "order rejected"
		}
		return &CommandError{Operation: "createOrders", Code: ack.Error}
	}
	if !ack.orderIDPresent {
		return fmt.Errorf("successful order acknowledgement missing order ID")
	}
	return nil
}

func validatePerpsLeverageResult(result PerpsLeverageResult) error {
	if result.Status == "ok" {
		return nil
	}
	if result.Error == "" {
		return fmt.Errorf("leverage update rejected")
	}
	return fmt.Errorf("%s", result.Error)
}

func (s *Session) sendSignedCommand(
	ctx context.Context,
	op []any,
	bodyOp map[string]any,
	expiresAt ...int64,
) (json.RawMessage, error) {
	var expiry int64
	if len(expiresAt) > 0 {
		expiry = expiresAt[0]
	}
	body, err := makePerpsSignedCommand(s.signer, s.chainID, op, bodyOp, expiry)
	if err != nil {
		return nil, err
	}
	body["req"] = "post"
	return s.sendCommand(ctx, body)
}

func perpsOrderWire(order PerpsOrderRequest) ([]any, map[string]any, error) {
	if order.InstrumentID < 0 || order.Quantity == "" || order.TimeInForce == "" {
		return nil, nil, fmt.Errorf("perps: instrument, quantity, and time-in-force are required")
	}
	if order.Side != PerpsOrderBuy && order.Side != PerpsOrderSell {
		return nil, nil, fmt.Errorf("perps: invalid order side %q", order.Side)
	}
	if order.TimeInForce != PerpsTIFGTC &&
		order.TimeInForce != PerpsTIFIOC &&
		order.TimeInForce != PerpsTIFFOK && order.TimeInForce != PerpsTIFGTD {
		return nil, nil, fmt.Errorf("perps: invalid time-in-force %q", order.TimeInForce)
	}
	if !validInstrumentID(order.InstrumentID) {
		return nil, nil, fmt.Errorf("perps: invalid instrument ID")
	}
	if _, err := parseFixedDecimal(order.Quantity, true); err != nil {
		return nil, nil, err
	}
	if order.Price != "" {
		if _, err := parseFixedDecimal(order.Price, true); err != nil {
			return nil, nil, err
		}
	}
	if (order.TimeInForce == PerpsTIFGTC || order.TimeInForce == PerpsTIFGTD) && order.Price == "" {
		return nil, nil, fmt.Errorf("perps: resting orders require a price")
	}
	if order.TimeInForce != PerpsTIFGTC && order.TimeInForce != PerpsTIFGTD && order.PostOnly {
		return nil, nil, fmt.Errorf("perps: post-only requires GTC or GTD")
	}
	if order.TimeInForce == PerpsTIFGTD {
		if order.GTDExpiry <= time.Now().UnixMilli() || order.GTDExpiry > 18446744073709 {
			return nil, nil, fmt.Errorf("perps: GTD expiry must be future and within venue range")
		}
	} else if order.GTDExpiry != 0 {
		return nil, nil, fmt.Errorf("perps: order expiry requires GTD")
	}
	if order.ClientOrderID != "" && !validPerpsClientOrderID(order.ClientOrderID) {
		return nil, nil, fmt.Errorf(
			"perps: client order ID must be 32 lowercase hexadecimal characters",
		)
	}
	buy := order.Side == PerpsOrderBuy
	raw := []any{order.InstrumentID, buy}
	if order.Price != "" {
		raw = append(raw, order.Price)
	}
	raw = append(raw, order.Quantity, string(order.TimeInForce), order.PostOnly)
	if order.ReduceOnly {
		raw = append(raw, true)
	}
	if order.ClientOrderID != "" {
		raw = append(raw, order.ClientOrderID)
	}
	body := map[string]any{
		"iid": order.InstrumentID,
		"buy": buy,
		"po":  order.PostOnly,
		"qty": order.Quantity,
		"tif": order.TimeInForce,
	}
	if order.ReduceOnly {
		body["ro"] = true
	}
	if order.Price != "" {
		body["p"] = order.Price
	}
	if order.ClientOrderID != "" {
		body["c"] = order.ClientOrderID
	}
	if order.GTDExpiry != 0 {
		raw = append(raw, order.GTDExpiry)
		body["gtd_expiry"] = order.GTDExpiry
	}
	return raw, body, nil
}

func validPerpsClientOrderID(value string) bool {
	if len(value) != 32 || strings.ToLower(value) != value {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}
