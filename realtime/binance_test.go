package realtime

import (
	"strings"
	"testing"
)

func TestBinanceProviderAndPriceSource(t *testing.T) {
	t.Parallel()
	keys, err := requestKeys(Request{
		Channel:  Crypto,
		Symbols:  []string{"btcusd"},
		Provider: Binance,
	})
	if err != nil || len(keys) != 1 || keys[0].provider != Binance {
		t.Fatalf("Binance request rejected: keys=%+v err=%v", keys, err)
	}
	sub := keys[0].wire()
	if sub.Filter.Provider != Binance || sub.Filter.Symbol != "btcusd" {
		t.Fatalf("Binance filter lost provider: %+v", sub)
	}
	wire := strings.Replace(
		priceFixture(string(Crypto), "btcusd", false),
		`"source":"newvendor"`, `"source":"binance"`, 1,
	)
	e, ok := parseEvent([]byte(wire))
	if !ok || e.Source != SourceBinance || e.Update == nil ||
		e.Update.Value != "123.450000000000000002" {
		t.Fatalf("Binance price provenance lost: %+v, parsed=%v", e, ok)
	}
}
