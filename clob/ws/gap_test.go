package ws

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	json "github.com/go-json-experiment/json"

	"github.com/coder/websocket"
)

// Event overflow invalidates queued state instead of stalling the reader
// indefinitely or allowing a consumer to apply silently incomplete deltas.
func TestLaggedConsumerReceivesGapAndReconnects(t *testing.T) {
	var connections atomic.Int32
	reconnected := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()

		if connections.Add(1) != 1 {
			close(reconnected)
			<-r.Context().Done()
			return
		}
		if _, _, err := conn.Read(r.Context()); err != nil {
			return
		}
		books := make([]BookEvent, 300)
		for i := range books {
			books[i] = BookEvent{
				BaseEvent: BaseEvent{EventType: EventTypeBook},
				AssetID:   strconv.Itoa(i),
				Timestamp: "1710000000",
			}
		}
		payload, err := json.Marshal(books)
		if err != nil {
			t.Errorf("marshal books: %v", err)
			return
		}
		if err := conn.Write(r.Context(), websocket.MessageText, payload); err != nil {
			t.Errorf("write books: %v", err)
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	client := NewClient("ws" + strings.TrimPrefix(server.URL, "http"))
	if err := client.Connect(t.Context()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer client.Close()
	if err := client.SubscribeOrderBook(t.Context(), []string{"1"}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	select {
	case <-reconnected:
	case <-time.After(3 * time.Second):
		t.Fatal("slow consumer blocked stream recovery")
	}
	select {
	case event := <-client.Events():
		gap, ok := event.(*GapEvent)
		if !ok || gap.Reason != GapConsumerLagged {
			t.Fatalf("event = %#v, want consumer lag marker", event)
		}
	case <-time.After(time.Second):
		t.Fatal("gap marker not delivered")
	}
}
