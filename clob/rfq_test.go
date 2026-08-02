package clob

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	json "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"

	"github.com/quagmt/udecimal"
)

func TestRFQResponsePricesPreserveDecimalPrecision(t *testing.T) {
	t.Parallel()

	var response struct {
		Requests []RFQRequest `json:"requests"`
		Quotes   []RFQQuote   `json:"quotes"`
	}
	if err := json.Unmarshal([]byte(`{
		"requests":[{"price":"0.123456789012345678901234567890"}],
		"quotes":[{"price":0.987654321098765432109876543210}]
	}`), &response); err != nil {
		t.Fatalf("decode RFQ prices: %v", err)
	}
	if got := response.Requests[0].Price.String(); got != "0.123456789012345678901234567890" {
		t.Fatalf("request price = %q", got)
	}
	if got := response.Quotes[0].Price.String(); got != "0.987654321098765432109876543210" {
		t.Fatalf("quote price = %q", got)
	}
}

func TestAcceptRFQQuoteMarshalJSON(t *testing.T) {
	t.Parallel()

	req := AcceptRFQQuoteRequest{
		RequestID: "req-1",
		QuoteID:   "quote-1",
		Owner:     "0xabc",
		RFQSignedOrder: RFQSignedOrder{
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
		RFQSignedOrder: RFQSignedOrder{
			Salt:       "999",
			Expiration: "0",
			Nonce:      "0",
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
		RFQSignedOrder: RFQSignedOrder{
			Salt:       "not-a-number",
			Expiration: "0",
			Nonce:      "0",
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
		RFQSignedOrder: RFQSignedOrder{
			Salt:       "1",
			Expiration: "abc",
			Nonce:      "0",
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
			// Server returns plain text "OK" for accept.
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("OK"))
		case rfqQuoteApproveEndpoint:
			data, _ := json.Marshal(ApproveRFQOrderResponse{
				TradeIDs: []string{"trade-1"},
			})
			w.Write(data)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewAuthenticatedClient(Config{
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

	// Accept Quote — server returns plain text "OK"
	if err := client.AcceptRFQQuote(ctx, AcceptRFQQuoteRequest{
		RequestID: "rfq-1",
		QuoteID:   "quote-1",
		RFQSignedOrder: RFQSignedOrder{
			Salt:       "123",
			Expiration: "0",
			Nonce:      "0",
		},
	}); err != nil {
		t.Fatalf("accept rfq quote: %v", err)
	}

	// Approve Order — server returns JSON with trade IDs
	approved, err := client.ApproveRFQOrder(ctx, ApproveRFQOrderRequest{
		RequestID: "rfq-1",
		QuoteID:   "quote-1",
		RFQSignedOrder: RFQSignedOrder{
			Salt:       "456",
			Expiration: "0",
			Nonce:      "0",
		},
	})
	if err != nil {
		t.Fatalf("approve rfq order: %v", err)
	}
	if len(approved.TradeIDs) != 1 || approved.TradeIDs[0] != "trade-1" {
		t.Errorf("unexpected approve trade ids: %v", approved.TradeIDs)
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

func TestRFQErrorCodes(t *testing.T) {
	t.Parallel()

	// Verify all error codes are defined with expected values.
	codes := map[RFQErrorCode]string{
		RFQCodeAddressMismatch:                 "ADDRESS_MISMATCH",
		RFQCodeAllowanceValidationFailed:       "ALLOWANCE_VALIDATION_FAILED",
		RFQCodeBalanceValidationFailed:         "BALANCE_VALIDATION_FAILED",
		RFQCodeContradictoryLegs:               "CONTRADICTORY_LEGS",
		RFQCodeExpiredRFQ:                      "EXPIRED_RFQ",
		RFQCodeInvalidAcceptance:               "INVALID_ACCEPTANCE",
		RFQCodeInvalidConfirmation:             "INVALID_CONFIRMATION",
		RFQCodeInvalidExecutionResult:          "INVALID_EXECUTION_RESULT",
		RFQCodeInvalidIdentity:                 "INVALID_IDENTITY",
		RFQCodeInvalidMessage:                  "INVALID_MESSAGE",
		RFQCodeInvalidQuote:                    "INVALID_QUOTE",
		RFQCodeInvalidRFQ:                      "INVALID_RFQ",
		RFQCodeInvalidRFQState:                 "INVALID_RFQ_STATE",
		RFQCodeInvalidRole:                     "INVALID_ROLE",
		RFQCodeInvalidSignature:                "INVALID_SIGNATURE",
		RFQCodeInternalError:                   "INTERNAL_ERROR",
		RFQCodeLegMetadataUnavailable:          "LEG_METADATA_UNAVAILABLE",
		RFQCodeMakerAlreadyResponded:           "MAKER_ALREADY_RESPONDED",
		RFQCodeMakerNotRequired:                "MAKER_NOT_REQUIRED",
		RFQCodeMakerQuoteLimited:               "MAKER_QUOTE_LIMITED",
		RFQCodePreExecBalanceReservationFailed: "PRE_EXECUTION_BALANCE_RESERVATION_FAILED",
		RFQCodeQuoteMismatch:                   "QUOTE_MISMATCH",
		RFQCodeQuoteUnavailable:                "QUOTE_UNAVAILABLE",
		RFQCodeRateLimited:                     "RATE_LIMITED",
		RFQCodeRequestFailed:                   "REQUEST_FAILED",
		RFQCodeServiceUnavailable:              "SERVICE_UNAVAILABLE",
		RFQCodeSubmissionWindowClosed:          "SUBMISSION_WINDOW_CLOSED",
		RFQCodeTradeSubmissionFailed:           "TRADE_SUBMISSION_FAILED",
		RFQCodeUnauthenticated:                 "UNAUTHENTICATED",
		RFQCodeUnauthorizedRole:                "UNAUTHORIZED_ROLE",
		RFQCodeUnknownRFQ:                      "UNKNOWN_RFQ",
	}

	for code, want := range codes {
		if string(code) != want {
			t.Errorf("RFQErrorCode %v = %q, want %q", code, string(code), want)
		}
	}
}

func TestRFQErrorInterface(t *testing.T) {
	t.Parallel()

	err := &RFQError{
		Code:      RFQCodeInvalidQuote,
		ErrorID:   "err-123",
		Message:   "quote expired",
		RequestID: "rfq-42",
	}

	want := "rfq error INVALID_QUOTE: quote expired (rfq=rfq-42)"
	if got := err.Error(); got != want {
		t.Errorf("RFQError.Error() = %q, want %q", got, want)
	}

	// Without request ID
	err2 := &RFQError{
		Code:    RFQCodeRateLimited,
		Message: "too many requests",
	}
	want2 := "rfq error RATE_LIMITED: too many requests"
	if got := err2.Error(); got != want2 {
		t.Errorf("RFQError.Error() = %q, want %q", got, want2)
	}
}

func TestRFQErrorPreservesUnknownCode(t *testing.T) {
	t.Parallel()

	var err RFQError
	if unmarshalErr := json.Unmarshal([]byte(`{
		"code":"FUTURE_ERROR_CODE",
		"error":"future rejection"
	}`), &err); unmarshalErr != nil {
		t.Fatalf("unmarshal RFQ error: %v", unmarshalErr)
	}
	if err.Code != RFQErrorCode("FUTURE_ERROR_CODE") {
		t.Fatalf("code = %q, want future code", err.Code)
	}
}

func TestComboMarketsSurface(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != rfqComboMarketsEndpoint {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		// Verify query params
		if got := r.URL.Query().Get("limit"); got != "50" {
			t.Errorf("limit = %q, want 50", got)
		}
		if got := r.URL.Query().Get("exclude"); got != "cond1,cond2" {
			t.Errorf("exclude = %q, want cond1,cond2", got)
		}

		w.Header().Set("Content-Type", "application/json")
		data, _ := json.Marshal(ComboMarketsPage{
			Markets: []ComboMarket{
				{
					ID:            "market-1",
					ConditionID:   "0xabc",
					Slug:          "test-market",
					Title:         "Test Market",
					Outcomes:      []string{"Yes", "No"},
					OutcomePrices: []string{"0.65", "0.35"},
					PositionIDs:   []string{"pos-yes", "pos-no"},
					Volume:        100000,
					Tags:          []string{"politics", "election"},
				},
			},
			NextCursor: "next-page",
		})
		w.Write(data)
	}))
	defer server.Close()

	client, err := NewClient(Config{Host: server.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	ctx := t.Context()
	page, err := client.ListComboMarkets(ctx, &ComboMarketFilterParams{
		Limit:   50,
		Exclude: []string{"cond1", "cond2"},
	})
	if err != nil {
		t.Fatalf("list combo markets: %v", err)
	}

	if len(page.Markets) != 1 {
		t.Fatalf("got %d markets, want 1", len(page.Markets))
	}

	m := page.Markets[0]
	if m.ID != "market-1" {
		t.Errorf("market ID = %q, want market-1", m.ID)
	}
	if m.ConditionID != "0xabc" {
		t.Errorf("condition ID = %q, want 0xabc", m.ConditionID)
	}

	outcomes, err := m.ParsedOutcomes()
	if err != nil {
		t.Fatalf("parsed outcomes: %v", err)
	}
	if outcomes.Yes.Label != "Yes" {
		t.Errorf("yes label = %q, want Yes", outcomes.Yes.Label)
	}
	if outcomes.Yes.Price != "0.65" {
		t.Errorf("yes price = %q, want 0.65", outcomes.Yes.Price)
	}
	if outcomes.No.PositionID != "pos-no" {
		t.Errorf("no position ID = %q, want pos-no", outcomes.No.PositionID)
	}
	if page.NextCursor != "next-page" {
		t.Errorf("next cursor = %q, want next-page", page.NextCursor)
	}
}

func TestRFQTradeEventJSON(t *testing.T) {
	t.Parallel()

	input := `{
		"type": "RFQ_TRADE",
		"rfq_id": "rfq-1",
		"requester_id": "req-1",
		"condition_id": "0xcond",
		"leg_position_ids": ["pos-1", "pos-2"],
		"direction": "BUY",
		"side": "YES",
		"price_e6": "650000",
		"size_e6": "100000000",
		"executed_at": 1700000000
	}`

	var event RFQTradeEvent
	if err := json.Unmarshal([]byte(input), &event); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if event.RFQID != "rfq-1" {
		t.Errorf("rfq_id = %q, want rfq-1", event.RFQID)
	}
	if event.Direction != RFQDirectionBuy {
		t.Errorf("direction = %q, want BUY", event.Direction)
	}
	if event.Side != RFQSideYes {
		t.Errorf("side = %q, want YES", event.Side)
	}
	if len(event.LegPositionIDs) != 2 {
		t.Errorf("leg position IDs count = %d, want 2", len(event.LegPositionIDs))
	}
	if event.PriceE6 != "650000" {
		t.Errorf("price_e6 = %q, want 650000", event.PriceE6)
	}
}

func TestGetRFQQuotes(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != rfqDataQuotesEndpoint {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		query := r.URL.Query()
		if query.Get("limit") != "5" {
			t.Errorf("limit = %q, want 5", query.Get("limit"))
		}
		if query.Get("offset") != "10" {
			t.Errorf("offset = %q, want 10", query.Get("offset"))
		}
		if query.Get("state") != "OPEN" {
			t.Errorf("state = %q, want OPEN", query.Get("state"))
		}
		if query.Get("requestIds") != "req-1" {
			t.Errorf("requestIds = %q, want req-1", query.Get("requestIds"))
		}
		if query.Get("quoteIds") != "quote-1" {
			t.Errorf("quoteIds = %q, want quote-1", query.Get("quoteIds"))
		}
		if query.Get("markets") != "m1,m2" {
			t.Errorf("markets = %q, want m1,m2", query.Get("markets"))
		}
		if query.Get("sizeMin") != "100" {
			t.Errorf("sizeMin = %q, want 100", query.Get("sizeMin"))
		}
		if query.Get("sizeMax") != "1000" {
			t.Errorf("sizeMax = %q, want 1000", query.Get("sizeMax"))
		}
		if query.Get("sizeUsdcMin") != "50" {
			t.Errorf("sizeUsdcMin = %q, want 50", query.Get("sizeUsdcMin"))
		}
		if query.Get("sizeUsdcMax") != "500" {
			t.Errorf("sizeUsdcMax = %q, want 500", query.Get("sizeUsdcMax"))
		}
		if query.Get("priceMin") != "0.2" {
			t.Errorf("priceMin = %q, want 0.2", query.Get("priceMin"))
		}
		if query.Get("priceMax") != "0.8" {
			t.Errorf("priceMax = %q, want 0.8", query.Get("priceMax"))
		}
		if query.Get("sortBy") != "price" {
			t.Errorf("sortBy = %q, want price", query.Get("sortBy"))
		}
		if query.Get("sortDir") != "desc" {
			t.Errorf("sortDir = %q, want desc", query.Get("sortDir"))
		}

		w.Header().Set("Content-Type", "application/json")
		resp := RFQQuotesResponse{
			Data: []RFQQuote{
				{
					ID:        "quote-1",
					RequestID: "req-1",
					Price:     "0.65",
					State:     "OPEN",
				},
			},
		}
		data, _ := json.Marshal(resp)
		w.Write(data)
	}))
	defer server.Close()

	client, err := NewAuthenticatedClient(Config{
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
	resp, err := client.GetRFQQuotes(ctx, &RFQQuoteFilterParams{
		Limit:       5,
		Offset:      "10",
		State:       "OPEN",
		RequestIDs:  []string{"req-1"},
		QuoteIDs:    []string{"quote-1"},
		Markets:     []string{"m1", "m2"},
		SizeMin:     "100",
		SizeMax:     "1000",
		SizeUSDcMin: "50",
		SizeUSDcMax: "500",
		PriceMin:    "0.2",
		PriceMax:    "0.8",
		SortBy:      "price",
		SortDir:     "desc",
	})
	if err != nil {
		t.Fatalf("GetRFQQuotes: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Fatalf("got %d quotes, want 1", len(resp.Data))
	}
	if resp.Data[0].ID != "quote-1" {
		t.Errorf("quote ID = %q, want quote-1", resp.Data[0].ID)
	}
}

func TestIterComboMarkets(t *testing.T) {
	t.Parallel()

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != rfqComboMarketsEndpoint {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		calls++

		query := r.URL.Query()
		var resp ComboMarketsPage
		if calls == 1 {
			if query.Get("cursor") != "" {
				t.Errorf("first call cursor = %q, want empty", query.Get("cursor"))
			}
			resp = ComboMarketsPage{
				Markets: []ComboMarket{
					{ID: "market-1"},
				},
				NextCursor: "cursor-2",
			}
		} else if calls == 2 {
			if query.Get("cursor") != "cursor-2" {
				t.Errorf("second call cursor = %q, want cursor-2", query.Get("cursor"))
			}
			resp = ComboMarketsPage{
				Markets: []ComboMarket{
					{ID: "market-2"},
				},
				NextCursor: "",
			}
		} else {
			t.Fatalf("unexpected call count: %d", calls)
		}

		w.Header().Set("Content-Type", "application/json")
		data, _ := json.Marshal(resp)
		w.Write(data)
	}))
	defer server.Close()

	client, err := NewClient(Config{Host: server.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	ctx := t.Context()
	var markets []ComboMarket
	for market, err := range client.IterComboMarkets(ctx, &ComboMarketFilterParams{Limit: 10}) {
		if err != nil {
			t.Fatalf("iterator error: %v", err)
		}
		markets = append(markets, market)
	}

	if len(markets) != 2 {
		t.Fatalf("got %d markets, want 2", len(markets))
	}
	if markets[0].ID != "market-1" || markets[1].ID != "market-2" {
		t.Errorf("unexpected markets order: %+v", markets)
	}
}

func TestGetAllComboMarkets(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := ComboMarketsPage{
			Markets: []ComboMarket{
				{ID: "market-abc"},
			},
			NextCursor: "",
		}
		data, _ := json.Marshal(resp)
		w.Write(data)
	}))
	defer server.Close()

	client, err := NewClient(Config{Host: server.URL})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	ctx := t.Context()
	markets, err := client.GetAllComboMarkets(ctx, nil)
	if err != nil {
		t.Fatalf("GetAllComboMarkets: %v", err)
	}

	if len(markets) != 1 {
		t.Fatalf("got %d markets, want 1", len(markets))
	}
	if markets[0].ID != "market-abc" {
		t.Errorf("market ID = %q, want market-abc", markets[0].ID)
	}
}
