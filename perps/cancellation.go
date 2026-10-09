package perps

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

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
		ctx,
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

func (r *PerpsCancelResult) UnmarshalJSON(data []byte) error {
	type alias PerpsCancelResult
	var out alias
	if err := json.Unmarshal(data, &out); err != nil {
		return err
	}
	var identity struct {
		OID  *int    `json:"oid"`
		COID *string `json:"coid"`
	}
	if err := json.Unmarshal(data, &identity); err != nil {
		return err
	}
	*r = PerpsCancelResult(out)
	r.orderIDPresent = identity.OID != nil
	r.clientIDPresent = identity.COID != nil
	return nil
}

// CancelRetry bounds only explicit order_in_flight item rejections. Transport
// failures and whole-request rejections are never blindly retried.
type CancelRetry struct {
	MaxAttempts int
	MaxElapsed  time.Duration
	Disable     bool
}
type CancelOrdersRequest struct {
	OrderIDs       []int
	ClientOrderIDs []string
	ExpiresAt      int64
	Retry          CancelRetry
}

// CancelRetryError retains prior results and unresolved input indexes if a
// subsequent attempt fails. Errors unwrap to the transport/protocol cause.
type CancelRetryError struct {
	Cause          error
	Results        []PerpsCancelResult
	PendingIndexes []int
}

func (e *CancelRetryError) Error() string {
	return fmt.Sprintf("perps: cancellation retry failed: %v", e.Cause)
}
func (e *CancelRetryError) Unwrap() error { return e.Cause }

func (s *Session) CancelOrdersWithRetry(
	ctx context.Context,
	p CancelOrdersRequest,
) ([]PerpsCancelResult, error) {
	if (len(p.OrderIDs) > 0) == (len(p.ClientOrderIDs) > 0) {
		return nil, fmt.Errorf("perps: select numeric IDs or client IDs")
	}
	name := "cancelOrders"
	n := len(p.OrderIDs)
	if n == 0 {
		name = "cancelOrdersCOID"
		n = len(p.ClientOrderIDs)
	}
	for _, id := range p.OrderIDs {
		if id < 0 {
			return nil, fmt.Errorf("perps: invalid order ID")
		}
	}
	for _, id := range p.ClientOrderIDs {
		if !validPerpsClientOrderID(id) {
			return nil, fmt.Errorf("perps: invalid client order ID")
		}
	}
	attempts := p.Retry.MaxAttempts
	if attempts == 0 {
		attempts = 4
	}
	elapsed := p.Retry.MaxElapsed
	if elapsed == 0 {
		elapsed = 2 * time.Second
	}
	if attempts < 1 || elapsed < 0 {
		return nil, fmt.Errorf("perps: invalid cancellation retry bounds")
	}
	if p.Retry.Disable {
		attempts = 1
	}
	deadline := time.Now().Add(elapsed)
	if p.ExpiresAt != 0 && time.UnixMilli(p.ExpiresAt).Before(deadline) {
		deadline = time.UnixMilli(p.ExpiresAt)
	}
	results := make([]PerpsCancelResult, n)
	pending := make([]int, n)
	for i := range pending {
		pending[i] = i
	}
	fail := func(err error, attempt int) ([]PerpsCancelResult, error) {
		if attempt == 0 {
			return nil, err
		}
		return results, &CancelRetryError{
			Cause:          err,
			Results:        slices.Clone(results),
			PendingIndexes: slices.Clone(pending),
		}
	}
	for attempt := 0; len(pending) > 0 && attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return fail(err, attempt)
		}
		if attempt > 0 {
			delay := min(100*time.Millisecond*time.Duration(1<<min(attempt-1, 4)), time.Second)
			if time.Now().Add(delay).After(deadline) {
				break
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return fail(ctx.Err(), attempt)
			case <-timer.C:
			}
		}
		var ids any
		if name == "cancelOrders" {
			batch := make([]int, len(pending))
			for i, index := range pending {
				batch[i] = p.OrderIDs[index]
			}
			ids = batch
		} else {
			batch := make([]string, len(pending))
			for i, index := range pending {
				batch[i] = p.ClientOrderIDs[index]
			}
			ids = batch
		}
		data, err := s.sendSignedCommand(
			ctx,
			[]any{name, ids},
			map[string]any{"type": name, "args": ids},
			p.ExpiresAt,
		)
		if err != nil {
			return fail(err, attempt)
		}
		var out []PerpsCancelResult
		if err := json.Unmarshal(data, &out); err != nil {
			return fail(fmt.Errorf("perps: decode cancellation: %w", err), attempt)
		}
		if len(out) == 1 && out[0].Status == "err" && !out[0].orderIDPresent &&
			!out[0].clientIDPresent {
			return fail(&CommandError{Operation: name, Code: out[0].Error}, attempt)
		}
		if len(out) != len(pending) {
			return fail(fmt.Errorf("perps: cancellation result count mismatch"), attempt)
		}
		retry := []int{}
		for i, result := range out {
			index := pending[i]
			if result.Status != "ok" && result.Status != "err" ||
				result.Status == "err" && result.Error == "" {
				return fail(fmt.Errorf("perps: invalid cancellation result"), attempt)
			}
			if result.orderIDPresent && name == "cancelOrders" &&
				result.OrderID != p.OrderIDs[index] ||
				result.clientIDPresent && name == "cancelOrdersCOID" &&
					result.ClientOrderID != p.ClientOrderIDs[index] {
				return fail(fmt.Errorf("perps: cancellation identity mismatch"), attempt)
			}
			results[index] = result
			if result.Status == "err" && result.Error == "order_in_flight" {
				retry = append(retry, index)
			}
		}
		pending = retry
	}
	return results, nil
}
