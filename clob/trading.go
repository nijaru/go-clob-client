package clob

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strconv"

	ethmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/nijaru/go-clob-client/internal/polyauth"
	"github.com/shopspring/decimal"
)

const (
	protocolName         = "Polymarket CTF Exchange"
	protocolVersion      = "1"
	collateralTokenScale = int32(6)
)

var roundingConfig = map[TickSize]roundConfig{
	TickSizeTenth:       {Price: 1, Size: 2, Amount: 3},
	TickSizeHundredth:   {Price: 2, Size: 2, Amount: 4},
	TickSizeThousandth:  {Price: 3, Size: 2, Amount: 5},
	TickSizeTenThousand: {Price: 4, Size: 2, Amount: 6},
}

// Bool returns a pointer to the provided bool.
func Bool(value bool) *bool {
	return &value
}

// CreateOrder builds and signs a limit order.
func (c *Client) CreateOrder(
	ctx context.Context,
	userOrder OrderArgs,
	options *CreateOrderOptions,
) (*SignedOrder, error) {
	if c.signer == nil {
		return nil, fmt.Errorf("create order requires a private key")
	}
	if err := validateLimitOrderArgs(userOrder); err != nil {
		return nil, err
	}

	tickSize, err := c.resolveTickSize(ctx, userOrder.TokenID, options)
	if err != nil {
		return nil, err
	}

	feeRateBps, err := c.resolveFeeRateBps(ctx, userOrder.TokenID, userOrder.FeeRateBps)
	if err != nil {
		return nil, err
	}
	userOrder.FeeRateBps = feeRateBps

	if err := validatePrice(userOrder.Price, tickSize); err != nil {
		return nil, err
	}

	negRisk, err := c.resolveNegRisk(ctx, userOrder.TokenID, options)
	if err != nil {
		return nil, err
	}

	return c.buildSignedLimitOrder(userOrder, CreateOrderOptions{
		TickSize: tickSize,
		NegRisk:  Bool(negRisk),
	})
}

// CreateMarketOrder builds and signs a market order.
func (c *Client) CreateMarketOrder(
	ctx context.Context,
	userOrder MarketOrderArgs,
	options *CreateOrderOptions,
) (*SignedOrder, error) {
	if c.signer == nil {
		return nil, fmt.Errorf("create market order requires a private key")
	}
	if err := validateMarketOrderArgs(userOrder); err != nil {
		return nil, err
	}

	tickSize, err := c.resolveTickSize(ctx, userOrder.TokenID, options)
	if err != nil {
		return nil, err
	}

	feeRateBps, err := c.resolveFeeRateBps(ctx, userOrder.TokenID, userOrder.FeeRateBps)
	if err != nil {
		return nil, err
	}
	userOrder.FeeRateBps = feeRateBps

	if userOrder.OrderType == "" {
		userOrder.OrderType = OrderTypeFOK
	}
	if userOrder.OrderType != OrderTypeFOK && userOrder.OrderType != OrderTypeFAK {
		return nil, fmt.Errorf("market orders only support FOK or FAK order types")
	}

	if userOrder.Price == 0 {
		price, err := c.CalculateMarketPrice(
			ctx,
			userOrder.TokenID,
			userOrder.Side,
			userOrder.Amount,
			userOrder.OrderType,
		)
		if err != nil {
			return nil, err
		}
		userOrder.Price = price
	}

	if err := validatePrice(userOrder.Price, tickSize); err != nil {
		return nil, err
	}

	negRisk, err := c.resolveNegRisk(ctx, userOrder.TokenID, options)
	if err != nil {
		return nil, err
	}

	return c.buildSignedMarketOrder(userOrder, CreateOrderOptions{
		TickSize: tickSize,
		NegRisk:  Bool(negRisk),
	})
}

