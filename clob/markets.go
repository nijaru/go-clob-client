package clob

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// GetOK returns the raw health-check payload for compatibility with the official SDKs.
func (c *Client) GetOK(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.getJSON(ctx, "/", nil, polyhttp.AuthNone, &out)
	return out, err
}

// HealthCheck returns the typed health-check response body.
func (c *Client) HealthCheck(ctx context.Context) (string, error) {
	var out string
	err := c.getJSON(ctx, "/", nil, polyhttp.AuthNone, &out)
	return out, err
}

// GetServerTime returns the server time reported by the CLOB API.
func (c *Client) GetServerTime(ctx context.Context) (int64, error) {
	var out int64
	err := c.getJSON(ctx, timeEndpoint, nil, polyhttp.AuthNone, &out)
	return out, err
}

// GetSamplingSimplifiedMarkets returns the raw compatibility sampling-simplified markets page.
func (c *Client) GetSamplingSimplifiedMarkets(
	ctx context.Context,
	nextCursor string,
) (*CursorPage, error) {
	return c.getPage(ctx, samplingSimplifiedMarketsEndpoint, nextCursor)
}

// GetSamplingSimplifiedMarketsPage returns a typed sampling-simplified markets page.
func (c *Client) GetSamplingSimplifiedMarketsPage(
	ctx context.Context,
	nextCursor string,
) (*Page[SimplifiedMarket], error) {
	return getTypedPage[SimplifiedMarket](ctx, c, samplingSimplifiedMarketsEndpoint, nextCursor)
}

// GetSamplingMarkets returns the raw compatibility sampling markets page.
func (c *Client) GetSamplingMarkets(ctx context.Context, nextCursor string) (*CursorPage, error) {
	return c.getPage(ctx, samplingMarketsEndpoint, nextCursor)
}

// GetSamplingMarketsPage returns a typed sampling markets page.
func (c *Client) GetSamplingMarketsPage(
	ctx context.Context,
	nextCursor string,
) (*Page[Market], error) {
	return getTypedPage[Market](ctx, c, samplingMarketsEndpoint, nextCursor)
}

// GetSimplifiedMarkets returns the raw compatibility simplified markets page.
func (c *Client) GetSimplifiedMarkets(ctx context.Context, nextCursor string) (*CursorPage, error) {
	return c.getPage(ctx, simplifiedMarketsEndpoint, nextCursor)
}

// GetSimplifiedMarketsPage returns a typed simplified markets page.
func (c *Client) GetSimplifiedMarketsPage(
	ctx context.Context,
	nextCursor string,
) (*Page[SimplifiedMarket], error) {
	return getTypedPage[SimplifiedMarket](ctx, c, simplifiedMarketsEndpoint, nextCursor)
}

// GetMarkets returns the raw compatibility markets page.
func (c *Client) GetMarkets(ctx context.Context, nextCursor string) (*CursorPage, error) {
	return c.getPage(ctx, marketsEndpoint, nextCursor)
}

// GetMarketsPage returns a typed markets page.
func (c *Client) GetMarketsPage(ctx context.Context, nextCursor string) (*Page[Market], error) {
	return getTypedPage[Market](ctx, c, marketsEndpoint, nextCursor)
}

// GetMarket returns the raw compatibility market payload for a condition ID.
func (c *Client) GetMarket(ctx context.Context, conditionID string) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.getJSON(ctx, marketEndpoint+conditionID, nil, polyhttp.AuthNone, &out)
	return out, err
}

// GetMarketInfo returns a typed market record for a condition ID.
func (c *Client) GetMarketInfo(ctx context.Context, conditionID string) (*Market, error) {
	var out Market
	err := c.getJSON(ctx, marketEndpoint+conditionID, nil, polyhttp.AuthNone, &out)
	return &out, err
}

// GetOrderBook returns the typed order book for a token.
func (c *Client) GetOrderBook(ctx context.Context, tokenID string) (*OrderBookSummary, error) {
	query := url.Values{}
	query.Set("token_id", tokenID)

	var out OrderBookSummary
	err := c.getJSON(ctx, orderBookEndpoint, query, polyhttp.AuthNone, &out)
	return &out, err
}

// GetOrderBooks returns the typed order books for multiple tokens.
func (c *Client) GetOrderBooks(
	ctx context.Context,
	books []BookParams,
) ([]OrderBookSummary, error) {
	var out []OrderBookSummary
	err := c.postJSON(ctx, orderBooksEndpoint, books, polyhttp.AuthNone, &out)
	return out, err
}

// GetMidpoint returns the current midpoint price for a token.
func (c *Client) GetMidpoint(ctx context.Context, tokenID string) (*PriceResponse, error) {
	return c.getPriceLike(ctx, midpointEndpoint, tokenID)
}

// GetMidpoints returns midpoint prices for multiple tokens.
func (c *Client) GetMidpoints(ctx context.Context, books []BookParams) ([]PriceResponse, error) {
	return c.postPriceLike(ctx, midpointsEndpoint, books)
}

