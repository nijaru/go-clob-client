package perps

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestPerpsCreateOrdersMessagePackHashMatchesOfficialFixture(t *testing.T) {
	op := []any{
		"createOrders",
		[]any{[]any{1, true, "100.50", "10", "gtc", false}},
	}
	payload, err := encodePerpsMsgpack(op)
	if err != nil {
		t.Fatalf("encodePerpsMsgpack: %v", err)
	}
	got := hex.EncodeToString(crypto.Keccak256(payload))
	const want = "817207b7b8b31044a8f27e43c16e24d9fd5e11d3f106feb962f104f3ef28d52a"
	if got != want {
		t.Fatalf("MessagePack op hash = %s, want %s", got, want)
	}
}

func TestPostOrdersSignsAndSendsOfficialCommandShape(t *testing.T) {
	const proxy = "0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf"
	const privateKey = "0x0000000000000000000000000000000000000000000000000000000000000001"
	command := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		for i := 1; i <= 3; i++ {
			_, payload, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			var frame map[string]any
			if err := json.Unmarshal(payload, &frame); err != nil {
				return
			}
			id := int(frame["id"].(float64))
			if i == 3 {
				command <- frame
			}
			response := map[string]any{"id": id, "data": map[string]any{"status": "ok"}}
			if i == 3 {
				response["data"] = []map[string]any{{"status": "ok", "oid": 123}}
			}
			encoded, _ := json.Marshal(response)
			if err := conn.Write(r.Context(), websocket.MessageText, encoded); err != nil {
				return
			}
		}
	}))
	t.Cleanup(server.Close)

	client, err := NewAuthenticated(AuthenticatedConfig{
		Config: Config{
			WebSocketHost: "ws" + strings.TrimPrefix(server.URL, "http"),
			ChainID:       31337,
		},
		Credentials: PerpsCredentials{Proxy: proxy, Secret: "secret", PrivateKey: privateKey},
	})
	if err != nil {
		t.Fatalf("NewAuthenticated: %v", err)
	}
	session, err := client.OpenSession(t.Context(), SessionConfig{})
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	acks, err := session.PostOrders(t.Context(), []PerpsOrderRequest{{
		InstrumentID: 1,
		Side:         PerpsOrderBuy,
		Price:        "100.50",
		Quantity:     "10",
		TimeInForce:  PerpsTIFGTC,
	}}, 0)
	if err != nil {
		t.Fatalf("PostOrders: %v", err)
	}
	if len(acks) != 1 || acks[0].OrderID != 123 {
		t.Fatalf("acks = %+v, want order 123", acks)
	}
	select {
	case frame := <-command:
		if frame["req"] != "post" || frame["sig"] == "" {
			t.Fatalf("command envelope = %+v", frame)
		}
		op, ok := frame["op"].(map[string]any)
		if !ok || op["type"] != "createOrders" {
			t.Fatalf("command op = %#v", frame["op"])
		}
		args, ok := op["args"].([]any)
		if !ok || len(args) != 1 {
			t.Fatalf("command args = %#v", op["args"])
		}
		order, ok := args[0].(map[string]any)
		if !ok || order["iid"] != float64(1) || order["buy"] != true || order["p"] != "100.50" {
			t.Fatalf("command order = %#v", args[0])
		}
	case <-t.Context().Done():
		t.Fatal("timed out waiting for command capture")
	}
}

func TestPlaceOrderWaitsForMatchingOrderUpdate(t *testing.T) {
	const proxy = "0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf"
	const privateKey = "0x0000000000000000000000000000000000000000000000000000000000000001"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		for i := 1; i <= 3; i++ {
			_, payload, err := conn.Read(r.Context())
			if err != nil {
				return
			}
			var frame map[string]any
			if err := json.Unmarshal(payload, &frame); err != nil {
				return
			}
			id := int(frame["id"].(float64))
			response := map[string]any{"id": id, "data": map[string]any{"status": "ok"}}
			if i == 3 {
				response["data"] = []map[string]any{{"status": "ok", "oid": 123}}
			}
			encoded, _ := json.Marshal(response)
			if err := conn.Write(r.Context(), websocket.MessageText, encoded); err != nil {
				return
			}
			if i == 3 {
				event, _ := json.Marshal(map[string]any{
					"ch": "orders",
					"ts": 1234,
					"sq": 9,
					"data": map[string]any{
						"oid":    123,
						"iid":    1,
						"buy":    true,
						"p":      "100.50",
						"qty":    "10",
						"tif":    "gtc",
						"po":     false,
						"ro":     false,
						"status": "open",
						"rest":   "10",
						"fill":   "0",
						"cts":    1230,
						"uts":    1234,
					},
				})
				_ = conn.Write(r.Context(), websocket.MessageText, event)
			}
		}
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	client, err := NewAuthenticated(AuthenticatedConfig{
		Config: Config{
			WebSocketHost: "ws" + strings.TrimPrefix(server.URL, "http"),
			ChainID:       31337,
		},
		Credentials: PerpsCredentials{Proxy: proxy, Secret: "secret", PrivateKey: privateKey},
	})
	if err != nil {
		t.Fatalf("NewAuthenticated: %v", err)
	}
	session, err := client.OpenSession(t.Context(), SessionConfig{})
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	order, err := session.PlaceOrder(t.Context(), PerpsOrderRequest{
		InstrumentID: 1,
		Side:         PerpsOrderBuy,
		Price:        "100.50",
		Quantity:     "10",
		TimeInForce:  PerpsTIFGTC,
	}, 0)
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	if order.ID != 123 || order.Status != PerpsOrderOpen || order.FilledQuantity != "0" {
		t.Fatalf("order = %+v, want open order 123", order)
	}
}

