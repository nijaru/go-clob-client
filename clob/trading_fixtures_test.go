package clob

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/quagmt/udecimal"
)

func TestNormalizeFunderAddressMatchesReferenceWalletDerivation(t *testing.T) {
	t.Parallel()

	const signer = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"

	proxy, err := normalizeFunderAddress(PolygonChainID, signer, SignatureTypePolyProxy, "")
	if err != nil {
		t.Fatalf("normalize proxy funder: %v", err)
	}
	if common.HexToAddress(
		proxy,
	) != common.HexToAddress(
		"0x365f0cA36ae1F641E02Fe3b7743673DA42A13a70",
	) {
		t.Fatalf("unexpected proxy address: %s", proxy)
	}

	safe, err := normalizeFunderAddress(PolygonChainID, signer, SignatureTypePolyGnosisSafe, "")
	if err != nil {
		t.Fatalf("normalize safe funder: %v", err)
	}
	if common.HexToAddress(
		safe,
	) != common.HexToAddress(
		"0xd93b25Cb943D14d0d34FBAf01fc93a0F8b5f6e47",
	) {
		t.Fatalf("unexpected safe address: %s", safe)
	}

	amoySafe, err := normalizeFunderAddress(80002, signer, SignatureTypePolyGnosisSafe, "")
	if err != nil {
		t.Fatalf("normalize amoy safe funder: %v", err)
	}
	if common.HexToAddress(
		amoySafe,
	) != common.HexToAddress(
		"0xd93b25Cb943D14d0d34FBAf01fc93a0F8b5f6e47",
	) {
		t.Fatalf("unexpected amoy safe address: %s", amoySafe)
	}

	if _, err := normalizeFunderAddress(80002, signer, SignatureTypePolyProxy, ""); err == nil {
		t.Fatal("expected proxy derivation on amoy to fail")
	}
	if _, err := normalizeFunderAddress(PolygonChainID, signer, SignatureTypeEOA, signer); err == nil {
		t.Fatal("expected EOA funder validation to fail")
	}
}

func newTradingFixtureServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case tickSizeEndpoint:
			_, _ = w.Write([]byte(`{"minimum_tick_size":"0.01"}`))
		case feeRateEndpoint:
			_, _ = w.Write([]byte(`{"base_fee":0}`))
		case negRiskEndpoint:
			_, _ = w.Write([]byte(`{"neg_risk":false}`))
		case orderBookEndpoint:
			_, _ = w.Write(
				[]byte(
					`{"market":"m","asset_id":"123","timestamp":"1","bids":[{"price":"0.44","size":"10"}],"asks":[{"price":"0.46","size":"10"}],"min_order_size":"1","tick_size":"0.01","neg_risk":false,"last_trade_price":"0.45","hash":"h"}`,
				),
			)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
}

