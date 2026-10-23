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

func TestEquityMarketStatus(t *testing.T) {
	spot := priceFixture("price.equity", "aapl", true)
	spot = strings.Replace(
		spot,
		`"source":"newvendor",`,
		`"source":"newvendor","market_status":"closed",`,
		1,
	)
	event, ok := parseEvent([]byte(spot))
	if !ok || event.MarketStatus != MarketClosed {
		t.Fatalf("snapshot status = %q (ok=%v), want closed", event.MarketStatus, ok)
	}
	twap := priceFixture("price.equity.twap", "aapl", false)
	twap = strings.Replace(
		twap,
		`"source":"newvendor",`,
		`"source":"newvendor","market_status":"open",`,
		1,
	)
	if event, ok := parseEvent([]byte(twap)); !ok || event.MarketStatus != MarketOpen {
		t.Fatalf("twap status = %q (ok=%v), want open", event.MarketStatus, ok)
	}
	// Omitted status means not stated, never open.
	if event, ok := parseEvent([]byte(priceFixture("price.equity", "aapl", false))); !ok ||
		event.MarketStatus != "" {
		t.Fatalf("absent status = %q (ok=%v), want unstated", event.MarketStatus, ok)
	}
	// Crypto channels carry no market status, even as an extension field.
	crypto := priceFixture("price.crypto", "btcusd", false)
	crypto = strings.Replace(
		crypto,
		`"source":"newvendor",`,
		`"source":"newvendor","market_status":"closed",`,
		1,
	)
	if event, ok := parseEvent([]byte(crypto)); !ok || event.MarketStatus != "" {
		t.Fatalf("crypto status = %q (ok=%v), want unstated", event.MarketStatus, ok)
	}
	// A new status value is retained rather than silencing the price feed.
	future := priceFixture("price.equity", "aapl", false)
	future = strings.Replace(
		future,
		`"source":"newvendor",`,
		`"source":"newvendor","market_status":"halted",`,
		1,
	)
	if event, ok := parseEvent([]byte(future)); !ok || event.MarketStatus != "halted" {
		t.Fatalf("future status = %q (ok=%v), want halted", event.MarketStatus, ok)
	}
	// A malformed status type still rejects the frame.
	bad := priceFixture("price.equity", "aapl", false)
	bad = strings.Replace(
		bad,
		`"source":"newvendor",`,
		`"source":"newvendor","market_status":7,`,
		1,
	)
	if _, ok := parseEvent([]byte(bad)); ok {
		t.Fatal("accepted a numeric market status")
	}
}

func TestStaleEquityStatusKeepsHistory(t *testing.T) {
	snapshot := `{"v":1,"channel":"price.equity","seq":1,"ts":123456,"snapshot":true,"payload":{"symbol":"aapl","source":"pyth","market_status":"open","data":[{"timestamp":123456,"value":123.45,"full_accuracy_value":"123.450000000000000001"}]}}`
	previous, ok := parseEvent([]byte(snapshot))
	if !ok {
		t.Fatal("snapshot rejected")
	}
	// A status-only update repeats the last price timestamp while the retained
	// history already extends past it.
	statusOnly := `{"v":1,"channel":"price.equity","seq":2,"ts":123458,"payload":{"symbol":"aapl","source":"pyth","market_status":"closed","timestamp":123450,"value":123.45,"full_accuracy_value":"123.450000000000000001","received_at":123458,"is_carried_forward":true}}`
	update, ok := parseEvent([]byte(statusOnly))
	if !ok {
		t.Fatal("status-only update rejected")
	}
	refreshed := refreshSnapshot(&previous, update)
	if refreshed.Type != SnapshotEvent || refreshed.MarketStatus != MarketClosed {
		t.Fatalf("status not applied: %+v", refreshed)
	}
	if len(refreshed.Snapshot.Data) != 1 ||
		refreshed.Snapshot.Data[0].Timestamp.UnixMilli() != 123456 {
		t.Fatalf("history regressed to the stale price: %+v", refreshed.Snapshot.Data)
	}
	// TWAP status changes only with a new TWAP price.
	twapSnapshot := strings.Replace(
		snapshot,
		`"channel":"price.equity"`,
		`"channel":"price.equity.twap"`,
		1,
	)
	twapSnapshot = strings.Replace(
		twapSnapshot,
		`"source":"pyth",`,
		`"source":"pyth","window_seconds":60,`,
		1,
	)
	twapPrevious, ok := parseEvent([]byte(twapSnapshot))
	if !ok {
		t.Fatal("twap snapshot rejected")
	}
	twapUpdate := strings.Replace(
		strings.Replace(statusOnly, `"channel":"price.equity"`, `"channel":"price.equity.twap"`, 1),
		`"window_seconds":60,`,
		``,
		1,
	)
	// The update fixture has no window yet; add the fixed TWAP window.
	twapUpdate = strings.Replace(
		twapUpdate,
		`"source":"pyth",`,
		`"source":"pyth","window_seconds":60,`,
		1,
	)
	update, ok = parseEvent([]byte(twapUpdate))
	if !ok {
		t.Fatalf("twap update rejected: %s", twapUpdate)
	}
	refreshed = refreshSnapshot(&twapPrevious, update)
	if refreshed.MarketStatus != MarketOpen {
		t.Fatalf("twap status adopted without a new price: %+v", refreshed)
	}
}
