package perps

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// PerpsCredentials are delegated credentials issued by the Perps account
// service. Keep the secret private; it authorizes account reads and sessions.
type PerpsCredentials struct {
	Proxy      string `json:"proxy"`
	Secret     string `json:"secret"`
	PrivateKey string `json:"private_key,omitempty"`
	ExpiresAt  int64  `json:"expires_at"`
}

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

// PerpsOrderStatus is the server lifecycle status of an authenticated order.
type PerpsOrderStatus string

const (
	PerpsOrderAccepted                   PerpsOrderStatus = "accepted"
	PerpsOrderOpen                       PerpsOrderStatus = "open"
	PerpsOrderPartial                    PerpsOrderStatus = "partial"
	PerpsOrderFilled                     PerpsOrderStatus = "filled"
	PerpsOrderCancelled                  PerpsOrderStatus = "cancelled"
	PerpsOrderAutoCancelled              PerpsOrderStatus = "auto_cancelled"
	PerpsOrderPostOnlyRejected           PerpsOrderStatus = "post_only_rejected"
	PerpsOrderFOKUnfilled                PerpsOrderStatus = "fok_unfilled"
	PerpsOrderIOCNoFill                  PerpsOrderStatus = "ioc_no_fill"
	PerpsOrderIOCExpired                 PerpsOrderStatus = "ioc_expired"
	PerpsOrderSTPCancelled               PerpsOrderStatus = "stp_cancelled"
	PerpsOrderZeroQuantity               PerpsOrderStatus = "zero_quantity"
	PerpsOrderDuplicate                  PerpsOrderStatus = "duplicate_order"
	PerpsOrderNotFound                   PerpsOrderStatus = "order_not_found"
	PerpsOrderReduceOnlyInvalid          PerpsOrderStatus = "reduce_only_invalid"
	PerpsOrderReduceOnlyExpired          PerpsOrderStatus = "reduce_only_expired"
	PerpsOrderExpired                    PerpsOrderStatus = "order_expired"
	PerpsOrderUntriggered                PerpsOrderStatus = "untriggered"
	PerpsOrderArmed                      PerpsOrderStatus = "armed"
	PerpsOrderTriggered                  PerpsOrderStatus = "triggered"
	PerpsOrderParentCancelled            PerpsOrderStatus = "parent_cancelled"
	PerpsOrderPositionClosed             PerpsOrderStatus = "position_closed"
	PerpsOrderPositionFlipped            PerpsOrderStatus = "position_flipped"
	PerpsOrderReduceOnlyInvalidAtTrigger PerpsOrderStatus = "reduce_only_invalid_at_trigger"
	PerpsOrderStatusExpired              PerpsOrderStatus = "expired"
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
	IsolatedOnly      bool            `json:"isolated_only"`
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

// UnmarshalJSON accepts the tuple wire format used by the official perps API
// and the object form emitted by older local fixtures.
func (l *PerpsBookLevel) UnmarshalJSON(data []byte) error {
	if isJSONArray(data) {
		var tuple []string
		if err := json.Unmarshal(data, &tuple); err != nil {
			return fmt.Errorf("perps book level tuple: %w", err)
		}
		if len(tuple) != 2 {
			return fmt.Errorf("perps book level tuple: got %d values, want 2", len(tuple))
		}
		l.Price, l.Quantity = tuple[0], tuple[1]
		return nil
	}

	type alias PerpsBookLevel
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*l = PerpsBookLevel(value)
	return nil
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

// UnmarshalJSON accepts the seven-element tuple used by the official perps
// API and the object form retained for compatibility with older callers.
func (c *PerpsCandle) UnmarshalJSON(data []byte) error {
	if isJSONArray(data) {
		var tuple []json.RawMessage
		if err := json.Unmarshal(data, &tuple); err != nil {
			return fmt.Errorf("perps candle tuple: %w", err)
		}
		if len(tuple) != 7 {
			return fmt.Errorf("perps candle tuple: got %d values, want 7", len(tuple))
		}
		var value PerpsCandle
		decode := func(index int, target any) error {
			if err := json.Unmarshal(tuple[index], target); err != nil {
				return fmt.Errorf("value %d: %w", index, err)
			}
			return nil
		}
		if err := decode(0, &value.Timestamp); err != nil {
			return fmt.Errorf("perps candle tuple: %w", err)
		}
		if err := decode(1, &value.Open); err != nil {
			return fmt.Errorf("perps candle tuple: %w", err)
		}
		if err := decode(2, &value.High); err != nil {
			return fmt.Errorf("perps candle tuple: %w", err)
		}
		if err := decode(3, &value.Low); err != nil {
			return fmt.Errorf("perps candle tuple: %w", err)
		}
		if err := decode(4, &value.Close); err != nil {
			return fmt.Errorf("perps candle tuple: %w", err)
		}
		if err := decode(5, &value.Volume); err != nil {
			return fmt.Errorf("perps candle tuple: %w", err)
		}
		if err := decode(6, &value.Trades); err != nil {
			return fmt.Errorf("perps candle tuple: %w", err)
		}
		*c = value
		return nil
	}

	type alias PerpsCandle
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*c = PerpsCandle(value)
	return nil
}

func isJSONArray(data []byte) bool {
	data = bytes.TrimSpace(data)
	return len(data) > 0 && data[0] == '['
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

// PerpsFeeTier is a volume-based maker/taker fee tier for a category.
type PerpsFeeTier struct {
	MinVolume30D string `json:"min_volume_30d"`
	TakerFeeRate string `json:"taker_fee_rate"`
	MakerFeeRate string `json:"maker_fee_rate"`
}

// PerpsFeeScheduleEntry is a taker/maker fee schedule for a category.
type PerpsFeeScheduleEntry struct {
	Category     PerpsCategory  `json:"category"`
	TakerFeeRate string         `json:"taker_fee_rate"`
	MakerFeeRate string         `json:"maker_fee_rate"`
	Tiers        []PerpsFeeTier `json:"tiers"`
}

// PerpsFeesInfo is the fee schedule response wrapper.
type PerpsFeesInfo struct {
	FeeSchedule []PerpsFeeScheduleEntry `json:"fee_schedule"`
}

// PerpsBalance is an authenticated account balance entry.
type PerpsBalance struct {
	Asset   string `json:"asset"`
	Balance string `json:"balance"`
	Value   string `json:"value"`
}

// PerpsAccountStats contains account-level seven-day volume statistics.
type PerpsAccountStats struct {
	Volume7d            string `json:"volume_7d"`
	TakerVolume7d       string `json:"taker_volume_7d"`
	MakerVolume7d       string `json:"maker_volume_7d"`
	AccountMakerShare7d string `json:"account_maker_share_7d"`
	EntityMakerShare7d  string `json:"entity_maker_share_7d,omitempty"`
	EntityID            int    `json:"entity_id,omitempty"`
	EntityName          string `json:"entity_name,omitempty"`
}

// PerpsPortfolioPosition is an open position in the authenticated portfolio.
type PerpsPortfolioPosition struct {
	InstrumentID      int    `json:"instrument_id"`
	Symbol            string `json:"symbol"`
	Size              string `json:"size"`
	EntryPrice        string `json:"entry_price"`
	Leverage          int    `json:"leverage"`
	Cross             bool   `json:"cross"`
	InitialMargin     string `json:"initial_margin"`
	MaintenanceMargin string `json:"maintenance_margin"`
	PositionValue     string `json:"position_value"`
	LiquidationPrice  string `json:"liquidation_price"`
	UnrealizedPnL     string `json:"unrealized_pnl"`
	ReturnOnEquity    string `json:"return_on_equity"`
	CumulativeFunding string `json:"cumulative_funding"`
}

// PerpsMarginSummary summarizes portfolio margin usage.
type PerpsMarginSummary struct {
	TotalAccountValue      string `json:"total_account_value"`
	TotalInitialMargin     string `json:"total_initial_margin"`
	TotalMaintenanceMargin string `json:"total_maintenance_margin"`
	TotalPositionValue     string `json:"total_position_value"`
}

// PerpsPortfolio is the authenticated account portfolio snapshot.
type PerpsPortfolio struct {
	Positions     []PerpsPortfolioPosition `json:"positions"`
	Margin        PerpsMarginSummary       `json:"margin"`
	Withdrawable  string                   `json:"withdrawable"`
	InLiquidation bool                     `json:"in_liquidation"`
	Timestamp     int64                    `json:"timestamp"`
}

// PerpsAccountConfig is leverage and margin mode for one instrument.
type PerpsAccountConfig struct {
	InstrumentID int  `json:"instrument_id"`
	Leverage     int  `json:"leverage"`
	Cross        bool `json:"cross"`
}

// PerpsOrder is an authenticated order returned by the account API.
type PerpsOrder struct {
	ID               int              `json:"order_id"`
	InstrumentID     int              `json:"instrument_id"`
	Buy              bool             `json:"buy"`
	Price            string           `json:"price"`
	Quantity         string           `json:"quantity"`
	TimeInForce      PerpsTimeInForce `json:"tif"`
	PostOnly         bool             `json:"post_only"`
	ReduceOnly       bool             `json:"ro"`
	Status           PerpsOrderStatus `json:"status"`
	RestingQuantity  string           `json:"resting_quantity"`
	FilledQuantity   string           `json:"filled_quantity"`
	CreatedTimestamp int64            `json:"created_timestamp"`
	UpdatedTimestamp int64            `json:"updated_timestamp"`
	ClientOrderID    string           `json:"client_order_id,omitempty"`
}

// PerpsPage is the wire page returned by authenticated history endpoints.
type PerpsPage[T any] struct {
	Data []T  `json:"data"`
	More bool `json:"more"`
}

// PerpsAccountFill is an authenticated trade fill.
type PerpsAccountFill struct {
	TradeID            int64     `json:"trade_id"`
	OrderID            int       `json:"order_id"`
	InstrumentID       int       `json:"instrument_id"`
	Side               PerpsSide `json:"side"`
	Price              string    `json:"price"`
	Quantity           string    `json:"quantity"`
	Taker              bool      `json:"taker"`
	Fee                string    `json:"fee"`
	FeeAsset           string    `json:"fee_asset"`
	PreviousSize       string    `json:"previous_size"`
	PreviousEntryPrice string    `json:"previous_entry_price"`
	PnL                string    `json:"pnl"`
	Liquidation        bool      `json:"liquidation"`
	Timestamp          int64     `json:"timestamp"`
	Hash               string    `json:"hash"`
}

// PerpsAccountFundingPayment is an account funding payment entry.
type PerpsAccountFundingPayment struct {
	InstrumentID int    `json:"instrument_id"`
	Size         string `json:"size"`
	FundingRate  string `json:"funding_rate"`
	FundingAsset string `json:"funding_asset"`
	Funding      string `json:"funding"`
	Timestamp    int64  `json:"timestamp"`
}

// PerpsDepositStatus is the lifecycle state of a collateral deposit.
type PerpsDepositStatus string

const (
	PerpsDepositPending   PerpsDepositStatus = "pending"
	PerpsDepositConfirmed PerpsDepositStatus = "confirmed"
	PerpsDepositRemoved   PerpsDepositStatus = "removed"
)

// PerpsDeposit is an authenticated collateral deposit entry.
type PerpsDeposit struct {
	Hash                  string             `json:"hash"`
	Asset                 string             `json:"asset"`
	Amount                string             `json:"amount"`
	Status                PerpsDepositStatus `json:"status"`
	From                  string             `json:"from"`
	To                    string             `json:"to"`
	Confirmations         int                `json:"confirmations"`
	RequiredConfirmations int                `json:"required_confirmations"`
	CreatedTimestamp      int64              `json:"created_timestamp"`
	ConfirmedTimestamp    int64              `json:"confirmed_timestamp,omitempty"`
}

// PerpsWithdrawalStatus is the lifecycle state of a collateral withdrawal.
type PerpsWithdrawalStatus string

const (
	PerpsWithdrawalPending   PerpsWithdrawalStatus = "pending"
	PerpsWithdrawalConfirmed PerpsWithdrawalStatus = "confirmed"
	PerpsWithdrawalRemoved   PerpsWithdrawalStatus = "removed"
	PerpsWithdrawalFailed    PerpsWithdrawalStatus = "failed"
)

// PerpsWithdrawal is an authenticated collateral withdrawal entry.
type PerpsWithdrawal struct {
	WithdrawalID          int                   `json:"withdraw_id"`
	Asset                 string                `json:"asset"`
	Amount                string                `json:"amount"`
	Fee                   string                `json:"fee"`
	Status                PerpsWithdrawalStatus `json:"status"`
	To                    string                `json:"to"`
	Hash                  string                `json:"hash"`
	Confirmations         int                   `json:"confirmations"`
	RequiredConfirmations int                   `json:"required_confirmations"`
	CreatedTimestamp      int64                 `json:"created_timestamp"`
	ConfirmedTimestamp    int64                 `json:"confirmed_timestamp,omitempty"`
}

// PerpsPnlInterval selects an equity or PnL history interval.
type PerpsPnlInterval string

const (
	PerpsPnl1h PerpsPnlInterval = "1h"
	PerpsPnl4h PerpsPnlInterval = "4h"
	PerpsPnl1d PerpsPnlInterval = "1d"
	PerpsPnl1w PerpsPnlInterval = "1w"
)

// PerpsEquityPoint is a timestamped equity history sample.
type PerpsEquityPoint struct {
	Timestamp int64
	Equity    string
}

// UnmarshalJSON decodes the official [timestamp, equity] tuple.
func (p *PerpsEquityPoint) UnmarshalJSON(data []byte) error {
	var tuple []json.RawMessage
	if err := json.Unmarshal(data, &tuple); err != nil {
		return fmt.Errorf("perps equity point: %w", err)
	}
	if len(tuple) != 2 {
		return fmt.Errorf("perps equity point: got %d values, want 2", len(tuple))
	}
	if err := json.Unmarshal(tuple[0], &p.Timestamp); err != nil {
		return fmt.Errorf("perps equity point timestamp: %w", err)
	}
	if err := json.Unmarshal(tuple[1], &p.Equity); err != nil {
		return fmt.Errorf("perps equity point value: %w", err)
	}
	return nil
}

// PerpsPnlPoint is a timestamped PnL history sample.
type PerpsPnlPoint struct {
	Timestamp int64
	PnL       string
}

// UnmarshalJSON decodes the official [timestamp, pnl] tuple.
func (p *PerpsPnlPoint) UnmarshalJSON(data []byte) error {
	var tuple []json.RawMessage
	if err := json.Unmarshal(data, &tuple); err != nil {
		return fmt.Errorf("perps pnl point: %w", err)
	}
	if len(tuple) != 2 {
		return fmt.Errorf("perps pnl point: got %d values, want 2", len(tuple))
	}
	if err := json.Unmarshal(tuple[0], &p.Timestamp); err != nil {
		return fmt.Errorf("perps pnl point timestamp: %w", err)
	}
	if err := json.Unmarshal(tuple[1], &p.PnL); err != nil {
		return fmt.Errorf("perps pnl point value: %w", err)
	}
	return nil
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
