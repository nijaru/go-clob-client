package clob

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/quagmt/udecimal"
)

// newComboGatewayMock serves the builder-gateway endpoints with scripted
// responses per path. It records the request bodies for assertions.
type comboGatewayMock struct {
	server        *httptest.Server
	requestBodies atomic.Value // map[string]string keyed by path suffix
}

func newComboGatewayMock(t *testing.T, respond func(path string) (int, string)) *comboGatewayMock {
	t.Helper()
	mock := &comboGatewayMock{}
	bodies := map[string]string{}
	mock.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		bodies[r.URL.Path] = string(body)
		status, payload := respond(r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(payload))
	}))
	mock.requestBodies.Store(bodies)
	t.Cleanup(mock.server.Close)
	return mock
}

func (m *comboGatewayMock) body(path string) string {
	return m.requestBodies.Load().(map[string]string)[path]
}

func newComboTestClient(t *testing.T, gatewayURL string) *AuthenticatedClient {
	t.Helper()
	client, err := NewAuthenticatedClient(Config{
		Host:               "https://clob.example.com",
		ChainID:            PolygonChainID,
		PrivateKey:         "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
		BuilderGatewayHost: gatewayURL,
		BuilderAuth:        staticBuilderAuth{},
		Credentials: &Credentials{
			Key:        "api-key",
			Secret:     "c2VjcmV0",
			Passphrase: "pass",
		},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	client.saltGenerator = func() (uint64, error) { return 1, nil }
	return client
}

const comboQuoteReadyBody = `{
	"rfq_id": "rfq-1",
	"status": "AWAITING_REQUESTER_ACCEPTANCE",
	"expires_at": 1755000000000,
	"builder_code": "0x0100000000000000000000000000000000000000000000000000000000000000",
	"request": {
		"condition_id": "0xc000000000000000000000000000000000000000000000000000000000000000",
		"yes_position_id": "999",
		"no_position_id": "998"
	},
	"quote": {
		"quote_id": "quote-1",
		"blended_price_e6": "500000",
		"maker_amount_e6": "100000000",
		"taker_amount_e6": "200000000",
		"total_required_e6": "100500000"
	}
}`

const comboNoQuotesBody = `{"rfq_id":"rfq-1","status":"EXPIRED","error":{"code":"NO_QUOTES","message":"no quotes"}}`

func TestRequestComboQuoteReturnsQuote(t *testing.T) {
	t.Parallel()

	mock := newComboGatewayMock(t, func(path string) (int, string) {
		if path == builderRFQRequestsEndpoint {
			return http.StatusOK, comboQuoteReadyBody
		}
		return http.StatusNotFound, `{}`
	})
	client := newComboTestClient(t, mock.server.URL)

	result, err := client.RequestComboQuote(t.Context(), RequestComboQuoteParams{
		LegPositionIDs: []string{"111", "222"},
		Direction:      RFQDirectionBuy,
		Amount:         mustDecimal(t, "100"),
	})
	if err != nil {
		t.Fatalf("RequestComboQuote: %v", err)
	}

	if result.Quote == nil {
		t.Fatalf("expected quote, got reason %s", result.Reason)
	}
	if result.RFQID != "rfq-1" || result.YesPositionID != "999" || result.NoPositionID != "998" {
		t.Fatalf("unexpected identifiers: %+v", result)
	}
	if result.ConditionID == "" || result.BuilderCode == "" {
		t.Fatalf("missing combo metadata: %+v", result)
	}
	// e6 values decode to human decimals with trailing zeros trimmed.
	if result.Quote.BlendedPrice != "0.5" {
		t.Fatalf("blended price = %s, want 0.5", result.Quote.BlendedPrice)
	}
	if result.Quote.MakerAmount != "100" || result.Quote.TakerAmount != "200" {
		t.Fatalf("amounts = %s/%s, want 100/200", result.Quote.MakerAmount, result.Quote.TakerAmount)
	}
	if result.Quote.TotalRequired != "100.5" {
		t.Fatalf("total required = %s, want 100.5", result.Quote.TotalRequired)
	}
	if result.Quote.ExpiresAt != 1755000000000 {
		t.Fatalf("expires at = %d", result.Quote.ExpiresAt)
	}

	// The request body must carry the gateway contract fields.
	body := mock.body(builderRFQRequestsEndpoint)
	for _, want := range []string{
		`"signer_address"`, `"maker_address"`, `"signature_type"`,
		`"leg_position_ids":["111","222"]`, `"direction":"BUY"`, `"side":"YES"`,
		`"unit":"notional"`, `"value_e6":"100000000"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("request body missing %s: %s", want, body)
		}
	}
}

func TestRequestComboQuoteNoQuotes(t *testing.T) {
	t.Parallel()

	mock := newComboGatewayMock(t, func(path string) (int, string) {
		return http.StatusOK, comboNoQuotesBody
	})
	client := newComboTestClient(t, mock.server.URL)

	result, err := client.RequestComboQuote(t.Context(), RequestComboQuoteParams{
		LegPositionIDs: []string{"111", "222"},
		Direction:      RFQDirectionBuy,
		Amount:         mustDecimal(t, "100"),
	})
	if err != nil {
		t.Fatalf("RequestComboQuote: %v", err)
	}
	if result.Quote != nil || result.Reason != ComboQuoteNoQuotes {
		t.Fatalf("expected no-quotes outcome, got %+v", result)
	}
}

func TestRequestComboQuoteUnknownRejectionIsError(t *testing.T) {
	t.Parallel()

	mock := newComboGatewayMock(t, func(path string) (int, string) {
		return http.StatusOK,
			`{"rfq_id":"rfq-1","status":"FAILED","error":{"code":"SOMETHING_NEW","message":"boom"}}`
	})
	client := newComboTestClient(t, mock.server.URL)

	_, err := client.RequestComboQuote(t.Context(), RequestComboQuoteParams{
		LegPositionIDs: []string{"111", "222"},
		Direction:      RFQDirectionBuy,
		Amount:         mustDecimal(t, "100"),
	})
	var rejection *ComboRFQRejectionError
	if err == nil || !errors.As(err, &rejection) || rejection.Code != "SOMETHING_NEW" {
		t.Fatalf("expected ComboRFQRejectionError, got %v", err)
	}
}

func TestRequestComboQuoteValidation(t *testing.T) {
	t.Parallel()

	mock := newComboGatewayMock(t, func(path string) (int, string) {
		return http.StatusOK, comboQuoteReadyBody
	})
	client := newComboTestClient(t, mock.server.URL)
	ctx := t.Context()

	if _, err := client.RequestComboQuote(ctx, RequestComboQuoteParams{
		LegPositionIDs: []string{"111"},
		Direction:      RFQDirectionBuy,
		Amount:         mustDecimal(t, "100"),
	}); err == nil {
		t.Fatal("expected error for a single leg")
	}
	if _, err := client.RequestComboQuote(ctx, RequestComboQuoteParams{
		LegPositionIDs: []string{"111", "111"},
		Direction:      RFQDirectionBuy,
		Amount:         mustDecimal(t, "100"),
	}); err == nil {
		t.Fatal("expected error for duplicate legs")
	}
	if _, err := client.RequestComboQuote(ctx, RequestComboQuoteParams{
		LegPositionIDs: []string{"111", "222"},
		Direction:      RFQDirectionBuy,
		Amount:         mustDecimal(t, "0"),
	}); err == nil {
		t.Fatal("expected error for non-positive amount")
	}
	if _, err := client.RequestComboQuote(ctx, RequestComboQuoteParams{
		LegPositionIDs: []string{"111", "222"},
		Direction:      "SIDEWAYS",
		Amount:         mustDecimal(t, "100"),
	}); err == nil {
		t.Fatal("expected error for invalid direction")
	}

	// Sell requests convert the share size into e6 units.
	sellBodySent := false
	sellMock := newComboGatewayMock(t, func(path string) (int, string) {
		sellBodySent = true
		return http.StatusOK, comboNoQuotesBody
	})
	sellClient := newComboTestClient(t, sellMock.server.URL)
	if _, err := sellClient.RequestComboQuote(ctx, RequestComboQuoteParams{
		LegPositionIDs: []string{"111", "222"},
		Direction:      RFQDirectionSell,
		Size:           mustDecimal(t, "12.5"),
	}); err != nil {
		t.Fatalf("sell request: %v", err)
	}
	if !sellBodySent {
		t.Fatal("sell request did not reach the gateway")
	}
}

func TestAcceptComboQuoteExecuting(t *testing.T) {
	t.Parallel()

	mock := newComboGatewayMock(t, func(path string) (int, string) {
		if strings.HasSuffix(path, "/accept") {
			return http.StatusOK, `{"rfq_id":"rfq-1","status":"EXECUTING","taker_order_hash":"0xabc"}`
		}
		return http.StatusOK, `{"rfq_id":"rfq-1","status":"EXECUTING"}`
	})
	client := newComboTestClient(t, mock.server.URL)

	result, err := client.AcceptComboQuote(t.Context(), AcceptComboQuoteParams{
		RFQID:       "rfq-1",
		Direction:   SideBuy,
		PositionID:  "999",
		BuilderCode: "0x0100000000000000000000000000000000000000000000000000000000000000",
		Quote: ComboQuoteReference{
			QuoteID:     "quote-1",
			MakerAmount: "100",
			TakerAmount: "200",
		},
	})
	if err != nil {
		t.Fatalf("AcceptComboQuote: %v", err)
	}
	if !result.Status.Executing() || result.TakerOrderHash != "0xabc" {
		t.Fatalf("unexpected acceptance: %+v", result)
	}

	// The signed order must carry the quote amounts in e6 base units, the
	// combo position as tokenId, the builder code, and a signature.
	body := mock.body(builderRFQRequestsEndpoint + "/rfq-1/accept")
	for _, want := range []string{
		`"quote_id":"quote-1"`, `"signed_order"`, `"tokenId":"999"`,
		`"makerAmount":"100000000"`, `"takerAmount":"200000000"`,
		`"side":"BUY"`, `"expiration":"0"`, `"signature":"0x`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("accept body missing %s: %s", want, body)
		}
	}
}

func TestAcceptComboQuoteMakerDeclined(t *testing.T) {
	t.Parallel()

	mock := newComboGatewayMock(t, func(path string) (int, string) {
		return http.StatusOK, `{"rfq_id":"rfq-1","status":"CANCELED"}`
	})
	client := newComboTestClient(t, mock.server.URL)

	result, err := client.AcceptComboQuote(t.Context(), AcceptComboQuoteParams{
		RFQID:       "rfq-1",
		Direction:   SideBuy,
		PositionID:  "999",
		BuilderCode: "0x0100000000000000000000000000000000000000000000000000000000000000",
		Quote: ComboQuoteReference{
			QuoteID:     "quote-1",
			MakerAmount: "100",
			TakerAmount: "200",
		},
	})
	if err != nil {
		t.Fatalf("AcceptComboQuote: %v", err)
	}
	if result.Status != ComboRFQCanceled || result.Reason != ComboAcceptMakerDeclined {
		t.Fatalf("unexpected declined outcome: %+v", result)
	}
}

func TestAcceptComboQuoteExpiredWindow(t *testing.T) {
	t.Parallel()

	mock := newComboGatewayMock(t, func(path string) (int, string) {
		return http.StatusConflict,
			`{"error":"EXPIRED_RFQ","message":"the acceptance window closed","code":"EXPIRED_RFQ"}`
	})
	client := newComboTestClient(t, mock.server.URL)

	result, err := client.AcceptComboQuote(t.Context(), AcceptComboQuoteParams{
		RFQID:       "rfq-1",
		Direction:   SideBuy,
		PositionID:  "999",
		BuilderCode: "0x0100000000000000000000000000000000000000000000000000000000000000",
		Quote: ComboQuoteReference{
			QuoteID:     "quote-1",
			MakerAmount: "100",
			TakerAmount: "200",
		},
	})
	if err != nil {
		t.Fatalf("AcceptComboQuote: %v", err)
	}
	if result.Status != ComboRFQExpired || result.Reason != ComboAcceptWindowExpired {
		t.Fatalf("unexpected expired outcome: %+v", result)
	}
}

func TestAcceptComboQuoteValidation(t *testing.T) {
	t.Parallel()

	mock := newComboGatewayMock(t, func(path string) (int, string) {
		return http.StatusOK, `{"rfq_id":"rfq-1","status":"EXECUTING"}`
	})
	client := newComboTestClient(t, mock.server.URL)
	valid := AcceptComboQuoteParams{
		RFQID:       "rfq-1",
		Direction:   SideBuy,
		PositionID:  "999",
		BuilderCode: "0x0100000000000000000000000000000000000000000000000000000000000000",
		Quote: ComboQuoteReference{
			QuoteID:     "quote-1",
			MakerAmount: "100",
			TakerAmount: "200",
		},
	}

	if _, err := client.AcceptComboQuote(t.Context(), valid); err != nil {
		t.Fatalf("valid accept: %v", err)
	}

	bothIDs := valid
	bothIDs.PositionID = "abc"
	if _, err := client.AcceptComboQuote(t.Context(), bothIDs); err == nil {
		t.Fatal("expected error for non-numeric positionId")
	}

	badBuilder := valid
	badBuilder.BuilderCode = "0x1234"
	if _, err := client.AcceptComboQuote(t.Context(), badBuilder); err == nil {
		t.Fatal("expected error for short builderCode")
	}

	badAmount := valid
	badAmount.Quote.MakerAmount = "0"
	if _, err := client.AcceptComboQuote(t.Context(), badAmount); err == nil {
		t.Fatal("expected error for non-positive maker amount")
	}
}

func TestGetComboRFQStatus(t *testing.T) {
	t.Parallel()

	mock := newComboGatewayMock(t, func(path string) (int, string) {
		if strings.HasSuffix(path, "/rfq-1") {
			return http.StatusOK,
				`{"rfq_id":"rfq-1","status":"MINED","taker_order_hash":"0xabc","tx_hash":"0xtx"}`
		}
		return http.StatusNotFound, `{}`
	})
	client := newComboTestClient(t, mock.server.URL)

	status, err := client.GetComboRFQStatus(t.Context(), "rfq-1")
	if err != nil {
		t.Fatalf("GetComboRFQStatus: %v", err)
	}
	if status.Status != ComboRFQMined || status.TxHash != "0xtx" {
		t.Fatalf("unexpected status: %+v", status)
	}
	if _, err := client.GetComboRFQStatus(t.Context(), ""); err == nil {
		t.Fatal("expected error for empty rfqId")
	}
}

func TestE6DecimalRoundTrip(t *testing.T) {
	t.Parallel()

	cases := []struct {
		e6   string
		want string
	}{
		{"1000000", "1"},
		{"500000", "0.5"},
		{"100500000", "100.5"},
		{"123456", "0.123456"},
		{"120000", "0.12"},
		{"0", "0"},
	}
	for _, tc := range cases {
		got, err := e6ToDecimal(tc.e6)
		if err != nil {
			t.Fatalf("e6ToDecimal(%s): %v", tc.e6, err)
		}
		if got != tc.want {
			t.Fatalf("e6ToDecimal(%s) = %s, want %s", tc.e6, got, tc.want)
		}
	}

	if got := decimalToE6(mustDecimal(t, "100.5")); got != "100500000" {
		t.Fatalf("decimalToE6(100.5) = %s", got)
	}
	if _, err := decimalToE6String("-1"); err == nil {
		t.Fatal("expected error for negative amount")
	}
	if _, err := e6ToDecimal(""); err == nil {
		t.Fatal("expected error for empty e6 value")
	}
}

// staticBuilderAuth satisfies BuilderAuth with fixed headers for tests.
type staticBuilderAuth struct{}

func (staticBuilderAuth) Headers(
	_ context.Context,
	_ BuilderHeaderRequest,
) (map[string]string, error) {
	return map[string]string{"Poly-Builder-Key": "test"}, nil
}

func mustDecimal(t *testing.T, value string) udecimal.Decimal {
	t.Helper()
	parsed, err := udecimal.Parse(value)
	if err != nil {
		t.Fatalf("parse decimal %q: %v", value, err)
	}
	return parsed
}
