package perps

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
	"github.com/quagmt/udecimal"
)

// PerpsOrderSide is the direction of an authenticated order.
type PerpsOrderSide string

const (
	PerpsOrderBuy  PerpsOrderSide = "buy"
	PerpsOrderSell PerpsOrderSide = "sell"
)

// PerpsOrderRequest is the stable entry-order subset accepted by the signed
// perps session command. TP/SL orchestration is intentionally separate.
type PerpsOrderRequest struct {
	InstrumentID  int
	Side          PerpsOrderSide
	Price         string
	Quantity      string
	TimeInForce   PerpsTimeInForce
	PostOnly      bool
	ReduceOnly    bool
	ClientOrderID string
}

// PerpsOrderAck is the acknowledgement returned by createOrders.
type PerpsOrderAck struct {
	Status         string `json:"status"`
	OrderID        int    `json:"oid,omitempty"`
	ClientOrderID  string `json:"coid,omitempty"`
	Error          string `json:"error,omitempty"`
	orderIDPresent bool
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
	Status        string `json:"status"`
	OrderID       int    `json:"oid,omitempty"`
	ClientOrderID string `json:"coid,omitempty"`
	Error         string `json:"error,omitempty"`
}

// ErrPerpsAutoCancelDailyLimit indicates that the account cannot arm another
// auto-cancel schedule until its daily trigger counter resets.
var ErrPerpsAutoCancelDailyLimit = errors.New("perps: auto-cancel daily trigger limit reached")

// PerpsLeverageResult is the acknowledgement returned by updateLeverage.
type PerpsLeverageResult struct {
	Status       string `json:"status"`
	InstrumentID int    `json:"instrument_id"`
	Leverage     int    `json:"leverage"`
	Cross        bool   `json:"cross"`
	Error        string `json:"error,omitempty"`
}

type perpsOrderUpdate struct {
	ID               int              `json:"oid"`
	InstrumentID     int              `json:"iid"`
	Buy              bool             `json:"buy"`
	Price            string           `json:"p"`
	Quantity         string           `json:"qty"`
	TimeInForce      PerpsTimeInForce `json:"tif"`
	PostOnly         bool             `json:"po"`
	ReduceOnly       bool             `json:"ro"`
	Status           PerpsOrderStatus `json:"status"`
	RestingQuantity  string           `json:"rest"`
	FilledQuantity   string           `json:"fill"`
	CreatedTimestamp int64            `json:"cts"`
	UpdatedTimestamp int64            `json:"uts"`
	ClientOrderID    string           `json:"coid,omitempty"`
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
	}
}

