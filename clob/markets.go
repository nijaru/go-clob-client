package clob

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

func (c *Client) GetOK(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.getJSON(ctx, "/", nil, polyhttp.AuthNone, &out)
	return out, err
}

func (c *Client) GetServerTime(ctx context.Context) (int64, error) {
	var out int64
	err := c.getJSON(ctx, timeEndpoint, nil, polyhttp.AuthNone, &out)
	return out, err
}

func (c *Client) GetSamplingSimplifiedMarkets(
	ctx context.Context,
	nextCursor string,
) (*CursorPage, error) {
	return c.getPage(ctx, samplingSimplifiedMarketsEndpoint, nextCursor)
}

func (c *Client) GetSamplingMarkets(ctx context.Context, nextCursor string) (*CursorPage, error) {
	return c.getPage(ctx, samplingMarketsEndpoint, nextCursor)
}

func (c *Client) GetSimplifiedMarkets(ctx context.Context, nextCursor string) (*CursorPage, error) {
	return c.getPage(ctx, simplifiedMarketsEndpoint, nextCursor)
}

func (c *Client) GetMarkets(ctx context.Context, nextCursor string) (*CursorPage, error) {
	return c.getPage(ctx, marketsEndpoint, nextCursor)
}

func (c *Client) GetMarket(ctx context.Context, conditionID string) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.getJSON(ctx, marketEndpoint+conditionID, nil, polyhttp.AuthNone, &out)
	return out, err
}

func (c *Client) GetOrderBook(ctx context.Context, tokenID string) (*OrderBookSummary, error) {
	query := url.Values{}
	query.Set("token_id", tokenID)

	var out OrderBookSummary
	err := c.getJSON(ctx, orderBookEndpoint, query, polyhttp.AuthNone, &out)
	return &out, err
}

func (c *Client) GetOrderBooks(
	ctx context.Context,
	books []BookParams,
) ([]OrderBookSummary, error) {
	var out []OrderBookSummary
	err := c.postJSON(ctx, orderBooksEndpoint, books, polyhttp.AuthNone, &out)
	return out, err
}

func (c *Client) GetMidpoint(ctx context.Context, tokenID string) (*PriceResponse, error) {
	return c.getPriceLike(ctx, midpointEndpoint, tokenID)
}

func (c *Client) GetMidpoints(ctx context.Context, books []BookParams) ([]PriceResponse, error) {
	return c.postPriceLike(ctx, midpointsEndpoint, books)
}

func (c *Client) GetPrice(ctx context.Context, tokenID, side string) (*PriceResponse, error) {
	query := url.Values{}
	query.Set("token_id", tokenID)
	query.Set("side", side)

	var out PriceResponse
	err := c.getJSON(ctx, priceEndpoint, query, polyhttp.AuthNone, &out)
	return &out, err
}

func (c *Client) GetPrices(ctx context.Context, books []BookParams) ([]PriceResponse, error) {
	return c.postPriceLike(ctx, pricesEndpoint, books)
}

func (c *Client) GetSpread(ctx context.Context, tokenID string) (*SpreadResponse, error) {
	query := url.Values{}
	query.Set("token_id", tokenID)

	var out SpreadResponse
	err := c.getJSON(ctx, spreadEndpoint, query, polyhttp.AuthNone, &out)
	return &out, err
}

func (c *Client) GetSpreads(ctx context.Context, books []BookParams) ([]SpreadResponse, error) {
	var out []SpreadResponse
	err := c.postJSON(ctx, spreadsEndpoint, books, polyhttp.AuthNone, &out)
	return out, err
}

func (c *Client) GetLastTradePrice(ctx context.Context, tokenID string) (*PriceResponse, error) {
	return c.getPriceLike(ctx, lastTradePriceEndpoint, tokenID)
}

func (c *Client) GetLastTradesPrices(
	ctx context.Context,
	books []BookParams,
) ([]PriceResponse, error) {
	return c.postPriceLike(ctx, lastTradesPricesEndpoint, books)
}

func (c *Client) GetTickSize(ctx context.Context, tokenID string) (*TickSizeResponse, error) {
	query := url.Values{}
	query.Set("token_id", tokenID)

	var out TickSizeResponse
	err := c.getJSON(ctx, tickSizeEndpoint, query, polyhttp.AuthNone, &out)
	return &out, err
}

func (c *Client) GetNegRisk(ctx context.Context, tokenID string) (*NegRiskResponse, error) {
	query := url.Values{}
	query.Set("token_id", tokenID)

	var out NegRiskResponse
	err := c.getJSON(ctx, negRiskEndpoint, query, polyhttp.AuthNone, &out)
	return &out, err
}

func (c *Client) GetFeeRate(ctx context.Context, tokenID string) (*FeeRateResponse, error) {
	query := url.Values{}
	query.Set("token_id", tokenID)

	var out FeeRateResponse
	err := c.getJSON(ctx, feeRateEndpoint, query, polyhttp.AuthNone, &out)
	return &out, err
}

func (c *Client) GetFeeRateBps(ctx context.Context, tokenID string) (int64, error) {
	response, err := c.GetFeeRate(ctx, tokenID)
	if err != nil {
		return 0, err
	}
	return response.BaseFee, nil
}

func (c *Client) getPage(ctx context.Context, endpoint, nextCursor string) (*CursorPage, error) {
	query := url.Values{}
	if nextCursor != "" {
		query.Set("next_cursor", nextCursor)
	}

	var out CursorPage
	err := c.getJSON(ctx, endpoint, query, polyhttp.AuthNone, &out)
	return &out, err
}

func (c *Client) getPriceLike(
	ctx context.Context,
	endpoint, tokenID string,
) (*PriceResponse, error) {
	query := url.Values{}
	query.Set("token_id", tokenID)

	var out PriceResponse
	err := c.getJSON(ctx, endpoint, query, polyhttp.AuthNone, &out)
	return &out, err
}

func (c *Client) postPriceLike(
	ctx context.Context,
	endpoint string,
	books []BookParams,
) ([]PriceResponse, error) {
	var out []PriceResponse
	err := c.postJSON(ctx, endpoint, books, polyhttp.AuthNone, &out)
	return out, err
}
