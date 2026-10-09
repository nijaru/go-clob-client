package clob

import (
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateProtectedMarketOrder(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, amount, price, explicit, maxSpend string
		side                                    Side
		tick                                    TickSize
		position                                bool
		maker, taker                            string
	}{
		// Python py-sdk 1ed8b7cd fixtures. BUY shares round down to cross
		// an ask at the cap, but cannot reach the next finest-grid ask.
		{name: "buy", amount: "100", price: "0.55", side: SideBuy, tick: TickSizeHundredth, maker: "100000000", taker: "181818100"},
		{name: "buy spend cap", amount: "100", price: "0.55", maxSpend: "50", side: SideBuy, tick: TickSizeHundredth, maker: "50000000", taker: "90909000"},
		{name: "sell", amount: "180", price: "0.54", side: SideSell, tick: TickSizeHundredth, maker: "180000000", taker: "97200000"},
		{name: "tenth", amount: "2", price: "0.6", side: SideBuy, tick: TickSizeTenth},
		{name: "half cent", amount: "1.5", price: "0.335", side: SideBuy, tick: TickSizeHalfCent},
		{name: "quarter cent", amount: "3", price: "0.4325", side: SideBuy, tick: TickSizeQuarterCent},
		{name: "finest grid", amount: "100", price: "0.4275", side: SideBuy, tick: TickSizeTenThousand},
		{name: "USDC precision", amount: "100.123456", price: "0.56", side: SideBuy, tick: TickSizeHundredth, maker: "100123456", taker: "178791800"},
		{name: "weaker explicit buy price", amount: "100", price: "0.55", explicit: "0.80", side: SideBuy, tick: TickSizeHundredth, maker: "100000000", taker: "181818100"},
		{name: "weaker explicit sell price", amount: "180", price: "0.54", explicit: "0.40", side: SideSell, tick: TickSizeHundredth, maker: "180000000", taker: "97200000"},
		{name: "tighter explicit buy price", amount: "100", price: "0.55", explicit: "0.50", side: SideBuy, tick: TickSizeHundredth, maker: "100000000", taker: "200000000"},
		{name: "tighter explicit sell price", amount: "180", price: "0.54", explicit: "0.60", side: SideSell, tick: TickSizeHundredth, maker: "180000000", taker: "108000000"},
		{name: "position buy", amount: "100", price: "0.55", side: SideBuy, tick: TickSizeHundredth, position: true, maker: "100000000", taker: "181818100"},
		{name: "position spend cap", amount: "100", price: "0.55", maxSpend: "50", side: SideBuy, tick: TickSizeHundredth, position: true, maker: "50000000", taker: "90909000"},
		{name: "position sell", amount: "180", price: "0.54", side: SideSell, tick: TickSizeHundredth, position: true, maker: "180000000", taker: "97200000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := protectedMarketTestClient(t, tc.tick)
			args := MarketOrderArgs{TokenID: "100", Amount: MustDec(tc.amount), Side: tc.side}
			if tc.position {
				args.TokenID, args.PositionID = "", "100"
			}
			if tc.explicit != "" {
				args.Price = MustDec(tc.explicit)
			}
			bound := MustDec(tc.price)
			if tc.side == SideBuy {
				args.MaxPrice = &bound
			} else {
				args.MinPrice = &bound
			}
			if tc.maxSpend != "" {
				cap := MustDec(tc.maxSpend)
				args.MaxSpend = &cap
			}
			order, err := client.CreateMarketOrder(t.Context(), args, nil)
			if err != nil {
				t.Fatalf("create protected order: %v", err)
			}
			if tc.maker != "" &&
				(order.Order.MakerAmount != tc.maker || order.Order.TakerAmount != tc.taker) {
				t.Fatalf(
					"amounts = %s/%s, want %s/%s",
					order.Order.MakerAmount,
					order.Order.TakerAmount,
					tc.maker,
					tc.taker,
				)
			}
			maker, _ := new(big.Rat).SetString(order.Order.MakerAmount)
			taker, _ := new(big.Rat).SetString(order.Order.TakerAmount)
			if maker.Sign() <= 0 || taker.Sign() <= 0 {
				t.Fatal("nonpositive signed amounts")
			}
			price, _ := new(big.Rat).SetString(tc.price)
			if tc.explicit != "" {
				explicit, _ := new(big.Rat).SetString(tc.explicit)
				if (tc.side == SideBuy && explicit.Cmp(price) < 0) ||
					(tc.side == SideSell && explicit.Cmp(price) > 0) {
					price = explicit
				}
			}
			if tc.side == SideBuy {
				ratio := new(big.Rat).Quo(maker, taker)
				next := new(big.Rat).Add(price, big.NewRat(1, 10000))
				if ratio.Cmp(price) < 0 || ratio.Cmp(next) >= 0 {
					t.Fatalf("encoded BUY price %s outside [%s, %s)", ratio, price, next)
				}
			} else if new(big.Rat).Quo(taker, maker).Cmp(price) < 0 {
				t.Fatal("encoded SELL price below floor")
			}
			if order.Signature == "" {
				t.Fatal("missing signature")
			}
		})
	}
}

