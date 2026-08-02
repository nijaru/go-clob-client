package ws

import (
	"strings"

	json "github.com/go-json-experiment/json"

	"github.com/nijaru/go-clob-client/clob"
)

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
	EventTypeMidpoint       EventType = "midpoint"
	EventTypeLastTradePrice EventType = "last_trade_price"
	EventTypeOrder          EventType = "order"
	EventTypeTrade          EventType = "trade"
	EventTypeBestBidAsk     EventType = "best_bid_ask"
	EventTypeNewMarket      EventType = "new_market"
	EventTypeMarketResolved EventType = "market_resolved"
)

// Event is implemented by all WebSocket event types.
// Use a type switch to handle specific event kinds:
//
//	switch ev := event.(type) {
//	case *BookEvent:           ...
//	case *PriceChangeEvent:    ...
//	case *TickSizeChangeEvent: ...
//	case *MidpointEvent:       ...
//	case *LastTradePriceEvent: ...
//	case *OrderEvent:          ...
//	case *TradeEvent:          ...
//	case *BestBidAskEvent:     ...
//	case *NewMarketEvent:      ...
//	case *MarketResolvedEvent: ...
//	}
type Event interface {
	isEvent()
}

func (*BookEvent) isEvent()           {}
func (*PriceChangeEvent) isEvent()    {}
func (*TickSizeChangeEvent) isEvent() {}
func (*MidpointEvent) isEvent()       {}
func (*LastTradePriceEvent) isEvent() {}
func (*OrderEvent) isEvent()          {}
func (*TradeEvent) isEvent()          {}
func (*BestBidAskEvent) isEvent()     {}
func (*NewMarketEvent) isEvent()      {}
func (*MarketResolvedEvent) isEvent() {}

// UserSubscription is the message sent to subscribe to user updates.
type UserSubscription struct {
	Type        Channel     `json:"type"`
	Auth        clob.WSAuth `json:"auth"`
	Markets     []string    `json:"markets,omitzero"`
	Operation   string      `json:"operation,omitzero"`
	InitialDump bool        `json:"initial_dump,omitzero"`
}

// MarketSubscription is the message sent to subscribe to market updates.
type MarketSubscription struct {
	Type                 Channel  `json:"type"`
	Operation            string   `json:"operation,omitzero"`
	Markets              []string `json:"markets,omitzero"`
	AssetIDs             []string `json:"assets_ids"`
	InitialDump          bool     `json:"initial_dump,omitzero"`
	CustomFeatureEnabled bool     `json:"custom_feature_enabled,omitzero"`
}

// BaseEvent contains fields common to all WebSocket events.
type BaseEvent struct {
	EventType EventType `json:"event_type"`
}

// BookEvent is a full order book snapshot emitted upon subscription or after a
// trade. The market channel may deliver these as a top-level JSON array; the
// client flattens that transport batch into one Event per asset.
type BookEvent struct {
	BaseEvent
	Market         string              `json:"market"`
	AssetID        string              `json:"asset_id"`
	Bids           []clob.OrderSummary `json:"bids"`
	Asks           []clob.OrderSummary `json:"asks"`
	Timestamp      string              `json:"timestamp"`
	Hash           string              `json:"hash,omitzero"`
	MinOrderSize   string              `json:"min_order_size,omitzero"`
	TickSize       clob.TickSize       `json:"tick_size,omitzero"`
	NegRisk        bool                `json:"neg_risk,omitzero"`
	LastTradePrice string              `json:"last_trade_price,omitzero"`
}

// PriceChange is one entry in a price_change batch.
type PriceChange struct {
	AssetID string    `json:"asset_id"`
	Price   string    `json:"price"`
	Size    string    `json:"size,omitzero"`
	Side    clob.Side `json:"side"`
	Hash    string    `json:"hash,omitzero"`
	BestBid string    `json:"best_bid,omitzero"`
	BestAsk string    `json:"best_ask,omitzero"`
}

