package perps

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Independent wire contract: TS 087f944 bindings/perps/common.ts and
// builders.ts use a nonempty string earning_id in REST and builderFills;
// Py ed8d04ca models/perps/builders.py agrees. These are constructed wire
// examples, not captured live receipts; no numeric/UUID format is promised.
func TestBuilderReceiptRecoveryIdentity(t *testing.T) {
	const backfill = `[{"earning_id":"receipt:opaque/01","trade_id":7,"sequence":10},{"earning_id":"0007","trade_id":7,"sequence":10}]`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/account/builder-earnings" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":` + backfill + `,"more":false}`))
	}))
	t.Cleanup(server.Close)
	client := fixtureClient(t, Config{Host: server.URL})
	recovered := map[string]PerpsBuilderEarning{}
	for items, err := range client.IterBuilderEarnings(t.Context(), BuilderReportingParams{}) {
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range items {
			recovered[item.EarningID] = item
		}
	}
	event := PerpsSessionEvent{
		Channel: "builderFills",
		Data: json.RawMessage(
			`[{"earning_id":"receipt:opaque/01","trade_id":7,"sequence":10},{"earning_id":"receipt:opaque/02","trade_id":7,"sequence":10}]`,
		),
	}
	items, err := event.AsBuilderFills()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		recovered[item.EarningID] = item
	}
	if len(recovered) != 3 || recovered["0007"].EarningID != "0007" ||
		recovered["receipt:opaque/02"].TradeID != 7 {
		t.Fatalf("recovery identity lost: %+v", recovered)
	}
}

func TestBuilderReceiptRejectsInvalidIdentity(t *testing.T) {
	for _, wire := range []string{`{}`, `{"earning_id":null}`, `{"earning_id":""}`, `{"earning_id":259}`} {
		var page BuilderEarningsPage
		if err := json.Unmarshal([]byte(`{"data":[`+wire+`]}`), &page); err == nil {
			t.Errorf("REST accepted %s", wire)
		}
		event := PerpsSessionEvent{Channel: "builderFills", Data: json.RawMessage(`[` + wire + `]`)}
		if _, err := event.AsBuilderFills(); err == nil {
			t.Errorf("stream accepted %s", wire)
		}
	}
}
