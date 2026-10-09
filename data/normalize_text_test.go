package data

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestOptionalDisplayTextAbsence(t *testing.T) {
	for _, tc := range []struct {
		name, required string
		value          any
		fields         map[string]string // Wire name to public field.
	}{
		{"trader", `"rank":1,"user_id":"wallet","pnl":0,"volume":0,"verified":false`, &TraderLeaderboardEntry{}, map[string]string{"user_name": "UserName", "profile_image": "ProfileImage", "x_username": "XUsername"}},
		{"standing", `"user_id":"wallet","pnl":0,"volume":0,"verified":false`, &TraderLeaderboardStanding{}, map[string]string{"user_name": "UserName", "profile_image": "ProfileImage", "x_username": "XUsername"}},
		{"builder", `"rank":1,"builder_name":"builder","builder_code":"0x01","verified":false,"volume":0,"active_users":0`, &BuilderStanding{}, map[string]string{"profile_image": "ProfileImage"}},
		{"builder volume", `"rank":1,"builder_name":"builder","builder_code":"0x01","verified":false,"volume":0,"active_users":0,"date":"2026-09-10"`, &BuilderVolumePoint{}, map[string]string{"profile_image": "ProfileImage"}},
		{"market winner", strings.TrimSuffix(strings.Replace(fmt.Sprintf(comboWinnerJSON, `"0xAB"`), `"kind":"combo"`, `"kind":"market","event_id":1`, 1), "}")[1:], &BiggestWinner{}, map[string]string{"user_name": "UserName", "profile_image": "ProfileImage", "event_title": "EventTitle", "event_slug": "EventSlug"}},
		{"combo winner", strings.TrimSuffix(fmt.Sprintf(comboWinnerJSON, `"0x03`+strings.Repeat("ab", 30)+`"`), "}")[1:], &BiggestWinner{}, map[string]string{"user_name": "UserName", "profile_image": "ProfileImage", "event_title": "EventTitle"}},
		{"event", `"event_id":"1"`, &ComboPositionMarketEvent{}, map[string]string{"event_slug": "EventSlug", "event_title": "EventTitle", "event_image": "EventImage"}},
		{"market", `"market_id":"1"`, &ComboPositionMarket{}, map[string]string{"slug": "Slug", "title": "Title", "question": "Question", "group_item_title": "GroupItemTitle", "sports_market_type": "SportsMarketType", "outcome": "Outcome", "image_url": "ImageURL", "icon_url": "IconURL", "category": "Category", "subcategory": "Subcategory"}},
		{"leg", `"leg_index":0,"leg_position_id":"position","leg_condition_id":"0x01","leg_outcome_index":1,"leg_status":"OPEN"`, &ComboPositionLeg{}, map[string]string{"leg_outcome_label": "LegOutcomeLabel"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, text := range []string{`"populated text"`, `" "`, `""`, `null`, "missing"} {
				raw := "{" + tc.required
				if text != "missing" {
					for wireName := range tc.fields {
						raw += fmt.Sprintf(",%q:%s", wireName, text)
					}
				}
				raw += "}"
				if err := json.Unmarshal([]byte(raw), tc.value); err != nil {
					t.Fatal(err)
				}
				for _, publicName := range tc.fields {
					field := reflect.ValueOf(tc.value).Elem().FieldByName(publicName)
					if text == `""` || text == `null` || text == "missing" {
						if !field.IsNil() {
							t.Fatalf("%s: %s retained an absence marker", text, publicName)
						}
					} else if field.IsNil() || fmt.Sprintf("%q", field.Elem().String()) != text {
						t.Fatalf("%s: %s changed populated text", text, publicName)
					}
				}
			}
		})
	}
}

func TestNestedComboTextAndRequiredLegValidation(t *testing.T) {
	leg := `{"leg_index":0,"leg_position_id":"leg","leg_condition_id":"0xABC","leg_outcome_index":1,"leg_status":"OPEN","leg_outcome_label":"","market":{"title":"","market_id":"market","event":{"event_title":""}}}`
	raw := strings.Replace(
		fmt.Sprintf(comboPositionJSON, `"0x03`+strings.Repeat("ab", 30)+`"`),
		`"legs":[]`,
		`"legs":[`+leg+`]`,
		1,
	)
	var position ComboPosition
	if err := json.Unmarshal([]byte(raw), &position); err != nil {
		t.Fatal(err)
	}
	got := position.Legs[0]
	if got.LegConditionID != "0xABC" || got.LegPositionID != "leg" || got.LegOutcomeLabel != nil ||
		got.Market.Title != nil ||
		got.Market.Event.EventTitle != nil ||
		got.Market.MarketID == nil ||
		*got.Market.MarketID != "market" {
		t.Fatalf("nested absence/identity changed: %+v", got)
	}
	broken := strings.Replace(raw, `"leg_position_id":"leg",`, "", 1)
	if err := json.Unmarshal([]byte(broken), &position); err == nil {
		t.Fatal("nested custom normalization bypassed required financial leg fields")
	}
}
