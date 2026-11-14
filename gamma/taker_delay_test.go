package gamma

import (
	"encoding/json"
	"testing"
)

// Both current official SDKs expose takerDelayMs independently of secondsDelay.
func TestMarketTakerDelayMilliseconds(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		payload string
		want    *int
	}{
		{payload: `{"secondsDelay":2}`},
		{payload: `{"secondsDelay":2,"takerDelayMs":null}`},
		{payload: `{"secondsDelay":2,"takerDelayMs":0}`, want: new(0)},
		{payload: `{"secondsDelay":2,"takerDelayMs":500}`, want: new(500)},
	} {
		var market Market
		if err := json.Unmarshal([]byte(tc.payload), &market); err != nil {
			t.Fatal(err)
		}
		if market.SecondsDelay == nil || *market.SecondsDelay != 2 {
			t.Fatalf("legacy delay was lost: %+v", market.SecondsDelay)
		}
		if (market.TakerDelayMs == nil) != (tc.want == nil) {
			t.Fatalf("taker delay presence: %+v", market.TakerDelayMs)
		}
		if market.TakerDelayMs != nil && *market.TakerDelayMs != *tc.want {
			t.Fatalf("taker delay = %d, want %d", *market.TakerDelayMs, *tc.want)
		}
	}
}