// PriceChangeEvent is an incremental order book update. The official market
// channel sends a batch in price_changes; the deprecated singular fields keep
// compatibility with older payloads and callers while the server migrates.
type PriceChangeEvent struct {
	BaseEvent
	Market       string        `json:"market,omitzero"`
	PriceChanges []PriceChange `json:"price_changes,omitzero"`
	Timestamp    string        `json:"timestamp,omitzero"`
	AssetID      string        `json:"asset_id,omitzero"`
	Price        string        `json:"price,omitzero"`
	Size         string        `json:"size,omitzero"`
	Side         clob.Side     `json:"side,omitzero"`
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

// MidpointEvent is derived from the best bid and ask in a book snapshot.
// It is emitted for assets subscribed with SubscribeMidpoints; the server does
// not send a separate midpoint event on the market WebSocket.
type MidpointEvent struct {
	BaseEvent
	AssetID   string `json:"asset_id"`
	Market    string `json:"market"`
	Midpoint  string `json:"midpoint"`
	Timestamp string `json:"timestamp"`
}

// LastTradePriceEvent is emitted for every trade execution.
type LastTradePriceEvent struct {
	BaseEvent
	AssetID         string    `json:"asset_id"`
	Market          string    `json:"market"`
	Price           string    `json:"price"`
	Size            string    `json:"size"`
	Side            clob.Side `json:"side"`
	FeeRateBps      string    `json:"fee_rate_bps"`
	Timestamp       string    `json:"timestamp"`
	TransactionHash string    `json:"transaction_hash,omitzero"`
}

// OrderEvent is emitted when a user's order status changes (placed, canceled).
type OrderEvent struct {
	BaseEvent
	OrderID         string             `json:"id"`
	Owner           string             `json:"owner"`
	AssetID         string             `json:"asset_id"`
	Market          string             `json:"market"`
	Price           string             `json:"price"`
	Size            string             `json:"size"`
	Side            clob.Side          `json:"side"`
	OrderOwner      string             `json:"order_owner,omitzero"`
	OriginalSize    string             `json:"original_size,omitzero"`
	SizeMatched     string             `json:"size_matched,omitzero"`
	AssociateTrades []string           `json:"associate_trades,omitzero"`
	Outcome         string             `json:"outcome,omitzero"`
	OrderEventType  UserOrderEventType `json:"type,omitzero"`
	CreatedAt       string             `json:"created_at,omitzero"`
	Expiration      string             `json:"expiration,omitzero"`
	OrderType       clob.OrderType     `json:"order_type,omitzero"`
	Status          OrderStatus        `json:"status,omitzero"`
	MakerAddress    string             `json:"maker_address,omitzero"`
	Reason          string             `json:"reason,omitzero"`
	Timestamp       string             `json:"timestamp"`
}

// UnmarshalJSON keeps compatibility with older user payloads that called the
// order identifier order_id instead of id.
func (e *OrderEvent) UnmarshalJSON(data []byte) error {
	type alias OrderEvent
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value.OrderID == "" {
		var legacy struct {
			OrderID string `json:"order_id"`
		}
		if err := json.Unmarshal(data, &legacy); err != nil {
			return err
		}
		value.OrderID = legacy.OrderID
	}
	*e = OrderEvent(value)
	return nil
}

// MakerOrder contains the maker-order details included with trade events.
type MakerOrder struct {
	OrderID       string    `json:"order_id"`
	Owner         string    `json:"owner"`
	MakerAddress  string    `json:"maker_address,omitzero"`
	MatchedAmount string    `json:"matched_amount"`
	Price         string    `json:"price"`
	FeeRateBps    string    `json:"fee_rate_bps,omitzero"`
	AssetID       string    `json:"asset_id"`
	Outcome       string    `json:"outcome,omitzero"`
	OutcomeIndex  *int      `json:"outcome_index,omitzero"`
	Side          clob.Side `json:"side"`
}

// TradeEvent is emitted when a user's order is filled (partially or fully).
type TradeEvent struct {
	BaseEvent
	TradeID         string       `json:"id"`
	TakerOrderID    string       `json:"taker_order_id,omitzero"`
	Market          string       `json:"market"`
	AssetID         string       `json:"asset_id"`
	Side            clob.Side    `json:"side"`
	Size            string       `json:"size"`
	FeeRateBps      string       `json:"fee_rate_bps,omitzero"`
	Price           string       `json:"price"`
	TradeType       string       `json:"type,omitzero"`
	Status          TradeStatus  `json:"status"`
	MatchTime       string       `json:"match_time,omitzero"`
	Matchtime       string       `json:"matchtime,omitzero"`
	LastUpdate      string       `json:"last_update,omitzero"`
	Outcome         string       `json:"outcome,omitzero"`
	Owner           string       `json:"owner,omitzero"`
	TradeOwner      string       `json:"trade_owner,omitzero"`
	MakerAddress    string       `json:"maker_address,omitzero"`
	TransactionHash string       `json:"transaction_hash,omitzero"`
	BucketIndex     *int         `json:"bucket_index,omitzero"`
	MakerOrders     []MakerOrder `json:"maker_orders,omitzero"`
	TraderSide      TraderSide   `json:"trader_side,omitzero"`
	Timestamp       string       `json:"timestamp"`
}

// UnmarshalJSON keeps compatibility with older user payloads that called the
// trade identifier trade_id instead of id.
func (e *TradeEvent) UnmarshalJSON(data []byte) error {
	type alias TradeEvent
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if value.TradeID == "" {
		var legacy struct {
			TradeID string `json:"trade_id"`
		}
		if err := json.Unmarshal(data, &legacy); err != nil {
			return err
		}
		value.TradeID = legacy.TradeID
	}
	*e = TradeEvent(value)
	return nil
}

// UserOrderEventType identifies the order lifecycle transition.
type UserOrderEventType string

const (
	UserOrderEventTypePlacement    UserOrderEventType = "PLACEMENT"
	UserOrderEventTypeUpdate       UserOrderEventType = "UPDATE"
	UserOrderEventTypeCancellation UserOrderEventType = "CANCELLATION"
)

// TraderSide identifies whether a user trade was maker or taker executed.
type TraderSide string

const (
	TraderSideMaker TraderSide = "MAKER"
	TraderSideTaker TraderSide = "TAKER"
)

// BestBidAskEvent is emitted when the best bid or ask for a market changes.
type BestBidAskEvent struct {
	BaseEvent
	Market    string `json:"market"`
	AssetID   string `json:"asset_id"`
	BestBid   string `json:"best_bid"`
	BestAsk   string `json:"best_ask"`
	Spread    string `json:"spread"`
	Timestamp string `json:"timestamp"`
}

// EventMessage contains metadata about a market's event.
type EventMessage struct {
	ID          string `json:"id"`
	Ticker      string `json:"ticker"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// NewMarketEvent is emitted when a new market is created.
type NewMarketEvent struct {
	BaseEvent
	ID           string        `json:"id"`
	Question     string        `json:"question"`
	Market       string        `json:"market"`
	Slug         string        `json:"slug"`
	Description  string        `json:"description"`
	AssetIDs     []string      `json:"assets_ids"`
	Outcomes     []string      `json:"outcomes"`
	EventMessage *EventMessage `json:"event_message,omitzero"`
	Timestamp    string        `json:"timestamp"`
}

// UnmarshalJSON accepts both the current assets_ids spelling and the older
// asset_ids spelling used by some market-event payloads.
func (e *NewMarketEvent) UnmarshalJSON(data []byte) error {
	type alias NewMarketEvent
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if len(value.AssetIDs) == 0 {
		var legacy struct {
			AssetIDs []string `json:"asset_ids"`
		}
		if err := json.Unmarshal(data, &legacy); err != nil {
			return err
		}
		value.AssetIDs = legacy.AssetIDs
	}
	*e = NewMarketEvent(value)
	return nil
}

// MarketResolvedEvent is emitted when a market is resolved.
type MarketResolvedEvent struct {
	BaseEvent
	ID             string        `json:"id"`
	Question       string        `json:"question,omitzero"`
	Market         string        `json:"market"`
	Slug           string        `json:"slug,omitzero"`
	Description    string        `json:"description,omitzero"`
	AssetIDs       []string      `json:"assets_ids"`
	Outcomes       []string      `json:"outcomes,omitzero"`
	WinningAssetID string        `json:"winning_asset_id"`
	WinningOutcome string        `json:"winning_outcome"`
	EventMessage   *EventMessage `json:"event_message,omitzero"`
	Timestamp      string        `json:"timestamp"`
}

// UnmarshalJSON accepts both the current assets_ids spelling and the older
// asset_ids spelling used by some market-event payloads.
func (e *MarketResolvedEvent) UnmarshalJSON(data []byte) error {
	type alias MarketResolvedEvent
	var value alias
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	if len(value.AssetIDs) == 0 {
		var legacy struct {
			AssetIDs []string `json:"asset_ids"`
		}
		if err := json.Unmarshal(data, &legacy); err != nil {
			return err
		}
		value.AssetIDs = legacy.AssetIDs
	}
	*e = MarketResolvedEvent(value)
	return nil
}

// OrderStatus represents the lifecycle state of an order as streamed by the user channel.
type OrderStatus string

const (
	OrderStatusOpen      OrderStatus = "OPEN"
	OrderStatusLive      OrderStatus = "LIVE"
	OrderStatusMatched   OrderStatus = "MATCHED"
	OrderStatusDelayed   OrderStatus = "DELAYED"
	OrderStatusUnmatched OrderStatus = "UNMATCHED"
	OrderStatusCanceled  OrderStatus = "CANCELED"
	OrderStatusFilled    OrderStatus = "FILLED"
	OrderStatusExpired   OrderStatus = "EXPIRED"
	OrderStatusRetrying  OrderStatus = "RETRYING"
	OrderStatusFailed    OrderStatus = "FAILED"
)

// TradeStatus represents the lifecycle state of a trade as streamed by the user channel.
type TradeStatus string

const (
	TradeStatusMatched               TradeStatus = "MATCHED"
	TradeStatusMatchedNotBroadcasted TradeStatus = "MATCHED_NOT_BROADCASTED"
	TradeStatusMined                 TradeStatus = "MINED"
	TradeStatusConfirmed             TradeStatus = "CONFIRMED"
	TradeStatusRetrying              TradeStatus = "RETRYING"
	TradeStatusFailed                TradeStatus = "FAILED"
)

// UnmarshalJSON implements case-insensitive deserialization for TradeStatus,
// matching the Rust SDK's serde alias behavior (e.g. both "matched" and "MATCHED"
// decode to TradeStatusMatched).
func (s *TradeStatus) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	raw = strings.ToUpper(raw)
	raw = strings.TrimPrefix(raw, "TRADE_STATUS_")
	*s = TradeStatus(raw)
	return nil
}
