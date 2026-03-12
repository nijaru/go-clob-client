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

var roundingConfig = map[TickSize]RoundConfig{
	TickSizeTenth:       {Price: 1, Size: 2, Amount: 3},
	TickSizeHundredth:   {Price: 2, Size: 2, Amount: 4},
	TickSizeThousandth:  {Price: 3, Size: 2, Amount: 5},
	TickSizeTenThousand: {Price: 4, Size: 2, Amount: 6},
}

func Bool(value bool) *bool {
	return &value
}

func (c *Client) CreateOrder(
	ctx context.Context,
	userOrder OrderArgs,
	options *CreateOrderOptions,
) (*SignedOrder, error) {
	if c.signer == nil {
		return nil, fmt.Errorf("create order requires a private key")
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

	if !priceValid(userOrder.Price, tickSize) {
		return nil, fmt.Errorf(
			"invalid price (%v), min: %s - max: %s",
			userOrder.Price,
			tickSize,
			decimal.NewFromInt(1).Sub(mustParseTickSize(tickSize)).String(),
		)
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

func (c *Client) CreateMarketOrder(
	ctx context.Context,
	userOrder MarketOrderArgs,
	options *CreateOrderOptions,
) (*SignedOrder, error) {
	if c.signer == nil {
		return nil, fmt.Errorf("create market order requires a private key")
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

	if !priceValid(userOrder.Price, tickSize) {
		return nil, fmt.Errorf(
			"invalid price (%v), min: %s - max: %s",
			userOrder.Price,
			tickSize,
			decimal.NewFromInt(1).Sub(mustParseTickSize(tickSize)).String(),
		)
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

func (c *Client) BuildPostOrderRequest(
	order SignedOrder,
	orderType OrderType,
	deferExec bool,
	postOnly bool,
) (PostOrderRequest, error) {
	if c.creds == nil {
		return PostOrderRequest{}, fmt.Errorf("build post order request requires API credentials")
	}

	if orderType == "" {
		orderType = OrderTypeGTC
	}

	if postOnly && orderType != OrderTypeGTC && orderType != OrderTypeGTD {
		return PostOrderRequest{}, fmt.Errorf("postOnly is only supported for GTC and GTD orders")
	}

	return PostOrderRequest{
		Order:     order,
		Owner:     c.creds.Key,
		OrderType: orderType,
		DeferExec: deferExec,
		PostOnly:  postOnly,
	}, nil
}

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
		Salt:          strconv.FormatUint(c.saltGenerator(), 10),
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

func priceValid(price float64, tickSize TickSize) bool {
	value := decimal.NewFromFloat(price)
	minimum := mustParseTickSize(tickSize)
	return value.GreaterThanOrEqual(minimum) &&
		value.LessThanOrEqual(decimal.NewFromInt(1).Sub(minimum))
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

func mustParseTickSize(value TickSize) decimal.Decimal {
	parsed, err := decimal.NewFromString(string(value))
	if err != nil {
		panic(err)
	}
	return parsed
}

func generateSalt() uint64 {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(err)
	}
	return binary.BigEndian.Uint64(raw[:]) & ((1 << 53) - 1)
}

func derefBool(value *bool) bool {
	return value != nil && *value
}
