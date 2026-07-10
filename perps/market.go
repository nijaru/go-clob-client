package perps

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"iter"
	"net/url"
	"strconv"
	"time"
)

const (
	instrumentsEndpoint = "/v1/info/instruments"
	tickersEndpoint     = "/v1/info/tickers"
	statisticsEndpoint  = "/v1/info/statistics"
	bookEndpoint        = "/v1/info/book"
	klinesEndpoint      = "/v1/info/klines"
	tradesEndpoint      = "/v1/info/trades"
	fundingEndpoint     = "/v1/info/funding"
	feesEndpoint        = "/v1/info/fees"
)

// GetInstruments lists perps instruments, optionally filtered.
func (c *Client) GetInstruments(ctx context.Context, p InstrumentsParams) ([]PerpsInstrument, error) {
	query := url.Values{}
	if p.InstrumentID != nil {
		query.Set("instrument_id", strconv.Itoa(*p.InstrumentID))
	}
	if p.Category != nil {
		query.Set("category", string(*p.Category))
	}
	var out []PerpsInstrument
	if err := c.getJSON(ctx, instrumentsEndpoint, query, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetTickers lists current tickers, enriched with 24h open price and volume
// from the statistics feed (mirrors the TS SDK's joined ticker response).
func (c *Client) GetTickers(ctx context.Context, p TickersParams) ([]PerpsTicker, error) {
	query := url.Values{}
	if p.InstrumentID != nil {
		query.Set("instrument_id", strconv.Itoa(*p.InstrumentID))
	}
	var tickers []PerpsTicker
	if err := c.getJSON(ctx, tickersEndpoint, query, &tickers); err != nil {
		return nil, err
	}
	stats, err := c.getStatistics(ctx, p)
	if err != nil {
		return nil, err
	}
	byID := make(map[int]*PerpsStatistic, len(stats))
	for i := range stats {
		byID[stats[i].InstrumentID] = &stats[i]
	}
	for i := range tickers {
		if s, ok := byID[tickers[i].InstrumentID]; ok {
			tickers[i].OpenPrice = s.OpenPrice
			tickers[i].Volume24h = s.Volume
		}
	}
	return tickers, nil
}

// GetTicker returns the enriched ticker for a single instrument.
func (c *Client) GetTicker(ctx context.Context, instrumentID int) (*PerpsTicker, error) {
	id := instrumentID
	tickers, err := c.GetTickers(ctx, TickersParams{InstrumentID: &id})
	if err != nil {
		return nil, err
	}
	if len(tickers) == 0 {
		return nil, fmt.Errorf("perps: no ticker for instrument %d", instrumentID)
	}
	return &tickers[0], nil
}

func (c *Client) getStatistics(ctx context.Context, p TickersParams) ([]PerpsStatistic, error) {
	query := url.Values{}
	if p.InstrumentID != nil {
		query.Set("instrument_id", strconv.Itoa(*p.InstrumentID))
	}
	var out []PerpsStatistic
	if err := c.getJSON(ctx, statisticsEndpoint, query, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetBook returns an order book snapshot for an instrument. Depth defaults to 100.
func (c *Client) GetBook(ctx context.Context, p BookParams) (*PerpsBook, error) {
	query := url.Values{}
	query.Set("instrument_id", strconv.Itoa(p.InstrumentID))
	if p.Depth == 0 {
		p.Depth = PerpsBookDepth100
	}
	query.Set("depth", strconv.Itoa(int(p.Depth)))
	var out PerpsBook
	if err := c.getJSON(ctx, bookEndpoint, query, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetFees returns the perps fee schedule.
func (c *Client) GetFees(ctx context.Context) ([]PerpsFeeScheduleEntry, error) {
	var out PerpsFeesInfo
	if err := c.getJSON(ctx, feesEndpoint, nil, &out); err != nil {
		return nil, err
	}
	return out.FeeSchedule, nil
}

// ---- Paginated list endpoints (SDK-owned cursor pagination) ----

// GetCandlesPage returns one page of candles and an opaque cursor for the next
// page (empty when exhausted). Pass CandlesParams.Cursor from a previous call to
// resume.
func (c *Client) GetCandlesPage(ctx context.Context, p CandlesParams) ([]PerpsCandle, string, error) {
	state, err := candlesState(p)
	if err != nil {
		return nil, "", err
	}
	items, more, last, err := c.candlesPage(ctx, state)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if more && last != nil {
		state.StartTimestamp = last.Timestamp + klineIntervalMs(state.Interval)
		next = encodeCursor(state)
	}
	return items, next, nil
}

// IterCandles ranges over all candle pages for an instrument.
func (c *Client) IterCandles(ctx context.Context, p CandlesParams) iter.Seq2[[]PerpsCandle, error] {
	return func(yield func([]PerpsCandle, error) bool) {
		state, err := candlesState(p)
		if err != nil {
			yield(nil, err)
			return
		}
		for {
			items, more, last, err := c.candlesPage(ctx, state)
			if err != nil {
				yield(nil, err)
				return
			}
			if !yield(items, nil) {
				return
			}
			if !more || last == nil {
				return
			}
			state.StartTimestamp = last.Timestamp + klineIntervalMs(state.Interval)
		}
	}
}

func (c *Client) candlesPage(ctx context.Context, s candlesCursor) ([]PerpsCandle, bool, *PerpsCandle, error) {
	query := url.Values{}
	query.Set("instrument_id", strconv.Itoa(s.InstrumentID))
	query.Set("interval", string(s.Interval))
	query.Set("start_timestamp", strconv.FormatInt(s.StartTimestamp, 10))
	query.Set("end_timestamp", strconv.FormatInt(s.EndTimestamp, 10))
	var out perpsDataResponse[PerpsCandle]
	if err := c.getJSON(ctx, klinesEndpoint, query, &out); err != nil {
		return nil, false, nil, err
	}
	var last *PerpsCandle
	if n := len(out.Data); n > 0 {
		last = &out.Data[n-1]
	}
	return out.Data, out.More, last, nil
}

// GetTradesPage returns one page of public trades and an opaque next cursor.
func (c *Client) GetTradesPage(ctx context.Context, p TradesParams) ([]PerpsPublicTrade, string, error) {
	state, err := tradesState(p)
	if err != nil {
		return nil, "", err
	}
	items, more, last, err := c.tradesPage(ctx, state)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if more && last != nil {
		// Advance endTimestamp to the last seen timestamp (minus 1 when the
		// last timestamp is shared with already-seen trades) and carry seen IDs.
		state.EndTimestamp = last.Timestamp
		if hasSeen(state.SeenTradeIDs, last.TradeID) {
			state.EndTimestamp--
		}
		next = encodeCursor(state)
	}
	return items, next, nil
}

// IterTrades ranges over all public-trade pages for an instrument, de-duplicating
// trades that share a timestamp across page boundaries.
func (c *Client) IterTrades(ctx context.Context, p TradesParams) iter.Seq2[[]PerpsPublicTrade, error] {
	return func(yield func([]PerpsPublicTrade, error) bool) {
		state, err := tradesState(p)
		if err != nil {
			yield(nil, err)
			return
		}
		for {
			items, more, last, err := c.tradesPage(ctx, state)
			if err != nil {
				yield(nil, err)
				return
			}
			if !yield(items, nil) {
				return
			}
			if !more || last == nil {
				return
			}
			if hasSeen(state.SeenTradeIDs, last.TradeID) {
				state.EndTimestamp = last.Timestamp - 1
			} else {
				state.EndTimestamp = last.Timestamp
			}
			for _, it := range items {
				if it.Timestamp == last.Timestamp {
					state.SeenTradeIDs = append(state.SeenTradeIDs, it.TradeID)
				}
			}
		}
	}
}

func (c *Client) tradesPage(ctx context.Context, s tradesCursor) ([]PerpsPublicTrade, bool, *PerpsPublicTrade, error) {
	query := url.Values{}
	query.Set("instrument_id", strconv.Itoa(s.InstrumentID))
	query.Set("start_timestamp", strconv.FormatInt(s.StartTimestamp, 10))
	query.Set("end_timestamp", strconv.FormatInt(s.EndTimestamp, 10))
	var out perpsDataResponse[PerpsPublicTrade]
	if err := c.getJSON(ctx, tradesEndpoint, query, &out); err != nil {
		return nil, false, nil, err
	}
	seen := make(map[int64]struct{}, len(s.SeenTradeIDs))
	for _, id := range s.SeenTradeIDs {
		seen[id] = struct{}{}
	}
	filtered := out.Data[:0]
	for _, t := range out.Data {
		if _, dup := seen[t.TradeID]; dup {
			continue
		}
		filtered = append(filtered, t)
	}
	var last *PerpsPublicTrade
	if n := len(filtered); n > 0 {
		last = &filtered[n-1]
	}
	return filtered, out.More, last, nil
}

// GetFundingHistoryPage returns one page of funding-rate samples and a next cursor.
func (c *Client) GetFundingHistoryPage(ctx context.Context, p FundingParams) ([]PerpsFundingRate, string, error) {
	state, err := fundingState(p)
	if err != nil {
		return nil, "", err
	}
	items, more, last, err := c.fundingPage(ctx, state)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if more && last != nil {
		state.EndTimestamp = last.Timestamp - 1
		next = encodeCursor(state)
	}
	return items, next, nil
}

// IterFundingHistory ranges over all funding-rate history pages for an instrument.
func (c *Client) IterFundingHistory(ctx context.Context, p FundingParams) iter.Seq2[[]PerpsFundingRate, error] {
	return func(yield func([]PerpsFundingRate, error) bool) {
		state, err := fundingState(p)
		if err != nil {
			yield(nil, err)
			return
		}
		for {
			items, more, last, err := c.fundingPage(ctx, state)
			if err != nil {
				yield(nil, err)
				return
			}
			if !yield(items, nil) {
				return
			}
			if !more || last == nil {
				return
			}
			state.EndTimestamp = last.Timestamp - 1
		}
	}
}

func (c *Client) fundingPage(ctx context.Context, s fundingCursor) ([]PerpsFundingRate, bool, *PerpsFundingRate, error) {
	query := url.Values{}
	query.Set("instrument_id", strconv.Itoa(s.InstrumentID))
	query.Set("start_timestamp", strconv.FormatInt(s.StartTimestamp, 10))
	query.Set("end_timestamp", strconv.FormatInt(s.EndTimestamp, 10))
	var out perpsDataResponse[PerpsFundingRate]
	if err := c.getJSON(ctx, fundingEndpoint, query, &out); err != nil {
		return nil, false, nil, err
	}
	var last *PerpsFundingRate
	if n := len(out.Data); n > 0 {
		last = &out.Data[n-1]
	}
	return out.Data, out.More, last, nil
}

// ---- Cursor state + helpers ----

type candlesCursor struct {
	Kind           string             `json:"kind"`
	InstrumentID   int                `json:"instrumentId"`
	Interval       PerpsKlineInterval `json:"interval"`
	StartTimestamp int64              `json:"startTimestamp"`
	EndTimestamp   int64              `json:"endTimestamp"`
}

type fundingCursor struct {
	Kind           string `json:"kind"`
	InstrumentID   int    `json:"instrumentId"`
	StartTimestamp int64  `json:"startTimestamp"`
	EndTimestamp   int64  `json:"endTimestamp"`
}

type tradesCursor struct {
	Kind           string  `json:"kind"`
	InstrumentID   int     `json:"instrumentId"`
	StartTimestamp int64   `json:"startTimestamp"`
	EndTimestamp   int64   `json:"endTimestamp"`
	SeenTradeIDs   []int64 `json:"seenTradeIds"`
}

func candlesState(p CandlesParams) (candlesCursor, error) {
	if p.Cursor != "" {
		var s candlesCursor
		if err := decodeCursor(p.Cursor, &s); err != nil {
			return candlesCursor{}, err
		}
		return s, nil
	}
	now := time.Now().UnixMilli()
	start := p.Start
	if start == 0 {
		start = now - 24*60*60*1000
	}
	end := p.End
	if end == 0 {
		end = now
	}
	return candlesCursor{
		Kind:           "perpsCandles",
		InstrumentID:   p.InstrumentID,
		Interval:       p.Interval,
		StartTimestamp: start,
		EndTimestamp:   end,
	}, nil
}

func fundingState(p FundingParams) (fundingCursor, error) {
	if p.Cursor != "" {
		var s fundingCursor
		if err := decodeCursor(p.Cursor, &s); err != nil {
			return fundingCursor{}, err
		}
		return s, nil
	}
	now := time.Now().UnixMilli()
	start := p.Start
	if start == 0 {
		start = now - 24*60*60*1000
	}
	end := p.End
	if end == 0 {
		end = now
	}
	return fundingCursor{
		Kind:           "perpsFundingHistory",
		InstrumentID:   p.InstrumentID,
		StartTimestamp: start,
		EndTimestamp:   end,
	}, nil
}

func tradesState(p TradesParams) (tradesCursor, error) {
	if p.Cursor != "" {
		var s tradesCursor
		if err := decodeCursor(p.Cursor, &s); err != nil {
			return tradesCursor{}, err
		}
		return s, nil
	}
	now := time.Now().UnixMilli()
	start := p.Start
	if start == 0 {
		start = now - 24*60*60*1000
	}
	end := p.End
	if end == 0 {
		end = now
	}
	return tradesCursor{
		Kind:           "perpsTrades",
		InstrumentID:   p.InstrumentID,
		StartTimestamp: start,
		EndTimestamp:   end,
	}, nil
}

func encodeCursor(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

func decodeCursor(s string, v any) error {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return fmt.Errorf("perps: invalid cursor: %w", err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("perps: invalid cursor: %w", err)
	}
	return nil
}

func hasSeen(ids []int64, target int64) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func klineIntervalMs(interval PerpsKlineInterval) int64 {
	switch interval {
	case PerpsKline1s:
		return 1000
	case PerpsKline1m:
		return 60 * 1000
	case PerpsKline5m:
		return 5 * 60 * 1000
	case PerpsKline15m:
		return 15 * 60 * 1000
	case PerpsKline1h:
		return 60 * 60 * 1000
	case PerpsKline4h:
		return 4 * 60 * 60 * 1000
	case PerpsKline1d:
		return 24 * 60 * 60 * 1000
	case PerpsKline1w:
		return 7 * 24 * 60 * 60 * 1000
	default:
		return 60 * 1000
	}
}