func TestCancelAllOrdersUsesSignedAuthenticatedREST(t *testing.T) {
	const proxy = "0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf"
	const privateKey = "0x0000000000000000000000000000000000000000000000000000000000000001"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/v1/trade/orders/all" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("POLYMARKET-PROXY") != proxy ||
			r.Header.Get("POLYMARKET-SECRET") != "secret" {
			t.Errorf("auth headers missing: %v", r.Header)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if body["sig"] == "" || body["op"] == nil {
			t.Errorf("signed body = %#v", body)
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	t.Cleanup(server.Close)
	client, err := NewAuthenticated(AuthenticatedConfig{
		Config:      Config{Host: server.URL, ChainID: 31337},
		Credentials: PerpsCredentials{Proxy: proxy, Secret: "secret", PrivateKey: privateKey},
	})
	if err != nil {
		t.Fatalf("NewAuthenticated: %v", err)
	}
	instrumentID := 7
	if err := client.CancelAllOrders(t.Context(), &instrumentID, 1234); err != nil {
		t.Fatalf("CancelAllOrders: %v", err)
	}
}

func TestPerpsTradingInputValidationMatchesOfficialConstraints(t *testing.T) {
	base := PerpsOrderRequest{
		InstrumentID: 1,
		Side:         PerpsOrderBuy,
		Price:        "100.50",
		Quantity:     "10",
		TimeInForce:  PerpsTIFGTC,
	}
	for name, order := range map[string]PerpsOrderRequest{
		"invalid time in force": func() PerpsOrderRequest {
			order := base
			order.TimeInForce = "day"
			return order
		}(),
		"post only immediate order": func() PerpsOrderRequest {
			order := base
			order.TimeInForce = PerpsTIFIOC
			order.PostOnly = true
			return order
		}(),
		"malformed client order id": func() PerpsOrderRequest {
			order := base
			order.ClientOrderID = "not-a-client-id"
			return order
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := perpsOrderWire(order); err == nil {
				t.Fatal("perpsOrderWire succeeded for invalid order")
			}
		})
	}
	if _, _, err := perpsOrderWire(base); err != nil {
		t.Fatalf("valid perpsOrderWire: %v", err)
	}

	tooMany := make([]PerpsOrderRequest, 16)
	if _, err := (&Session{}).PostOrders(t.Context(), tooMany, 0); err == nil {
		t.Fatal("PostOrders accepted more than the official batch limit")
	}
	if _, err := (&Session{}).CancelOrders(t.Context(), []int{-1}, 0); err == nil {
		t.Fatal("CancelOrders accepted a negative order ID")
	}
	if _, err := (&Session{}).CancelOrdersByClientID(t.Context(), []string{"bad"}, 0); err == nil {
		t.Fatal("CancelOrdersByClientID accepted a malformed client ID")
	}
	if _, err := (&Session{}).UpdateLeverage(t.Context(), 1, 0, true); err == nil {
		t.Fatal("UpdateLeverage accepted non-positive leverage")
	}
}

func TestPerpsPostOrderAckRequiresOrderID(t *testing.T) {
	var missing PerpsOrderAck
	if err := json.Unmarshal([]byte(`{"status":"ok"}`), &missing); err != nil {
		t.Fatalf("decode missing order ID ack: %v", err)
	}
	if err := validatePerpsPostOrderAck(missing); err == nil {
		t.Fatal("ack without order ID was accepted")
	}

	var zero PerpsOrderAck
	if err := json.Unmarshal([]byte(`{"status":"ok","oid":0}`), &zero); err != nil {
		t.Fatalf("decode zero order ID ack: %v", err)
	}
	if err := validatePerpsPostOrderAck(zero); err != nil {
		t.Fatalf("valid zero order ID ack rejected: %v", err)
	}
}

func TestPerpsLeverageRejectedResultReturnsError(t *testing.T) {
	if err := validatePerpsLeverageResult(PerpsLeverageResult{
		Status: "err",
		Error:  "insufficient margin",
	}); err == nil {
		t.Fatal("rejected leverage result was accepted")
	}
	if err := validatePerpsLeverageResult(PerpsLeverageResult{Status: "ok"}); err != nil {
		t.Fatalf("successful leverage result rejected: %v", err)
	}
}
