package clob

import (
	"context"
	"net/url"
	"strconv"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// GetOk returns the health-check response body.
func (c *Client) GetOk(ctx context.Context) (string, error) {
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

// GetSamplingSimplifiedMarkets returns all simplified markets from the sampling endpoint.
func (c *Client) GetSamplingSimplifiedMarkets(ctx context.Context) ([]SimplifiedMarket, error) {
	cursor := initialCursor
	var markets []SimplifiedMarket

	for cursor != endCursor {
		page, err := c.GetSamplingSimplifiedMarketsPage(ctx, cursor)
		if err != nil {
			return nil, err
		}
		markets = append(markets, page.Data...)

		nextCursor, done := nextPageCursor(cursor, page.NextCursor)
		if done {
			return markets, nil
		}
		cursor = nextCursor
	}

	return markets, nil
}

// GetSamplingSimplifiedMarketsPage returns a typed sampling-simplified markets page.
func (c *Client) GetSamplingSimplifiedMarketsPage(
	ctx context.Context,
	nextCursor string,
) (*Page[SimplifiedMarket], error) {
	return getTypedPage[SimplifiedMarket](ctx, c, samplingSimplifiedMarketsEndpoint, nextCursor)
}

// GetSamplingMarkets returns all markets from the sampling endpoint.
func (c *Client) GetSamplingMarkets(ctx context.Context) ([]Market, error) {
	cursor := initialCursor
	var markets []Market

	for cursor != endCursor {
		page, err := c.GetSamplingMarketsPage(ctx, cursor)
		if err != nil {
			return nil, err
		}
		markets = append(markets, page.Data...)

		nextCursor, done := nextPageCursor(cursor, page.NextCursor)
		if done {
			return markets, nil
		}
		cursor = nextCursor
	}

	return markets, nil
}

// GetSamplingMarketsPage returns a typed sampling markets page.
func (c *Client) GetSamplingMarketsPage(
	ctx context.Context,
	nextCursor string,
) (*Page[Market], error) {
	return getTypedPage[Market](ctx, c, samplingMarketsEndpoint, nextCursor)
}

// GetSimplifiedMarkets returns all simplified markets.
func (c *Client) GetSimplifiedMarkets(ctx context.Context) ([]SimplifiedMarket, error) {
	cursor := initialCursor
	var markets []SimplifiedMarket

	for cursor != endCursor {
		page, err := c.GetSimplifiedMarketsPage(ctx, cursor)
		if err != nil {
			return nil, err
		}
		markets = append(markets, page.Data...)

		nextCursor, done := nextPageCursor(cursor, page.NextCursor)
		if done {
			return markets, nil
		}
		cursor = nextCursor
	}

	return markets, nil
}

// GetSimplifiedMarketsPage returns a typed simplified markets page.
func (c *Client) GetSimplifiedMarketsPage(
	ctx context.Context,
	nextCursor string,
) (*Page[SimplifiedMarket], error) {
	return getTypedPage[SimplifiedMarket](ctx, c, simplifiedMarketsEndpoint, nextCursor)
}

// GetMarkets returns all markets.
func (c *Client) GetMarkets(ctx context.Context) ([]Market, error) {
	cursor := initialCursor
	var markets []Market

	for cursor != endCursor {
		page, err := c.GetMarketsPage(ctx, cursor)
		if err != nil {
			return nil, err
		}
		markets = append(markets, page.Data...)

		nextCursor, done := nextPageCursor(cursor, page.NextCursor)
		if done {
			return markets, nil
		}
		cursor = nextCursor
	}

	return markets, nil
}

// GetMarketsPage returns a typed markets page.
func (c *Client) GetMarketsPage(ctx context.Context, nextCursor string) (*Page[Market], error) {
	return getTypedPage[Market](ctx, c, marketsEndpoint, nextCursor)
}

// GetMarket returns a typed market record for a condition ID.
func (c *Client) GetMarket(ctx context.Context, conditionID string) (*Market, error) {
	var out Market
	err := c.getJSON(ctx, marketEndpoint+conditionID, nil, polyhttp.AuthNone, &out)
	return &out, err
}

// CheckGeoblock returns the Polymarket geoblock status for the current client IP.
func (c *Client) CheckGeoblock(ctx context.Context) (*GeoblockResponse, error) {
	var out GeoblockResponse
	err := c.getGeoblockJSON(ctx, geoblockEndpoint, nil, &out)
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
func (c *Client) GetMidpoint(ctx context.Context, tokenID string) (*MidpointResponse, error) {
	query := url.Values{}
	query.Set("token_id", tokenID)

	var out MidpointResponse
	err := c.getJSON(ctx, midpointEndpoint, query, polyhttp.AuthNone, &out)
	return &out, err
}

// GetMidpoints returns midpoint prices for multiple tokens, keyed by token ID.
func (c *Client) GetMidpoints(ctx context.Context, books []BookParams) (MidpointsResponse, error) {
	var out MidpointsResponse
	err := c.postJSON(ctx, midpointsEndpoint, books, polyhttp.AuthNone, &out)
	return out, err
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

// GetPrices returns prices for multiple tokens, keyed by token ID and side.
func (c *Client) GetPrices(ctx context.Context, books []BookParams) (PricesResponse, error) {
	var out PricesResponse
	err := c.postJSON(ctx, pricesEndpoint, books, polyhttp.AuthNone, &out)
	return out, err
}

// GetAllPrices returns prices for all available tokens, keyed by token ID and side.
func (c *Client) GetAllPrices(ctx context.Context) (PricesResponse, error) {
	var out PricesResponse
	err := c.getJSON(ctx, pricesEndpoint, nil, polyhttp.AuthNone, &out)
	return out, err
}

// GetSpread returns the current spread for a token.
func (c *Client) GetSpread(ctx context.Context, tokenID string) (*SpreadResponse, error) {
	query := url.Values{}
	query.Set("token_id", tokenID)

	var out SpreadResponse
	err := c.getJSON(ctx, spreadEndpoint, query, polyhttp.AuthNone, &out)
	return &out, err
}

// GetSpreads returns spreads for multiple tokens, keyed by token ID.
func (c *Client) GetSpreads(ctx context.Context, books []BookParams) (SpreadsResponse, error) {
	var out SpreadsResponse
	err := c.postJSON(ctx, spreadsEndpoint, books, polyhttp.AuthNone, &out)
	return out, err
}

// GetLastTradePrice returns the last trade price for a token.
func (c *Client) GetLastTradePrice(
	ctx context.Context,
	tokenID string,
) (*LastTradePriceResponse, error) {
	query := url.Values{}
	query.Set("token_id", tokenID)

	var out LastTradePriceResponse
	err := c.getJSON(ctx, lastTradePriceEndpoint, query, polyhttp.AuthNone, &out)
	return &out, err
}

// GetLastTradesPrices returns the last trade prices for multiple tokens.
func (c *Client) GetLastTradesPrices(
	ctx context.Context,
	books []BookParams,
) ([]LastTradesPricesResponse, error) {
	var out []LastTradesPricesResponse
	err := c.postJSON(ctx, lastTradesPricesEndpoint, books, polyhttp.AuthNone, &out)
	return out, err
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

// GetPricesHistory returns the typed price-history series for the supplied filter.
func (c *Client) GetPricesHistory(
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

// GetMarketTradesEvents returns live market activity events for a condition ID.
func (c *Client) GetMarketTradesEvents(
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
