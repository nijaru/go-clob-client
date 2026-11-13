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
			return EventType(data[start:i]), true
		default:
			i++
		}
	}
	return "", false
}

func (c *Client) handleMessage(ctx context.Context, data []byte) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		// Non-JSON message (text heartbeat, etc.) — not an error.
		return
	}
	if trimmed[0] == '[' {
		var messages []stdjson.RawMessage
		if err := stdjson.Unmarshal(trimmed, &messages); err != nil {
			return
		}
		for _, message := range messages {
			// Rust tolerates malformed/unknown elements inside a transport batch;
			// report only malformed singleton frames to the asynchronous error path.
			c.handleEventMessage(ctx, message, false)
		}
		return
	}
	c.handleEventMessage(ctx, trimmed, true)
}

func (c *Client) handleEventMessage(ctx context.Context, data []byte, reportDecodeError bool) {
	eventType, ok := extractEventType(data)
	if !ok {
		return
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
		// Unknown event types are forward-compatible additions to the wire protocol.
		// Drop them while keeping the connection usable, as the official SDKs do.
		return
	}

	if err := json.Unmarshal(data, event); err != nil {
		if reportDecodeError {
			c.emitError(fmt.Errorf("decode %s event: %w", eventType, err))
		}
		return
	}
	c.emitEvent(ctx, event)

	book, ok := event.(*BookEvent)
	if !ok || !c.isMidpointSubscribed(book.AssetID) {
		return
	}
	midpoint, ok := midpointFromBook(book)
	if ok {
		c.emitEvent(ctx, midpoint)
	}
}

func (c *Client) isMidpointSubscribed(assetID string) bool {
	if assetID == "" {
		return false
	}

	c.subsMu.RLock()
	defer c.subsMu.RUnlock()
	for _, sub := range c.subs {
		if sub.channelType != channelTypeMidpoints {
			continue
		}
		for _, subscribedAssetID := range sub.assetIDs {
			if subscribedAssetID == assetID {
				return true
			}
		}
	}
	return false
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

