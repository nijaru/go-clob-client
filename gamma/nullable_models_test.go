package gamma

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDiscoveryRetainsNullableStateAndExactFees(t *testing.T) {
	// Rust response.rs models these scalars as Option; TS GammaMarketSchema
	// also accepts fractional legacy fees and nullish state flags.
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(
			w,
			`[{"id":"1","acceptingOrders":null,"closed":null,"question":null,"imageOptimized":null,"feeSchedule":null},{"id":"2","acceptingOrders":false,"closed":false,"question":"","secondsDelay":0,"makerBaseFee":0.125,"takerBaseFee":"0.5000","makerRebatesFeeShareBps":2.5,"feeSchedule":{"exponent":1.500,"rate":"0.00000000000000000001","takerOnly":false}}]`,
		)
	})
	markets, err := client.GetMarkets(t.Context(), MarketFilterParams{})
	if err != nil {
		t.Fatal(err)
	}
	unknown, zero := markets[0], markets[1]
	if unknown.AcceptingOrders != nil || unknown.Closed != nil || unknown.Question != nil ||
		unknown.ImageOptimized != nil ||
		unknown.FeeSchedule != nil {
		t.Fatalf("invented state for null metadata: %+v", unknown)
	}
	if zero.AcceptingOrders == nil || *zero.AcceptingOrders || zero.Closed == nil || *zero.Closed ||
		zero.Question == nil ||
		*zero.Question != "" ||
		zero.SecondsDelay == nil ||
		*zero.SecondsDelay != 0 {
		t.Fatalf("lost explicit zero metadata: %+v", zero)
	}
	if zero.MakerBaseFee != "0.125" || zero.TakerBaseFee != "0.5000" ||
		zero.MakerRebatesFeeShareBps != "2.5" ||
		zero.FeeSchedule.Exponent != "1.500" ||
		zero.FeeSchedule.Rate != "0.00000000000000000001" {
		t.Fatalf("fee precision lost: %+v", zero)
	}
	encoded, err := json.Marshal(zero)
	if err != nil {
		t.Fatal(err)
	}
	for _, present := range []string{`"acceptingOrders":false`, `"closed":false`, `"question":""`, `"secondsDelay":0`} {
		if !strings.Contains(string(encoded), present) {
			t.Fatalf("lost present value %s: %s", present, encoded)
		}
	}
	// Reusing a destination must clear a prior scalar rather than treat null as false.
	if err := json.Unmarshal([]byte(`{"id":"3"}`), &zero); err != nil {
		t.Fatal(err)
	}
	if zero.AcceptingOrders != nil || zero.Closed != nil || zero.Question != nil {
		t.Fatalf("stale state: %+v", zero)
	}
}

func TestDiscoveryNestedNullAndZero(t *testing.T) {
	// Unknown versus false matters when evaluating event readiness and moderation.
	var event Event
	if err := json.Unmarshal([]byte(`{"id":"1","active":null,"negRisk":null,"teams":[{"id":1,"providerId":null},{"id":2,"providerId":0}],"eventCreators":[{"id":"a","creatorName":null}],"markets":[{"id":"1","acceptingOrders":false}]}`), &event); err != nil {
		t.Fatal(err)
	}
	if event.Active != nil || event.NegativeRisk != nil || event.Teams[0].ProviderID != nil ||
		event.Teams[1].ProviderID == nil ||
		*event.Teams[1].ProviderID != 0 ||
		event.EventCreators[0].CreatorName != nil ||
		event.Markets[0].AcceptingOrders == nil ||
		*event.Markets[0].AcceptingOrders {
		t.Fatalf("nested optional metadata: %+v", event)
	}
	var comment Comment
	if err := json.Unmarshal([]byte(`{"id":"1","profile":null,"reactions":[{"id":"r","profile":{"isMod":false,"displayUsernamePublic":null}}]}`), &comment); err != nil {
		t.Fatal(err)
	}
	if comment.Profile != nil || comment.Reactions[0].Profile.IsMod == nil ||
		*comment.Reactions[0].Profile.IsMod ||
		comment.Reactions[0].Profile.DisplayUsernamePublic != nil {
		t.Fatalf("comment moderation state: %+v", comment)
	}
}

func TestTimestampWireNormalization(t *testing.T) {
	// Python gamma/common.py::parse_optional_datetime accepts all these forms.
	for _, input := range []string{`"2026-10-09 12:13:14.123456+00"`, `"2026-10-09T14:13:14.123456+02:00"`, `"2026-10-09T12:13:14.123456+0000"`, `"2026-10-09T12:13:14.123456"`} {
		var event Event
		if err := json.Unmarshal([]byte(`{"id":"1","createdAt":`+input+`,"startTime":`+input+`}`), &event); err != nil {
			t.Fatal(err)
		}
		if event.CreatedAt != "2026-10-09T12:13:14.123456Z" || event.StartTime != event.CreatedAt {
			t.Fatalf("timestamp %s: %+v", input, event)
		}
		instant, err := event.CreatedAt.Time()
		if err != nil || instant.Location() != time.UTC || instant.Nanosecond() != 123456000 {
			t.Fatalf("time: %v %v", instant, err)
		}
	}
	for _, input := range []string{`null`, `""`} {
		var event Event
		if err := json.Unmarshal([]byte(`{"id":"1","createdAt":`+input+`}`), &event); err != nil {
			t.Fatal(err)
		}
		if event.CreatedAt != "" {
			t.Fatalf("invented absent time: %s", event.CreatedAt)
		}
	}
	var market Market
	if err := json.Unmarshal([]byte(`{"id":"1","endDate":"2026-10-09","gameStartTime":"2026-10-09 12:13:14+00"}`), &market); err != nil {
		t.Fatal(err)
	}
	if market.EndDate != "2026-10-09T00:00:00Z" || market.GameStartTime != "2026-10-09T12:13:14Z" {
		t.Fatalf("market schedule: %+v", market)
	}
	var minutes Timestamp
	if err := json.Unmarshal([]byte(`"2026-10-09T12:13+00:00"`), &minutes); err != nil ||
		minutes != "2026-10-09T12:13:00Z" {
		t.Fatalf("minute timestamp: %s %v", minutes, err)
	}
	for _, input := range []string{`true`, `12`, `"2026-02-30"`, `"not-a-date"`} {
		if err := json.Unmarshal([]byte(input), new(Timestamp)); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}

func TestOutcomeAssetWireIndex(t *testing.T) {
	var market Market
	if err := json.Unmarshal([]byte(`{"id":"1","outcomes":"[\"A\",\"B\",\"C\"]","outcomePrices":["0.1234567890123456789"],"clobTokenIds":["11","12"],"positionIds":["21","22","23"]}`), &market); err != nil {
		t.Fatal(err)
	}
	assets := market.OutcomeDetails()
	if len(assets) != 3 || assets[0].Index != 0 || assets[0].Label != "A" ||
		assets[0].Price != "0.1234567890123456789" ||
		*assets[0].TokenID != "11" ||
		*assets[0].PositionID != "21" ||
		assets[2].Index != 2 ||
		assets[2].TokenID != nil ||
		assets[2].Price != "" ||
		*assets[2].PositionID != "23" {
		t.Fatalf("outcome routing: %+v", assets)
	}
	*assets[0].TokenID = "changed"
	if market.CLOBTokenIDs[0] != "11" {
		t.Fatal("derived asset mutated wire metadata")
	}
}