// CreateAndPostOrder builds, signs, and posts a limit order in one step.
func (c *Client) CreateAndPostOrder(
	ctx context.Context,
	userOrder OrderArgs,
	options *CreateOrderOptions,
	orderType OrderType,
	deferExec bool,
	postOnly bool,
) (*PostOrderResponse, error) {
	order, err := c.CreateOrder(ctx, userOrder, options)
	if err != nil {
		return nil, err
	}

	request, err := c.BuildPostOrderRequest(*order, orderType, deferExec, postOnly)
	if err != nil {
		return nil, err
	}

	return c.PostOrder(ctx, request)
}

// CreateAndPostMarketOrder builds, signs, and posts a market order in one step.
func (c *Client) CreateAndPostMarketOrder(
	ctx context.Context,
	userOrder MarketOrderArgs,
	options *CreateOrderOptions,
	orderType OrderType,
	deferExec bool,
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

	order, err := c.CreateMarketOrder(ctx, userOrder, options)
	if err != nil {
		return nil, err
	}

	request, err := c.BuildPostOrderRequest(*order, orderType, deferExec, false)
	if err != nil {
		return nil, err
	}

	return c.PostOrder(ctx, request)
}

// BuildPostOrderRequest wraps a signed order in the authenticated post-order payload.
func (c *Client) BuildPostOrderRequest(
	order SignedOrder,
	orderType OrderType,
	deferExec bool,
	postOnly bool,
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
		return PostOrderRequest{}, fmt.Errorf("postOnly is only supported for GTC and GTD orders")
	}

	return PostOrderRequest{
		Order:     order,
		Owner:     creds.Key,
		OrderType: orderType,
		DeferExec: deferExec,
		PostOnly:  postOnly,
	}, nil
}

// CalculateMarketPrice derives a marketable price from the current order book.
func (c *Client) CalculateMarketPrice(
	ctx context.Context,
	tokenID string,
	side Side,
	amount float64,
	orderType OrderType,
) (float64, error) {
	if orderType == "" {
		orderType = OrderTypeFOK
	}

	book, err := c.GetOrderBook(ctx, tokenID)
	if err != nil {
		return 0, err
	}

	target := decimal.NewFromFloat(amount)
	if target.LessThanOrEqual(decimal.Zero) {
		return 0, fmt.Errorf("amount must be positive")
	}

	var levels []OrderSummary
	switch side {
	case SideBuy:
		levels = book.Asks
	case SideSell:
		levels = book.Bids
	default:
		return 0, fmt.Errorf("invalid side %q", side)
	}

	if len(levels) == 0 {
		return 0, fmt.Errorf("no opposing orders for token %s", tokenID)
	}

	sum := decimal.Zero
	// The Polymarket API returns Bids sorted ascending (lowest to highest price)
	// and Asks sorted descending (highest to lowest price). In both cases,
	// the "top of the book" (best price) is at the end of the array. Therefore,
	// iterating backwards always starts at the most competitive price.
	for i := len(levels) - 1; i >= 0; i-- {
		level := levels[i]
		size, err := decimal.NewFromString(level.Size)
		if err != nil {
			return 0, fmt.Errorf("parse orderbook size: %w", err)
		}
		price, err := decimal.NewFromString(level.Price)
		if err != nil {
			return 0, fmt.Errorf("parse orderbook price: %w", err)
		}

		if side == SideBuy {
			sum = sum.Add(size.Mul(price))
		} else {
			sum = sum.Add(size)
		}

		if sum.GreaterThanOrEqual(target) {
			value, _ := price.Float64()
			return value, nil
		}
	}

	if orderType == OrderTypeFOK {
		return 0, fmt.Errorf("insufficient liquidity to fill amount %.6f", amount)
	}

	firstPrice, err := decimal.NewFromString(levels[0].Price)
	if err != nil {
		return 0, fmt.Errorf("parse fallback price: %w", err)
	}
	value, _ := firstPrice.Float64()
	return value, nil
}