// PlaceOrder submits one entry order and waits for its first authenticated
// orders update. Use PostOrders when the acknowledgement is sufficient or when
// submitting a batch. TP/SL order groups remain a separate API.
func (s *Session) PlaceOrder(
	ctx context.Context,
	order PerpsOrderRequest,
	expiresAt int64,
) (*PerpsOrder, error) {
	acknowledgements, err := s.PostOrders(ctx, []PerpsOrderRequest{order}, expiresAt)
	if err != nil {
		return nil, err
	}
	if len(acknowledgements) != 1 {
		return nil, fmt.Errorf(
			"perps: expected one order acknowledgement, got %d",
			len(acknowledgements),
		)
	}
	acknowledgement := acknowledgements[0]
	if err := validatePerpsPostOrderAck(acknowledgement); err != nil {
		return nil, fmt.Errorf("perps: %w", err)
	}
	waitContext, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	update, err := s.waitForOrderUpdate(waitContext, acknowledgement.OrderID)
	if err != nil {
		return nil, fmt.Errorf("perps: wait for order update: %w", err)
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
	if len(orders) > 15 {
		return nil, fmt.Errorf("perps: at most 15 orders may be submitted at once")
	}
	rawOrders := make([]any, len(orders))
	bodyOrders := make([]any, len(orders))
	for i, order := range orders {
		raw, body, err := perpsOrderWire(order)
		if err != nil {
			return nil, err
		}
		rawOrders[i] = raw
		bodyOrders[i] = body
	}
	op := []any{"createOrders", rawOrders}
	data, err := s.sendSignedCommand(ctx, op, map[string]any{
		"type": "createOrders",
		"args": bodyOrders,
	}, expiresAt)
	if err != nil {
		return nil, err
	}
	var acknowledgements []PerpsOrderAck
	if err := json.Unmarshal(data, &acknowledgements); err != nil {
		return nil, fmt.Errorf("perps: decode order acknowledgement: %w", err)
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
	op := []any{"cancelOrders", orderIDs}
	data, err := s.sendSignedCommand(ctx, op, map[string]any{
		"type": "cancelOrders",
		"args": orderIDs,
	}, expiresAt)
	if err != nil {
		return nil, err
	}
	var results []PerpsCancelResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("perps: decode cancel acknowledgement: %w", err)
	}
	return results, nil
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
	op := []any{"cancelOrdersCOID", clientOrderIDs}
	data, err := s.sendSignedCommand(ctx, op, map[string]any{
		"type": "cancelOrdersCOID",
		"args": clientOrderIDs,
	}, expiresAt)
	if err != nil {
		return nil, err
	}
	var results []PerpsCancelResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, fmt.Errorf("perps: decode client cancel acknowledgement: %w", err)
	}
	return results, nil
}

// CancelAllOrders cancels all open orders, optionally scoped to one instrument.
// The official service accepts this signed command over authenticated REST;
// the Session method delegates here rather than pretending it is a WS frame.
func (c *AuthenticatedClient) CancelAllOrders(
	ctx context.Context,
	instrumentID *int,
	expiresAt int64,
) error {
	if instrumentID != nil && *instrumentID < 0 {
		return fmt.Errorf("perps: instrument ID must be non-negative")
	}
	var rawArgs []any
	bodyArgs := map[string]any{}
	if instrumentID != nil {
		rawArgs = []any{*instrumentID}
		bodyArgs["iid"] = *instrumentID
	} else {
		rawArgs = []any{}
	}
	signer, err := c.delegatedSigner()
	if err != nil {
		return err
	}
	command, err := makePerpsSignedCommand(
		signer,
		c.chainID,
		[]any{"cancelAll", rawArgs},
		map[string]any{"type": "cancelAll", "args": bodyArgs},
		expiresAt,
	)
	if err != nil {
		return err
	}
	var ack struct {
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}
	if err := c.http.DoJSON(
		ctx,
		http.MethodDelete,
		"/v1/trade/orders/all",
		nil,
		command,
		polyhttp.AuthNone,
		nil,
		map[string]string{
			"POLYMARKET-PROXY":  c.credentials.Proxy,
			"POLYMARKET-SECRET": c.credentials.Secret,
		},
		&ack,
	); err != nil {
		return err
	}
	if ack.Status != "ok" {
		if ack.Error == "" {
			ack.Error = "cancel-all rejected"
		}
		return fmt.Errorf("perps: %s", ack.Error)
	}
	return nil
}

// CancelAllOrders cancels all open orders, optionally scoped to one instrument.
func (s *Session) CancelAllOrders(
	ctx context.Context,
	instrumentID *int,
	expiresAt int64,
) error {
	return s.client.CancelAllOrders(ctx, instrumentID, expiresAt)
}

