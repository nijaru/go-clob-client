package perps

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strconv"
	"time"
)

// ErrPaginationNonProgress prevents endless loops or silently skipping records
// when a timestamp-only endpoint cannot expose the rest of a full boundary.
var ErrPaginationNonProgress = errors.New(
	"perps: pagination cannot advance without risking omitted records",
)

type PerpsInternalTransfer struct {
	ID               int64  `json:"transfer_id"`
	Type             string `json:"type"`
	Asset            string `json:"asset"`
	Amount           string `json:"amount"`
	Direction        string `json:"direction"`
	Counterparty     string `json:"counterparty"`
	Label            string `json:"label,omitempty"`
	CreatedTimestamp int64  `json:"created_timestamp"`
}
type historyCursor struct {
	Kind   string               `json:"kind"`
	Params AccountHistoryParams `json:"params"`
	Seen   []string             `json:"seen"`
}

// descendingHistory owns opaque continuation state for endpoints whose wire
// cursor is a timestamp. Boundary keys are retained to avoid duplicate records.
func descendingHistory[T any](
	ctx context.Context,
	c *AuthenticatedClient,
	path string,
	p AccountHistoryParams,
	window time.Duration,
	key func(T) string,
	timestamp func(T) int64,
	overlapMillis bool,
) (PerpsPage[T], error) {
	state := historyCursor{Kind: path, Params: p}
	state.Params.Cursor = ""
	if p.Cursor != "" {
		if err := decodeCursor(p.Cursor, &state); err != nil {
			return PerpsPage[T]{}, err
		}
		if state.Kind != path {
			return PerpsPage[T]{}, fmt.Errorf("perps: history cursor belongs to another endpoint")
		}
	} else {
		now := time.Now().UnixMilli()
		if state.Params.End == 0 {
			state.Params.End = now
		}
		if state.Params.Start == 0 {
			state.Params.Start = now - window.Milliseconds()
		}
	}
	if state.Params.Start < 0 || state.Params.End < state.Params.Start ||
		state.Params.InstrumentID != nil && !validInstrumentID(*state.Params.InstrumentID) {
		return PerpsPage[T]{}, fmt.Errorf("perps: invalid history range")
	}
	var out PerpsPage[T]
	if err := c.getAuthenticatedJSON(ctx, path, historyQuery(state.Params), &out); err != nil {
		return PerpsPage[T]{}, err
	}
	seen := map[string]bool{}
	for _, id := range state.Seen {
		seen[id] = true
	}
	items := make([]T, 0, len(out.Data))
	for _, item := range out.Data {
		if !seen[key(item)] {
			items = append(items, item)
		}
	}
	if out.More {
		if len(out.Data) == 0 {
			return PerpsPage[T]{}, ErrPaginationNonProgress
		}
		next := timestamp(out.Data[len(out.Data)-1])
		if overlapMillis {
			next++
		}
		next = min(state.Params.End, next)
		if next == state.Params.End && len(items) == 0 {
			return PerpsPage[T]{}, ErrPaginationNonProgress
		}
		if next < state.Params.Start {
			return PerpsPage[T]{}, ErrPaginationNonProgress
		}
		if next < state.Params.End {
			state.Seen = nil
		}
		state.Params.End = next
		known := map[string]bool{}
		for _, id := range state.Seen {
			known[id] = true
		}
		cutoff := next
		if overlapMillis {
			cutoff--
		}
		for _, item := range out.Data {
			if timestamp(item) >= cutoff && !known[key(item)] {
				id := key(item)
				state.Seen = append(state.Seen, id)
				known[id] = true
			}
		}
		cursor, err := encodeCursor(state)
		if err != nil {
			return PerpsPage[T]{}, err
		}
		out.NextCursor = cursor
	}
	out.Data = items
	return out, nil
}

func (c *AuthenticatedClient) GetInternalTransfersPage(
	ctx context.Context,
	p AccountHistoryParams,
) (PerpsPage[PerpsInternalTransfer], error) {
	return descendingHistory(
		ctx,
		c,
		"/v1/account/internal-transfers",
		p,
		90*24*time.Hour,
		func(t PerpsInternalTransfer) string { return strconv.FormatInt(t.ID, 10) },
		func(t PerpsInternalTransfer) int64 { return t.CreatedTimestamp },
		true,
	)
}

func iterateHistory[T any](
	ctx context.Context,
	p AccountHistoryParams,
	get func(context.Context, AccountHistoryParams) (PerpsPage[T], error),
) iter.Seq2[[]T, error] {
	return func(yield func([]T, error) bool) {
		for {
			page, err := get(ctx, p)
			if err != nil {
				yield(nil, err)
				return
			}
			if !yield(page.Data, nil) || !page.More {
				return
			}
			if page.NextCursor == "" || page.NextCursor == p.Cursor {
				yield(nil, ErrPaginationNonProgress)
				return
			}
			p.Cursor = page.NextCursor
		}
	}
}

