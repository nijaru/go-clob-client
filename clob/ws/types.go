package ws

import "github.com/nijaru/go-clob-client/clob"

// Channel represents a WebSocket channel type.
type Channel string

const (
	ChannelMarket Channel = "market"
	ChannelUser   Channel = "user"
)

// EventType identifies the type of a WebSocket event.
type EventType string

const (
	EventTypeBook           EventType = "book"
	EventTypePriceChange    EventType = "price_change"
	EventTypeTickSizeChange EventType = "tick_size_change"
	EventTypeLastTradePrice EventType = "last_trade_price"
	EventTypeOrder          EventType = "order"
	EventTypeTrade          EventType = "trade"
)

// UserSubscription is the message sent to subscribe to user updates.
type UserSubscription struct {
	Type Channel     `json:"type"`
	Auth clob.WSAuth `json:"auth"`
}

// MarketSubscription is the message sent to subscribe to market updates.
type MarketSubscription struct {
	Type                 Channel  `json:"type"`
	AssetIDs             []string `json:"assets_ids"`
	CustomFeatureEnabled bool     `json:"custom_feature_enabled"`
}

// BaseEvent contains fields common to all WebSocket events.
type BaseEvent struct {
	EventType EventType `json:"event_type"`
}

// BookEvent is a full order book snapshot emitted upon subscription.
type BookEvent struct {
	BaseEvent
	AssetID   string              `json:"asset_id"`
	Bids      []clob.OrderSummary `json:"bids"`
	Asks      []clob.OrderSummary `json:"asks"`
	Timestamp string              `json:"timestamp"`
}

// PriceChangeEvent is an incremental order book update.
type PriceChangeEvent struct {
	BaseEvent
	AssetID string              `json:"asset_id"`
	Changes []PriceChangeDetail `json:"changes"`
}

// PriceChangeDetail represents a single price level update.
type PriceChangeDetail struct {
	Price string    `json:"price"`
	Size  string    `json:"size"`
	Side  clob.Side `json:"side"`
}

// TickSizeChangeEvent is emitted when a market's tick size changes.
type TickSizeChangeEvent struct {
	BaseEvent
	AssetID     string        `json:"asset_id"`
	Market      string        `json:"market"`
	OldTickSize clob.TickSize `json:"old_tick_size"`
	NewTickSize clob.TickSize `json:"new_tick_size"`
	Timestamp   string        `json:"timestamp"`
}

// LastTradePriceEvent is emitted for every trade execution.
type LastTradePriceEvent struct {
	BaseEvent
	AssetID    string    `json:"asset_id"`
	Market     string    `json:"market"`
	Price      string    `json:"price"`
	Size       string    `json:"size"`
	Side       clob.Side `json:"side"`
	FeeRateBps string    `json:"fee_rate_bps"`
	Timestamp  string    `json:"timestamp"`
}

// OrderEvent is emitted when a user's order status changes (placed, canceled).
type OrderEvent struct {
	BaseEvent
	OrderID   string      `json:"order_id"`
	AssetID   string      `json:"asset_id"`
	Market    string      `json:"market"`
	Price     string      `json:"price"`
	Size      string      `json:"size"`
	Side      clob.Side   `json:"side"`
	Status    OrderStatus `json:"status"`
	Reason    string      `json:"reason,omitempty"`
	Timestamp string      `json:"timestamp"`
}

// TradeEvent is emitted when a user's order is filled (partially or fully).
type TradeEvent struct {
	BaseEvent
	TradeID   string    `json:"trade_id"`
	AssetID   string    `json:"asset_id"`
	Market    string    `json:"market"`
	Price     string    `json:"price"`
	Size      string    `json:"size"`
	Side      clob.Side `json:"side"`
	Status    string    `json:"status"`
	Timestamp string    `json:"timestamp"`
}

type OrderStatus string

const (
	OrderStatusOpen     OrderStatus = "OPEN"
	OrderStatusCanceled OrderStatus = "CANCELED"
	OrderStatusFilled   OrderStatus = "FILLED"
	OrderStatusExpired  OrderStatus = "EXPIRED"
)