func (c *Client) buildSignedLimitOrder(
	userOrder OrderArgs,
	options CreateOrderOptions,
) (*SignedOrder, error) {
	roundConfig, ok := roundingConfig[options.TickSize]
	if !ok {
		return nil, fmt.Errorf("unsupported tick size %q", options.TickSize)
	}

	price := decimal.NewFromFloat(userOrder.Price)
	size := decimal.NewFromFloat(userOrder.Size)
	rawPrice := roundNormal(price, roundConfig.Price)

	var rawMakerAmount decimal.Decimal
	var rawTakerAmount decimal.Decimal

	switch userOrder.Side {
	case SideBuy:
		rawTakerAmount = roundDown(size, roundConfig.Size)
		rawMakerAmount = rawTakerAmount.Mul(rawPrice)
		if decimalPlaces(rawMakerAmount) > roundConfig.Amount {
			rawMakerAmount = roundUp(rawMakerAmount, roundConfig.Amount+4)
			if decimalPlaces(rawMakerAmount) > roundConfig.Amount {
				rawMakerAmount = roundDown(rawMakerAmount, roundConfig.Amount)
			}
		}
	case SideSell:
		rawMakerAmount = roundDown(size, roundConfig.Size)
		rawTakerAmount = rawMakerAmount.Mul(rawPrice)
		if decimalPlaces(rawTakerAmount) > roundConfig.Amount {
			rawTakerAmount = roundUp(rawTakerAmount, roundConfig.Amount+4)
			if decimalPlaces(rawTakerAmount) > roundConfig.Amount {
				rawTakerAmount = roundDown(rawTakerAmount, roundConfig.Amount)
			}
		}
	default:
		return nil, fmt.Errorf("invalid side %q", userOrder.Side)
	}

	return c.signOrder(orderBuildInput{
		TokenID:       userOrder.TokenID,
		MakerAmount:   toTokenDecimals(rawMakerAmount),
		TakerAmount:   toTokenDecimals(rawTakerAmount),
		Side:          userOrder.Side,
		FeeRateBps:    userOrder.FeeRateBps,
		Nonce:         userOrder.Nonce,
		Expiration:    userOrder.Expiration,
		Taker:         normalizeTaker(userOrder.Taker),
		NegRisk:       derefBool(options.NegRisk),
		SignatureType: c.signatureType,
	})
}

func (c *Client) buildSignedMarketOrder(
	userOrder MarketOrderArgs,
	options CreateOrderOptions,
) (*SignedOrder, error) {
	roundConfig, ok := roundingConfig[options.TickSize]
	if !ok {
		return nil, fmt.Errorf("unsupported tick size %q", options.TickSize)
	}

	price := roundDown(decimal.NewFromFloat(userOrder.Price), roundConfig.Price)
	amount := decimal.NewFromFloat(userOrder.Amount)

	var rawMakerAmount decimal.Decimal
	var rawTakerAmount decimal.Decimal

	switch userOrder.Side {
	case SideBuy:
		rawMakerAmount = roundDown(amount, roundConfig.Size)
		rawTakerAmount = rawMakerAmount.Div(price)
		if decimalPlaces(rawTakerAmount) > roundConfig.Amount {
			rawTakerAmount = roundUp(rawTakerAmount, roundConfig.Amount+4)
			if decimalPlaces(rawTakerAmount) > roundConfig.Amount {
				rawTakerAmount = roundDown(rawTakerAmount, roundConfig.Amount)
			}
		}
	case SideSell:
		rawMakerAmount = roundDown(amount, roundConfig.Size)
		rawTakerAmount = rawMakerAmount.Mul(price)
		if decimalPlaces(rawTakerAmount) > roundConfig.Amount {
			rawTakerAmount = roundUp(rawTakerAmount, roundConfig.Amount+4)
			if decimalPlaces(rawTakerAmount) > roundConfig.Amount {
				rawTakerAmount = roundDown(rawTakerAmount, roundConfig.Amount)
			}
		}
	default:
		return nil, fmt.Errorf("invalid side %q", userOrder.Side)
	}

	return c.signOrder(orderBuildInput{
		TokenID:       userOrder.TokenID,
		MakerAmount:   toTokenDecimals(rawMakerAmount),
		TakerAmount:   toTokenDecimals(rawTakerAmount),
		Side:          userOrder.Side,
		FeeRateBps:    userOrder.FeeRateBps,
		Nonce:         userOrder.Nonce,
		Expiration:    0,
		Taker:         normalizeTaker(userOrder.Taker),
		NegRisk:       derefBool(options.NegRisk),
		SignatureType: c.signatureType,
	})
}