// UpdateLeverage signs and submits a leverage/margin-mode update.
func (s *Session) UpdateLeverage(
	ctx context.Context,
	instrumentID, leverage int,
	cross bool,
) (*PerpsLeverageResult, error) {
	if instrumentID < 0 {
		return nil, fmt.Errorf("perps: instrument ID must be non-negative")
	}
	if leverage <= 0 {
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

const (
	perpsAutoCancelMinimumDelay = 5 * time.Second
	autoCancelDailyLimitCode    = "auto_cancel_daily_limit_reached"
)

// ArmAutoCancel arms the one-shot schedule that cancels all open orders at
// cancelAt, expressed as Unix milliseconds. The deadline must be at least five
// seconds in the future. Arming again replaces the prior schedule.
func (s *Session) ArmAutoCancel(
	ctx context.Context,
	cancelAt int64,
	expiresAt int64,
) error {
	if cancelAt < time.Now().Add(perpsAutoCancelMinimumDelay).UnixMilli() {
		return fmt.Errorf("perps: cancel time must be at least 5 seconds in the future")
	}
	return s.updateAutoCancel(ctx, cancelAt, expiresAt)
}

// DisarmAutoCancel clears the current auto-cancel schedule without triggering
// it. It remains allowed after the daily trigger limit is reached.
func (s *Session) DisarmAutoCancel(ctx context.Context, expiresAt int64) error {
	return s.updateAutoCancel(ctx, 0, expiresAt)
}

// GetAutoCancelStatus returns the schedule and daily trigger counters for the
// session account.
func (s *Session) GetAutoCancelStatus(
	ctx context.Context,
) (*PerpsAutoCancelStatus, error) {
	return s.client.GetAutoCancelStatus(ctx)
}

func (s *Session) updateAutoCancel(
	ctx context.Context,
	deadline int64,
	expiresAt int64,
) error {
	if expiresAt < 0 {
		return fmt.Errorf("perps: expiration must not be negative")
	}
	op := []any{"autoCancel", []any{deadline}}
	body, err := makePerpsSignedCommand(
		s.signer,
		s.chainID,
		op,
		map[string]any{"type": "autoCancel", "args": map[string]any{"time": deadline}},
		expiresAt,
	)
	if err != nil {
		return err
	}
	var response PerpsAutoCancelResponse
	if err := s.client.patchAuthenticatedJSON(
		ctx,
		"/v1/trade/auto-cancel",
		body,
		&response,
	); err != nil {
		if isAutoCancelDailyLimitError(err) {
			return fmt.Errorf("%w: %w", ErrPerpsAutoCancelDailyLimit, err)
		}
		return err
	}
	if response.Status != "ok" {
		if response.Error == "" {
			response.Error = "auto-cancel update rejected"
		}
		return fmt.Errorf("perps: %s", response.Error)
	}
	return nil
}

func isAutoCancelDailyLimitError(err error) bool {
	var apiErr *polyhttp.APIError
	return errors.As(err, &apiErr) && strings.Contains(apiErr.Message, autoCancelDailyLimitCode)
}

// UpdateMargin adjusts isolated margin for an instrument. Positive amounts add
// margin and negative amounts remove it.
func (s *Session) UpdateMargin(
	ctx context.Context,
	instrumentID int,
	amount string,
) error {
	if instrumentID < 0 {
		return fmt.Errorf("perps: instrument ID must be non-negative")
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
	decimal, err := udecimal.Parse(value)
	if err != nil {
		return "", fmt.Errorf("perps: invalid margin amount %q: %w", value, err)
	}
	return decimal.String(), nil
}

func validatePerpsPostOrderAck(ack PerpsOrderAck) error {
	if ack.Status != "ok" {
		if ack.Error == "" {
			ack.Error = "order rejected"
		}
		return fmt.Errorf("%s", ack.Error)
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
		order.TimeInForce != PerpsTIFFOK {
		return nil, nil, fmt.Errorf("perps: invalid time-in-force %q", order.TimeInForce)
	}
	if order.TimeInForce == PerpsTIFGTC && order.Price == "" {
		return nil, nil, fmt.Errorf("perps: GTC orders require a price")
	}
	if order.TimeInForce != PerpsTIFGTC && order.PostOnly {
		return nil, nil, fmt.Errorf("perps: post-only is only valid for GTC orders")
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
