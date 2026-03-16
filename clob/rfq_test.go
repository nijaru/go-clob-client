package clob

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quagmt/udecimal"
)

func TestRFQSurfaces(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case rfqRequestEndpoint:
			if r.Method == http.MethodPost {
				_ = json.NewEncoder(w).Encode(RFQRequestResponse{
					RequestID: "rfq-1",
				})
				return
			}
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		case rfqDataRequestsEndpoint:
			_ = json.NewEncoder(w).Encode(RFQRequestsResponse{
				Data: []RFQRequest{
					{ID: "rfq-1", State: "active"},
				},
			})
		case rfqQuoteEndpoint:
			if r.Method == http.MethodPost {
				_ = json.NewEncoder(w).Encode(RFQQuoteResponse{
					QuoteID: "quote-1",
				})
				return
			}
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		case rfqRequestAcceptEndpoint:
			_ = json.NewEncoder(w).Encode(AcceptRFQQuoteResponse{
				TradeIDs: []string{"trade-1"},
			})
		case rfqQuoteApproveEndpoint:
			w.WriteHeader(http.StatusOK)
		case rfqBestQuoteEndpoint:
			_ = json.NewEncoder(w).Encode(RFQQuote{
				ID:        "quote-1",
				RequestID: "rfq-1",
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := New(Config{
		Host:       server.URL,
		PrivateKey: "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		Credentials: &Credentials{
			Key:        "api-key",
			Secret:     "c2VjcmV0",
			Passphrase: "pass",
		},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	ctx := t.Context()

	// Create Request
	req, err := client.CreateRFQRequest(ctx, CreateRFQRequestParams{
		AssetIn:   "asset-1",
		AssetOut:  "asset-2",
		AmountIn:  udecimal.MustParse("100"),
		AmountOut: udecimal.MustParse("50"),
	})
	if err != nil {
		t.Fatalf("create rfq: %v", err)
	}
	if req.RequestID != "rfq-1" {
		t.Errorf("unexpected rfq id: %s", req.RequestID)
	}

	// Get Requests
	list, err := client.GetRFQRequests(ctx, &RFQRequestFilterParams{State: "active"})
	if err != nil {
		t.Fatalf("get rfq requests: %v", err)
	}
	if len(list.Data) != 1 || list.Data[0].ID != "rfq-1" {
		t.Errorf("unexpected rfq list: %+v", list)
	}

	// Create Quote
	quote, err := client.CreateRFQQuote(ctx, CreateRFQQuoteParams{
		RequestID: "rfq-1",
		AssetIn:   "asset-1",
		AmountIn:  udecimal.MustParse("100"),
	})
	if err != nil {
		t.Fatalf("create rfq quote: %v", err)
	}
	if quote.QuoteID != "quote-1" {
		t.Errorf("unexpected quote id: %s", quote.QuoteID)
	}

	// Get Best Quote
	best, err := client.GetRFQBestQuote(ctx, "rfq-1")
	if err != nil {
		t.Fatalf("get best quote: %v", err)
	}
	if best.ID != "quote-1" {
		t.Errorf("unexpected best quote id: %s", best.ID)
	}

	// Accept Quote
	resp, err := client.AcceptRFQQuote(ctx, AcceptRFQQuoteRequest{
		RequestID: "rfq-1",
		QuoteID:   "quote-1",
		SignedOrder: SignedOrder{
			Salt:       "123",
			Expiration: "0",
		},
	})
	if err != nil {
		t.Fatalf("accept rfq quote: %v", err)
	}
	if len(resp.TradeIDs) != 1 || resp.TradeIDs[0] != "trade-1" {
		t.Errorf("unexpected accepted trade ids: %v", resp.TradeIDs)
	}

	// Approve Order
	err = client.ApproveRFQOrder(ctx, ApproveRFQOrderRequest{
		RequestID: "rfq-1",
		QuoteID:   "quote-1",
		SignedOrder: SignedOrder{
			Salt:       "456",
			Expiration: "0",
		},
	})
	if err != nil {
		t.Fatalf("approve rfq order: %v", err)
	}
	// Cancel Quote
	err = client.CancelRFQQuote(ctx, "quote-1")
	if err != nil {
		t.Fatalf("cancel rfq quote: %v", err)
	}

	// Cancel Request
	err = client.CancelRFQRequest(ctx, "rfq-1")
	if err != nil {
		t.Fatalf("cancel rfq request: %v", err)
	}
}