type orderBuildInput struct {
	TokenID       string
	MakerAmount   decimal.Decimal
	TakerAmount   decimal.Decimal
	Side          Side
	FeeRateBps    int64
	Nonce         uint64
	Expiration    uint64
	Taker         string
	NegRisk       bool
	SignatureType SignatureType
}

func (c *Client) signOrder(input orderBuildInput) (*SignedOrder, error) {
	contracts, err := getContractConfig(c.chainID)
	if err != nil {
		return nil, err
	}

	verifyingContract := contracts.Exchange
	if input.NegRisk {
		verifyingContract = contracts.NegRiskExchange
	}

	signerAddress := c.signer.Address().Hex()
	maker := signerAddress
	if c.funderAddress != "" {
		maker = c.funderAddress
	}

	order := SignedOrder{
		Maker:         maker,
		Signer:        signerAddress,
		Taker:         input.Taker,
		TokenID:       input.TokenID,
		MakerAmount:   input.MakerAmount.StringFixed(0),
		TakerAmount:   input.TakerAmount.StringFixed(0),
		Expiration:    strconv.FormatUint(input.Expiration, 10),
		Nonce:         strconv.FormatUint(input.Nonce, 10),
		FeeRateBps:    strconv.FormatInt(input.FeeRateBps, 10),
		Side:          input.Side,
		SignatureType: input.SignatureType,
	}

	salt, err := c.saltGenerator()
	if err != nil {
		return nil, fmt.Errorf("generate order salt: %w", err)
	}
	order.Salt = strconv.FormatUint(salt, 10)

	signature, err := polyauth.SignTypedData(
		c.signer,
		buildOrderTypedData(c.chainID, verifyingContract, order),
	)
	if err != nil {
		return nil, err
	}
	order.Signature = signature

	return &order, nil
}

func buildOrderTypedData(
	chainID int64,
	verifyingContract string,
	order SignedOrder,
) apitypes.TypedData {
	return apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": {
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
			"Order": {
				{Name: "salt", Type: "uint256"},
				{Name: "maker", Type: "address"},
				{Name: "signer", Type: "address"},
				{Name: "taker", Type: "address"},
				{Name: "tokenId", Type: "uint256"},
				{Name: "makerAmount", Type: "uint256"},
				{Name: "takerAmount", Type: "uint256"},
				{Name: "expiration", Type: "uint256"},
				{Name: "nonce", Type: "uint256"},
				{Name: "feeRateBps", Type: "uint256"},
				{Name: "side", Type: "uint8"},
				{Name: "signatureType", Type: "uint8"},
			},
		},
		PrimaryType: "Order",
		Domain: apitypes.TypedDataDomain{
			Name:              protocolName,
			Version:           protocolVersion,
			ChainId:           ethmath.NewHexOrDecimal256(chainID),
			VerifyingContract: verifyingContract,
		},
		Message: apitypes.TypedDataMessage{
			"salt":          order.Salt,
			"maker":         order.Maker,
			"signer":        order.Signer,
			"taker":         order.Taker,
			"tokenId":       order.TokenID,
			"makerAmount":   order.MakerAmount,
			"takerAmount":   order.TakerAmount,
			"expiration":    order.Expiration,
			"nonce":         order.Nonce,
			"feeRateBps":    order.FeeRateBps,
			"side":          strconv.Itoa(sideValue(order.Side)),
			"signatureType": strconv.Itoa(int(order.SignatureType)),
		},
	}
}

