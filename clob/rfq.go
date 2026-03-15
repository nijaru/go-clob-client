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
func (c *Client) CreateRFQRequest(
	ctx context.Context,
	params CreateRFQRequestParams,
) (*RFQRequest, error) {
	var resp RFQRequest
	if err := c.postJSON(ctx, rfqRequestEndpoint, params, polyhttp.AuthL2, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CancelRFQRequest cancels an existing RFQ request.
// Level 2 Auth required.
func (c *Client) CancelRFQRequest(ctx context.Context, requestID string) error {
	body := map[string]string{"id": requestID}
	return c.deleteJSON(ctx, rfqRequestEndpoint, body, polyhttp.AuthL2, nil)
}

// GetRFQRequests retrieves RFQ requests, optionally filtered by state or IDs.
// Level 2 Auth required.
func (c *Client) GetRFQRequests(
	ctx context.Context,
	params *RFQRequestFilterParams,
) (RFQRequestsResponse, error) {
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
		if len(params.RequestIDs) > 0 {
			query.Set("requestIds", strings.Join(params.RequestIDs, ","))
		}
	}

	var resp RFQRequestsResponse
	if err := c.getJSON(ctx, rfqDataRequestsEndpoint, query, polyhttp.AuthL2, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// CreateRFQQuote responds to an RFQ request with a quote.
// Level 2 Auth required.
func (c *Client) CreateRFQQuote(
	ctx context.Context,
	params CreateRFQQuoteParams,
) (*RFQQuote, error) {
	var resp RFQQuote
	if err := c.postJSON(ctx, rfqQuoteEndpoint, params, polyhttp.AuthL2, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CancelRFQQuote cancels an existing RFQ quote.
// Level 2 Auth required.
func (c *Client) CancelRFQQuote(ctx context.Context, quoteID string) error {
	body := map[string]string{"id": quoteID}
	return c.deleteJSON(ctx, rfqQuoteEndpoint, body, polyhttp.AuthL2, nil)
}

// GetRequesterQuotes retrieves quotes on requests created by the authenticated user.
// Level 2 Auth required.
func (c *Client) GetRequesterQuotes(
	ctx context.Context,
	params *RFQQuoteFilterParams,
) (RFQQuotesResponse, error) {
	query := url.Values{}
	if params != nil {
		if params.Limit > 0 {
			query.Set("limit", strconv.Itoa(params.Limit))
		}
		if params.Offset != "" {
			query.Set("offset", params.Offset)
		}
		if len(params.RequestIDs) > 0 {
			query.Set("requestIds", strings.Join(params.RequestIDs, ","))
		}
	}

	var resp RFQQuotesResponse
	if err := c.getJSON(ctx, rfqRequesterQuotesEndpoint, query, polyhttp.AuthL2, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetQuoterQuotes retrieves quotes created by the authenticated user.
// Level 2 Auth required.
func (c *Client) GetQuoterQuotes(
	ctx context.Context,
	params *RFQQuoteFilterParams,
) (RFQQuotesResponse, error) {
	query := url.Values{}
	if params != nil {
		if params.Limit > 0 {
			query.Set("limit", strconv.Itoa(params.Limit))
		}
		if params.Offset != "" {
			query.Set("offset", params.Offset)
		}
		if len(params.RequestIDs) > 0 {
			query.Set("requestIds", strings.Join(params.RequestIDs, ","))
		}
	}

	var resp RFQQuotesResponse
	if err := c.getJSON(ctx, rfqQuoterQuotesEndpoint, query, polyhttp.AuthL2, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetBestQuote retrieves the current best quote for a specific request.
// Level 2 Auth required.
func (c *Client) GetBestQuote(ctx context.Context, requestID string) (*RFQQuote, error) {
	query := url.Values{}
	query.Set("requestId", requestID)

	var resp RFQQuote
	if err := c.getJSON(ctx, rfqBestQuoteEndpoint, query, polyhttp.AuthL2, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// AcceptRFQQuote accepts a specific RFQ quote.
// Returns an AcceptRFQQuoteResponse containing the order to be signed.
// Level 2 Auth required.
func (c *Client) AcceptRFQQuote(
	ctx context.Context,
	quoteID string,
) (*AcceptRFQQuoteResponse, error) {
	body := AcceptRFQQuoteRequest{QuoteID: quoteID}
	var resp AcceptRFQQuoteResponse
	if err := c.postJSON(ctx, rfqQuoteAcceptEndpoint, body, polyhttp.AuthL2, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ApproveRFQOrder allows a quoter to approve the final order.
// Level 2 Auth required.
func (c *Client) ApproveRFQOrder(ctx context.Context, params ApproveRFQOrderRequest) error {
	return c.postJSON(ctx, rfqOrderApproveEndpoint, params, polyhttp.AuthL2, nil)
}
