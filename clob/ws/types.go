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
)

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
