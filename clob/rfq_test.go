package clob

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/quagmt/udecimal"
)

func TestAcceptRFQQuoteMarshalJSON(t *testing.T) {
	t.Parallel()

	req := AcceptRFQQuoteRequest{
		RequestID: "req-1",
		QuoteID:   "quote-1",
		Owner:     "0xabc",
		SignedOrder: SignedOrder{
			Salt:          "1234567890",
			Maker:         "0xmaker",
			Signer:        "0xsigner",
			Taker:         "0xtaker",
			TokenID:       "token-1",
			MakerAmount:   "100",
			TakerAmount:   "200",
			Expiration:    "1700000000",
			Nonce:         "1",
			FeeRateBps:    "50",
			Side:          SideBuy,
			SignatureType: SignatureTypeEOA,
			Signature:     "0xsig",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Verify salt and expiration are JSON numbers, not strings
	var raw map[string]jsontext.Value
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}

	// salt must be a JSON number
	if len(raw["salt"]) == 0 || raw["salt"][0] == '"' {
		t.Errorf("salt should be a JSON number, got: %s", raw["salt"])
	}
	if string(raw["salt"]) != "1234567890" {
		t.Errorf("salt = %s, want 1234567890", raw["salt"])
	}

	// expiration must be a JSON number
	if len(raw["expiration"]) == 0 || raw["expiration"][0] == '"' {
		t.Errorf("expiration should be a JSON number, got: %s", raw["expiration"])
	}
	if string(raw["expiration"]) != "1700000000" {
		t.Errorf("expiration = %s, want 1700000000", raw["expiration"])
	}

	// Verify requestId/quoteId are present
	if string(raw["requestId"]) != `"req-1"` {
		t.Errorf("requestId = %s, want %q", raw["requestId"], "req-1")
	}
	if string(raw["quoteId"]) != `"quote-1"` {
		t.Errorf("quoteId = %s, want %q", raw["quoteId"], "quote-1")
	}
}

func TestApproveRFQOrderMarshalJSON(t *testing.T) {
	t.Parallel()

	req := ApproveRFQOrderRequest{
		RequestID: "req-2",
		QuoteID:   "quote-2",
		Owner:     "0xdef",
		SignedOrder: SignedOrder{
			Salt:       "999",
			Expiration: "0",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]jsontext.Value
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}

	if string(raw["salt"]) != "999" {
		t.Errorf("salt = %s, want 999", raw["salt"])
	}
	if string(raw["expiration"]) != "0" {
		t.Errorf("expiration = %s, want 0", raw["expiration"])
	}
}

func TestMarshalRFQOrderInvalidSalt(t *testing.T) {
	t.Parallel()

	req := AcceptRFQQuoteRequest{
		RequestID: "req-1",
		QuoteID:   "q-1",
		SignedOrder: SignedOrder{
			Salt:       "not-a-number",
			Expiration: "0",
		},
	}

	_, err := json.Marshal(req)
	if err == nil {
		t.Fatal("expected error for invalid salt")
	}
	if !strings.Contains(err.Error(), "parse order salt") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMarshalRFQOrderInvalidExpiration(t *testing.T) {
	t.Parallel()

	req := AcceptRFQQuoteRequest{
		RequestID: "req-1",
		QuoteID:   "q-1",
		SignedOrder: SignedOrder{
			Salt:       "1",
			Expiration: "abc",
		},
	}

	_, err := json.Marshal(req)
	if err == nil {
		t.Fatal("expected error for invalid expiration")
	}
	if !strings.Contains(err.Error(), "parse order expiration") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRFQSurfaces(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case rfqRequestEndpoint:
			if r.Method == http.MethodPost {
				data, _ := json.Marshal(RFQRequestResponse{
					RequestID: "rfq-1",
				})
				w.Write(data)
				return
			}
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		case rfqDataRequestsEndpoint:
			data, _ := json.Marshal(RFQRequestsResponse{
				Data: []RFQRequest{
					{ID: "rfq-1", State: "active"},
				},
			})
			w.Write(data)
		case rfqQuoteEndpoint:
			if r.Method == http.MethodPost {
				data, _ := json.Marshal(RFQQuoteResponse{
					QuoteID: "quote-1",
				})
				w.Write(data)
				return
			}
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		case rfqRequestAcceptEndpoint:
			data, _ := json.Marshal(AcceptRFQQuoteResponse{
				TradeIDs: []string{"trade-1"},
			})
			w.Write(data)
		case rfqQuoteApproveEndpoint:
			w.WriteHeader(http.StatusOK)
		case rfqBestQuoteEndpoint:
			data, _ := json.Marshal(RFQQuote{
				ID:        "quote-1",
				RequestID: "rfq-1",
			})
			w.Write(data)
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
