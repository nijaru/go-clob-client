package perps

// PerpsCategory is the market category of a perps instrument.
type PerpsCategory string

const (
	PerpsCategoryEquity    PerpsCategory = "equity"
	PerpsCategoryCommodity PerpsCategory = "commodity"
	PerpsCategoryIndex     PerpsCategory = "index"
	PerpsCategoryCrypto    PerpsCategory = "crypto"
)

// PerpsSide is the position side of a perps trade or order.
type PerpsSide string

const (
	PerpsSideLong  PerpsSide = "long"
	PerpsSideShort PerpsSide = "short"
)

// PerpsTimeInForce is the time-in-force of a perps order.
type PerpsTimeInForce string

const (
	PerpsTIFGTC PerpsTimeInForce = "gtc"
	PerpsTIFIOC PerpsTimeInForce = "ioc"
	PerpsTIFFOK PerpsTimeInForce = "fok"
)

// PerpsKlineInterval is the candle interval for perps klines.
type PerpsKlineInterval string

const (
	PerpsKline1s  PerpsKlineInterval = "1s"
	PerpsKline1m  PerpsKlineInterval = "1m"
	PerpsKline5m  PerpsKlineInterval = "5m"
	PerpsKline15m PerpsKlineInterval = "15m"
	PerpsKline1h  PerpsKlineInterval = "1h"
	PerpsKline4h  PerpsKlineInterval = "4h"
	PerpsKline1d  PerpsKlineInterval = "1d"
	PerpsKline1w  PerpsKlineInterval = "1w"
)

// PerpsBookDepth is the number of price levels returned per side of a book.
type PerpsBookDepth int

const (
	PerpsBookDepth10   PerpsBookDepth = 10
	PerpsBookDepth100  PerpsBookDepth = 100
	PerpsBookDepth500  PerpsBookDepth = 500
	PerpsBookDepth1000 PerpsBookDepth = 1000
)

// PerpsInstrument is a tradable perps instrument.
type PerpsInstrument struct {
	ID                int             `json:"instrument_id"`
	Category          PerpsCategory   `json:"category"`
	Symbol            string          `json:"symbol"`
	BaseAsset         string          `json:"base_asset"`
	QuoteAsset        string          `json:"quote_asset"`
	FundingInterval   string          `json:"funding_interval"`
	QuantityDecimals  int             `json:"quantity_decimals"`
	PriceDecimals     int             `json:"price_decimals"`
	PriceBounds       string          `json:"price_bounds"`
	LiquidationFee    string          `json:"liquidation_fee"`
	MaxOrderCount     int             `json:"max_order_count"`
	MinNotional       string          `json:"min_notional"`
	MaxMarketNotional string          `json:"max_market_notional"`
	MaxLimitNotional  string          `json:"max_limit_notional"`
	MaxLeverage       int             `json:"max_leverage"`
	RiskTiers         []PerpsRiskTier `json:"risk_tiers"`
}

// PerpsRiskTier maps a notional lower bound to a maximum allowed leverage.
type PerpsRiskTier struct {
	LowerBound  string `json:"lower_bound"`
	MaxLeverage int    `json:"max_leverage"`
}

// PerpsTicker is the current market state of an instrument. OpenPrice and
// Volume24h are populated only by GetTicker(s), which joins the statistics feed.
type PerpsTicker struct {
	InstrumentID int    `json:"instrument_id"`
	Symbol       string `json:"symbol"`
	IndexPrice   string `json:"index_price"`
	MarkPrice    string `json:"mark_price"`
	LastPrice    string `json:"last_price"`
	MidPrice     string `json:"mid_price"`
	OpenInterest string `json:"open_interest"`
	FundingRate  string `json:"funding_rate"`
	NextFunding  int64  `json:"next_funding"`
	Timestamp    int64  `json:"timestamp,omitempty"`

	OpenPrice string `json:"open_price,omitempty"`
	Volume24h string `json:"volume,omitempty"`
}

// PerpsStatistic is a 24h statistics entry, joined into tickers for open price
// and volume. It also carries recent klines.
type PerpsStatistic struct {
	InstrumentID int           `json:"instrument_id"`
	Symbol       string        `json:"symbol,omitempty"`
	Volume       string        `json:"volume"`
	OpenPrice    string        `json:"open_price"`
	Klines       []PerpsCandle `json:"klines"`
}

