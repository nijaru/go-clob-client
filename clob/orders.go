package clob

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

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

func (c *Client) CreateOrDeriveAPIKey(ctx context.Context, nonce int64) (*Credentials, error) {
	creds, err := c.CreateAPIKey(ctx, nonce)
	if err == nil {
		return creds, nil
	}
	return c.DeriveAPIKey(ctx, nonce)
}

func (c *Client) GetAPIKeys(ctx context.Context) (*APIKeysResponse, error) {
	var out APIKeysResponse
	err := c.getJSON(ctx, getAPIKeysEndpoint, nil, polyhttp.AuthL2, &out)
	return &out, err
}

func (c *Client) DeleteAPIKey(ctx context.Context) error {
	return c.deleteJSON(ctx, deleteAPIKeyEndpoint, nil, polyhttp.AuthL2, nil)
}

func (c *Client) GetClosedOnly(ctx context.Context) (*BanStatus, error) {
	var out BanStatus
	err := c.getJSON(ctx, closedOnlyEndpoint, nil, polyhttp.AuthL2, &out)
	return &out, err
}

func (c *Client) GetOpenOrders(
	ctx context.Context,
	params OpenOrderParams,
) (json.RawMessage, error) {
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

	var out json.RawMessage
	err := c.getJSON(ctx, openOrdersEndpoint, query, polyhttp.AuthL2, &out)
	return out, err
}

func (c *Client) GetOrder(ctx context.Context, orderID string) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.getJSON(ctx, orderEndpoint+orderID, nil, polyhttp.AuthL2, &out)
	return out, err
}

func (c *Client) GetTrades(ctx context.Context, params TradeParams) (json.RawMessage, error) {
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

	var out json.RawMessage
	err := c.getJSON(ctx, tradesEndpoint, query, polyhttp.AuthL2, &out)
	return out, err
}

func (c *Client) PostOrder(ctx context.Context, request PostOrderRequest) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.postJSON(ctx, postOrderEndpoint, request, polyhttp.AuthL2, &out)
	return out, err
}

func (c *Client) PostOrders(
	ctx context.Context,
	requests []PostOrderRequest,
) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.postJSON(ctx, postOrdersEndpoint, requests, polyhttp.AuthL2, &out)
	return out, err
}

func (c *Client) CancelOrder(ctx context.Context, orderID string) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.deleteJSON(
		ctx,
		cancelOrderEndpoint,
		OrderPayload{OrderID: orderID},
		polyhttp.AuthL2,
		&out,
	)
	return out, err
}

func (c *Client) CancelOrders(ctx context.Context, orderIDs []string) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.deleteJSON(ctx, cancelOrdersEndpoint, orderIDs, polyhttp.AuthL2, &out)
	return out, err
}

func (c *Client) CancelAll(ctx context.Context) (json.RawMessage, error) {
	var out json.RawMessage
	err := c.deleteJSON(ctx, cancelAllEndpoint, nil, polyhttp.AuthL2, &out)
	return out, err
}
