package clob

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

func TestPositionOrdersResolveMarketTickBookAndFees(t *testing.T) {
	t.Parallel()
	var markets atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case marketsByTokenEndpoint + "999":
			_, _ = w.Write([]byte(`{"condition_id":"cid"}`))
		case clobMarketEndpoint + "/cid":
			markets.Add(1)
			_, _ = w.Write(
				[]byte(
					`{"c":"cid","mts":"0.001","nr":true,"fd":{"r":"0.02","e":1},"t":[{"t":"999"}]}`,
				),
			)
		case tickSizeEndpoint:
			if r.URL.Query().Get("token_id") != "999" {
				t.Error("position ID not used for tick request")
			}
			_, _ = w.Write([]byte(`{"minimum_tick_size":"0.001"}`))
		case orderBookEndpoint:
			if r.URL.Query().Get("token_id") != "999" {
				t.Error("position ID not used for book request")
			}
			_, _ = w.Write(
				[]byte(
					`{"asset_id":"999","bids":[{"price":"0.543","size":"100"}],"asks":[{"price":"0.543","size":"100"}]}`,
				),
			)
		default:
			t.Errorf("position order made unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := NewSignerClient(
		Config{
			Host:       server.URL,
			PrivateKey: "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	limit, err := client.CreateOrder(
		t.Context(),
		OrderArgs{PositionID: "999", Price: MustDec("0.543"), Size: MustDec("100"), Side: SideBuy},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if limit.Order.MakerAmount != "54300000" || limit.Order.TakerAmount != "100000000" {
		t.Fatalf("position limit rounded against wrong grid: %+v", limit.Order)
	}
	budget := MustDec("10")
	capped, err := client.CreateOrder(
		t.Context(),
		OrderArgs{
			PositionID: "999",
			Price:      MustDec("0.543"),
			Size:       MustDec("100"),
			Side:       SideBuy,
			MaxSpend:   &budget,
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	cappedMaker, _ := new(big.Rat).SetString(capped.Order.MakerAmount)
	if new(big.Rat).Mul(cappedMaker, big.NewRat(100914, 100000)).Cmp(big.NewRat(10000000, 1)) > 0 {
		t.Fatal("fee-adjusted limit order exceeded budget")
	}
	market, err := client.CreateMarketOrder(
		t.Context(),
		MarketOrderArgs{PositionID: "999", Amount: budget, MaxSpend: &budget, Side: SideBuy},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	maker, _ := new(big.Rat).SetString(market.Order.MakerAmount)
	cost := new(big.Rat).Mul(maker, big.NewRat(100914, 100000)) // 1 + 0.02 * (1 - 0.543)
	if cost.Cmp(big.NewRat(10000000, 1)) > 0 {
		t.Fatal("native-position fee-adjusted BUY exceeded budget")
	}
	contracts, _ := getContractConfig(PolygonChainID)
	typed := buildOrderTypedData(PolygonChainID, "3", contracts.ExchangeV3, *market)
	digest, _, err := apitypes.TypedDataAndHash(typed)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := json.Marshal(market)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip SignedOrder
	if err := json.Unmarshal(sig, &roundTrip); err != nil {
		t.Fatal(err)
	}
	raw := strings.TrimPrefix(roundTrip.Signature, "0x")
	signature, err := hex.DecodeString(raw)
	if err != nil {
		t.Fatal(err)
	}
	signature[64] -= 27
	pub, err := crypto.SigToPub(digest, signature)
	if err != nil {
		t.Fatal(err)
	}
	if crypto.PubkeyToAddress(*pub) != client.signer.Address() {
		t.Fatal("native order did not sign Exchange V3 domain")
	}
	if markets.Load() != 1 {
		t.Fatalf("market metadata loads = %d", markets.Load())
	}
}

func TestPositionPriceGuardRefreshesMarketWithoutRelaxation(t *testing.T) {
	t.Parallel()
	client := protectedMarketTestClient(t, TickSizeThousandth)
	client.SetTickSize("100", TickSizeHundredth)
	bound := MustDec("0.543")
	order, err := client.CreateMarketOrder(
		t.Context(),
		MarketOrderArgs{PositionID: "100", Amount: MustDec("100"), Side: SideBuy, MaxPrice: &bound},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	maker, _ := new(big.Rat).SetString(order.Order.MakerAmount)
	taker, _ := new(big.Rat).SetString(order.Order.TakerAmount)
	ratio := new(big.Rat).Quo(maker, taker)
	if ratio.Cmp(big.NewRat(543, 1000)) < 0 || ratio.Cmp(big.NewRat(5431, 10000)) >= 0 {
		t.Fatalf("refreshed protection violated: %s", ratio)
	}
}

func TestExchangeV3ExactAmountsSupportUint256AndDepositSigner(t *testing.T) {
	t.Parallel()
	client, err := NewSignerClient(
		Config{
			PrivateKey:    "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
			SignatureType: SignatureTypePoly1271,
			FunderAddress: "0x1111111111111111111111111111111111111111",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	large := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	order, err := client.CreateExchangeV3OrderFromAmounts(
		t.Context(),
		OrderAmountsArgs{
			TokenID:     "123",
			MakerAmount: large,
			TakerAmount: big.NewInt(1),
			Side:        SideBuy,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if order.Order.MakerAmount != large.String() || order.Order.Signer != client.funderAddress {
		t.Fatal("amount precision or deposit signer lost")
	}
	if _, err := client.CreateExchangeV3OrderFromAmounts(t.Context(), OrderAmountsArgs{TokenID: "123", MakerAmount: new(big.Int).Lsh(big.NewInt(1), 256), TakerAmount: big.NewInt(1), Side: SideBuy}); err == nil {
		t.Fatal("accepted uint256 overflow")
	}
}