func (c *AuthenticatedClient) IterFills(
	ctx context.Context,
	p AccountHistoryParams,
) iter.Seq2[[]PerpsAccountFill, error] {
	return iterateHistory(ctx, p, c.GetFillsPage)
}

func (c *AuthenticatedClient) IterFundingPayments(
	ctx context.Context,
	p AccountHistoryParams,
) iter.Seq2[[]PerpsAccountFundingPayment, error] {
	return iterateHistory(ctx, p, c.GetFundingPaymentsPage)
}

func (c *AuthenticatedClient) IterDeposits(
	ctx context.Context,
	p AccountHistoryParams,
) iter.Seq2[[]PerpsDeposit, error] {
	return iterateHistory(ctx, p, c.GetDepositsPage)
}

func (c *AuthenticatedClient) IterWithdrawals(
	ctx context.Context,
	p AccountHistoryParams,
) iter.Seq2[[]PerpsWithdrawal, error] {
	return iterateHistory(ctx, p, c.GetWithdrawalsPage)
}

func (c *AuthenticatedClient) IterInternalTransfers(
	ctx context.Context,
	p AccountHistoryParams,
) iter.Seq2[[]PerpsInternalTransfer, error] {
	return iterateHistory(ctx, p, c.GetInternalTransfersPage)
}

type intervalCursor struct {
	Kind   string
	Params AccountIntervalHistoryParams
}

func intervalHistory[T any](
	ctx context.Context,
	c *AuthenticatedClient,
	path string,
	p AccountIntervalHistoryParams,
	timestamp func(T) int64,
) (PerpsPage[T], error) {
	state := intervalCursor{Kind: path, Params: p}
	state.Params.Cursor = ""
	if p.Cursor != "" {
		if err := decodeCursor(p.Cursor, &state); err != nil {
			return PerpsPage[T]{}, err
		}
		if state.Kind != path {
			return PerpsPage[T]{}, fmt.Errorf("perps: interval cursor belongs to another endpoint")
		}
	}
	if state.Params.End == 0 {
		state.Params.End = time.Now().UnixMilli()
	}
	step := int64(0)
	switch state.Params.Interval {
	case PerpsPnl1h:
		step = 3600000
	case PerpsPnl4h:
		step = 14400000
	case PerpsPnl1d:
		step = 86400000
	case PerpsPnl1w:
		step = 604800000
	default:
		return PerpsPage[T]{}, fmt.Errorf("perps: history interval required")
	}
	if state.Params.Start < 0 || state.Params.Start > state.Params.End {
		return PerpsPage[T]{}, fmt.Errorf("perps: invalid history range")
	}
	var out PerpsPage[T]
	if err := c.getAuthenticatedJSON(ctx, path, intervalHistoryQuery(state.Params), &out); err != nil {
		return out, err
	}
	if out.More {
		if len(out.Data) == 0 {
			return PerpsPage[T]{}, ErrPaginationNonProgress
		}
		last := timestamp(out.Data[len(out.Data)-1])
		next := last + step
		if next <= state.Params.Start || next <= last {
			return PerpsPage[T]{}, ErrPaginationNonProgress
		}
		if next > state.Params.End {
			out.More = false
		} else {
			state.Params.Start = next
			cursor, err := encodeCursor(state)
			if err != nil {
				return PerpsPage[T]{}, err
			}
			out.NextCursor = cursor
		}
	}
	return out, nil
}

func iterateInterval[T any](
	ctx context.Context,
	p AccountIntervalHistoryParams,
	get func(context.Context, AccountIntervalHistoryParams) (PerpsPage[T], error),
) iter.Seq2[[]T, error] {
	return func(yield func([]T, error) bool) {
		for {
			page, err := get(ctx, p)
			if err != nil {
				yield(nil, err)
				return
			}
			if !yield(page.Data, nil) || !page.More {
				return
			}
			if page.NextCursor == "" || page.NextCursor == p.Cursor {
				yield(nil, ErrPaginationNonProgress)
				return
			}
			p.Cursor = page.NextCursor
		}
	}
}

func (c *AuthenticatedClient) IterEquityHistory(
	ctx context.Context,
	p AccountIntervalHistoryParams,
) iter.Seq2[[]PerpsEquityPoint, error] {
	return iterateInterval(ctx, p, c.GetEquityHistoryPage)
}

func (c *AuthenticatedClient) IterPnlHistory(
	ctx context.Context,
	p AccountIntervalHistoryParams,
) iter.Seq2[[]PerpsPnlPoint, error] {
	return iterateInterval(ctx, p, c.GetPnlHistoryPage)
}