func validateLimitOrderArgs(order OrderArgs) error {
	if order.TokenID == "" {
		return fmt.Errorf("token id is required")
	}
	if order.Size <= 0 {
		return fmt.Errorf("size must be positive")
	}
	if order.Price <= 0 {
		return fmt.Errorf("price must be positive")
	}
	if order.Side != SideBuy && order.Side != SideSell {
		return fmt.Errorf("invalid side %q", order.Side)
	}
	return nil
}

func validateMarketOrderArgs(order MarketOrderArgs) error {
	if order.TokenID == "" {
		return fmt.Errorf("token id is required")
	}
	if order.Amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	if order.Price < 0 {
		return fmt.Errorf("price cannot be negative")
	}
	if order.Side != SideBuy && order.Side != SideSell {
		return fmt.Errorf("invalid side %q", order.Side)
	}
	return nil
}

func validatePrice(price float64, tickSize TickSize) error {
	value := decimal.NewFromFloat(price)
	minimum, err := parseTickSize(tickSize)
	if err != nil {
		return err
	}
	maximum := decimal.NewFromInt(1).Sub(minimum)
	if value.GreaterThanOrEqual(minimum) && value.LessThanOrEqual(maximum) {
		return nil
	}
	return fmt.Errorf("invalid price (%v), min: %s - max: %s", price, minimum, maximum)
}

func sideValue(side Side) int {
	if side == SideSell {
		return 1
	}
	return 0
}

func normalizeTaker(taker string) string {
	if taker == "" {
		return zeroAddress
	}
	return taker
}

func (c *Client) resolveTickSize(
	ctx context.Context,
	tokenID string,
	options *CreateOrderOptions,
) (TickSize, error) {
	if options != nil && options.TickSize != "" {
		return options.TickSize, nil
	}

	response, err := c.GetTickSize(ctx, tokenID)
	if err != nil {
		return "", err
	}
	return response.MinimumTickSize, nil
}

func (c *Client) resolveNegRisk(
	ctx context.Context,
	tokenID string,
	options *CreateOrderOptions,
) (bool, error) {
	if options != nil && options.NegRisk != nil {
		return *options.NegRisk, nil
	}

	response, err := c.GetNegRisk(ctx, tokenID)
	if err != nil {
		return false, err
	}
	return response.NegRisk, nil
}

func (c *Client) resolveFeeRateBps(
	ctx context.Context,
	tokenID string,
	userFeeRateBps int64,
) (int64, error) {
	marketFeeRateBps, err := c.GetFeeRateBps(ctx, tokenID)
	if err != nil {
		return 0, err
	}

	if marketFeeRateBps > 0 && userFeeRateBps != 0 && userFeeRateBps != marketFeeRateBps {
		return 0, fmt.Errorf(
			"invalid user provided fee rate: %d, fee rate for the market must be %d",
			userFeeRateBps,
			marketFeeRateBps,
		)
	}

	return marketFeeRateBps, nil
}

func roundDown(value decimal.Decimal, places int32) decimal.Decimal {
	return value.RoundFloor(places)
}

func roundNormal(value decimal.Decimal, places int32) decimal.Decimal {
	return value.Round(places)
}

func roundUp(value decimal.Decimal, places int32) decimal.Decimal {
	return value.RoundCeil(places)
}

func decimalPlaces(value decimal.Decimal) int32 {
	return -value.Exponent()
}

func toTokenDecimals(value decimal.Decimal) decimal.Decimal {
	return value.Shift(collateralTokenScale).Truncate(0)
}

func parseTickSize(value TickSize) (decimal.Decimal, error) {
	parsed, err := decimal.NewFromString(string(value))
	if err != nil {
		return decimal.Zero, fmt.Errorf("parse tick size %q: %w", value, err)
	}
	return parsed, nil
}

func generateSalt() (uint64, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(raw[:]) & ((1 << 53) - 1), nil
}

func derefBool(value *bool) bool {
	return value != nil && *value
}
