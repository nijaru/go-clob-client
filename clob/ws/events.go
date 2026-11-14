package ws

import (
	"bytes"
	"context"
	stdjson "encoding/json"
	"fmt"

	json "github.com/go-json-experiment/json"
	"github.com/nijaru/go-clob-client/clob"
)

var eventTypeKey = []byte(`"event_type"`)

// extractEventType scans JSON bytes for the "event_type" key using a direct
// byte search so the hot path avoids decoder allocations.
func extractEventType(data []byte) (EventType, bool) {
	idx := bytes.Index(data, eventTypeKey)
	if idx < 0 {
		return "", false
	}

	i := idx + len(eventTypeKey)
	for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
		i++
	}
	if i >= len(data) || data[i] != ':' {
		return "", false
	}
	i++
	for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
		i++
	}
	if i >= len(data) || data[i] != '"' {
		return "", false
	}
	i++
	start := i
	for i < len(data) {
		switch data[i] {
		case '\\':
			// Event types are fixed identifiers, not escaped strings.
			return "", false
		case '"':
			// The temporary byte-to-string conversion used only for a
			// switch comparison does not allocate in Go. Return canonical
			// constants on the common path; retain unknown event names
			// for forward compatibility.
			switch string(data[start:i]) {
			case "book":
				return EventTypeBook, true
			case "price_change":
				return EventTypePriceChange, true
			case "tick_size_change":
				return EventTypeTickSizeChange, true
			case "midpoint":
				return EventTypeMidpoint, true
			case "last_trade_price":
				return EventTypeLastTradePrice, true
			case "order":
				return EventTypeOrder, true
			case "trade":
				return EventTypeTrade, true
			case "best_bid_ask":
				return EventTypeBestBidAsk, true
			case "new_market":
				return EventTypeNewMarket, true
			case "market_resolved":
				return EventTypeMarketResolved, true
			default:
				return EventType(data[start:i]), true
			}
		default:
			i++
		}
	}
	return "", false
}

// handleMessage returns false only when consumer lag prevents safe delivery.
// A caller must abandon the connection and report a continuity gap in that case.
func (c *Client) handleMessage(ctx context.Context, data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		return true
	}
	if trimmed[0] == '[' {
		var messages []stdjson.RawMessage
		if err := stdjson.Unmarshal(trimmed, &messages); err != nil {
			return true
		}
		for _, message := range messages {
			if !c.handleEventMessage(ctx, message, false) {
				return false
			}
		}
		return true
	}
	return c.handleEventMessage(ctx, trimmed, true)
}

func (c *Client) handleEventMessage(ctx context.Context, data []byte, reportDecodeError bool) bool {
	eventType, ok := extractEventType(data)
	if !ok {
		return true
	}

	var event Event
	switch eventType {
	case EventTypeBook:
		event = &BookEvent{}
	case EventTypePriceChange:
		event = &PriceChangeEvent{}
	case EventTypeTickSizeChange:
		event = &TickSizeChangeEvent{}
	case EventTypeMidpoint:
		event = &MidpointEvent{}
	case EventTypeLastTradePrice:
		event = &LastTradePriceEvent{}
	case EventTypeOrder:
		event = &OrderEvent{}
	case EventTypeTrade:
		event = &TradeEvent{}
	case EventTypeBestBidAsk:
		event = &BestBidAskEvent{}
	case EventTypeNewMarket:
		event = &NewMarketEvent{}
	case EventTypeMarketResolved:
		event = &MarketResolvedEvent{}
	default:
		// Unknown event types are forward-compatible protocol additions.
		return true
	}

	if err := json.Unmarshal(data, event); err != nil {
		if reportDecodeError {
			c.emitError(fmt.Errorf("decode %s event: %w", eventType, err))
		}
		return true
	}
	if !c.emitEvent(ctx, event) {
		return false
	}

	book, ok := event.(*BookEvent)
	if !ok || !c.isMidpointSubscribed(book.AssetID) {
		return true
	}
	midpoint, ok := midpointFromBook(book)
	if ok {
		return c.emitEvent(ctx, midpoint)
	}
	return true
}

func (c *Client) isMidpointSubscribed(assetID string) bool {
	c.subsMu.RLock()
	_, ok := c.midpointRefs[assetID]
	c.subsMu.RUnlock()
	return ok
}

func midpointFromBook(book *BookEvent) (*MidpointEvent, bool) {
	if len(book.Bids) == 0 || len(book.Asks) == 0 {
		return nil, false
	}

	bid, err := clob.Dec(book.Bids[0].Price)
	if err != nil {
		return nil, false
	}
	ask, err := clob.Dec(book.Asks[0].Price)
	if err != nil {
		return nil, false
	}
	midpoint, err := bid.Add(ask).Div(clob.MustDec("2"))
	if err != nil {
		return nil, false
	}

	return &MidpointEvent{
		BaseEvent: BaseEvent{EventType: EventTypeMidpoint},
		AssetID:   book.AssetID,
		Market:    book.Market,
		Midpoint:  midpoint.String(),
		Timestamp: book.Timestamp,
	}, true
}
