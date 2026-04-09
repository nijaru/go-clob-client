package ws

import (
	"context"
	"testing"

	json "github.com/go-json-experiment/json"

	"github.com/nijaru/go-clob-client/clob"
)

func BenchmarkExtractEventType(b *testing.B) {
	data, err := json.Marshal(BookEvent{
		BaseEvent: BaseEvent{EventType: EventTypeBook},
		AssetID:   "asset-1",
		Bids:      []clob.OrderSummary{{Price: "0.45", Size: "10"}},
		Asks:      []clob.OrderSummary{{Price: "0.55", Size: "12"}},
		Timestamp: "1710000000",
	})
	if err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
		if _, ok := extractEventType(data); !ok {
			b.Fatal("extractEventType returned false")
		}
	}
}

func BenchmarkHandleMessageBookEvent(b *testing.B) {
	ctx := context.Background()
	c := NewClient("")

	data, err := json.Marshal(BookEvent{
		BaseEvent: BaseEvent{EventType: EventTypeBook},
		AssetID:   "asset-1",
		Bids:      []clob.OrderSummary{{Price: "0.45", Size: "10"}},
		Asks:      []clob.OrderSummary{{Price: "0.55", Size: "12"}},
		Timestamp: "1710000000",
	})
	if err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
		c.handleMessage(ctx, data)
		select {
		case <-c.Events():
		default:
			b.Fatal("expected event")
		}
	}
}
