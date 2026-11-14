package data

import (
	"fmt"
	"testing"

	json "github.com/go-json-experiment/json"
)

// These are the directional and threshold event contracts introduced in the
// official Python/TypeScript SDKs in November 2026.
func TestRangeEventActivityContracts(t *testing.T) {
	t.Parallel()
	base := `"proxy_wallet":"0x7c3db723f1d4d8cb9c550095203b686cb11e5c6b",
		"timestamp":1785425912,"transaction_hash":"0x01",
		"title":"Range","event_slug":"range-1"`

	for _, kind := range []ActivityType{
		ActivityTypeDirectionalSplit, ActivityTypeDirectionalMerge,
	} {
		t.Run(string(kind), func(t *testing.T) {
			raw := fmt.Sprintf(
				`{"type":%q,%s,"usdc_size":25,"condition_id":"","side":"",
				"outcome":"","outcome_index":999}`, kind, base,
			)
			var row Activity
			if err := json.Unmarshal([]byte(raw), &row); err != nil {
				t.Fatal(err)
			}
			if row.Type != kind || row.Amount == nil || *row.Amount != "25" ||
				row.ConditionID != nil || row.Title == nil || *row.Title != "Range" ||
				row.EventSlug == nil || *row.EventSlug != "range-1" || row.Raw != nil {
				t.Fatalf("directional row lost event-level information: %+v", row)
			}
			if err := json.Unmarshal(
				[]byte(fmt.Sprintf(`{"type":%q,%s}`, kind, base)),
				&row,
			); err == nil {
				t.Fatal("accepted directional row without a collateral amount")
			}
		})
	}
	for _, kind := range []ActivityType{
		ActivityTypeThresholdCompose, ActivityTypeThresholdDecompose,
	} {
		t.Run(string(kind), func(t *testing.T) {
			raw := fmt.Sprintf(
				`{"type":%q,%s,"condition_id":"0x01",
				"size":100.005,"usdc_size":0,"outcome":"Yes","outcome_index":0}`,
				kind, base,
			)
			var row Activity
			if err := json.Unmarshal([]byte(raw), &row); err != nil {
				t.Fatal(err)
			}
			if row.Type != kind || row.ConditionID == nil ||
				*row.ConditionID != "0x01" || row.Shares == nil ||
				*row.Shares != "100.005" || row.Amount != nil ||
				row.Outcome == nil || *row.Outcome != "Yes" ||
				row.OutcomeIndex == nil || *row.OutcomeIndex != 0 ||
				row.Raw != nil {
				t.Fatalf("threshold row lost share/market information: %+v", row)
			}
			for _, missing := range []string{
				`"condition_id":"0x01"`,
				`"size":100`,
			} {
				if err := json.Unmarshal(
					[]byte(fmt.Sprintf(`{"type":%q,%s,%s}`, kind, base, missing)),
					&row,
				); err == nil {
					t.Fatalf("accepted incomplete threshold row: %s", missing)
				}
			}
		})
	}
}
