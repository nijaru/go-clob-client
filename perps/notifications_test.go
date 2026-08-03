package perps

import (
	"context"
	"encoding/base64"
	stdjson "encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPerpsNotificationPageDecodesVariantsAndSkipsUnknown(t *testing.T) {
	var page PerpsNotificationsPage
	input := []byte(`{
		"items":[
			{
				"notification":{
					"id":"n-open",
					"type":"position_opened",
					"instrument_id":7,
					"side":"long",
					"size":"1.25",
					"avg_price":"100.5",
					"leverage":5,
					"order_type":"market"
				},
				"read_at":null,
				"ts":100
			},
			{
				"notification":{"id":"n-future","type":"future_notification"},
				"read_at":101,
				"ts":101
			},
			{
				"notification":{
					"id":"n-cross",
					"type":"liquidation_warning",
					"margin_type":"cross",
					"instrument_id":null,
					"mark_price":"90",
					"affected_instruments":[7,8]
				},
				"read_at":102,
				"ts":102
			}
		],
		"unread":2,
		"durable_source_seq":42,
		"has_more":true,
		"next_cursor":"cursor-2"
	}`)
	if err := stdjson.Unmarshal(input, &page); err != nil {
		t.Fatalf("decode notifications page: %v", err)
	}
	if len(page.Items) != 2 || page.Unread != 2 || page.DurableSourceSequence != 42 ||
		!page.More || page.NextCursor != "cursor-2" {
		t.Fatalf("page = %+v", page)
	}
	open := page.Items[0].Notification.PositionChange
	if open == nil || open.ID != "n-open" || open.Size != "1.25" ||
		open.OrderType != PerpsNotificationOrderMarket {
		t.Fatalf("position notification = %+v", page.Items[0].Notification)
	}
	cross := page.Items[1].Notification.LiquidationWarning
	if cross == nil || cross.InstrumentID != nil || len(cross.AffectedInstruments) != 2 {
		t.Fatalf("cross liquidation warning = %+v", cross)
	}
}

func TestPerpsNotificationRejectsMalformedKnownVariant(t *testing.T) {
	for _, input := range []string{
		`{"id":"n","type":"position_closed","instrument_id":7,"side":"long","size":"1","avg_price":"2"}`,
		`{"id":"n","type":"liquidation_warning","margin_type":"isolated","instrument_id":7,"mark_price":"2"}`,
		`{"id":"n","type":"liquidation_warning","margin_type":"cross","instrument_id":7,"mark_price":"2"}`,
	} {
		var notification PerpsNotification
		if err := stdjson.Unmarshal([]byte(input), &notification); err == nil {
			t.Fatalf("input %s: expected validation error", input)
		}
	}
}

func TestPerpsNotificationUnknownType(t *testing.T) {
	var notification PerpsNotification
	err := stdjson.Unmarshal([]byte(`{"id":"n","type":"future_notification"}`), &notification)
	if !errors.Is(err, ErrUnknownPerpsNotification) {
		t.Fatalf("error = %v, want ErrUnknownPerpsNotification", err)
	}
}

