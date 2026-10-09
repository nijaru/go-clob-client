package clob

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	json "github.com/go-json-experiment/json"
)

func TestNotificationScalarAndDateUnion(t *testing.T) {
	// TS 087f944 bindings/clob/notifications.ts: Decimalish and date-like
	// payloads. Raw independent JSON is important: encoding Go fixtures hid
	// numeric-string mismatches and ISO envelope timestamp failures.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(
			w,
			`[{"id":1,"owner":"owner","type":3,"timestamp":"2026-10-09T12:13:14.123Z","payload":{"minimum_order_size":9007199254740993.125,"minimum_tick_size":0.0001,"maker_base_fee":0.5,"accepting_order_timestamp":1791547994123,"end_date_iso":"2026-10-09","game_start_time":"2026-10-09T14:13:14.123+02:00","tokens":[{"token_id":"native","outcome":"Yes","price":0.125,"winner":true}],"rewards":{"min_size":"9007199254740993.125","max_spread":0.25,"rates":[{"asset_address":"asset","rewards_daily_rate":"0.00000000000000000001"}]}}},{"id":2,"owner":"owner","type":6,"timestamp":1791547994123,"payload":{"id":"comment-123","parentCommentID":"parent-1","createdAt":"2026-10-09T12:13:14.123Z","profile":{"name":"Author","isMod":false}}},{"id":3,"owner":"owner","type":5,"timestamp":"1791547994123","payload":{"reward":9007199254740993.125}}]`,
		)
	}))
	t.Cleanup(server.Close)
	client, err := NewAuthenticatedClient(
		Config{
			Host:        server.URL,
			PrivateKey:  gaslessTestKey,
			Credentials: &Credentials{Key: "owner", Secret: "c2VjcmV0", Passphrase: "p"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	notifications, err := client.GetNotifications(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	market := notifications[0].Payload.MarketRegistered
	if notifications[0].Timestamp != 1791547994123 ||
		market.MinimumOrderSize != "9007199254740993.125" ||
		market.MinimumTickSize != "0.0001" ||
		market.MakerBaseFee == nil ||
		*market.MakerBaseFee != "0.5" ||
		market.Tokens[0].Price != "0.125" ||
		market.Rewards.MinSize != "9007199254740993.125" ||
		market.Rewards.Rates[0].DailyRate != "0.00000000000000000001" {
		t.Fatalf("lost exact metadata: %+v", market)
	}
	if market.AcceptingOrdersTimestamp == nil ||
		market.AcceptingOrdersTimestamp.UnixMilli() != 1791547994123 ||
		market.EndDate == nil ||
		market.EndDate.Format(time.DateOnly) != "2026-10-09" ||
		market.GameStartTime == nil ||
		market.GameStartTime.UnixMilli() != 1791547994123 {
		t.Fatalf("date-like metadata: %+v", market)
	}
	comment := notifications[1].Payload.ChildComment
	if comment.ID != "comment-123" || comment.ParentCommentID == nil ||
		*comment.ParentCommentID != "parent-1" ||
		comment.CreatedAt == nil ||
		comment.Profile == nil ||
		comment.Profile.Name == nil ||
		*comment.Profile.Name != "Author" {
		t.Fatalf("comment identity/profile: %+v", comment)
	}
	if notifications[2].Payload.RewardPayout.Reward != "9007199254740993.125" {
		t.Fatalf("reward rounded: %+v", notifications[2])
	}
}

func TestNotificationReuseAndNumericCommentID(t *testing.T) {
	var notification Notification
	if err := json.Unmarshal([]byte(`{"type":6,"timestamp":1791547994123,"payload":{"id":123,"parentCommentID":456}}`), &notification); err != nil {
		t.Fatal(err)
	}
	if notification.Payload.ChildComment.ID != "123" ||
		*notification.Payload.ChildComment.ParentCommentID != "456" {
		t.Fatalf("numeric comment IDs: %+v", notification)
	}
	if err := json.Unmarshal([]byte(`{"type":7,"timestamp":1791547994123,"payload":{"amount":0.125}}`), &notification); err != nil {
		t.Fatal(err)
	}
	if notification.Payload.ChildComment != nil || notification.Payload.YieldPayout == nil ||
		notification.Payload.YieldPayout.Amount != "0.125" {
		t.Fatalf("multiple/stale variants: %+v", notification)
	}
	for _, timestamp := range []string{`true`, `"not-a-date"`, `{}`} {
		if err := json.Unmarshal([]byte(`{"type":7,"timestamp":`+timestamp+`,"payload":{"amount":"0.1"}}`), &notification); err == nil {
			t.Fatalf("accepted invalid time %s", timestamp)
		}
	}
}

func TestBalanceNumericWire(t *testing.T) {
	var result BalanceAllowanceResponse
	if err := json.Unmarshal([]byte(`{"balance":9007199254740993,"allowances":{"asset":9007199254740995,"quoted":"10"}}`), &result); err != nil {
		t.Fatal(err)
	}
	if result.Balance != "9007199254740993" || result.Allowances["asset"] != "9007199254740995" ||
		result.Allowances["quoted"] != "10" {
		t.Fatalf("integer units rounded: %+v", result)
	}
}
