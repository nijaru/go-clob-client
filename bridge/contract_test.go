package bridge

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestQuoteWirePrecisionAndWideChainIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			FromAmount string `json:"fromAmountBaseUnit"`
			FromChain  string `json:"fromChainId"`
			ToChain    string `json:"toChainId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.FromAmount != "115792089237316195423570985008687907853269984665640564039457584007913129639935" ||
			body.FromChain != "18446744073709551615" ||
			body.ToChain != "1151111081099710" {
			t.Errorf("wrong numeric wire strings: %+v", body)
		}
		fmt.Fprint(
			w,
			`{"quoteId":"q","estInputUsd":12345678901234567890.123456789012345678901,"estOutputUsd":"1.000000000000000000001","estToTokenBaseUnit":"9007199254740993","estFeeBreakdown":{"gasUsd":0.000000000000000000000001}}`,
		)
	}))
	defer server.Close()
	client := bridgeTestClient(t, server.URL)
	quote, err := client.GetQuote(
		t.Context(),
		QuoteRequest{
			FromAmountBaseUnit: "115792089237316195423570985008687907853269984665640564039457584007913129639935",
			FromChainID:        ChainID(math.MaxUint64),
			ToChainID:          1151111081099710,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if quote.EstInputUSD != "12345678901234567890.123456789012345678901" ||
		quote.EstOutputUSD != "1.000000000000000000001" ||
		quote.EstFeeBreakdown.GasUSD != "0.000000000000000000000001" ||
		quote.EstToTokenBaseUnit != "9007199254740993" {
		t.Fatalf("quote rounded: %+v", quote)
	}
	var chain ChainID
	if err := json.Unmarshal([]byte(`"18446744073709551615"`), &chain); err != nil ||
		chain != ChainID(math.MaxUint64) {
		t.Fatalf("wide chain not decoded: %v %v", chain, err)
	}
}

func TestBaseUnitValidationPrecedesRequest(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) { calls.Add(1); fmt.Fprint(w, `{}`) },
		),
	)
	defer server.Close()
	client := bridgeTestClient(t, server.URL)
	for _, amount := range []string{"", "-1", "1.5", "01", strings.Repeat("9", 78)} {
		quote, err := client.GetQuote(
			t.Context(),
			QuoteRequest{FromAmountBaseUnit: BaseUnits(amount)},
		)
		if quote != nil || err == nil {
			t.Fatalf("accepted invalid amount %q", amount)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid amount reached service")
	}
	if _, err := ParseBaseUnits("0"); err != nil {
		t.Fatal("zero is a valid uint256", err)
	}
}

func TestStatusAddressCannotChangeRequestScope(t *testing.T) {
	address := "address/with?query#fragment"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/status/"+url.PathEscape(address) || r.URL.RawQuery != "" {
			t.Errorf("address changed request scope: %s", r.URL)
		}
		fmt.Fprint(w, `{"transactions":[]}`)
	}))
	defer server.Close()
	client := bridgeTestClient(t, server.URL+"/")
	if _, err := client.GetStatus(t.Context(), address); err != nil {
		t.Fatal(err)
	}
	for _, address := range []string{"", ".", ".."} {
		if _, err := client.GetStatus(t.Context(), address); err == nil {
			t.Fatalf("accepted unsafe address %q", address)
		}
	}
}

func TestAPIErrorDoesNotReturnApparentResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		fmt.Fprint(w, `{"error":"unsupported route"}`)
	}))
	defer server.Close()
	client := bridgeTestClient(t, server.URL)
	quote, err := client.GetQuote(t.Context(), QuoteRequest{FromAmountBaseUnit: "1"})
	apiErr, ok := errors.AsType[*APIError](err)
	if quote != nil || !ok || apiErr.StatusCode != 422 || apiErr.Message != "unsupported route" {
		t.Fatalf("API failure lost: %+v %v", quote, err)
	}
}