// PerpsBookLevel is a single price level in a perps order book.
type PerpsBookLevel struct {
	Price    string `json:"price"`
	Quantity string `json:"quantity"`
}

// PerpsBook is a perps order book snapshot.
type PerpsBook struct {
	InstrumentID int              `json:"instrument_id"`
	Bids         []PerpsBookLevel `json:"bids"`
	Asks         []PerpsBookLevel `json:"asks"`
	Timestamp    int64            `json:"timestamp"`
	Sequence     int              `json:"sequence"`
}

// PerpsCandle is a single OHLCV candle.
type PerpsCandle struct {
	Timestamp int64  `json:"timestamp"`
	Open      string `json:"open"`
	High      string `json:"high"`
	Low       string `json:"low"`
	Close     string `json:"close"`
	Volume    string `json:"volume"`
	Trades    int    `json:"trades"`
}

// PerpsPublicTrade is a single public trade print.
type PerpsPublicTrade struct {
	TradeID      int64     `json:"trade_id"`
	InstrumentID int       `json:"instrument_id"`
	Side         PerpsSide `json:"side"`
	Price        string    `json:"price"`
	Quantity     string    `json:"quantity"`
	Timestamp    int64     `json:"timestamp"`
	Hash         string    `json:"hash"`
}

// PerpsFundingRate is a single funding-rate sample.
type PerpsFundingRate struct {
	FundingRate string `json:"funding_rate"`
	Timestamp   int64  `json:"timestamp"`
}

// PerpsFeeScheduleEntry is a taker/maker fee rate for a category.
type PerpsFeeScheduleEntry struct {
	Category     PerpsCategory `json:"category"`
	TakerFeeRate string        `json:"taker_fee_rate"`
	MakerFeeRate string        `json:"maker_fee_rate"`
}

// PerpsFeesInfo is the fee schedule response wrapper.
type PerpsFeesInfo struct {
	FeeSchedule []PerpsFeeScheduleEntry `json:"fee_schedule"`
}

// perpsDataResponse is the paginated list envelope used by candles, trades, and
// funding-history endpoints.
type perpsDataResponse[T any] struct {
	Data []T  `json:"data"`
	More bool `json:"more"`
}

// ---- Request parameters ----

// InstrumentsParams filters the instruments list.
type InstrumentsParams struct {
	InstrumentID *int           `url:"instrument_id,omitempty"`
	Category     *PerpsCategory `url:"category,omitempty"`
}

// TickerParams selects a single instrument's ticker.
type TickerParams struct {
	InstrumentID int `url:"instrument_id"`
}

// TickersParams filters the tickers list.
type TickersParams struct {
	InstrumentID *int `url:"instrument_id,omitempty"`
}

// BookParams selects a book snapshot and depth.
type BookParams struct {
	InstrumentID int            `url:"instrument_id"`
	Depth        PerpsBookDepth `url:"depth"`
}

// CandlesParams lists candles for an instrument with optional time bounds.
// Cursor is an opaque pagination token returned by a previous page; when set,
// Start/End are ignored.
type CandlesParams struct {
	InstrumentID int                `url:"instrument_id"`
	Interval     PerpsKlineInterval `url:"interval"`
	Start        int64              `url:"start_timestamp,omitempty"`
	End          int64              `url:"end_timestamp,omitempty"`
	Cursor       string             `url:"-"`
}

// TradesParams lists public trades for an instrument with optional time bounds.
type TradesParams struct {
	InstrumentID int    `url:"instrument_id"`
	Start        int64  `url:"start_timestamp,omitempty"`
	End          int64  `url:"end_timestamp,omitempty"`
	Cursor       string `url:"-"`
}

// FundingParams lists funding-rate history for an instrument with optional bounds.
type FundingParams struct {
	InstrumentID int    `url:"instrument_id"`
	Start        int64  `url:"start_timestamp,omitempty"`
	End          int64  `url:"end_timestamp,omitempty"`
	Cursor       string `url:"-"`
}
