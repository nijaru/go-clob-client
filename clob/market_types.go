package clob

import (
	stdjson "encoding/json" //nolint:depguard // compact CLOB market wire decoding
	"fmt"

	"github.com/quagmt/udecimal"
)

// Market is the typed response for a full market record.
type Market struct {
	EnableOrderBook      bool           `json:"enable_order_book"`
	Active               bool           `json:"active"`
	Closed               bool           `json:"closed"`
	Archived             bool           `json:"archived"`
	AcceptingOrders      bool           `json:"accepting_orders"`
	AcceptingOrderTime   *string        `json:"accepting_order_timestamp"`
	MinimumOrderSize     string         `json:"minimum_order_size"`
	MinimumTickSize      string         `json:"minimum_tick_size"`
	ConditionID          *string        `json:"condition_id"`
	QuestionID           *string        `json:"question_id"`
	Question             string         `json:"question"`
	Description          string         `json:"description"`
	MarketSlug           string         `json:"market_slug"`
	EndDateISO           *string        `json:"end_date_iso"`
	GameStartTime        *string        `json:"game_start_time"`
	SecondsDelay         int64          `json:"seconds_delay"`
	FPMM                 *string        `json:"fpmm"`
	MakerBaseFee         string         `json:"maker_base_fee"`
	TakerBaseFee         string         `json:"taker_base_fee"`
	NotificationsEnabled bool           `json:"notifications_enabled"`
	NegRisk              bool           `json:"neg_risk"`
	NegRiskMarketID      *string        `json:"neg_risk_market_id"`
	NegRiskRequestID     *string        `json:"neg_risk_request_id"`
	Icon                 string         `json:"icon"`
	Image                string         `json:"image"`
	Rewards              Rewards        `json:"rewards"`
	IsFiftyFiftyOutcome  bool           `json:"is_50_50_outcome"`
	Tokens               []OutcomeToken `json:"tokens"`
	Tags                 []string       `json:"tags"`
}

// SimplifiedMarket is a compact market representation used by sampling endpoints.
type SimplifiedMarket struct {
	ConditionID     *string        `json:"condition_id"`
	Tokens          []OutcomeToken `json:"tokens"`
	Rewards         Rewards        `json:"rewards"`
	Active          bool           `json:"active"`
	Closed          bool           `json:"closed"`
	Archived        bool           `json:"archived"`
	AcceptingOrders bool           `json:"accepting_orders"`
}

// OutcomeToken is a market token and its current outcome metadata.
type OutcomeToken struct {
	TokenID string `json:"token_id"`
	Outcome string `json:"outcome"`
	Price   string `json:"price"`
	Winner  bool   `json:"winner"`
}

// RewardRate is a reward rate entry embedded in market responses.
type RewardRate struct {
	AssetAddress     string `json:"asset_address"`
	RewardsDailyRate string `json:"rewards_daily_rate"`
}

// Rewards is the rewards summary embedded in market responses.
type Rewards struct {
	Rates     []RewardRate `json:"rates"`
	MinSize   string       `json:"min_size"`
	MaxSpread string       `json:"max_spread"`
}

// BookParams identifies a token whose order-book-derived values should be fetched.
// Side is optional; when set it filters price results to the specified side.
type BookParams struct {
	TokenID string `json:"token_id"`
	Side    Side   `json:"side,omitzero"`
}

// OrderSummary is a single order book level.
type OrderSummary struct {
	Price string `json:"price"`
	Size  string `json:"size"`
}

// OrderBookSummary is the typed response from the order book endpoint.
type OrderBookSummary struct {
	Market         string         `json:"market"`
	AssetID        string         `json:"asset_id"`
	Timestamp      string         `json:"timestamp"`
	Bids           []OrderSummary `json:"bids"`
	Asks           []OrderSummary `json:"asks"`
	MinOrderSize   string         `json:"min_order_size"`
	TickSize       string         `json:"tick_size"`
	NegRisk        bool           `json:"neg_risk"`
	LastTradePrice string         `json:"last_trade_price"`
	Hash           string         `json:"hash"`
}

// TickSizeResponse reports the minimum supported market tick size.
type TickSizeResponse struct {
	MinimumTickSize TickSize `json:"minimum_tick_size"`
}

