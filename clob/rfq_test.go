package clob

import (
	"context"
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
				_ = json.NewEncoder(w).Encode(RFQRequest{
					ID:        "rfq-1",
					AssetIn:   "asset-1",
					AssetOut:  "asset-2",
					AmountIn:  "100",
					AmountOut: "50",
					Status:    "active",
				})
				return
			}
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		case rfqDataRequestsEndpoint:
			_ = json.NewEncoder(w).Encode(RFQRequestsResponse{
				{ID: "rfq-1", Status: "active"},
			})
		case rfqQuoteEndpoint:
			if r.Method == http.MethodPost {
				_ = json.NewEncoder(w).Encode(RFQQuote{
					ID:        "quote-1",
					RequestID: "rfq-1",
					Status:    "active",
				})
				return
			}
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		case rfqQuoteAcceptEndpoint:
			_ = json.NewEncoder(w).Encode(AcceptRFQQuoteResponse{
				Order: SignedOrder{TokenID: "123", Salt: "456"},
			})
		case rfqOrderApproveEndpoint:
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

	ctx := context.Background()

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
	if req.ID != "rfq-1" {
		t.Errorf("unexpected rfq id: %s", req.ID)
	}

	// Get Requests
	list, err := client.GetRFQRequests(ctx, &RFQRequestFilterParams{State: "active"})
	if err != nil {
		t.Fatalf("get rfq requests: %v", err)
	}
	if len(list) != 1 || list[0].ID != "rfq-1" {
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
	if quote.ID != "quote-1" {
		t.Errorf("unexpected quote id: %s", quote.ID)
	}

	// Get Best Quote
	best, err := client.GetBestQuote(ctx, "rfq-1")
	if err != nil {
		t.Fatalf("get best quote: %v", err)
	}
	if best.ID != "quote-1" {
		t.Errorf("unexpected best quote id: %s", best.ID)
	}

	// Accept Quote
	resp, err := client.AcceptRFQQuote(ctx, "quote-1")
	if err != nil {
		t.Fatalf("accept rfq quote: %v", err)
	}
	if resp.Order.TokenID != "123" {
		t.Errorf("unexpected accepted order token: %s", resp.Order.TokenID)
	}

	// Approve Order
	err = client.ApproveRFQOrder(ctx, ApproveRFQOrderRequest{
		RequestID: "rfq-1",
		Order:     resp.Order,
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