func TestPerpsNotificationAccountMethods(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/account/notifications", func(w http.ResponseWriter, r *http.Request) {
		assertPerpsAuth(t, r)
		query := r.URL.Query()
		switch query.Get("limit") {
		case "1":
			_ = stdjson.NewEncoder(w).Encode(map[string]any{"unread": 4})
		default:
			if query.Get("since_seq") != "10" || query.Get("limit") != "2" || query.Get("cursor") != "cursor-1" {
				t.Errorf("notification query = %v", query)
			}
			_ = stdjson.NewEncoder(w).Encode(map[string]any{
				"items":              []any{},
				"unread":             3,
				"durable_source_seq": 9,
				"has_more":           true,
				"next_cursor":        "cursor-2",
			})
		}
	})
	mux.HandleFunc("/v1/account/notifications/read", func(w http.ResponseWriter, r *http.Request) {
		assertPerpsAuth(t, r)
		var body map[string]any
		if err := stdjson.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode read body: %v", err)
		}
		if ids, ok := body["ids"].([]any); ok {
			if len(ids) != 2 || ids[0] != "n-1" || ids[1] != "n-2" {
				t.Errorf("ids body = %#v", body)
			}
		} else {
			before, ok := body["before"].(string)
			if !ok || before == "" {
				t.Errorf("before body = %#v", body)
			}
			decoded, err := base64.RawURLEncoding.DecodeString(before)
			if err != nil {
				t.Errorf("decode before cursor: %v", err)
			} else if string(decoded) != `{"ts":123,"id":"n-1"}` {
				t.Errorf("before cursor = %s", decoded)
			}
		}
		_ = stdjson.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client, err := NewAuthenticated(AuthenticatedConfig{
		Config:      Config{Host: server.URL},
		Credentials: testPerpsCredentials(),
	})
	if err != nil {
		t.Fatalf("NewAuthenticated: %v", err)
	}
	page, err := client.GetNotificationsPage(t.Context(), NotificationsParams{
		SinceSequence: 10,
		Limit:         2,
		Cursor:        "cursor-1",
	})
	if err != nil || page.NextCursor != "cursor-2" || !page.More {
		t.Fatalf("GetNotificationsPage = %+v, %v", page, err)
	}
	unread, err := client.GetUnreadNotificationsCount(t.Context())
	if err != nil || unread != 4 {
		t.Fatalf("GetUnreadNotificationsCount = %d, %v", unread, err)
	}
	if err := client.MarkNotificationsRead(t.Context(), MarkNotificationsReadParams{
		IDs: []string{"n-1", "n-2"},
	}); err != nil {
		t.Fatalf("mark IDs read: %v", err)
	}
	if err := client.MarkNotificationsRead(t.Context(), MarkNotificationsReadParams{
		Before: &PerpsNotificationReadCursor{ID: "n-1", Timestamp: 123},
	}); err != nil {
		t.Fatalf("mark before read: %v", err)
	}
	for _, params := range []MarkNotificationsReadParams{
		{},
		{IDs: []string{""}},
		{IDs: []string{"n"}, Before: &PerpsNotificationReadCursor{ID: "n", Timestamp: 1}},
	} {
		if err := client.MarkNotificationsRead(context.Background(), params); err == nil {
			t.Fatalf("params %+v: expected validation error", params)
		}
	}
	if _, err := client.GetNotificationsPage(context.Background(), NotificationsParams{SinceSequence: -1}); err == nil {
		t.Fatal("expected negative since sequence error")
	}
	if _, err := client.GetNotificationsPage(context.Background(), NotificationsParams{Limit: -1}); err == nil {
		t.Fatal("expected negative limit error")
	}
}

func TestPerpsSessionSequenceGapResync(t *testing.T) {
	session := &Session{
		ctx:          context.Background(),
		events:       make(chan PerpsSessionEvent, 4),
		errors:       make(chan error, 1),
		orderWaiters: make(map[int][]chan orderWaitResponse),
	}
	session.handlePayload([]byte(`{"ch":"balances","ts":100,"sq":7,"data":{"asset":"USDC"}}`))
	session.handlePayload([]byte(`{"ch":"balances","ts":101,"sq":9,"data":{"asset":"USDC"}}`))

	first := <-session.events
	if first.Channel != "balances" {
		t.Fatalf("first event = %+v", first)
	}
	gap := <-session.events
	if gap.Type != "resync" || gap.Resync == nil ||
		gap.Resync.Reason != PerpsResyncSequenceGap ||
		gap.Resync.PreviousSequence == nil || *gap.Resync.PreviousSequence != 7 ||
		gap.Resync.Sequence != 9 {
		t.Fatalf("gap event = %+v", gap)
	}
	second := <-session.events
	if second.Channel != "balances" || second.Sequence != 9 {
		t.Fatalf("second event = %+v", second)
	}
}

func TestPerpsSessionTypedNotificationAndResync(t *testing.T) {
	session := &Session{
		ctx:          context.Background(),
		events:       make(chan PerpsSessionEvent, 2),
		errors:       make(chan error, 1),
		orderWaiters: make(map[int][]chan orderWaitResponse),
	}
	session.handlePayload([]byte(`{"ch":"notifications","ts":100,"sq":7,"data":{"id":"n-1","type":"position_reduced","instrument_id":7,"side":"short","size":"2","avg_price":"50","leverage":2}}`))
	session.handlePayload([]byte(`{"ch":"notifications","type":"resync","ts":101,"sq":8}`))

	update := <-session.events
	if update.Type != "notification" || update.Notification == nil ||
		update.Notification.PositionChange == nil || update.Notification.ID != "n-1" {
		t.Fatalf("notification event = %+v", update)
	}
	resync := <-session.events
	if resync.Type != "resync" || resync.Resync == nil ||
		resync.Resync.Reason != PerpsResyncServerRequest || resync.Sequence != 8 {
		t.Fatalf("resync event = %+v", resync)
	}
}
