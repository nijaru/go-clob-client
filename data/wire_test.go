package data

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	json "github.com/go-json-experiment/json"
)

func TestIncompleteFinancialRowsAreRejected(t *testing.T) {
	for _, raw := range []string{`{}`, `null`, `{"proxy_wallet":"wallet","current_value":0}`} {
		var position Position
		if err := json.Unmarshal([]byte(raw), &position); err == nil {
			t.Fatalf("accepted incomplete position %s", raw)
		}
	}
	missingPrice := strings.Replace(tradeFixture("42", "1"), `"price":"0.5",`, "", 1)
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(
			w,
			`{"data":[%s],"pagination":{"has_more":false,"next_cursor":null}}`,
			missingPrice,
		)
	})
	if _, err := client.GetTrades(t.Context(), TradesParams{}); err == nil {
		t.Fatal("trade with no price succeeded")
	}
	base := `"proxy_wallet":"0x7c3db723f1d4d8cb9c550095203b686cb11e5c6b","timestamp":1785425912,"transaction_hash":"0x01"`
	for _, body := range []string{
		`{"type":"DEPOSIT",` + base + `}`,
		`{"type":"REWARD",` + base + `}`,
		`{"type":"SPLIT",` + base + `,"usdc_size":1}`,
	} {
		var activity Activity
		if err := json.Unmarshal([]byte(body), &activity); err == nil {
			t.Fatalf("accepted incomplete financial activity %s", body)
		}
	}
}

func TestGlobalInterestAndUnknownOutcome(t *testing.T) {
	var interest OpenInterest
	if err := json.Unmarshal([]byte(`{"condition_id":"GLOBAL","value":1.125}`), &interest); err != nil {
		t.Fatal(err)
	}
	if interest.ConditionID != nil || interest.Value != "1.125" {
		t.Fatalf("global interest misclassified: %+v", interest)
	}
	tradeRaw := strings.Replace(
		tradeFixture("42", "1"),
		`"price":"0.5",`,
		`"price":"0.5","outcome_index":999,"title":"",`,
		1,
	)
	var trade Trade
	if err := json.Unmarshal([]byte(tradeRaw), &trade); err != nil {
		t.Fatal(err)
	}
	if trade.OutcomeIndex != nil || trade.Title != nil {
		t.Fatal("trade absence markers were preserved as real metadata")
	}
	activityRaw := strings.Replace(
		tradeRaw,
		`"price":"0.5",`,
		`"price":"0.5","type":"TRADE","usdc_size":0.5,`,
		1,
	)
	var activity Activity
	if err := json.Unmarshal([]byte(activityRaw), &activity); err != nil {
		t.Fatal(err)
	}
	if activity.OutcomeIndex != nil || activity.Title != nil {
		t.Fatal("activity absence markers were preserved as real metadata")
	}
}
