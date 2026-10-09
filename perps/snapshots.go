package perps

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

type PositionSnapshotFill struct {
	InstrumentID int    `json:"instrument_id"`
	TradeID      string `json:"trade_id"`
	Timestamp    int64  `json:"timestamp"`
}
type PositionSnapshotSelection struct {
	ActiveInstrumentIDs []int                  `json:"active_instrument_ids,omitempty"`
	HistoryFills        []PositionSnapshotFill `json:"history_fills,omitempty"`
}
type PositionSnapshotsRequest struct {
	Address string `json:"address"`
	PositionSnapshotSelection
}
type PositionSnapshotCandle struct {
	Position float64 `json:"position"`
	Open     float64 `json:"open"`
	High     float64 `json:"high"`
	Low      float64 `json:"low"`
	Close    float64 `json:"close"`
}
type PositionSnapshotMarker struct {
	Position float64 `json:"position"`
	Kind     string  `json:"kind"`
}
type PositionSnapshotChart struct {
	Candles []PositionSnapshotCandle `json:"candles"`
	Markers []PositionSnapshotMarker `json:"markers"`
}
type PositionSnapshot struct {
	PositionCycleID string                `json:"position_cycle_id"`
	Side            PerpsSide             `json:"side"`
	StartedAt       int64                 `json:"started_at"`
	AsOfAt          int64                 `json:"as_of_at"`
	Closed          bool                  `json:"is_closed"`
	SizeAfter       string                `json:"size_after"`
	EntryPrice      string                `json:"entry_price"`
	AsOfPrice       string                `json:"as_of_price"`
	PnL             string                `json:"pnl"`
	PnLPercent      *string               `json:"pnl_percent"`
	Leverage        *int                  `json:"leverage"`
	Chart           PositionSnapshotChart `json:"chart"`
}
type PositionSnapshotResult struct {
	InstrumentID int               `json:"instrument_id"`
	TradeID      string            `json:"trade_id,omitempty"`
	Status       string            `json:"status"`
	Snapshot     *PositionSnapshot `json:"snapshot,omitempty"`
}
type PositionSnapshots struct {
	HistoryAsOfAt int64                    `json:"history_as_of_at"`
	Active        []PositionSnapshotResult `json:"active"`
	History       []PositionSnapshotResult `json:"history"`
}

func (s PositionSnapshotSelection) validate() error {
	if len(s.ActiveInstrumentIDs) > 20 || len(s.HistoryFills) > 4 ||
		len(s.ActiveInstrumentIDs)+len(s.HistoryFills) == 0 {
		return fmt.Errorf("perps: select 1-20 active instruments and/or 1-4 historical fills")
	}
	ids := make(map[int]bool)
	for _, id := range s.ActiveInstrumentIDs {
		if !validInstrumentID(id) || ids[id] {
			return fmt.Errorf("perps: invalid or duplicate active instrument")
		}
		ids[id] = true
	}
	keys := make(map[string]bool)
	for _, fill := range s.HistoryFills {
		id, err := strconv.ParseUint(fill.TradeID, 10, 64)
		if err != nil || len(fill.TradeID) > 20 || strings.Trim(fill.TradeID, "0123456789") != "" ||
			!validInstrumentID(fill.InstrumentID) ||
			fill.Timestamp < 0 ||
			fill.Timestamp > 18446744073709 {
			return fmt.Errorf("perps: invalid historical fill selection")
		}
		key := fmt.Sprintf("%d:%d", fill.InstrumentID, id)
		if keys[key] {
			return fmt.Errorf("perps: duplicate historical fill")
		}
		keys[key] = true
	}
	return nil
}

func validateSnapshots(out PositionSnapshots, selection PositionSnapshotSelection) error {
	if len(out.Active) != len(selection.ActiveInstrumentIDs) ||
		len(out.History) != len(selection.HistoryFills) {
		return fmt.Errorf("perps: snapshot result count mismatch")
	}
	check := func(item PositionSnapshotResult) error {
		switch item.Status {
		case "ok":
			if item.Snapshot == nil {
				return fmt.Errorf("perps: successful snapshot missing payload")
			}
		case "not_found",
			"history_pending",
			"history_limit",
			"resource_limit",
			"temporarily_unavailable",
			"unavailable":
			if item.Snapshot != nil {
				return fmt.Errorf("perps: rejected snapshot contains payload")
			}
		default:
			return fmt.Errorf("perps: unknown snapshot status %q", item.Status)
		}
		return nil
	}
	for i, item := range out.Active {
		if item.InstrumentID != selection.ActiveInstrumentIDs[i] {
			return fmt.Errorf("perps: snapshot instrument mismatch")
		}
		if err := check(item); err != nil {
			return err
		}
	}
	for i, item := range out.History {
		f := selection.HistoryFills[i]
		id, _ := strconv.ParseUint(f.TradeID, 10, 64)
		if item.InstrumentID != f.InstrumentID || item.TradeID != strconv.FormatUint(id, 10) {
			return fmt.Errorf("perps: historical snapshot identity mismatch")
		}
		if err := check(item); err != nil {
			return err
		}
	}
	return nil
}

// GetPositionSnapshots is always anonymous, even through an authenticated client.
// Item statuses are not retried; callers decide whether history is ready.
func (c *Client) GetPositionSnapshots(
	ctx context.Context,
	request PositionSnapshotsRequest,
) (*PositionSnapshots, error) {
	if !common.IsHexAddress(request.Address) {
		return nil, fmt.Errorf("perps: invalid snapshot address")
	}
	if err := request.PositionSnapshotSelection.validate(); err != nil {
		return nil, err
	}
	var out PositionSnapshots
	if err := c.http.DoJSON(ctx, http.MethodPost, "/v1/info/position-snapshots", nil, request, polyhttp.AuthNone, nil, nil, &out); err != nil {
		return nil, err
	}
	if err := validateSnapshots(out, request.PositionSnapshotSelection); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *AuthenticatedClient) GetOwnPositionSnapshots(
	ctx context.Context,
	selection PositionSnapshotSelection,
) (*PositionSnapshots, error) {
	if err := selection.validate(); err != nil {
		return nil, err
	}
	info, err := c.GetCredentials(ctx)
	if err != nil {
		return nil, err
	}
	if !common.IsHexAddress(info.Address) {
		return nil, fmt.Errorf("perps: invalid credential owner address")
	}
	var out PositionSnapshots
	if err := c.postAuthenticatedJSON(ctx, "/v1/info/position-snapshots", PositionSnapshotsRequest{Address: info.Address, PositionSnapshotSelection: selection}, &out); err != nil {
		return nil, err
	}
	if err := validateSnapshots(out, selection); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetRegistration(ctx context.Context, address string) (bool, error) {
	if !common.IsHexAddress(address) {
		return false, fmt.Errorf("perps: invalid registration address")
	}
	var out struct {
		Registered bool `json:"registered"`
	}
	if err := c.getJSON(ctx, "/v1/info/registered", url.Values{"address": {address}}, &out); err != nil {
		return false, err
	}
	return out.Registered, nil
}
