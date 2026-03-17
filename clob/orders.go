package clob

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"net/url"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// CreateAPIKey creates a new Polymarket API key using L1 authentication.
func (c *SignerClient) CreateAPIKey(ctx context.Context, nonce int64) (*Credentials, error) {
	var raw apiKeyRaw
	err := c.getJSONWithNonce(ctx, createAPIKeyEndpoint, nil, polyhttp.AuthL1, nonce, &raw)
	if err != nil {
		return nil, err
	}
	return &Credentials{
		Key:        raw.APIKey,
		Secret:     raw.Secret,
		Passphrase: raw.Passphrase,
	}, nil
}

// DeriveAPIKey derives the existing Polymarket API key for the signer using L1 authentication.
func (c *SignerClient) DeriveAPIKey(ctx context.Context, nonce int64) (*Credentials, error) {
	var raw apiKeyRaw
	err := c.getJSONWithNonce(ctx, deriveAPIKeyEndpoint, nil, polyhttp.AuthL1, nonce, &raw)
	if err != nil {
		return nil, err
	}
	return &Credentials{
		Key:        raw.APIKey,
		Secret:     raw.Secret,
		Passphrase: raw.Passphrase,
	}, nil
}

// CreateOrDeriveAPIKey creates a new API key or derives the existing one when the server
// rejects creation with an API error indicating the key already exists.
func (c *SignerClient) CreateOrDeriveAPIKey(ctx context.Context, nonce int64) (*Credentials, error) {
	creds, err := c.CreateAPIKey(ctx, nonce)
	if err == nil {
		return creds, nil
	}

	if apiErr, ok := errors.AsType[*APIError](err); ok {
		if apiErr.StatusCode == 400 || apiErr.StatusCode == 409 {
			return c.DeriveAPIKey(ctx, nonce)
		}
	}
	return nil, err
}

// GetAPIKeys lists the authenticated account's API keys.
func (c *AuthenticatedClient) GetAPIKeys(ctx context.Context) (*APIKeysResponse, error) {
	var out APIKeysResponse
	err := c.getJSON(ctx, getAPIKeysEndpoint, nil, polyhttp.AuthL2, &out)
	return &out, err
}

// DeleteAPIKey deletes the currently authenticated API key.
func (c *AuthenticatedClient) DeleteAPIKey(ctx context.Context) error {
	return c.deleteJSON(ctx, deleteAPIKeyEndpoint, nil, polyhttp.AuthL2, nil)
}

// GetClosedOnlyMode returns whether the account is restricted to closed-only mode.
func (c *AuthenticatedClient) GetClosedOnlyMode(ctx context.Context) (*BanStatus, error) {
	var out BanStatus
	err := c.getJSON(ctx, closedOnlyEndpoint, nil, polyhttp.AuthL2, &out)
	return &out, err
}