func TestNewDerivesFunderForProxySignatureTypes(t *testing.T) {
	t.Parallel()

	client, err := New(Config{
		Host:          "https://clob.polymarket.com",
		ChainID:       PolygonChainID,
		PrivateKey:    "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
		SignatureType: SignatureTypePolyGnosisSafe,
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	if common.HexToAddress(
		client.funderAddress,
	) != common.HexToAddress(
		"0xd93b25Cb943D14d0d34FBAf01fc93a0F8b5f6e47",
	) {
		t.Fatalf("unexpected derived funder: %s", client.funderAddress)
	}
}

func TestDeterministicSignedOrderFixtures(t *testing.T) {
	t.Parallel()

	server := newTradingFixtureServer(t)
	defer server.Close()

	type fixture struct {
		name   string
		client Config
		build  func(*Client) (*SignedOrder, error)
		expect SignedOrder
	}

	fixtures := []fixture{
		{
			name: "limit-buy-eoa",
			client: Config{
				Host:       server.URL,
				PrivateKey: "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
			},
			build: func(client *Client) (*SignedOrder, error) {
				return client.CreateOrder(context.Background(), OrderArgs{
					TokenID: "123",
					Price:   udecimal.MustParse("0.5"),
					Size:    udecimal.MustParse("100"),
					Side:    SideBuy,
				}, &CreateOrderOptions{TickSize: TickSizeTenth, NegRisk: Bool(false)})
			},
			expect: SignedOrder{
				Salt:          "1",
				Maker:         "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
				Signer:        "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
				Taker:         zeroAddress,
				TokenID:       "123",
				MakerAmount:   "50000000",
				TakerAmount:   "100000000",
				Expiration:    "0",
				Nonce:         "0",
				FeeRateBps:    "0",
				Side:          SideBuy,
				SignatureType: SignatureTypeEOA,
				Signature:     "0x27aafd2229f338a19b15f5507ed35953a98cfc7da5a99594621d348ffa41f32f09d7cb64c6a8513a6ddec3bd007c03cf78e1e2d60d8111624f19b76e9fbfc6961b",
			},
		},
		{
			name: "limit-sell-eoa",
			client: Config{
				Host:       server.URL,
				PrivateKey: "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
			},
			build: func(client *Client) (*SignedOrder, error) {
				return client.CreateOrder(context.Background(), OrderArgs{
					TokenID: "123",
					Price:   udecimal.MustParse("0.5"),
					Size:    udecimal.MustParse("100"),
					Side:    SideSell,
				}, &CreateOrderOptions{TickSize: TickSizeTenth, NegRisk: Bool(false)})
			},
			expect: SignedOrder{
				Salt:          "1",
				Maker:         "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
				Signer:        "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
				Taker:         zeroAddress,
				TokenID:       "123",
				MakerAmount:   "100000000",
				TakerAmount:   "50000000",
				Expiration:    "0",
				Nonce:         "0",
				FeeRateBps:    "0",
				Side:          SideSell,
				SignatureType: SignatureTypeEOA,
				Signature:     "0xd1f188271157eefc7d0499334130355e04e2c90d65477b160aa1f7d9333d213e618bef27311956d7733a5dc5fd9cb3898c4b0b9536e2c1c05a72bc4396efeda91c",
			},
		},
		{
			name: "market-buy-eoa",
			client: Config{
				Host:       server.URL,
				PrivateKey: "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
			},
			build: func(client *Client) (*SignedOrder, error) {
				return client.CreateMarketOrder(context.Background(), MarketOrderArgs{
					TokenID:   "123",
					Price:     udecimal.MustParse("0.56"),
					Amount:    udecimal.MustParse("100"),
					Side:      SideBuy,
					OrderType: OrderTypeFOK,
				}, &CreateOrderOptions{TickSize: TickSizeHundredth, NegRisk: Bool(false)})
			},
			expect: SignedOrder{
				Salt:          "1",
				Maker:         "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
				Signer:        "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
				Taker:         zeroAddress,
				TokenID:       "123",
				MakerAmount:   "100000000",
				TakerAmount:   "178571400",
				Expiration:    "0",
				Nonce:         "0",
				FeeRateBps:    "0",
				Side:          SideBuy,
				SignatureType: SignatureTypeEOA,
				Signature:     "0xf4bee7f8bb53140b8ce26dfa9399005f04ccaa83dd96e8f7a9d21f5b0d7c5ed41b05e358b0eeb9ba2ac8389dea2917cb3c5c547f87c1e0855ee6db100296b5501b",
			},
		},
		{
			name: "market-sell-eoa",
			client: Config{
				Host:       server.URL,
				PrivateKey: "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
			},
			build: func(client *Client) (*SignedOrder, error) {
				return client.CreateMarketOrder(context.Background(), MarketOrderArgs{
					TokenID:   "123",
					Price:     udecimal.MustParse("0.56"),
					Amount:    udecimal.MustParse("100"),
					Side:      SideSell,
					OrderType: OrderTypeFOK,
				}, &CreateOrderOptions{TickSize: TickSizeHundredth, NegRisk: Bool(false)})
			},
			expect: SignedOrder{
				Salt:          "1",
				Maker:         "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
				Signer:        "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
				Taker:         zeroAddress,
				TokenID:       "123",
				MakerAmount:   "100000000",
				TakerAmount:   "56000000",
				Expiration:    "0",
				Nonce:         "0",
				FeeRateBps:    "0",
				Side:          SideSell,
				SignatureType: SignatureTypeEOA,
				Signature:     "0xd30829eaae6f1fcef3f3d8177a598e71c0b77e938aad04f61a43b6c1c3bfc23d3f32685c2df956840d84e47a76ec64c8cb17ce63d6513ccff9e9e324740738361b",
			},
		},
		{
			name: "limit-buy-neg-risk-proxy-funder",
			client: Config{
				Host:          server.URL,
				ChainID:       PolygonChainID,
				PrivateKey:    "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
				SignatureType: SignatureTypePolyProxy,
				FunderAddress: "0xaDEFf2158d668f64308C62ef227C5CcaCAAf976D",
			},
			build: func(client *Client) (*SignedOrder, error) {
				return client.CreateOrder(context.Background(), OrderArgs{
					TokenID: "123",
					Price:   udecimal.MustParse("0.512"),
					Size:    udecimal.MustParse("100"),
					Side:    SideBuy,
					Nonce:   2,
					Taker:   "0xf7fB45986800e2D259BAa25B56466bd02dA37a44",
				}, &CreateOrderOptions{TickSize: TickSizeThousandth, NegRisk: Bool(true)})
			},
			expect: SignedOrder{
				Salt:          "1",
				Maker:         "0xaDEFf2158d668f64308C62ef227C5CcaCAAf976D",
				Signer:        "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
				Taker:         "0xf7fB45986800e2D259BAa25B56466bd02dA37a44",
				TokenID:       "123",
				MakerAmount:   "51200000",
				TakerAmount:   "100000000",
				Expiration:    "0",
				Nonce:         "2",
				FeeRateBps:    "0",
				Side:          SideBuy,
				SignatureType: SignatureTypePolyProxy,
				Signature:     "0x2bcecd7034ad97abe202a4bf23c8b0ba1c73285561e22bbeac2a32f89d28d2886b4607aef711ff67c831881b5773e04fc5ec25a2ed24fb17098a89b16e09077e1c",
			},
		},
	}

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			client, err := New(fixture.client)
			if err != nil {
				t.Fatalf("new client: %v", err)
			}
			client.saltGenerator = func() (uint64, error) { return 1, nil }

			order, err := fixture.build(client)
			if err != nil {
				t.Fatalf("build order: %v", err)
			}

			if fixture.expect.Signature == "" {
				t.Fatalf("fill fixture signature for %s: %+v", fixture.name, *order)
			}

			if *order != fixture.expect {
				t.Fatalf(
					"unexpected order for %s:\nwant: %+v\ngot:  %+v",
					fixture.name,
					fixture.expect,
					*order,
				)
			}
		})
	}
}
