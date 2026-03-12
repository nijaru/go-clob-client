package clob

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// CreateReadonlyAPIKey creates a readonly API key for the authenticated account.
func (c *Client) CreateReadonlyAPIKey(ctx context.Context) (*ReadonlyAPIKeyResponse, error) {
	var out ReadonlyAPIKeyResponse
	err := c.postJSON(ctx, createReadonlyAPIKeyEndpoint, nil, polyhttp.AuthL2, &out)
	return &out, err
}

// GetReadonlyAPIKeys lists readonly API keys for the authenticated account.
func (c *Client) GetReadonlyAPIKeys(ctx context.Context) ([]string, error) {
	var out []string
	err := c.getJSON(ctx, getReadonlyAPIKeysEndpoint, nil, polyhttp.AuthL2, &out)
	return out, err
}

// DeleteReadonlyAPIKey deletes a readonly API key by value.
func (c *Client) DeleteReadonlyAPIKey(ctx context.Context, key string) (bool, error) {
	var out bool
	err := c.deleteJSON(
		ctx,
		deleteReadonlyAPIKeyEndpoint,
		DeleteReadonlyAPIKeyRequest{Key: key},
		polyhttp.AuthL2,
		&out,
	)
	return out, err
}

// ValidateReadonlyAPIKey validates a readonly API key for the given address.
func (c *Client) ValidateReadonlyAPIKey(
	ctx context.Context,
	address string,
	key string,
) (string, error) {
	query := url.Values{}
	query.Set("address", address)
	query.Set("key", key)

	var out string
	err := c.getJSON(ctx, validateReadonlyAPIKeyEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// GetNotifications returns all notifications for the authenticated account.
func (c *Client) GetNotifications(ctx context.Context) ([]Notification, error) {
	query := url.Values{}
	query.Set("signature_type", signatureTypeString(c.signatureType))

	var out []Notification
	err := c.getJSON(ctx, notificationsEndpoint, query, polyhttp.AuthL2, &out)
	return out, err
}

// DeleteNotifications deletes notifications by ID when provided, or all notifications otherwise.
func (c *Client) DeleteNotifications(
	ctx context.Context,
	params DeleteNotificationsParams,
) error {
	query := url.Values{}
	if len(params.IDs) > 0 {
		query.Set("ids", strings.Join(params.IDs, ","))
	}

	return c.deleteJSONQuery(ctx, notificationsEndpoint, query, nil, polyhttp.AuthL2, nil)
}

// GetBalanceAllowance returns the current balance and allowances for the requested asset.
func (c *Client) GetBalanceAllowance(
	ctx context.Context,
	params BalanceAllowanceParams,
) (*BalanceAllowanceResponse, error) {
	query := balanceAllowanceQuery(params, c.signatureType)

	var out BalanceAllowanceResponse
	err := c.getJSON(ctx, balanceAllowanceEndpoint, query, polyhttp.AuthL2, &out)
	return &out, err
}

// UpdateBalanceAllowance triggers a balance-allowance refresh for the requested asset.
func (c *Client) UpdateBalanceAllowance(
	ctx context.Context,
	params BalanceAllowanceParams,
) error {
	query := balanceAllowanceQuery(params, c.signatureType)
	return c.getJSON(ctx, updateBalanceAllowanceEndpoint, query, polyhttp.AuthL2, nil)
}

// IsOrderScoring returns whether a single order is scoring for rewards.
func (c *Client) IsOrderScoring(
	ctx context.Context,
	params OrderScoringParams,
) (*OrderScoringResponse, error) {
	query := url.Values{}
	if params.OrderID != "" {
		query.Set("order_id", params.OrderID)
	}

	var out OrderScoringResponse
	err := c.getJSON(ctx, orderScoringEndpoint, query, polyhttp.AuthL2, &out)
	return &out, err
}

// AreOrdersScoring returns the scoring state for a batch of orders.
func (c *Client) AreOrdersScoring(
	ctx context.Context,
	params OrdersScoringParams,
) (OrdersScoringResponse, error) {
	var out OrdersScoringResponse
	err := c.postJSON(ctx, ordersScoringEndpoint, params.OrderIDs, polyhttp.AuthL2, &out)
	return out, err
}

// CancelMarketOrders cancels orders scoped to a market and/or asset.
func (c *Client) CancelMarketOrders(
	ctx context.Context,
	request CancelMarketOrdersRequest,
) (*CancelOrdersResponse, error) {
	var out CancelOrdersResponse
	err := c.deleteJSON(ctx, cancelMarketOrdersEndpoint, request, polyhttp.AuthL2, &out)
	return &out, err
}

func balanceAllowanceQuery(
	params BalanceAllowanceParams,
	defaultSignatureType SignatureType,
) url.Values {
	query := url.Values{}
	if params.AssetType != "" {
		query.Set("asset_type", string(params.AssetType))
	}
	if params.TokenID != "" {
		query.Set("token_id", params.TokenID)
	}

	signatureType := defaultSignatureType
	if params.SignatureType != nil {
		signatureType = *params.SignatureType
	}
	query.Set("signature_type", signatureTypeString(signatureType))
	return query
}

func signatureTypeString(signatureType SignatureType) string {
	return strconv.Itoa(int(signatureType))
}
