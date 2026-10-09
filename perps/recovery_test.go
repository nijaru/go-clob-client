package perps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// The protocol supports batches and account updates independently of request
// responses. Handshake must not discard updates adjacent to acknowledgements.
func TestSessionHandshakePreservesInterleavedBatches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		for i := 1; i <= 2; i++ {
			_, payload, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			var command fixtureCommand
			if err := json.Unmarshal(payload, &command); err != nil {
				t.Error(err)
				return
			}
			update := fmt.Sprintf(
				`{"ch":"balances","ts":%d,"sq":%d,"data":{"asset":"USDC","balance":"%d","value":"1"}}`,
				i,
				i,
				i,
			)
			if i == 2 {
				if err := conn.Write(r.Context(), websocket.MessageText, []byte(update)); err != nil {
					t.Error(err)
					return
				}
				update = `{"ch":"portfolio","ts":2,"sq":1,"data":{"positions":[]}}`
			}
			batch := fmt.Sprintf(`[%s,{"id":%d,"data":{"status":"ok"}}]`, update, command.ID)
			if err := conn.Write(r.Context(), websocket.MessageText, []byte(batch)); err != nil {
				t.Error(err)
				return
			}
		}
		for {
			if _, _, err := conn.Read(r.Context()); err != nil {
				return
			}
		}
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	client := fixtureClient(t, Config{WebSocketHost: "ws" + strings.TrimPrefix(server.URL, "http")})
	session, err := client.OpenSession(ctx, SessionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	for _, channel := range []string{"balances", "balances", "portfolio"} {
		select {
		case event := <-session.Events():
			if event.Channel != channel || event.Resync != nil {
				t.Fatalf("event=%+v, want %s", event, channel)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
}

// A create can succeed even when its orders update does not arrive. Retain both
// generated client identity and accepted server identity for history queries.
func TestPlaceOrderRetainsIdentityWhenUpdateIsMissing(t *testing.T) {
	s := fixtureSession(t, func(ctx context.Context, conn *websocket.Conn, c fixtureCommand) {
		fixtureReply(t, ctx, conn, c.ID, `[{"status":"ok","oid":42}]`)
	})
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	_, err := s.PlaceOrder(
		ctx,
		PerpsOrderRequest{
			InstrumentID: 1,
			Side:         PerpsOrderBuy,
			Price:        "100",
			Quantity:     "1",
			TimeInForce:  PerpsTIFGTC,
		},
		0,
	)
	var placement *OrderPlacementError
	if !errors.As(err, &placement) || !errors.Is(err, context.DeadlineExceeded) ||
		!validPerpsClientOrderID(placement.ClientOrderID) ||
		len(placement.Acknowledgements) != 1 ||
		placement.Acknowledgements[0].OrderID != 42 {
		t.Fatalf("placement=%+v error=%v", placement, err)
	}
}

func TestPublicTradeLowerBoundExhaustsPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"trade_id":1,"timestamp":100}],"more":true}`))
	}))
	t.Cleanup(server.Close)
	items, cursor, err := New(
		Config{Host: server.URL},
	).GetTradesPage(t.Context(), TradesParams{InstrumentID: 1, Start: 100, End: 200})
	if err != nil || len(items) != 1 || cursor != "" {
		t.Fatalf("items=%v cursor=%q error=%v", items, cursor, err)
	}
}
