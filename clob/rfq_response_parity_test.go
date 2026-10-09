package clob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	experimentjson "github.com/go-json-experiment/json"
)

const (
	rfqMaxUint256   = "115792089237316195423570985008687907853269984665640564039457584007913129639935"
	rfqMaxE6Decimal = "115792089237316195423570985008687907853269984665640564039457584007913129.639935"
)

func TestRFQResponseNumericWire(t *testing.T) {
	// Raw wire is independent of the Go response serializer, including numbers
	// that would be rounded if passed through float64 or bounded decimals.
	body := `{"requestId":"r","quoteId":"q","token":` + rfqMaxUint256 + `,"complement":"native-position","sizeIn":123456789012345678901234567890.123456789,"sizeOut":"2.000000001","price":0.1234567890123456789,"expiry":1755000000}`
	for _, decode := range []func([]byte, any) error{json.Unmarshal, func(b []byte, v any) error { return experimentjson.Unmarshal(b, v) }} {
		var request RFQRequest
		var quote RFQQuote
		if err := decode([]byte(body), &request); err != nil {
			t.Fatal(err)
		}
		if err := decode([]byte(body), &quote); err != nil {
			t.Fatal(err)
		}
		if request.Token != rfqMaxUint256 || request.Complement != "native-position" ||
			string(request.SizeIn) != "123456789012345678901234567890.123456789" ||
			request.SizeOut != "2.000000001" ||
			request.Price != "0.1234567890123456789" ||
			request.Expiry != "1755000000" {
			t.Fatalf("request = %+v", request)
		}
		if quote.Token != request.Token || quote.SizeIn != request.SizeIn ||
			quote.SizeOut != request.SizeOut ||
			quote.Expiry != request.Expiry {
			t.Fatalf("quote = %+v", quote)
		}
	}
	for _, body := range []string{`{"token":true}`, `{"complement":1.5}`, `{"expiry":{}}`, `{"sizeIn":[]}`} {
		var request RFQRequest
		if json.Unmarshal([]byte(body), &request) == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}

func TestRFQResponsePaginationHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case rfqDataRequestsEndpoint:
			fmt.Fprint(
				w,
				`{"limit":1,"count":1,"next_cursor":"next","total_count":0,"data":[{"requestId":"r","sizeIn":1.25}]}`,
			)
		case rfqDataQuotesEndpoint:
			fmt.Fprint(
				w,
				`{"limit":1,"count":1,"next_cursor":null,"total_count":12,"data":[{"quoteId":"q","sizeOut":2.5}]}`,
			)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, err := NewAuthenticatedClient(
		Config{
			Host:        server.URL,
			ChainID:     PolygonChainID,
			PrivateKey:  gaslessTestKey,
			Credentials: &Credentials{Key: "k", Secret: "c2VjcmV0", Passphrase: "p"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	requests, err := client.GetRFQRequests(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	quotes, err := client.GetRFQQuotes(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if requests.TotalCount == nil || *requests.TotalCount != 0 ||
		requests.Data[0].SizeIn != "1.25" ||
		requests.NextCursor != "next" {
		t.Fatalf("requests = %+v", requests)
	}
	if quotes.TotalCount == nil || *quotes.TotalCount != 12 || quotes.Data[0].SizeOut != "2.5" {
		t.Fatalf("quotes = %+v", quotes)
	}
	var absent RFQQuotesResponse
	if err := experimentjson.Unmarshal([]byte(`{"data":[],"total_count":null}`), &absent); err != nil ||
		absent.TotalCount != nil {
		t.Fatalf("absent count: %+v %v", absent, err)
	}
}

func TestComboMarketResponseVariants(t *testing.T) {
	for _, arrays := range []string{
		`"outcomes":["Yes","No"],"position_ids":["native-yes","native-no"],"outcome_prices":[0.1234567890123456789,"0.9"]`,
		`"outcomes":"[\"Yes\",\"No\"]","position_ids":"[\"native-yes\",\"native-no\"]","outcome_prices":"[0.1234567890123456789,\"0.9\"]"`,
	} {
		var market ComboMarket
		body := `{"id":9007199254740993,"volume":12345678901234567890.000000001,"pending":true,` + arrays + `}`
		if err := experimentjson.Unmarshal([]byte(body), &market); err != nil {
			t.Fatal(err)
		}
		outcomes, err := market.ParsedOutcomes()
		if err != nil {
			t.Fatal(err)
		}
		if market.ID != "9007199254740993" || market.Volume != "12345678901234567890.000000001" ||
			!market.Pending ||
			outcomes.Yes.Price != "0.1234567890123456789" ||
			outcomes.No.PositionID != "native-no" {
			t.Fatalf("market = %+v / %+v", market, outcomes)
		}
	}
	for _, body := range []string{`{"outcomes":"{}"}`, `{"outcomes":[2]}`, `{"position_ids":[3]}`, `{"outcome_prices":[false]}`} {
		var market ComboMarket
		if json.Unmarshal([]byte(body), &market) == nil {
			t.Fatalf("accepted %s", body)
		}
	}
	for _, market := range []ComboMarket{{}, {Outcomes: []string{"Y", "N"}}, {Outcomes: []string{"Y", "N"}, PositionIDs: []string{"1", "2"}}} {
		if _, err := market.ParsedOutcomes(); err == nil {
			t.Fatal("accepted nonbinary/misaligned catalog")
		}
	}
}

func TestCollateralReturnExactHTTPResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(
			w,
			`{"plan_hash":"0xplan","chain_id":137,"wallet":"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","block_number":9007199254740993,"starting_pusd":"10.000000001","net_pusd_out":"1","final_pusd":"11.000000001","operations":[{"kind":"future","event_id":9007199254740995,"amount":"%s"},{"kind":"merge_on_event","event_id":"0xabc","amount":"1000000"}],"operation_count":2,"truncated":true,"estimated_cost":0.12345678901234567890123456789,"required_pusd_input":"0","required_positions":[{"position_id":"p","amount":"1"}],"candidate_position_ids":["p"],"router_call":{"to":"0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","data":"0x1234"}}`,
			rfqMaxUint256,
		)
	}))
	defer server.Close()
	client := newCollateralReturnClient(t, server.URL, SignatureTypePolyGnosisSafe)
	plan, err := client.PlanCollateralReturn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if plan.BlockNumber != "9007199254740993" ||
		plan.EstimatedCost != "0.12345678901234567890123456789" ||
		plan.Operations[0].EventID != "9007199254740995" ||
		plan.Operations[0].Kind != "future" ||
		plan.Operations[0].Amount != rfqMaxUint256 ||
		plan.Operations[1].EventID != "0xabc" ||
		plan.Operations[1].Amount != "1000000" ||
		plan.StartingPUSD != "10.000000001" ||
		!plan.Truncated ||
		len(plan.PositionSummary.Created) != 0 {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestComboQuoteResponseEchoAndSellProceeds(t *testing.T) {
	for _, tc := range []struct {
		name, old, replacement string
		sell, reject           bool
	}{
		{name: "buy"},
		{name: "sell", sell: true},
		{name: "sell missing proceeds", old: `"net_receive_e6":"` + rfqMaxUint256 + `",`, sell: true, reject: true},
		{name: "rfq", old: "\t\t\"rfq_id\": \"rfq-1\"", replacement: "\t\t\"rfq_id\": \"other\"", reject: true},
		{name: "direction", old: `"direction": "BUY"`, replacement: `"direction":"SELL"`, reject: true},
		{name: "side", old: `"side": "YES"`, replacement: `"side":"NO"`, reject: true},
		{name: "legs", old: `["111", "222"]`, replacement: `["222", "111"]`, reject: true},
		{name: "size unit", old: `"unit":"notional"`, replacement: `"unit":"shares"`, reject: true},
		{name: "size value", old: `"value_e6":"100000000"`, replacement: `"value_e6":"100000001"`, reject: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := strings.Replace(
				comboQuoteReadyBody,
				`"quote_id": "quote-1",`,
				`"quote_id": "quote-1","net_receive_e6":"`+rfqMaxUint256+`",`,
				1,
			)
			if tc.old != "" {
				old := strings.ReplaceAll(tc.old, `\n`, "\n")
				if !strings.Contains(body, old) {
					t.Fatalf("missing fixture target %q", old)
				}
				body = strings.Replace(body, old, tc.replacement, 1)
			}
			params := RequestComboQuoteParams{
				LegPositionIDs: []string{"222", "111"},
				Direction:      RFQDirectionBuy,
				Amount:         MustDec("100"),
			}
			if tc.sell {
				body = strings.ReplaceAll(body, `"direction": "BUY"`, `"direction": "SELL"`)
				body = strings.ReplaceAll(body, `"unit":"notional"`, `"unit":"shares"`)
				params.Direction, params.Size = RFQDirectionSell, MustDec("100")
			}
			mock := newComboGatewayMock(
				t,
				func(string) (int, string) { return http.StatusOK, body },
			)
			result, err := newComboTestClient(
				t,
				mock.server.URL,
			).RequestComboQuote(t.Context(), params)
			if tc.reject {
				if err == nil || result != nil {
					t.Fatalf("unsafe quote returned: %+v %v", result, err)
				}
				if !tc.sell {
					var mismatch *ComboRFQEchoMismatchError
					if !errors.As(err, &mismatch) {
						t.Fatalf("not inspectable mismatch: %v", err)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.Request == nil || result.Request.RFQID != result.RFQID ||
				result.Request.Direction != params.Direction ||
				result.Request.RequestedSize.ValueE6 != "100000000" ||
				result.Quote.NetReceive == nil ||
				string(*result.Quote.NetReceive) != rfqMaxE6Decimal {
				t.Fatalf("result = %+v / %+v", result, result.Quote)
			}
			if params.LegPositionIDs[0] != "222" {
				t.Fatal("mutated caller legs")
			}
		})
	}
	var quote builderRfqQuoteWire
	if err := json.Unmarshal([]byte(`{"quote_id":"q","blended_price_e6":"1","maker_amount_e6":"1","taker_amount_e6":"1","total_required_e6":"1","net_receive_e6":"0"}`), &quote); err != nil {
		t.Fatal(err)
	}
	result, err := comboQuoteFromWire(&quote, 0)
	if err != nil || result.NetReceive == nil || *result.NetReceive != "0" {
		t.Fatalf("zero proceeds: %+v %v", result, err)
	}
	quote.NetReceiveE6 = nil
	result, err = comboQuoteFromWire(&quote, 0)
	if err != nil || result.NetReceive != nil {
		t.Fatalf("absent proceeds: %+v %v", result, err)
	}
}

func TestComboQuoteKeepsSubmittedRequestSnapshot(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	mock := newComboGatewayMock(t, func(string) (int, string) {
		close(entered)
		<-release
		return http.StatusOK, comboQuoteReadyBody
	})
	client := newComboTestClient(t, mock.server.URL)
	params := RequestComboQuoteParams{
		LegPositionIDs: []string{"111", "222"},
		Direction:      RFQDirectionBuy,
		Amount:         MustDec("100"),
	}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := client.RequestComboQuote(ctx, params)
		done <- err
	}()
	select {
	case <-entered: // Serialization has finished reading caller input.
	case err := <-done:
		close(release)
		t.Fatalf("request ended before reaching the gateway: %v", err)
	}
	params.LegPositionIDs[0] = "333"
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestComboAcceptPreservesResponseDetails(t *testing.T) {
	for _, status := range []string{"FAILED", "EXECUTING"} {
		t.Run(status, func(t *testing.T) {
			mock := newComboGatewayMock(t, func(string) (int, string) {
				return http.StatusOK, fmt.Sprintf(
					`{"rfq_id":"r","status":%q,"taker_order_hash":"order-hash","tx_hash":"tx-hash","error":{"code":"FUTURE_ERROR","message":"detail"}}`,
					status,
				)
			})
			result, err := newComboTestClient(
				t,
				mock.server.URL,
			).AcceptComboQuote(t.Context(), AcceptComboQuoteParams{
				RFQID: "r", Direction: SideBuy, PositionID: "1", BuilderCode: zeroBytes32,
				Quote: ComboQuoteReference{QuoteID: "q", MakerAmount: "1", TakerAmount: "2"},
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.TakerOrderHash != "order-hash" || result.TxHash != "tx-hash" ||
				result.Error == nil ||
				result.Error.Code != "FUTURE_ERROR" {
				t.Fatalf("result = %+v", result)
			}
		})
	}
}

func TestComboRFQFullUint256WSResponse(t *testing.T) {
	url := newComboWSServer(t, func(ctx context.Context, conn *websocket.Conn) {
		_ = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"auth","success":true}`))
		body := `{"type":"RFQ_REQUEST","rfq_id":"r","requestor_public_id":"p","leg_position_ids":["1","2"],"condition_id":"` + comboNativeCondition + `","yes_position_id":"` + comboNativeYes + `","no_position_id":"` + comboNativeNo + `","direction":"SELL","side":"YES","requested_size":{"unit":"shares","value_e6":"` + rfqMaxUint256 + `"},"submission_deadline":1700000000000}`
		_ = conn.Write(ctx, websocket.MessageText, []byte(body))
		_, _, _ = conn.Read(ctx)
	})
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	session, err := newComboTestClient(
		t,
		"https://unused.invalid",
	).OpenComboRFQSession(ctx, ComboRFQSessionOptions{URL: url})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	select {
	case event := <-session.Events():
		request, ok := event.(ComboRFQQuoteRequest)
		if !ok || request.RequestedSize.Value != rfqMaxE6Decimal ||
			request.Direction != RFQDirectionSell {
			t.Fatalf("event = %+v", event)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