// NegRiskResponse reports whether a token trades on a neg-risk market.
type NegRiskResponse struct {
	NegRisk bool `json:"neg_risk"`
}

// FeeInfo holds V2 fee parameters for a market.
// Fee is applied as: Rate * (price * (1 - price))^Exponent.
type FeeInfo struct {
	Rate     float64 `json:"rate"`
	Exponent uint32  `json:"exponent"`
}

// FeeDetails is the wire format for fee info from /clob-markets.
type FeeDetails struct {
	Rate     float64 `json:"r"`
	Exponent uint32  `json:"e"`
}

// ClobMarketInfoResponse is the wire format for /clob-markets/{condition_id}.
type ClobMarketInfoResponse struct {
	ConditionID  string            `json:"c"`
	MinTickSize  string            `json:"mts"`
	MinOrderSize string            `json:"mos"`
	NegRisk      bool              `json:"nr"`
	FeeDetails   *FeeDetails       `json:"fd"`
	Tokens       []ClobMarketToken `json:"t"`
}

// UnmarshalJSON accepts the compact Rust/server wire shape, including numeric
// tick and minimum-size values.
func (r *ClobMarketInfoResponse) UnmarshalJSON(data []byte) error {
	var wire struct {
		ConditionID  string             `json:"c"`
		MinTickSize  stdjson.RawMessage `json:"mts"`
		MinOrderSize stdjson.RawMessage `json:"mos"`
		NegRisk      bool               `json:"nr"`
		FeeDetails   *FeeDetails        `json:"fd"`
		Tokens       []ClobMarketToken  `json:"t"`
	}
	if err := stdjson.Unmarshal(data, &wire); err != nil {
		return fmt.Errorf("clob market: decode object: %w", err)
	}
	minTickSize, err := decodeStringOrNumber(wire.MinTickSize)
	if err != nil {
		return fmt.Errorf("clob market mts: %w", err)
	}
	minOrderSize, err := decodeStringOrNumber(wire.MinOrderSize)
	if err != nil {
		return fmt.Errorf("clob market mos: %w", err)
	}
	*r = ClobMarketInfoResponse{
		ConditionID:  wire.ConditionID,
		MinTickSize:  minTickSize,
		MinOrderSize: minOrderSize,
		NegRisk:      wire.NegRisk,
		FeeDetails:   wire.FeeDetails,
		Tokens:       wire.Tokens,
	}
	return nil
}

// ClobMarketToken is a token entry in the /clob-markets response.
type ClobMarketToken struct {
	TokenID string `json:"t"`
	Outcome string `json:"o"`
}

// UnmarshalJSON accepts both compact server keys and the older long token key.
func (t *ClobMarketToken) UnmarshalJSON(data []byte) error {
	var fields map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(data, &fields); err != nil {
		return fmt.Errorf("clob market token: decode object: %w", err)
	}
	tokenID, err := decodeStringOrNumber(firstRaw(fields, "t", "token_id"))
	if err != nil {
		return fmt.Errorf("clob market token id: %w", err)
	}
	outcome, err := decodeStringOrNumber(firstRaw(fields, "o", "outcome"))
	if err != nil {
		return fmt.Errorf("clob market token outcome: %w", err)
	}
	*t = ClobMarketToken{TokenID: tokenID, Outcome: outcome}
	return nil
}

func firstRaw(fields map[string]stdjson.RawMessage, keys ...string) stdjson.RawMessage {
	for _, key := range keys {
		if value, ok := fields[key]; ok {
			return value
		}
	}
	return nil
}

func decodeStringOrNumber(raw stdjson.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var text string
	if err := stdjson.Unmarshal(raw, &text); err == nil {
		return text, nil
	}
	var number stdjson.Number
	if err := stdjson.Unmarshal(raw, &number); err != nil {
		return "", err
	}
	if number.String() == "" {
		return "", fmt.Errorf("empty number")
	}
	return number.String(), nil
}

// BuilderFeeRateResponse reports the maker and taker fee rates for a builder code.
type BuilderFeeRateResponse struct {
	BuilderMakerFeeRateBps uint32 `json:"builder_maker_fee_rate_bps"`
	BuilderTakerFeeRateBps uint32 `json:"builder_taker_fee_rate_bps"`
}

