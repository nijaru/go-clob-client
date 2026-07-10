package clob

import (
	"net/http"
	"net/http/httptest"
	"testing"

	json "github.com/go-json-experiment/json"
)

func TestGetFeeExponent(t *testing.T) {
	t.Parallel()

	t.Run("returns exponent from fee details", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.Path; got != clobMarketEndpoint+"/123" {
				t.Fatalf("unexpected path: %s", got)
			}
			data, _ := json.Marshal(ClobMarketInfoResponse{
				ConditionID: "cid",
				FeeDetails:  &FeeDetails{Rate: 0.01, Exponent: 3},
			})
			w.Write(data)
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

	t.Run("defaults to 0 on legacy markets without fee details", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			data, _ := json.Marshal(ClobMarketInfoResponse{ConditionID: "cid", FeeDetails: nil})
			w.Write(data)
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
	})
}