// GetOpenOrders returns all paginated open orders that match the provided filters.
func (c *AuthenticatedClient) GetOpenOrders(
	ctx context.Context,
	params OpenOrderParams,
) ([]OpenOrder, error) {
	orders := make([]OpenOrder, 0, 64)
	for order, err := range c.IterOpenOrders(ctx, params) {
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, nil
}

// IterOpenOrders returns an iterator over open orders matching the provided filters.
func (c *AuthenticatedClient) IterOpenOrders(
	ctx context.Context,
	params OpenOrderParams,
) iter.Seq2[OpenOrder, error] {
	return func(yield func(OpenOrder, error) bool) {
		cursor := initialCursor
		for cursor != endCursor {
			page, err := c.GetOpenOrdersPage(ctx, params, cursor)
			if err != nil {
				yield(OpenOrder{}, err)
				return
			}
			for _, order := range page.Data {
				if !yield(order, nil) {
					return
				}
			}
			nextCursor, done := nextPageCursor(cursor, page.NextCursor)
			if done {
				return
			}
			cursor = nextCursor
		}
	}
}

// GetOpenOrdersPage returns a single page of authenticated open orders.
func (c *AuthenticatedClient) GetOpenOrdersPage(
	ctx context.Context,
	params OpenOrderParams,
	nextCursor string,
) (*Page[OpenOrder], error) {
	query := openOrdersQuery(params, normalizedCursor(nextCursor))

	var out Page[OpenOrder]
	err := c.getJSON(ctx, openOrdersEndpoint, query, polyhttp.AuthL2Builder, &out)
	return &out, err
}

// GetOrder fetches a single authenticated open order by ID.
func (c *AuthenticatedClient) GetOrder(ctx context.Context, orderID string) (*OpenOrder, error) {
	var out OpenOrder
	err := c.getJSON(ctx, orderEndpoint+orderID, nil, polyhttp.AuthL2Builder, &out)
	return &out, err
}

// GetTrades returns all paginated authenticated trades that match the provided filters.
func (c *AuthenticatedClient) GetTrades(ctx context.Context, params TradeParams) ([]Trade, error) {
	trades := make([]Trade, 0, 64)
	for trade, err := range c.IterTrades(ctx, params) {
		if err != nil {
			return nil, err
		}
		trades = append(trades, trade)
	}
	return trades, nil
}

// IterTrades returns an iterator over authenticated trades matching the provided filters.
func (c *AuthenticatedClient) IterTrades(
	ctx context.Context,
	params TradeParams,
) iter.Seq2[Trade, error] {
	return func(yield func(Trade, error) bool) {
		cursor := initialCursor
		for cursor != endCursor {
			page, err := c.GetTradesPage(ctx, params, cursor)
			if err != nil {
				yield(Trade{}, err)
				return
			}
			for _, trade := range page.Data {
				if !yield(trade, nil) {
					return
				}
			}
			nextCursor, done := nextPageCursor(cursor, page.NextCursor)
			if done {
				return
			}
			cursor = nextCursor
		}
	}
}

// GetTradesPaginated is an alias for GetTradesPage.
func (c *AuthenticatedClient) GetTradesPaginated(
	ctx context.Context,
	params TradeParams,
	nextCursor string,
) (*Page[Trade], error) {
	return c.GetTradesPage(ctx, params, nextCursor)
}

// GetTradesPage returns a single page of authenticated trades.
func (c *AuthenticatedClient) GetTradesPage(
	ctx context.Context,
	params TradeParams,
	nextCursor string,
) (*Page[Trade], error) {
	query := tradesQuery(params, normalizedCursor(nextCursor))

	var out Page[Trade]
	err := c.getJSON(ctx, tradesEndpoint, query, polyhttp.AuthL2Builder, &out)
	return &out, err
}

// PostOrder posts a single signed order.
func (c *AuthenticatedClient) PostOrder(
	ctx context.Context,
	request PostOrderRequest,
) (*PostOrderResponse, error) {
	var out PostOrderResponse
	err := c.postJSON(ctx, postOrderEndpoint, request, polyhttp.AuthL2Builder, &out)
	return &out, err
}

// PostOrders posts multiple signed orders in a batch.
func (c *AuthenticatedClient) PostOrders(
	ctx context.Context,
	requests []PostOrderRequest,
) ([]PostOrderResponse, error) {
	if len(requests) > PostOrdersBatchLimit {
		return nil, fmt.Errorf("batch size exceeds limit of %d (2026 standard)", PostOrdersBatchLimit)
	}
	var out []PostOrderResponse
	err := c.postJSON(ctx, postOrdersEndpoint, requests, polyhttp.AuthL2Builder, &out)
	return out, err
}

// CancelOrder cancels a single order by ID.
func (c *AuthenticatedClient) CancelOrder(ctx context.Context, orderID string) (*CancelOrdersResponse, error) {
	var out CancelOrdersResponse
	err := c.deleteJSON(
		ctx,
		cancelOrderEndpoint,
		OrderPayload{OrderID: orderID},
		polyhttp.AuthL2Builder,
		&out,
	)
	return &out, err
}

// CancelOrders cancels multiple orders in a single request.
func (c *AuthenticatedClient) CancelOrders(
	ctx context.Context,
	orderIDs []string,
) (*CancelOrdersResponse, error) {
	var out CancelOrdersResponse
	err := c.deleteJSON(ctx, cancelOrdersEndpoint, orderIDs, polyhttp.AuthL2Builder, &out)
	return &out, err
}

// CancelAll cancels all open orders for the authenticated account.
func (c *AuthenticatedClient) CancelAll(ctx context.Context) (*CancelOrdersResponse, error) {
	var out CancelOrdersResponse
	err := c.deleteJSON(ctx, cancelAllEndpoint, nil, polyhttp.AuthL2Builder, &out)
	return &out, err
}

// CreateBuilderAPIKey creates a new builder API key using L2 authentication.
func (c *AuthenticatedClient) CreateBuilderAPIKey(ctx context.Context) (*Credentials, error) {
	var raw apiKeyRaw
	err := c.postJSON(ctx, createBuilderAPIKeyEndpoint, nil, polyhttp.AuthL2, &raw)
	if err != nil {
		return nil, err
	}
	return &Credentials{
		Key:        raw.APIKey,
		Secret:     raw.Secret,
		Passphrase: raw.Passphrase,
	}, nil
}

// GetBuilderAPIKeys lists builder API keys for the authenticated account.
func (c *AuthenticatedClient) GetBuilderAPIKeys(ctx context.Context) ([]BuilderAPIKey, error) {
	var out []BuilderAPIKey
	err := c.getJSON(ctx, getBuilderAPIKeysEndpoint, nil, polyhttp.AuthL2, &out)
	return out, err
}

// RevokeBuilderAPIKey revokes the currently configured builder API key.
func (c *AuthenticatedClient) RevokeBuilderAPIKey(ctx context.Context) error {
	if c.builderAuth == nil {
		return fmt.Errorf("builder auth not configured")
	}
	headers, err := c.builderOnlyHeaders(ctx, http.MethodDelete, revokeBuilderAPIKeyEndpoint, nil)
	if err != nil {
		return err
	}
	return c.doJSON(
		ctx,
		http.MethodDelete,
		revokeBuilderAPIKeyEndpoint,
		nil,
		nil,
		polyhttp.AuthNone,
		nil,
		headers,
	)
}

// GetBuilderTrades returns all paginated builder trades that match the provided filters.
func (c *AuthenticatedClient) GetBuilderTrades(
	ctx context.Context,
	params TradeParams,
) ([]BuilderTrade, error) {
	cursor := initialCursor
	trades := make([]BuilderTrade, 0, 64)

	for cursor != endCursor {
		page, err := c.GetBuilderTradesPage(ctx, params, cursor)
		if err != nil {
			return nil, err
		}
		trades = append(trades, page.Data...)

		nextCursor, done := nextPageCursor(cursor, page.NextCursor)
		if done {
			return trades, nil
		}
		cursor = nextCursor
	}

	return trades, nil
}

// GetBuilderTradesPage returns a single page of builder trades.
func (c *AuthenticatedClient) GetBuilderTradesPage(
	ctx context.Context,
	params TradeParams,
	nextCursor string,
) (*Page[BuilderTrade], error) {
	if c.builderAuth == nil {
		return nil, fmt.Errorf("builder auth not configured")
	}
	headers, err := c.builderOnlyHeaders(ctx, http.MethodGet, builderTradesEndpoint, nil)
	if err != nil {
		return nil, err
	}

	query := tradesQuery(params, normalizedCursor(nextCursor))

	var out Page[BuilderTrade]
	err = c.doJSON(
		ctx,
		http.MethodGet,
		builderTradesEndpoint,
		query,
		nil,
		polyhttp.AuthNone,
		&out,
		headers,
	)
	return &out, err
}

func openOrdersQuery(params OpenOrderParams, nextCursor string) url.Values {
	query := url.Values{}
	if params.ID != "" {
		query.Set("id", params.ID)
	}
	if params.Market != "" {
		query.Set("market", params.Market)
	}
	if params.AssetID != "" {
		query.Set("asset_id", params.AssetID)
	}
	if nextCursor != "" {
		query.Set("next_cursor", nextCursor)
	}
	return query
}

func tradesQuery(params TradeParams, nextCursor string) url.Values {
	query := url.Values{}
	if params.ID != "" {
		query.Set("id", params.ID)
	}
	if params.MakerAddress != "" {
		query.Set("maker_address", params.MakerAddress)
	}
	if params.Market != "" {
		query.Set("market", params.Market)
	}
	if params.AssetID != "" {
		query.Set("asset_id", params.AssetID)
	}
	if params.Before != "" {
		query.Set("before", params.Before)
	}
	if params.After != "" {
		query.Set("after", params.After)
	}
	if nextCursor != "" {
		query.Set("next_cursor", nextCursor)
	}
	return query
}

func normalizedCursor(nextCursor string) string {
	if nextCursor == "" {
		return initialCursor
	}
	return nextCursor
}

func nextPageCursor(currentCursor, nextCursor string) (string, bool) {
	switch nextCursor {
	case "":
		return "", true
	case currentCursor:
		return "", true
	case endCursor:
		return endCursor, false
	default:
		return nextCursor, false
	}
}