// FeeRateResponse reports the base fee rate in BPS for a token.
type FeeRateResponse struct {
	BaseFee uint32 `json:"base_fee"`
}

// MarketByTokenResponse identifies the condition and token pair for a market.
type MarketByTokenResponse struct {
	ConditionID      string `json:"condition_id"`
	PrimaryTokenID   string `json:"primary_token_id"`
	SecondaryTokenID string `json:"secondary_token_id"`
}

// MidpointResponse reports the mid-market price for a token.
type MidpointResponse struct {
	Mid string `json:"mid"`
}

// PriceResponse reports the best available price for a token on a specific side.
type PriceResponse struct {
	Price string `json:"price"`
}

// SpreadResponse is the typed spread response for a token.
type SpreadResponse struct {
	Spread string `json:"spread"`
}

// MidpointsResponse maps token IDs to midpoint prices.
type MidpointsResponse map[string]string

// PricesResponse maps token IDs to side-specific prices.
type PricesResponse map[string]map[Side]string

// SpreadsResponse maps token IDs to spreads.
type SpreadsResponse map[string]string

// LastTradePriceResponse reports the most recent trade price and side for a token.
type LastTradePriceResponse struct {
	Price string `json:"price"`
	Side  Side   `json:"side"`
}

// LastTradesPricesResponse reports the most recent trade price for a token in a batch response.
type LastTradesPricesResponse struct {
	TokenID string `json:"token_id"`
	Price   string `json:"price"`
	Side    Side   `json:"side"`
}

// GeoblockResponse reports whether the current client IP is geographically blocked.
type GeoblockResponse struct {
	Blocked bool   `json:"blocked"`
	IP      string `json:"ip"`
	Country string `json:"country"`
	Region  string `json:"region"`
}

// MarketPrice is a single point in a market price-history response.
type MarketPrice struct {
	T int64            `json:"t"`
	P udecimal.Decimal `json:"p"`
}

// PriceHistoryInterval controls the server-side time bucket for price-history queries.
type PriceHistoryInterval string

const (
	// PriceHistoryIntervalMax represents the maximum available price history.
	PriceHistoryIntervalMax PriceHistoryInterval = "max"
	// PriceHistoryIntervalOneWeek represents a 1-week price history interval.
	PriceHistoryIntervalOneWeek PriceHistoryInterval = "1w"
	// PriceHistoryIntervalOneDay represents a 1-day price history interval.
	PriceHistoryIntervalOneDay PriceHistoryInterval = "1d"
	// PriceHistoryIntervalSixHours represents a 6-hour price history interval.
	PriceHistoryIntervalSixHours PriceHistoryInterval = "6h"
	// PriceHistoryIntervalOneHour represents a 1-hour price history interval.
	PriceHistoryIntervalOneHour PriceHistoryInterval = "1h"
)

// PriceHistoryFilterParams filters price-history requests.
type PriceHistoryFilterParams struct {
	Market   string
	StartTs  int64
	EndTs    int64
	Fidelity int
	Interval PriceHistoryInterval
}

// MarketTradeEvent is a live market activity event.
type MarketTradeEvent struct {
	EventType       string                 `json:"event_type"`
	Market          MarketTradeEventMarket `json:"market"`
	User            MarketTradeEventUser   `json:"user"`
	Side            Side                   `json:"side"`
	Size            string                 `json:"size"`
	FeeRateBps      string                 `json:"fee_rate_bps"`
	Price           string                 `json:"price"`
	Outcome         string                 `json:"outcome"`
	OutcomeIndex    int                    `json:"outcome_index"`
	TransactionHash string                 `json:"transaction_hash"`
	Timestamp       string                 `json:"timestamp"`
}

// MarketTradeEventMarket is the market metadata embedded in a trade event.
type MarketTradeEventMarket struct {
	ConditionID string `json:"condition_id"`
	AssetID     string `json:"asset_id"`
	Question    string `json:"question"`
	Icon        string `json:"icon"`
	Slug        string `json:"slug"`
}

// MarketTradeEventUser is the user metadata embedded in a trade event.
type MarketTradeEventUser struct {
	Address                 string `json:"address"`
	Username                string `json:"username"`
	ProfilePicture          string `json:"profile_picture"`
	OptimizedProfilePicture string `json:"optimized_profile_picture"`
	Pseudonym               string `json:"pseudonym"`
}
