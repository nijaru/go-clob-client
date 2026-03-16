package clob

import (
	"context"
	"encoding/json/jsontext"
	"net/url"
	"strconv"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// CreateRFQRequest initiates a new Request for Quote.
// Level 2 Auth required.
func (c *Client) CreateRFQRequest(
	ctx context.Context,
	params CreateRFQRequestParams,
) (*RFQRequestResponse, error) {
	var resp RFQRequestResponse
	if err := c.postJSON(ctx, rfqRequestEndpoint, params, polyhttp.AuthL2, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CancelRFQRequest cancels an existing RFQ request.
// Level 2 Auth required.
func (c *Client) CancelRFQRequest(ctx context.Context, requestID string) error {
	body := map[string]string{"requestId": requestID}
	return c.deleteJSON(ctx, rfqRequestEndpoint, body, polyhttp.AuthL2, nil)
}

// GetRFQRequests retrieves RFQ requests, optionally filtered by state or IDs.
// Level 2 Auth required.
func (c *Client) GetRFQRequests(
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
		for _, m := range params.Markets {
			query.Add("markets", m)
		}
	}

	var resp RFQRequestsResponse
	if err := c.getJSON(ctx, rfqDataRequestsEndpoint, query, polyhttp.AuthL2, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateRFQQuote responds to an RFQ request with a quote.
// Level 2 Auth required.
func (c *Client) CreateRFQQuote(
	ctx context.Context,
	params CreateRFQQuoteParams,
) (*RFQQuoteResponse, error) {
	var resp RFQQuoteResponse
	if err := c.postJSON(ctx, rfqQuoteEndpoint, params, polyhttp.AuthL2, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CancelRFQQuote cancels an existing RFQ quote.
// Level 2 Auth required.
func (c *Client) CancelRFQQuote(ctx context.Context, quoteID string) error {
	body := map[string]string{"quoteId": quoteID}
	return c.deleteJSON(ctx, rfqQuoteEndpoint, body, polyhttp.AuthL2, nil)
}

// GetRFQRequesterQuotes retrieves quotes on requests created by the authenticated user.
// Level 2 Auth required.
func (c *Client) GetRFQRequesterQuotes(
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
		for _, id := range params.RequestIDs {
			query.Add("requestIds", id)
		}
		for _, id := range params.QuoteIDs {
			query.Add("quoteIds", id)
		}
	}

	var resp RFQQuotesResponse
	if err := c.getJSON(ctx, rfqRequesterQuotesEndpoint, query, polyhttp.AuthL2, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetRFQQuoterQuotes retrieves quotes created by the authenticated user.
// Level 2 Auth required.
func (c *Client) GetRFQQuoterQuotes(
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
		for _, id := range params.RequestIDs {
			query.Add("requestIds", id)
		}
		for _, id := range params.QuoteIDs {
			query.Add("quoteIds", id)
		}
	}

	var resp RFQQuotesResponse
	if err := c.getJSON(ctx, rfqQuoterQuotesEndpoint, query, polyhttp.AuthL2, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetRFQBestQuote retrieves the current best quote for a specific request.
// Level 2 Auth required.
func (c *Client) GetRFQBestQuote(ctx context.Context, requestID string) (*RFQQuote, error) {
	query := url.Values{}
	query.Set("requestId", requestID)

	var resp RFQQuote
	if err := c.getJSON(ctx, rfqBestQuoteEndpoint, query, polyhttp.AuthL2, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// AcceptRFQQuote accepts a specific RFQ quote.
// Returns an AcceptRFQQuoteResponse containing the resulting trade IDs.
// Level 2 Auth required.
func (c *Client) AcceptRFQQuote(
	ctx context.Context,
	params AcceptRFQQuoteRequest,
) (*AcceptRFQQuoteResponse, error) {
	var resp AcceptRFQQuoteResponse
	if err := c.postJSON(ctx, rfqRequestAcceptEndpoint, params, polyhttp.AuthL2, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ApproveRFQOrder allows a quoter to approve the final order.
// Level 2 Auth required.
func (c *Client) ApproveRFQOrder(ctx context.Context, params ApproveRFQOrderRequest) error {
	return c.postJSON(ctx, rfqQuoteApproveEndpoint, params, polyhttp.AuthL2, nil)
}

// GetRFQConfig retrieves the current RFQ configuration.
// Level 2 Auth required.
func (c *Client) GetRFQConfig(ctx context.Context) (jsontext.Value, error) {
	var resp jsontext.Value
	if err := c.getJSON(ctx, rfqConfigEndpoint, nil, polyhttp.AuthL2, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
