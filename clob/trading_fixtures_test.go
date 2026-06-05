package clob

import (
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
			_, _ = w.Write([]byte(`{"minimum_tick_size":"0.001"}`))
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

	client, err := NewSignerClient(Config{
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
		name    string
		client  Config
		build   func(*SignerClient) (*SignedOrder, error)
		want    Order // check EIP-712 fields
		wantExp string
	}

	fixtures := []fixture{
		{
			name: "limit-buy-eoa",
			client: Config{
				Host:       server.URL,
				PrivateKey: "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
				Credentials: &Credentials{
					Key:        "api-key",
					Secret:     "c2VjcmV0",
					Passphrase: "pass",
				},
			},
			build: func(client *SignerClient) (*SignedOrder, error) {
				return client.CreateOrder(t.Context(), OrderArgs{
					TokenID: "123",
					Price:   udecimal.MustParse("0.5"),
					Size:    udecimal.MustParse("100"),
					Side:    SideBuy,
				}, &CreateOrderOptions{TickSize: TickSizeTenth, NegRisk: new(false)})
			},
			want: Order{
				Salt:          "1",
				Maker:         "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
				Signer:        "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
				TokenID:       "123",
				MakerAmount:   "50000000",
				TakerAmount:   "100000000",
				Side:          SideBuy,
				SignatureType: SignatureTypeEOA,
			},
			wantExp: "0",
		},
		{
			name: "limit-sell-eoa",
			client: Config{
				Host:       server.URL,
				PrivateKey: "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
				Credentials: &Credentials{
					Key:        "api-key",
					Secret:     "c2VjcmV0",
					Passphrase: "pass",
				},
			},
			build: func(client *SignerClient) (*SignedOrder, error) {
				return client.CreateOrder(t.Context(), OrderArgs{
					TokenID: "123",
					Price:   udecimal.MustParse("0.5"),
					Size:    udecimal.MustParse("100"),
					Side:    SideSell,
				}, &CreateOrderOptions{TickSize: TickSizeTenth, NegRisk: new(false)})
			},
			want: Order{
				Salt:          "1",
				Maker:         "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
				Signer:        "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
				TokenID:       "123",
				MakerAmount:   "100000000",
				TakerAmount:   "50000000",
				Side:          SideSell,
				SignatureType: SignatureTypeEOA,
			},
			wantExp: "0",
		},
		{
			name: "market-buy-eoa",
			client: Config{
				Host:       server.URL,
				PrivateKey: "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
				Credentials: &Credentials{
					Key:        "api-key",
					Secret:     "c2VjcmV0",
					Passphrase: "pass",
				},
			},
			build: func(client *SignerClient) (*SignedOrder, error) {
				return client.CreateMarketOrder(t.Context(), MarketOrderArgs{
					TokenID:   "123",
					Price:     udecimal.MustParse("0.56"),
					Amount:    udecimal.MustParse("100"),
					Side:      SideBuy,
					OrderType: OrderTypeFOK,
				}, &CreateOrderOptions{TickSize: TickSizeHundredth, NegRisk: new(false)})
			},
			want: Order{
				Salt:          "1",
				Maker:         "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
				Signer:        "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
				TokenID:       "123",
				MakerAmount:   "100000000",
				TakerAmount:   "178571400",
				Side:          SideBuy,
				SignatureType: SignatureTypeEOA,
			},
			wantExp: "0",
		},
		{
			name: "market-sell-eoa",
			client: Config{
				Host:       server.URL,
				PrivateKey: "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
				Credentials: &Credentials{
					Key:        "api-key",
					Secret:     "c2VjcmV0",
					Passphrase: "pass",
				},
			},
			build: func(client *SignerClient) (*SignedOrder, error) {
				return client.CreateMarketOrder(t.Context(), MarketOrderArgs{
					TokenID:    "123",
					Price:      udecimal.MustParse("0.56"),
					Amount:     udecimal.MustParse("100"),
					AmountKind: AmountShares,
					Side:       SideSell,
					OrderType:  OrderTypeFOK,
				}, &CreateOrderOptions{TickSize: TickSizeHundredth, NegRisk: new(false)})
			},
			want: Order{
				Salt:          "1",
				Maker:         "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
				Signer:        "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
				TokenID:       "123",
				MakerAmount:   "100000000",
				TakerAmount:   "56000000",
				Side:          SideSell,
				SignatureType: SignatureTypeEOA,
			},
			wantExp: "0",
		},
		{
			name: "limit-buy-neg-risk-proxy-funder",
			client: Config{
				Host:          server.URL,
				ChainID:       PolygonChainID,
				PrivateKey:    "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
				SignatureType: SignatureTypePolyProxy,
				FunderAddress: "0xaDEFf2158d668f64308C62ef227C5CcaCAAf976D",
				Credentials: &Credentials{
					Key:        "api-key",
					Secret:     "c2VjcmV0",
					Passphrase: "pass",
				},
			},
			build: func(client *SignerClient) (*SignedOrder, error) {
				return client.CreateOrder(t.Context(), OrderArgs{
					TokenID: "123",
					Price:   udecimal.MustParse("0.512"),
					Size:    udecimal.MustParse("100"),
					Side:    SideBuy,
				}, &CreateOrderOptions{TickSize: TickSizeThousandth, NegRisk: new(true)})
			},
			want: Order{
				Salt:          "1",
				Maker:         "0xaDEFf2158d668f64308C62ef227C5CcaCAAf976D",
				Signer:        "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
				TokenID:       "123",
				MakerAmount:   "51200000",
				TakerAmount:   "100000000",
				Side:          SideBuy,
				SignatureType: SignatureTypePolyProxy,
			},
			wantExp: "0",
		},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			client, err := NewAuthenticatedClient(fixture.client)
			if err != nil {
				t.Fatalf("new client: %v", err)
			}
			client.saltGenerator = func() (uint64, error) { return 1, nil }

			order, err := fixture.build(client.SignerClient)
			if err != nil {
				t.Fatalf("build order: %v", err)
			}

			if order.Signature == "" {
				t.Fatal("signature should not be empty")
			}

			// Check EIP-712 fields.
			if order.Order.Salt != fixture.want.Salt {
				t.Fatalf("salt = %s, want %s", order.Order.Salt, fixture.want.Salt)
			}
			if order.Order.Maker != fixture.want.Maker {
				t.Fatalf("maker = %s, want %s", order.Order.Maker, fixture.want.Maker)
			}
			if order.Order.Signer != fixture.want.Signer {
				t.Fatalf("signer = %s, want %s", order.Order.Signer, fixture.want.Signer)
			}
			if order.Order.TokenID != fixture.want.TokenID {
				t.Fatalf("tokenID = %s, want %s", order.Order.TokenID, fixture.want.TokenID)
			}
			if order.Order.MakerAmount != fixture.want.MakerAmount {
				t.Fatalf(
					"makerAmount = %s, want %s",
					order.Order.MakerAmount,
					fixture.want.MakerAmount,
				)
			}
			if order.Order.TakerAmount != fixture.want.TakerAmount {
				t.Fatalf(
					"takerAmount = %s, want %s",
					order.Order.TakerAmount,
					fixture.want.TakerAmount,
				)
			}
			if order.Order.Side != fixture.want.Side {
				t.Fatalf("side = %s, want %s", order.Order.Side, fixture.want.Side)
			}
			if order.Order.SignatureType != fixture.want.SignatureType {
				t.Fatalf(
					"signatureType = %d, want %d",
					order.Order.SignatureType,
					fixture.want.SignatureType,
				)
			}
			// V2 fields: timestamp is dynamic, metadata/builder default to zero
			if order.Order.Timestamp == "" {
				t.Fatal("timestamp should not be empty")
			}
			if order.Order.Metadata != zeroBytes32 {
				t.Fatalf("metadata = %s, want %s", order.Order.Metadata, zeroBytes32)
			}
			if order.Order.Builder != zeroBytes32 {
				t.Fatalf("builder = %s, want %s", order.Order.Builder, zeroBytes32)
			}

			if order.Expiration != fixture.wantExp {
				t.Fatalf("expiration = %s, want %s", order.Expiration, fixture.wantExp)
			}
		})
	}
}
