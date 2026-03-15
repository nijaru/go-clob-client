package clob

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// CreateAPIKey creates a new Polymarket API key using L1 authentication.
func (c *Client) CreateAPIKey(ctx context.Context, nonce int64) (*Credentials, error) {
	var raw apiKeyRaw
	err := c.postJSONWithNonce(ctx, createAPIKeyEndpoint, nil, polyhttp.AuthL1, nonce, &raw)
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
func (c *Client) DeriveAPIKey(ctx context.Context, nonce int64) (*Credentials, error) {
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
// Non-API errors (network failures, timeouts) are returned without falling back.
func (c *Client) CreateOrDeriveAPIKey(ctx context.Context, nonce int64) (*Credentials, error) {
	creds, err := c.CreateAPIKey(ctx, nonce)
	if err == nil {
		return creds, nil
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		// If the error indicates the key already exists (400 or 409), fall back to derivation.
		if apiErr.StatusCode == 400 || apiErr.StatusCode == 409 {
			return c.DeriveAPIKey(ctx, nonce)
		}
	}
	return nil, err
}

// GetAPIKeys lists the authenticated account's API keys.
func (c *Client) GetAPIKeys(ctx context.Context) (*APIKeysResponse, error) {
	var out APIKeysResponse
	err := c.getJSON(ctx, getAPIKeysEndpoint, nil, polyhttp.AuthL2, &out)
	return &out, err
}

// DeleteAPIKey deletes the currently authenticated API key.
func (c *Client) DeleteAPIKey(ctx context.Context) error {
	return c.deleteJSON(ctx, deleteAPIKeyEndpoint, nil, polyhttp.AuthL2, nil)
}

// GetClosedOnlyMode returns whether the account is restricted to closed-only mode.
func (c *Client) GetClosedOnlyMode(ctx context.Context) (*BanStatus, error) {
	var out BanStatus
	err := c.getJSON(ctx, closedOnlyEndpoint, nil, polyhttp.AuthL2, &out)
	return &out, err
}

// GetOpenOrders returns all paginated open orders that match the provided filters.
func (c *Client) GetOpenOrders(
	ctx context.Context,
	params OpenOrderParams,
) ([]OpenOrder, error) {
	cursor := initialCursor
	var orders []OpenOrder

	for cursor != endCursor {
		page, err := c.GetOpenOrdersPage(ctx, params, cursor)
		if err != nil {
			return nil, err
		}
		orders = append(orders, page.Data...)

		nextCursor, done := nextPageCursor(cursor, page.NextCursor)
		if done {
			return orders, nil
		}
		cursor = nextCursor
	}

	return orders, nil
}

// GetOpenOrdersPage returns a single page of authenticated open orders.
func (c *Client) GetOpenOrdersPage(
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
func (c *Client) GetOrder(ctx context.Context, orderID string) (*OpenOrder, error) {
	var out OpenOrder
	err := c.getJSON(ctx, orderEndpoint+orderID, nil, polyhttp.AuthL2Builder, &out)
	return &out, err
}

// GetTrades returns all paginated authenticated trades that match the provided filters.
func (c *Client) GetTrades(ctx context.Context, params TradeParams) ([]Trade, error) {
	cursor := initialCursor
	var trades []Trade

	for cursor != endCursor {
		page, err := c.GetTradesPage(ctx, params, cursor)
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

// GetTradesPaginated is an alias for GetTradesPage.
func (c *Client) GetTradesPaginated(
	ctx context.Context,
	params TradeParams,
	nextCursor string,
) (*Page[Trade], error) {
	return c.GetTradesPage(ctx, params, nextCursor)
}

// GetTradesPage returns a single page of authenticated trades.
func (c *Client) GetTradesPage(
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
func (c *Client) PostOrder(
	ctx context.Context,
	request PostOrderRequest,
) (*PostOrderResponse, error) {
	var out PostOrderResponse
	err := c.postJSON(ctx, postOrderEndpoint, request, polyhttp.AuthL2Builder, &out)
	return &out, err
}

// PostOrders posts multiple signed orders in a batch.
func (c *Client) PostOrders(
	ctx context.Context,
	requests []PostOrderRequest,
) ([]PostOrderResponse, error) {
	var out []PostOrderResponse
	err := c.postJSON(ctx, postOrdersEndpoint, requests, polyhttp.AuthL2Builder, &out)
	return out, err
}

// CancelOrder cancels a single order by ID.
func (c *Client) CancelOrder(ctx context.Context, orderID string) (*CancelOrdersResponse, error) {
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
func (c *Client) CancelOrders(
	ctx context.Context,
	orderIDs []string,
) (*CancelOrdersResponse, error) {
	var out CancelOrdersResponse
	err := c.deleteJSON(ctx, cancelOrdersEndpoint, orderIDs, polyhttp.AuthL2Builder, &out)
	return &out, err
}

// CancelAll cancels all open orders for the authenticated account.
func (c *Client) CancelAll(ctx context.Context) (*CancelOrdersResponse, error) {
	var out CancelOrdersResponse
	err := c.deleteJSON(ctx, cancelAllEndpoint, nil, polyhttp.AuthL2Builder, &out)
	return &out, err
}

// CreateBuilderAPIKey creates a new builder API key using L2 authentication.
func (c *Client) CreateBuilderAPIKey(ctx context.Context) (*Credentials, error) {
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
func (c *Client) GetBuilderAPIKeys(ctx context.Context) ([]BuilderAPIKey, error) {
	var out []BuilderAPIKey
	err := c.getJSON(ctx, getBuilderAPIKeysEndpoint, nil, polyhttp.AuthL2, &out)
	return out, err
}

// RevokeBuilderAPIKey revokes the currently configured builder API key.
func (c *Client) RevokeBuilderAPIKey(ctx context.Context) error {
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
func (c *Client) GetBuilderTrades(
	ctx context.Context,
	params TradeParams,
) ([]BuilderTrade, error) {
	cursor := initialCursor
	var trades []BuilderTrade

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
func (c *Client) GetBuilderTradesPage(
	ctx context.Context,
	params TradeParams,
	nextCursor string,
) (*Page[BuilderTrade], error) {
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

// PostHeartbeat posts a builder/session heartbeat for the authenticated account.
func (c *Client) PostHeartbeat(
	ctx context.Context,
	heartbeatID *string,
) (*HeartbeatResponse, error) {
	request := struct {
		HeartbeatID *string `json:"heartbeat_id"`
	}{
		HeartbeatID: heartbeatID,
	}

	var out HeartbeatResponse
	err := c.postJSON(ctx, heartbeatEndpoint, request, polyhttp.AuthL2, &out)
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
	switch {
	case nextCursor == "":
		return "", true
	case nextCursor == currentCursor:
		return "", true
	case nextCursor == endCursor:
		return endCursor, false
	default:
		return nextCursor, false
	}
}
