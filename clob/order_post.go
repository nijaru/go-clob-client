package clob

import (
	"context"
	"fmt"
)

// CreateAndPostOrder builds, signs, and posts a limit order in one step.
func (c *AuthenticatedClient) CreateAndPostOrder(
	ctx context.Context,
	userOrder OrderArgs,
	options *CreateOrderOptions,
	orderType OrderType,
	postOnly bool,
) (*PostOrderResponse, error) {
	order, err := c.CreateOrder(ctx, userOrder, options)
	if err != nil {
		return nil, err
	}

	request, err := c.BuildPostOrderRequest(*order, orderType, postOnly, userOrder.DeferExec)
	if err != nil {
		return nil, err
	}

	return c.PostOrder(ctx, request)
}

// CreateAndPostMarketOrder builds, signs, and posts a market order in one step.
func (c *AuthenticatedClient) CreateAndPostMarketOrder(
	ctx context.Context,
	userOrder MarketOrderArgs,
	options *CreateOrderOptions,
	orderType OrderType,
) (*PostOrderResponse, error) {
	// Resolve the effective order type once here; pass it down to avoid double normalization.
	if orderType == "" {
		orderType = userOrder.OrderType
	}
	if orderType == "" {
		orderType = OrderTypeFOK
	}
	if orderType != OrderTypeFOK && orderType != OrderTypeFAK {
		return nil, fmt.Errorf("market orders only support FOK or FAK order types")
	}
	userOrder.OrderType = orderType

	order, err := c.CreateMarketOrder(ctx, userOrder, options)
	if err != nil {
		return nil, err
	}

	request, err := c.BuildPostOrderRequest(*order, orderType, false, userOrder.DeferExec)
	if err != nil {
		return nil, err
	}

	return c.PostOrder(ctx, request)
}

// BuildAndPostOrder builds, signs, and posts a limit order. When the server
// rejects the order with order_version_mismatch and its protocol version has
// changed, the order is rebuilt, re-signed against the new exchange, and
// posted once more (Rust build_sign_and_post semantics).
func (c *AuthenticatedClient) BuildAndPostOrder(
	ctx context.Context,
	userOrder OrderArgs,
	options *CreateOrderOptions,
	orderType OrderType,
	postOnly bool,
) (*PostOrderResponse, error) {
	beforeVersion := c.orderBuildServerVersion(ctx, userOrder.TokenID, userOrder.PositionID)

	post := func() (*PostOrderResponse, error) {
		order, err := c.CreateOrder(ctx, userOrder, options)
		if err != nil {
			return nil, err
		}
		request, err := c.BuildPostOrderRequest(*order, orderType, postOnly, userOrder.DeferExec)
		if err != nil {
			return nil, err
		}
		return c.PostOrder(ctx, request)
	}

	response, err := post()
	if isOrderVersionMismatch(err) {
		c.invalidateServerVersion()
		if afterVersion := c.orderBuildServerVersion(
			ctx,
			userOrder.TokenID,
			userOrder.PositionID,
		); afterVersion != beforeVersion {
			return post()
		}
	}
	return response, err
}

// BuildAndPostMarketOrder builds, signs, and posts a market order. When the
// server rejects the order with order_version_mismatch and its protocol
// version has changed, the order is rebuilt, re-signed against the new
// exchange, and posted once more (Rust build_sign_and_post semantics).
func (c *AuthenticatedClient) BuildAndPostMarketOrder(
	ctx context.Context,
	userOrder MarketOrderArgs,
	options *CreateOrderOptions,
	orderType OrderType,
) (*PostOrderResponse, error) {
	if orderType == "" {
		orderType = userOrder.OrderType
	}
	if orderType == "" {
		orderType = OrderTypeFOK
	}
	if orderType != OrderTypeFOK && orderType != OrderTypeFAK {
		return nil, fmt.Errorf("market orders only support FOK or FAK order types")
	}
	userOrder.OrderType = orderType

	beforeVersion := c.orderBuildServerVersion(ctx, userOrder.TokenID, userOrder.PositionID)

	post := func() (*PostOrderResponse, error) {
		order, err := c.CreateMarketOrder(ctx, userOrder, options)
		if err != nil {
			return nil, err
		}
		request, err := c.BuildPostOrderRequest(*order, orderType, false, userOrder.DeferExec)
		if err != nil {
			return nil, err
		}
		return c.PostOrder(ctx, request)
	}

	response, err := post()
	if isOrderVersionMismatch(err) {
		c.invalidateServerVersion()
		if afterVersion := c.orderBuildServerVersion(
			ctx,
			userOrder.TokenID,
			userOrder.PositionID,
		); afterVersion != beforeVersion {
			return post()
		}
	}
	return response, err
}

// orderBuildServerVersion returns the server protocol version an order build
// will sign against, or 0 when it cannot be resolved. Position-backed orders
// always sign against Exchange V3, so they never participate in version
// mismatch recovery.
func (c *SignerClient) orderBuildServerVersion(
	ctx context.Context,
	tokenID, positionID string,
) uint32 {
	if positionID != "" {
		return 0
	}
	if tokenID == "" {
		return 0
	}
	version, err := c.resolveServerVersion(ctx, false)
	if err != nil {
		return 0
	}
	return version
}

// BuildPostOrderRequest wraps a signed order in the authenticated post-order payload.
func (c *AuthenticatedClient) BuildPostOrderRequest(
	order SignedOrder,
	orderType OrderType,
	postOnly bool,
	deferExec bool,
) (PostOrderRequest, error) {
	creds := c.credentials()
	if creds == nil {
		return PostOrderRequest{}, fmt.Errorf("build post order request requires API credentials")
	}

	if orderType == "" {
		orderType = OrderTypeGTC
	}
	if order.Expiration != "0" && orderType != OrderTypeGTD {
		return PostOrderRequest{}, fmt.Errorf("only GTD orders may have a non-zero expiration")
	}

	if postOnly && orderType != OrderTypeGTC && orderType != OrderTypeGTD {
		return PostOrderRequest{}, fmt.Errorf(
			"postOnly is only supported for GTC and GTD orders (2026 standard)",
		)
	}

	return PostOrderRequest{
		Order:     order,
		Owner:     creds.Key,
		OrderType: orderType,
		PostOnly:  postOnly,
		DeferExec: deferExec,
	}, nil
}

// PostOrdersBatchLimit is the maximum number of orders allowed in a single batch.
const PostOrdersBatchLimit = 15