// GetPrice returns the best price for a token and side.
func (c *Client) GetPrice(ctx context.Context, tokenID, side string) (*PriceResponse, error) {
	query := url.Values{}
	query.Set("token_id", tokenID)
	query.Set("side", side)

	var out PriceResponse
	err := c.getJSON(ctx, priceEndpoint, query, polyhttp.AuthNone, &out)
	return &out, err
}

// GetPrices returns prices for multiple tokens.
func (c *Client) GetPrices(ctx context.Context, books []BookParams) ([]PriceResponse, error) {
	return c.postPriceLike(ctx, pricesEndpoint, books)
}

// GetSpread returns the current spread for a token.
func (c *Client) GetSpread(ctx context.Context, tokenID string) (*SpreadResponse, error) {
	query := url.Values{}
	query.Set("token_id", tokenID)

	var out SpreadResponse
	err := c.getJSON(ctx, spreadEndpoint, query, polyhttp.AuthNone, &out)
	return &out, err
}

// GetSpreads returns spreads for multiple tokens.
func (c *Client) GetSpreads(ctx context.Context, books []BookParams) ([]SpreadResponse, error) {
	var out []SpreadResponse
	err := c.postJSON(ctx, spreadsEndpoint, books, polyhttp.AuthNone, &out)
	return out, err
}

// GetLastTradePrice returns the last trade price for a token.
func (c *Client) GetLastTradePrice(ctx context.Context, tokenID string) (*PriceResponse, error) {
	return c.getPriceLike(ctx, lastTradePriceEndpoint, tokenID)
}

// GetLastTradesPrices returns the last trade prices for multiple tokens.
func (c *Client) GetLastTradesPrices(
	ctx context.Context,
	books []BookParams,
) ([]PriceResponse, error) {
	return c.postPriceLike(ctx, lastTradesPricesEndpoint, books)
}

// GetTickSize returns the minimum tick size for a token.
func (c *Client) GetTickSize(ctx context.Context, tokenID string) (*TickSizeResponse, error) {
	query := url.Values{}
	query.Set("token_id", tokenID)

	var out TickSizeResponse
	err := c.getJSON(ctx, tickSizeEndpoint, query, polyhttp.AuthNone, &out)
	return &out, err
}

// GetNegRisk returns whether a token is part of a neg-risk market.
func (c *Client) GetNegRisk(ctx context.Context, tokenID string) (*NegRiskResponse, error) {
	query := url.Values{}
	query.Set("token_id", tokenID)

	var out NegRiskResponse
	err := c.getJSON(ctx, negRiskEndpoint, query, polyhttp.AuthNone, &out)
	return &out, err
}

// GetFeeRate returns the fee-rate response for a token.
func (c *Client) GetFeeRate(ctx context.Context, tokenID string) (*FeeRateResponse, error) {
	query := url.Values{}
	query.Set("token_id", tokenID)

	var out FeeRateResponse
	err := c.getJSON(ctx, feeRateEndpoint, query, polyhttp.AuthNone, &out)
	return &out, err
}

// GetFeeRateBps returns the fee rate in basis points for a token.
func (c *Client) GetFeeRateBps(ctx context.Context, tokenID string) (int64, error) {
	response, err := c.GetFeeRate(ctx, tokenID)
	if err != nil {
		return 0, err
	}
	return response.BaseFee, nil
}

// GetPriceHistory returns the typed price-history series for the supplied filter.
func (c *Client) GetPriceHistory(
	ctx context.Context,
	params PriceHistoryFilterParams,
) ([]MarketPrice, error) {
	query := url.Values{}
	if params.Market != "" {
		query.Set("market", params.Market)
	}
	if params.StartTs != 0 {
		query.Set("startTs", strconv.FormatInt(params.StartTs, 10))
	}
	if params.EndTs != 0 {
		query.Set("endTs", strconv.FormatInt(params.EndTs, 10))
	}
	if params.Fidelity != 0 {
		query.Set("fidelity", strconv.Itoa(params.Fidelity))
	}
	if params.Interval != "" {
		query.Set("interval", string(params.Interval))
	}

	var out []MarketPrice
	err := c.getJSON(ctx, priceHistoryEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// GetMarketTradeEvents returns live market activity events for a condition ID.
func (c *Client) GetMarketTradeEvents(
	ctx context.Context,
	conditionID string,
) ([]MarketTradeEvent, error) {
	var out []MarketTradeEvent
	err := c.getJSON(
		ctx,
		marketTradesEventsEndpoint+conditionID,
		nil,
		polyhttp.AuthNone,
		&out,
	)
	return out, err
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

func getTypedPage[T any](
	ctx context.Context,
	client *Client,
	endpoint, nextCursor string,
) (*Page[T], error) {
	query := url.Values{}
	if nextCursor != "" {
		query.Set("next_cursor", nextCursor)
	}

	var out Page[T]
	err := client.getJSON(ctx, endpoint, query, polyhttp.AuthNone, &out)
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
