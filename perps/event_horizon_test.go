package perps

import (
	"fmt"
	"testing"
)

func TestParseEventHorizon(t *testing.T) {
	// Zero attests no event horizon; it is not an event at the Unix epoch.
	// An absent field means the server did not state a horizon.
	for _, tc := range []struct {
		name string
		wire *int64
		want *int64
	}{
		{"absent", nil, nil},
		{"zero", ptr(int64(0)), nil},
		{"milliseconds", ptr(int64(1791547994123)), ptr(int64(1791547994123))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseEventHorizon(tc.wire)
			if err != nil {
				t.Fatal(err)
			}
			if (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
				t.Fatalf("horizon = %v, want %v", got, tc.want)
			}
		})
	}
	if _, err := parseEventHorizon(ptr(int64(-1))); err == nil {
		t.Fatal("accepted a negative event horizon")
	}
}

func TestMarketEventCarriesHorizon(t *testing.T) {
	f := newMarketFixture(t)
	id := 1
	spec := MarketSubscription{Topic: MarketTickers, InstrumentID: &id}
	result := f.subscribe(f.ctx, spec)
	cmd := f.command("sub", "tickers::1")
	f.write(
		cmd.conn,
		fmt.Sprintf(
			`[{"ch":"tickers::1","ts":1791547994200,"ets":1791547994123,"sq":1,"data":{"iid":1,"mark":"10"}}]`,
		),
	)
	f.ack(cmd)
	handle := f.handle(result)
	event := f.event(handle)
	if event.EventTimestamp == nil || *event.EventTimestamp != 1791547994123 {
		t.Fatalf("event horizon = %v, want 1791547994123", event.EventTimestamp)
	}
	if event.Timestamp != 1791547994200 {
		t.Fatalf("delivery timestamp = %d", event.Timestamp)
	}
	// A malformed horizon rejects the frame instead of reporting epoch data.
	f.write(
		cmd.conn,
		`[{"ch":"tickers::1","ts":1791547994300,"ets":-1,"sq":2,"data":{"iid":1,"mark":"11"}}]`,
	)
	select {
	case <-handle.Errors():
	case <-f.ctx.Done():
		t.Fatal("negative horizon was not reported")
	}
}

func ptr[T any](value T) *T { return &value }