func TestCreateProtectedMarketOrderRejectsUnsafeInputs(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, amount, max, min, spend, price string
		side                                 Side
	}{
		{name: "min on buy", amount: "1", min: "0.5", side: SideBuy},
		{name: "max on sell", amount: "1", max: "0.5", side: SideSell},
		{name: "spend on sell", amount: "1", min: "0.5", spend: "1", side: SideSell},
		{name: "zero cap", amount: "1", max: "0", side: SideBuy},
		{name: "negative floor", amount: "1", min: "-0.5", side: SideSell},
		{name: "zero spend", amount: "1", max: "0.5", spend: "0", side: SideBuy},
		{name: "off grid", amount: "1", max: "0.555", side: SideBuy},
		{name: "out of range", amount: "1", max: "1", side: SideBuy},
		{name: "rounding crosses next ask", amount: "0.01", max: "0.7", price: "0.7", side: SideBuy},
		{name: "BUY rounds to zero", amount: "0.0000001", max: "0.5", side: SideBuy},
		{name: "SELL rounds to zero", amount: "0.001", min: "0.5", side: SideSell},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := protectedMarketTestClient(t, TickSizeHundredth)
			args := MarketOrderArgs{
				TokenID: "100",
				Amount:  MustDec(tc.amount),
				Side:    tc.side,
				Price:   MustDec("0.5"),
			}
			if tc.price != "" {
				args.Price = MustDec(tc.price)
			}
			if tc.max != "" {
				v := MustDec(tc.max)
				args.MaxPrice = &v
			}
			if tc.min != "" {
				v := MustDec(tc.min)
				args.MinPrice = &v
			}
			if tc.spend != "" {
				v := MustDec(tc.spend)
				args.MaxSpend = &v
			}
			if _, err := client.CreateMarketOrder(t.Context(), args, nil); err == nil {
				t.Fatal("expected invalid or unsafe protected order to be rejected")
			}
		})
	}
}

func TestProtectedBuyExcludesNextAskAtExactBoundary(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		maker string
		safe  bool
	}{
		{"700099", true}, {"700100", false}, {"700101", false}, {"699999", false},
	} {
		t.Run(tc.maker, func(t *testing.T) {
			err := validateProtectedMarketAmounts(
				MustDec(tc.maker),
				MustDec("1000000"),
				SideBuy,
				MustDec("0.7"),
			)
			if (err == nil) != tc.safe {
				t.Fatalf("safe = %v, error = %v", tc.safe, err)
			}
		})
	}
}

func TestProtectedMarketOrderRefreshesCachedTick(t *testing.T) {
	t.Parallel()
	client := protectedMarketTestClient(t, TickSizeThousandth)
	client.SetTickSize("100", TickSizeHundredth)
	cap := MustDec("0.555")
	order, err := client.CreateMarketOrder(t.Context(), MarketOrderArgs{
		TokenID: "100", Amount: MustDec("100"), Side: SideBuy, MaxPrice: &cap,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if order.Order.MakerAmount != "100000000" || order.Order.TakerAmount != "180180180" {
		t.Fatalf(
			"refreshed order amounts = %s/%s",
			order.Order.MakerAmount,
			order.Order.TakerAmount,
		)
	}
}

func TestProtectedMarketOrderRefreshesUnsafeRounding(t *testing.T) {
	t.Parallel()
	for _, override := range []bool{false, true} {
		t.Run(fmt.Sprint("explicit tick=", override), func(t *testing.T) {
			client := protectedMarketTestClient(t, TickSizeHundredth)
			client.SetTickSize("100", TickSizeTenth)
			cap := MustDec("0.7")
			var options *CreateOrderOptions
			if override {
				options = &CreateOrderOptions{TickSize: TickSizeTenth}
			}
			order, err := client.CreateMarketOrder(t.Context(), MarketOrderArgs{
				TokenID: "100", Amount: MustDec("1"), Side: SideBuy, MaxPrice: &cap,
			}, options)
			if override {
				if err == nil {
					t.Fatal("unsafe explicit tick must not be overridden")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if order.Order.MakerAmount != "1000000" || order.Order.TakerAmount != "1428500" {
				t.Fatalf(
					"refreshed order amounts = %s/%s",
					order.Order.MakerAmount,
					order.Order.TakerAmount,
				)
			}
		})
	}
}

// Protected orders must not depend on book liquidity. Only metadata and
// protocol-version reads are allowed; unexpected requests fail the test.
func protectedMarketTestClient(t *testing.T, tick TickSize) *SignerClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case versionEndpoint:
			_, _ = w.Write([]byte(`{"version":2}`))
		case tickSizeEndpoint:
			_, _ = fmt.Fprintf(w, `{"minimum_tick_size":%q}`, tick)
		case negRiskEndpoint:
			_, _ = w.Write([]byte(`{"neg_risk":false}`))
		case marketsByTokenEndpoint + "100":
			_, _ = w.Write([]byte(`{"condition_id":"cid"}`))
		case clobMarketEndpoint + "/cid":
			_, _ = fmt.Fprintf(
				w,
				`{"c":"cid","mts":%q,"nr":false,"fd":{"r":"0","e":0},"t":[{"t":"100"}]}`,
				tick,
			)
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)
	client, err := NewSignerClient(Config{
		Host: server.URL, RetryMax: 0,
		PrivateKey: "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}
