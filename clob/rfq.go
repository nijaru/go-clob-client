package clob

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// CreateRFQRequest initiates a new Request for Quote.
// Level 2 Auth required.
func (c *AuthenticatedClient) CreateRFQRequest(
	ctx context.Context,
	params CreateRFQRequestParams,
) (*RFQRequestResponse, error) {
	var resp RFQRequestResponse
	if err := c.postJSON(ctx, rfqRequestEndpoint, params, polyhttp.AuthL2Builder, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CancelRFQRequest cancels an existing RFQ request.
// Level 2 Auth required.
func (c *AuthenticatedClient) CancelRFQRequest(ctx context.Context, requestID string) error {
	body := map[string]string{"requestId": requestID}
	return c.deleteJSON(ctx, rfqRequestEndpoint, body, polyhttp.AuthL2Builder, nil)
}

// GetRFQRequests retrieves RFQ requests, optionally filtered by state or IDs.
// Level 2 Auth required.
func (c *AuthenticatedClient) GetRFQRequests(
	ctx context.Context,
	params *RFQRequestFilterParams,
) (*RFQRequestsResponse, error) {
	query := url.Values{}
	if params != nil {
		if params.Limit > 0 {
			query.Set("limit", strconv.Itoa(params.Limit))
		}
		if params.Offset != "" {
			query.Set("offset", params.Offset)
		}
		if params.State != "" {
			query.Set("state", params.State)
		}
		for _, id := range params.RequestIDs {
			query.Add("requestIds", id)
		}
		if len(params.Markets) > 0 {
			query.Set("markets", strings.Join(params.Markets, ","))
		}
		if params.SizeMin != "" {
			query.Set("sizeMin", params.SizeMin)
		}
		if params.SizeMax != "" {
			query.Set("sizeMax", params.SizeMax)
		}
		if params.SizeUSDcMin != "" {
			query.Set("sizeUsdcMin", params.SizeUSDcMin)
		}
		if params.SizeUSDcMax != "" {
			query.Set("sizeUsdcMax", params.SizeUSDcMax)
		}
		if params.PriceMin != "" {
			query.Set("priceMin", params.PriceMin)
		}
		if params.PriceMax != "" {
			query.Set("priceMax", params.PriceMax)
		}
		if params.SortBy != "" {
			query.Set("sortBy", params.SortBy)
		}
		if params.SortDir != "" {
			query.Set("sortDir", params.SortDir)
		}
	}

	var resp RFQRequestsResponse
	if err := c.getJSON(ctx, rfqDataRequestsEndpoint, query, polyhttp.AuthL2Builder, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateRFQQuote responds to an RFQ request with a quote.
// Level 2 Auth required.
func (c *AuthenticatedClient) CreateRFQQuote(
	ctx context.Context,
	params CreateRFQQuoteParams,
) (*RFQQuoteResponse, error) {
	var resp RFQQuoteResponse
	if err := c.postJSON(ctx, rfqQuoteEndpoint, params, polyhttp.AuthL2Builder, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CancelRFQQuote cancels an existing RFQ quote.
// Level 2 Auth required.
func (c *AuthenticatedClient) CancelRFQQuote(ctx context.Context, quoteID string) error {
	body := map[string]string{"quoteId": quoteID}
	return c.deleteJSON(ctx, rfqQuoteEndpoint, body, polyhttp.AuthL2Builder, nil)
}

// GetRFQQuotes retrieves RFQ quotes visible to the authenticated user.
// As requester, returns quotes on requests you created.
// As quoter, returns quotes you have submitted.
// Level 2 Auth required.
func (c *AuthenticatedClient) GetRFQQuotes(
	ctx context.Context,
	params *RFQQuoteFilterParams,
) (*RFQQuotesResponse, error) {
	query := url.Values{}
	if params != nil {
		if params.Limit > 0 {
			query.Set("limit", strconv.Itoa(params.Limit))
		}
		if params.Offset != "" {
			query.Set("offset", params.Offset)
		}
		if params.State != "" {
			query.Set("state", params.State)
		}
		for _, id := range params.RequestIDs {
			query.Add("requestIds", id)
		}
		for _, id := range params.QuoteIDs {
			query.Add("quoteIds", id)
		}
		if len(params.Markets) > 0 {
			query.Set("markets", strings.Join(params.Markets, ","))
		}
		if params.SizeMin != "" {
			query.Set("sizeMin", params.SizeMin)
		}
		if params.SizeMax != "" {
			query.Set("sizeMax", params.SizeMax)
		}
		if params.SizeUSDcMin != "" {
			query.Set("sizeUsdcMin", params.SizeUSDcMin)
		}
		if params.SizeUSDcMax != "" {
			query.Set("sizeUsdcMax", params.SizeUSDcMax)
		}
		if params.PriceMin != "" {
			query.Set("priceMin", params.PriceMin)
		}
		if params.PriceMax != "" {
			query.Set("priceMax", params.PriceMax)
		}
		if params.SortBy != "" {
			query.Set("sortBy", params.SortBy)
		}
		if params.SortDir != "" {
			query.Set("sortDir", params.SortDir)
		}
	}

	var resp RFQQuotesResponse
	if err := c.getJSON(ctx, rfqDataQuotesEndpoint, query, polyhttp.AuthL2Builder, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// AcceptRFQQuote accepts a specific RFQ quote.
// The server returns plain text "OK" on success.
// Level 2 Auth required.
func (c *AuthenticatedClient) AcceptRFQQuote(
	ctx context.Context,
	params AcceptRFQQuoteRequest,
) error {
	return c.postJSON(ctx, rfqRequestAcceptEndpoint, params, polyhttp.AuthL2Builder, nil)
}

// ApproveRFQOrder allows a quoter to approve the final order during the last-look window.
// Returns trade IDs queued for on-chain execution.
// Level 2 Auth required.
func (c *AuthenticatedClient) ApproveRFQOrder(
	ctx context.Context,
	params ApproveRFQOrderRequest,
) (*ApproveRFQOrderResponse, error) {
	var resp ApproveRFQOrderResponse
	if err := c.postJSON(ctx, rfqQuoteApproveEndpoint, params, polyhttp.AuthL2Builder, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
