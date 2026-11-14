package ws

import (
	"context"
	stdjson "encoding/json"
	"strconv"
	"testing"

	json "github.com/go-json-experiment/json"

	"github.com/nijaru/go-clob-client/clob"
)

// Temporary A/B baseline to compare the previous RawMessage batch path with
// the streaming decoder on the same runner and payload.
func BenchmarkHandleMessageBookBatchLegacy(b *testing.B) {
	ctx := context.Background()
	c := NewClient("")
	books := make([]BookEvent, 32)
	for i := range books {
		books[i] = BookEvent{
			BaseEvent: BaseEvent{EventType: EventTypeBook},
			AssetID:   strconv.Itoa(i),
			Bids: []clob.OrderSummary{
				{Price: "0.45", Size: "10"},
				{Price: "0.44", Size: "20"},
			},
			Asks: []clob.OrderSummary{
				{Price: "0.55", Size: "12"},
				{Price: "0.56", Size: "30"},
			},
			Timestamp: "1710000000",
		}
	}
	data, err := json.Marshal(books)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		var messages []stdjson.RawMessage
		if err := stdjson.Unmarshal(data, &messages); err != nil {
			b.Fatal(err)
		}
		for _, raw := range messages {
			if !c.handleEventMessage(ctx, raw, false) {
				b.Fatal("unexpected consumer lag")
			}
		}
		for range len(messages) {
			select {
			case <-c.Events():
			default:
				b.Fatal("missing event")
			}
		}
	}
}
