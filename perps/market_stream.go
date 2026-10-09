package perps

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type MarketTopic string

const (
	MarketTrades     MarketTopic = "trades"
	MarketBBO        MarketTopic = "bbo"
	MarketBook       MarketTopic = "book"
	MarketTickers    MarketTopic = "tickers"
	MarketStatistics MarketTopic = "statistics"
	MarketCandles    MarketTopic = "klines"
)

// MarketSubscription selects one public topic. A nil instrument selects all
// only for tickers/statistics. WS candles exclude the REST-only 1s interval.
type MarketSubscription struct {
	Topic        MarketTopic
	InstrumentID *int
	Interval     PerpsKlineInterval
}

func (p MarketSubscription) channel() (string, error) {
	if p.InstrumentID != nil && !validInstrumentID(*p.InstrumentID) {
		return "", fmt.Errorf("perps: invalid subscription instrument")
	}
	id := "all"
	if p.InstrumentID != nil {
		id = strconv.Itoa(*p.InstrumentID)
	}
	switch p.Topic {
	case MarketTickers, MarketStatistics:
	case MarketTrades, MarketBBO, MarketBook, MarketCandles:
		if p.InstrumentID == nil {
			return "", fmt.Errorf("perps: subscription requires an instrument")
		}
	default:
		return "", fmt.Errorf("perps: invalid public topic %q", p.Topic)
	}
	channel := string(p.Topic) + "::" + id
	if p.Topic == MarketCandles {
		switch p.Interval {
		case PerpsKline1m,
			PerpsKline5m,
			PerpsKline15m,
			PerpsKline1h,
			PerpsKline4h,
			PerpsKline1d,
			PerpsKline1w:
		default:
			return "", fmt.Errorf("perps: invalid WS candle interval")
		}
		channel += "::" + string(p.Interval)
	} else if p.Interval != "" {
		return "", fmt.Errorf("perps: interval applies only to candles")
	}
	return channel, nil
}

type PerpsBBO struct {
	InstrumentID int    `json:"iid"`
	BidPrice     string `json:"bp"`
	BidQuantity  string `json:"bq"`
	AskPrice     string `json:"ap"`
	AskQuantity  string `json:"aq"`
}

// MarketEvent holds exactly one topic payload, using the same semantic types as
// REST reads. Book events are deltas, not reconstructed snapshots. Reconnect or
// sequence-gap events require a fresh REST book before applying more deltas.
type MarketEvent struct {
	Topic        MarketTopic
	InstrumentID int
	Interval     PerpsKlineInterval
	Trades       []PerpsPublicTrade
	BBO          *PerpsBBO
	Book         *PerpsBook
	Ticker       *PerpsTicker
	Statistic    *PerpsStatistic
	Candles      []PerpsCandle
}

func (e PerpsSessionEvent) AsMarket() (*MarketEvent, error) {
	if e.Market != nil {
		return e.Market, nil
	}
	parts := strings.Split(e.Channel, "::")
	if len(parts) < 2 {
		return nil, fmt.Errorf("perps: not a public market event")
	}
	out := MarketEvent{Topic: MarketTopic(parts[0])}
	if parts[1] != "all" {
		id, err := strconv.ParseUint(parts[1], 10, 32)
		if err != nil {
			return nil, err
		}
		out.InstrumentID = int(id)
	}
	switch out.Topic {
	case MarketTrades:
		var wire []struct {
			ID         int64     `json:"tid"`
			IID        int       `json:"iid"`
			Side       PerpsSide `json:"side"`
			Settlement bool      `json:"settlement"`
			Price      string    `json:"p"`
			Quantity   string    `json:"qty"`
			Timestamp  int64     `json:"ts"`
			Hash       string    `json:"hash"`
		}
		if err := json.Unmarshal(e.Data, &wire); err != nil {
			return nil, err
		}
		out.Trades = make([]PerpsPublicTrade, len(wire))
		for i, t := range wire {
			out.Trades[i] = PerpsPublicTrade{
				TradeID:      t.ID,
				InstrumentID: t.IID,
				Side:         t.Side,
				Settlement:   t.Settlement,
				Price:        t.Price,
				Quantity:     t.Quantity,
				Timestamp:    t.Timestamp,
				Hash:         t.Hash,
			}
		}
	case MarketBBO:
		var value PerpsBBO
		if err := json.Unmarshal(e.Data, &value); err != nil {
			return nil, err
		}
		out.BBO = &value
		out.InstrumentID = value.InstrumentID
	case MarketBook:
		var wire struct {
			Bids []PerpsBookLevel `json:"b"`
			Asks []PerpsBookLevel `json:"a"`
		}
		if err := json.Unmarshal(e.Data, &wire); err != nil {
			return nil, err
		}
		out.Book = &PerpsBook{
			InstrumentID: out.InstrumentID,
			Bids:         wire.Bids,
			Asks:         wire.Asks,
			Timestamp:    e.Timestamp,
			Sequence:     int(e.Sequence),
		}
	case MarketTickers:
		var wire struct {
			IID   int    `json:"iid"`
			Index string `json:"idx"`
			Mark  string `json:"mark"`
			Last  string `json:"last"`
			Mid   string `json:"mid"`
			OI    string `json:"oi"`
			FR    string `json:"fr"`
			Next  int64  `json:"nxf"`
		}
		if err := json.Unmarshal(e.Data, &wire); err != nil {
			return nil, err
		}
		out.InstrumentID = wire.IID
		out.Ticker = &PerpsTicker{
			InstrumentID: wire.IID,
			IndexPrice:   wire.Index,
			MarkPrice:    wire.Mark,
			LastPrice:    wire.Last,
			MidPrice:     wire.Mid,
			OpenInterest: wire.OI,
			FundingRate:  wire.FR,
			NextFunding:  wire.Next,
			Timestamp:    e.Timestamp,
		}
	case MarketStatistics:
		var wire struct {
			IID    int           `json:"iid"`
			Volume string        `json:"vol"`
			Open   string        `json:"open"`
			Klines []PerpsCandle `json:"klines"`
		}
		if err := json.Unmarshal(e.Data, &wire); err != nil {
			return nil, err
		}
		out.InstrumentID = wire.IID
		out.Statistic = &PerpsStatistic{
			InstrumentID: wire.IID,
			Volume:       wire.Volume,
			OpenPrice:    wire.Open,
			Klines:       wire.Klines,
		}
	case MarketCandles:
		if len(parts) != 3 {
			return nil, fmt.Errorf("perps: candle event missing interval")
		}
		out.Interval = PerpsKlineInterval(parts[2])
		if err := json.Unmarshal(e.Data, &out.Candles); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("perps: not a public market event")
	}
	return &out, nil
}
