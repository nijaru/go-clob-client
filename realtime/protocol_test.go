package realtime

import (
	"strings"
	"testing"
)

func TestEnvelopeValidation(t *testing.T) {
	valid := priceFixture("price.crypto", "btcusd", false)
	for _, row := range []struct{ name, old, replacement string }{
		{"future version", `"v":1`, `"v":2`},
		{"missing sequence", `"seq":2,`, ``},
		{"negative sequence", `"seq":2`, `"seq":-1`},
		{"fractional timestamp", `"ts":123457`, `"ts":1.5`},
		{"unknown channel", `price.crypto`, `price.future`},
		{"missing source", `"source":"newvendor",`, ``},
		{"null source", `"source":"newvendor"`, `"source":null`},
		{"nonfinite exact", `123.450000000000000002`, `NaN`},
		{"missing exact", `"full_accuracy_value":"123.450000000000000002",`, ``},
		{"numeric exact", `"full_accuracy_value":"123.450000000000000002"`, `"full_accuracy_value":123.45`},
		{"boolean approximate", `"value":123.45`, `"value":true`},
		{"nonfinite approximate", `"value":123.45`, `"value":"Infinity"`},
		{"overflowing numeric approximate", `"value":123.45`, `"value":1e400`},
		{"negative dropped", `"dropped":3`, `"dropped":-1`},
		{"null dropped", `"dropped":3`, `"dropped":null`},
		{"invalid carry", `"is_carried_forward":false`, `"is_carried_forward":"false"`},
		{"null received", `"received_at":123457`, `"received_at":null`},
		{"null snapshot", `"v":1,`, `"v":1,"snapshot":null,`},
	} {
		t.Run(row.name, func(t *testing.T) {
			if _, ok := parseEvent([]byte(strings.Replace(valid, row.old, row.replacement, 1))); ok {
				t.Fatal("accepted malformed envelope")
			}
		})
	}
	for _, channel := range []string{"price.crypto.twap", "price.equity.twap"} {
		for _, snapshot := range []bool{false, true} {
			raw := priceFixture(channel, "eurusd", snapshot)
			if _, ok := parseEvent([]byte(raw)); !ok {
				t.Fatal("valid TWAP rejected")
			}
			if _, ok := parseEvent([]byte(strings.Replace(raw, `"window_seconds":60`, `"window_seconds":30`, 1))); ok {
				t.Fatal("unsupported TWAP window accepted")
			}
		}
	}
	snap := priceFixture("price.equity", "aapl", true)
	if _, ok := parseEvent([]byte(strings.Replace(snap, `"data":[`, `"data":null,"ignored":[`, 1))); ok {
		t.Fatal("null snapshot data accepted")
	}
	// String approximate prices and scientific exact decimals are both allowed;
	// arbitrary precision is retained without float or fixed-scale conversion.
	raw := strings.Replace(valid, `"value":123.45`, `"value":"123.45"`, 1)
	raw = strings.Replace(
		raw,
		`123.450000000000000002`,
		`1.2345000000000000000000000000000000000002e100`,
		1,
	)
	event, ok := parseEvent([]byte(raw))
	if !ok || event.Update.Value != "1.2345000000000000000000000000000000000002e100" {
		t.Fatal("exact price was not retained")
	}
}

func TestRequestValidationBeforeNetworkAllocation(t *testing.T) {
	f := startFeed(t, nil)
	c := testClient(t, testConfig(f))
	for _, request := range []Request{
		{Channel: "bad", Symbols: []string{"btcusd"}},
		{Channel: Crypto},
		{Channel: Crypto, Symbols: []string{"btc/usd"}},
		{Channel: Crypto, Symbols: []string{"BTCUSD"}},
		{Channel: Crypto, Symbols: []string{"btcusdt"}},
		{Channel: Crypto, Symbols: []string{"btcusd", "not-a-pair"}},
		{Channel: Equity, Symbols: []string{" "}},
		{Channel: Equity, Symbols: []string{"a$pl"}},
		{Channel: Equity, Symbols: []string{strings.Repeat("a", 65)}},
		{Channel: Crypto, Symbols: []string{"btcusd"}, Provider: "massive"},
		{Channel: Crypto, Symbols: []string{"btcusd"}, Types: []EventType{Accepted}},
	} {
		if _, err := c.Subscribe(t.Context(), request); err == nil {
			t.Fatalf("accepted invalid request: %+v", request)
		}
	}
	c.Close()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.observations) != 0 {
		t.Fatal("invalid request opened a connection")
	}
}
