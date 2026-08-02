package clob

import (
	"net/http"
	"net/http/httptest"
	"testing"

	json "github.com/go-json-experiment/json"
)

func TestClobMarketInfoDecodesRustWire(t *testing.T) {
	t.Parallel()

	var response ClobMarketInfoResponse
	if err := json.Unmarshal([]byte(`{
		"c":"0xcondition",
		"t":[{"t":123,"o":"YES"},null,{"t":"456","o":"NO"}],
		"mts":0.001,
		"mos":5,
		"nr":true,
		"fd":{"r":0.03,"e":1}
	}`), &response); err != nil {
		t.Fatalf("decode compact market: %v", err)
	}
	if response.MinTickSize != "0.001" || response.MinOrderSize != "5" {
		t.Fatalf("sizes = %q/%q", response.MinTickSize, response.MinOrderSize)
	}
	if len(response.Tokens) != 3 || response.Tokens[0].TokenID != "123" ||
		response.Tokens[0].Outcome != "YES" || response.Tokens[1].TokenID != "" ||
		response.Tokens[2].TokenID != "456" {
		t.Fatalf("tokens = %#v", response.Tokens)
	}
}

func TestGetFeeExponent(t *testing.T) {
	t.Parallel()

	t.Run("returns exponent from fee details", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case marketsByTokenEndpoint + "123":
				_, _ = w.Write([]byte(`{"condition_id":"cid","primary_token_id":"123","secondary_token_id":"456"}`))
			case clobMarketEndpoint + "/cid":
				data, _ := json.Marshal(ClobMarketInfoResponse{
					ConditionID: "cid",
					FeeDetails:  &FeeDetails{Rate: 0.01, Exponent: 3},
				})
				_, _ = w.Write(data)
			default:
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
		}))
		defer server.Close()

		client, err := NewClient(Config{Host: server.URL})
		if err != nil {
			t.Fatalf("new client: %v", err)
		}
		got, err := client.GetFeeExponent(t.Context(), "123")
		if err != nil {
			t.Fatalf("GetFeeExponent: %v", err)
		}
		if got != 3 {
			t.Fatalf("expected exponent 3, got %d", got)
		}
	})

	t.Run("returns zero fee info on legacy markets without fee details", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case marketsByTokenEndpoint + "123":
				_, _ = w.Write([]byte(`{"condition_id":"cid","primary_token_id":"123","secondary_token_id":"456"}`))
			case clobMarketEndpoint + "/cid":
				data, _ := json.Marshal(ClobMarketInfoResponse{ConditionID: "cid", FeeDetails: nil})
				_, _ = w.Write(data)
			default:
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
		}))
		defer server.Close()

		client, err := NewClient(Config{Host: server.URL})
		if err != nil {
			t.Fatalf("new client: %v", err)
		}
		got, err := client.GetFeeExponent(t.Context(), "123")
		if err != nil {
			t.Fatalf("GetFeeExponent: %v", err)
		}
		if got != 0 {
			t.Fatalf("expected exponent 0 on legacy market, got %d", got)
		}
		feeInfo, err := client.GetFeeInfo(t.Context(), "123")
		if err != nil {
			t.Fatalf("GetFeeInfo: %v", err)
		}
		if feeInfo.Rate != 0 || feeInfo.Exponent != 0 {
			t.Fatalf("expected zero fee info on legacy market, got %+v", feeInfo)
		}
	})
}
