package clob

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/nijaru/go-clob-client/internal/polyauth"
	"github.com/quagmt/udecimal"
)

const (
	protocolName    = "Polymarket CTF Exchange"
	protocolVersion = "2"

	// EIP-1271 deposit wallet constants (Solady-style wrapping).
	depositWalletName    = "DepositWallet"
	depositWalletVersion = "1"
	orderTypeString      = "Order(uint256 salt,address maker,address signer,uint256 tokenId," +
		"uint256 makerAmount,uint256 takerAmount,uint8 side,uint8 signatureType," +
		"uint256 timestamp,bytes32 metadata,bytes32 builder)"
	soladyTypeString = "TypedDataSign(Order contents,string name,string version,uint256 chainId," +
		"address verifyingContract,bytes32 salt)" + orderTypeString
)

const zeroBytes32 = "0x0000000000000000000000000000000000000000000000000000000000000000"

var tokenScaleFactor = udecimal.MustFromInt64(1000000, 0) // 10^6

var roundingConfig = map[TickSize]roundConfig{
	TickSizeTenth:       {Price: 1, Size: 2, Amount: 3},
	TickSizeHundredth:   {Price: 2, Size: 2, Amount: 4},
	TickSizeHalfCent:    {Price: 3, Size: 2, Amount: 5},
	TickSizeQuarterCent: {Price: 4, Size: 2, Amount: 6},
	TickSizeThousandth:  {Price: 3, Size: 2, Amount: 5},
	TickSizeTenThousand: {Price: 4, Size: 2, Amount: 6},
}

// CreateOrder builds and signs a limit order.
func (c *SignerClient) CreateOrder(
	ctx context.Context,
	userOrder OrderArgs,
	options *CreateOrderOptions,
) (*SignedOrder, error) {
	if err := validateLimitOrderArgs(userOrder); err != nil {
		return nil, err
	}

	tickSize, err := c.resolveTickSize(ctx, userOrder.TokenID, options)
	if err != nil {
		return nil, err
	}

	if err := validatePrice(userOrder.Price, tickSize); err != nil {
		return nil, err
	}
	if err := validateLimitPricePrecision(userOrder.Price, tickSize); err != nil {
		return nil, err
	}
	if err := validateTickAlignment(userOrder.Price, tickSize); err != nil {
		return nil, err
	}

	isNegRisk, err := c.resolveNegRisk(ctx, userOrder.TokenID, options)
	if err != nil {
		return nil, err
	}

	return c.buildSignedLimitOrder(ctx, userOrder, CreateOrderOptions{
		TickSize: tickSize,
		NegRisk:  new(isNegRisk),
	})
}

// CreateMarketOrder builds and signs a market order.
func (c *SignerClient) CreateMarketOrder(
	ctx context.Context,
	userOrder MarketOrderArgs,
	options *CreateOrderOptions,
) (*SignedOrder, error) {
	if err := validateMarketOrderArgs(userOrder); err != nil {
		return nil, err
	}

	tickSize, err := c.resolveTickSize(ctx, userOrder.TokenID, options)
	if err != nil {
		return nil, err
	}

	if userOrder.OrderType == "" {
		userOrder.OrderType = OrderTypeFOK
	}
	if userOrder.OrderType != OrderTypeFOK && userOrder.OrderType != OrderTypeFAK {
		return nil, fmt.Errorf("market orders only support FOK or FAK order types")
	}

	if userOrder.Price.IsZero() {
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
	if err := validateTickAlignment(userOrder.Price, tickSize); err != nil {
		return nil, err
	}

	isNegRisk, err := c.resolveNegRisk(ctx, userOrder.TokenID, options)
	if err != nil {
		return nil, err
	}

	return c.buildSignedMarketOrder(ctx, userOrder, CreateOrderOptions{
		TickSize: tickSize,
		NegRisk:  new(isNegRisk),
	})
}

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

// BuildAndPostOrder builds, signs, and posts a limit order with version mismatch retry.
func (c *AuthenticatedClient) BuildAndPostOrder(
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

// BuildAndPostMarketOrder builds, signs, and posts a market order with version mismatch retry.
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

// CalculateMarketPrice derives a marketable price from the current order book.
// For BUY orders, amount is the USDC notional to spend; for SELL orders, amount
// is the number of shares to sell.
func (c *Client) CalculateMarketPrice(
	ctx context.Context,
	tokenID string,
	side Side,
	amount udecimal.Decimal,
	orderType OrderType,
) (udecimal.Decimal, error) {
	if amount.Cmp(udecimal.Zero) <= 0 {
		return udecimal.Zero, fmt.Errorf("amount must be positive")
	}
	if orderType == "" {
		orderType = OrderTypeFOK
	}

	book, err := c.GetOrderBook(ctx, tokenID)
	if err != nil {
		return udecimal.Zero, err
	}

	var levels []OrderSummary
	switch side {
	case SideBuy:
		levels = book.Asks
	case SideSell:
		levels = book.Bids
	default:
		return udecimal.Zero, fmt.Errorf("invalid side %q", side)
	}

	if len(levels) == 0 {
		return udecimal.Zero, fmt.Errorf("no opposing orders for token %s", tokenID)
	}

	// Pre-parse the book levels to avoid string conversion in the loop
	type levelData struct {
		Price udecimal.Decimal
		Size  udecimal.Decimal
	}
	parsedLevels := make([]levelData, len(levels))
	for i, l := range levels {
		p, err := udecimal.Parse(l.Price)
		if err != nil {
			return udecimal.Zero, fmt.Errorf("parse price: %w", err)
		}
		s, err := udecimal.Parse(l.Size)
		if err != nil {
			return udecimal.Zero, fmt.Errorf("parse size: %w", err)
		}
		parsedLevels[i] = levelData{Price: p, Size: s}
	}

	sum := udecimal.Zero
	// Top of the book is at the end of the array (API returns Bids ASC, Asks DESC).
	for i := len(parsedLevels) - 1; i >= 0; i-- {
		level := parsedLevels[i]
		if side == SideBuy {
			// BUY: amount is USDC notional, accumulate size * price
			sum = sum.Add(level.Size.Mul(level.Price))
		} else {
			// SELL: amount is shares, accumulate size
			sum = sum.Add(level.Size)
		}

		if sum.Cmp(amount) >= 0 {
			return level.Price, nil
		}
	}

	if orderType == OrderTypeFOK {
		return udecimal.Zero, fmt.Errorf("insufficient liquidity to fill amount %s", amount)
	}

	return parsedLevels[0].Price, nil
}

func (c *SignerClient) buildSignedLimitOrder(
	ctx context.Context,
	userOrder OrderArgs,
	options CreateOrderOptions,
) (*SignedOrder, error) {
	roundConfig, ok := roundingConfig[options.TickSize]
	if !ok {
		return nil, fmt.Errorf("unsupported tick size %q", options.TickSize)
	}

	price := userOrder.Price
	size := userOrder.Size

	if decimalPlaces(size) > roundConfig.Size {
		return nil, fmt.Errorf(
			"size %s exceeds maximum %d decimal places for tick size %q",
			size, roundConfig.Size, options.TickSize,
		)
	}

	rawPrice := roundNormal(price, roundConfig.Price)

	var rawMakerAmount udecimal.Decimal
	var rawTakerAmount udecimal.Decimal

	switch userOrder.Side {
	case SideBuy:
		rawTakerAmount = roundDown(size, roundConfig.Size)
		rawMakerAmount = roundToAmount(rawTakerAmount.Mul(rawPrice), roundConfig)
	case SideSell:
		rawMakerAmount = roundDown(size, roundConfig.Size)
		rawTakerAmount = roundToAmount(rawMakerAmount.Mul(rawPrice), roundConfig)
	default:
		return nil, fmt.Errorf("invalid side %q", userOrder.Side)
	}

	return c.signOrder(ctx, orderBuildInput{
		TokenID:       userOrder.TokenID,
		MakerAmount:   toTokenDecimals(rawMakerAmount),
		TakerAmount:   toTokenDecimals(rawTakerAmount),
		Side:          userOrder.Side,
		Expiration:    userOrder.Expiration,
		NegRisk:       derefBool(options.NegRisk),
		SignatureType: c.signatureType,
		Metadata:      userOrder.Metadata,
		BuilderCode:   userOrder.BuilderCode,
		DeferExec:     userOrder.DeferExec,
	})
}

func (c *SignerClient) buildSignedMarketOrder(
	ctx context.Context,
	userOrder MarketOrderArgs,
	options CreateOrderOptions,
) (*SignedOrder, error) {
	roundConfig, ok := roundingConfig[options.TickSize]
	if !ok {
		return nil, fmt.Errorf("unsupported tick size %q", options.TickSize)
	}

	price := roundDown(userOrder.Price, roundConfig.Price)
	amount := userOrder.Amount

	var rawMakerAmount udecimal.Decimal
	var rawTakerAmount udecimal.Decimal

	switch userOrder.Side {
	case SideBuy:
		// BUY: Amount is USDC notional. Adjust for fees if MaxSpend is set.
		adjustedAmount := amount
		if userOrder.MaxSpend != nil && !userOrder.MaxSpend.IsZero() {
			feeInfo, err := c.GetFeeInfo(ctx, userOrder.TokenID)
			if err == nil {
				builderTakerFeeRate := 0.0
				if userOrder.BuilderCode != "" {
					builderFee, err := c.GetBuilderFeeRate(ctx, userOrder.BuilderCode)
					if err == nil {
						builderTakerFeeRate = float64(
							builderFee.BuilderTakerFeeRateBps,
						) / 10000.0
					}
				}
				adj, err := adjustMarketBuyAmount(
					amount,
					*userOrder.MaxSpend,
					price,
					feeInfo.Rate,
					feeInfo.Exponent,
					builderTakerFeeRate,
				)
				if err != nil {
					return nil, err
				}
				adjustedAmount = adj
			}
		}
		// Preserve the full USDC amount; only the derived share quantity is quantized.
		rawMakerAmount = adjustedAmount
		val, err := rawMakerAmount.Div(price)
		if err != nil {
			return nil, fmt.Errorf("calculation error: %w", err)
		}
		rawTakerAmount = roundToAmount(val, roundConfig)
	case SideSell:
		// SELL: Amount is shares.
		rawMakerAmount = roundDown(amount, roundConfig.Size)
		rawTakerAmount = roundToAmount(rawMakerAmount.Mul(price), roundConfig)
	default:
		return nil, fmt.Errorf("invalid side %q", userOrder.Side)
	}

	return c.signOrder(ctx, orderBuildInput{
		TokenID:       userOrder.TokenID,
		MakerAmount:   toTokenDecimals(rawMakerAmount),
		TakerAmount:   toTokenDecimals(rawTakerAmount),
		Side:          userOrder.Side,
		Expiration:    0,
		NegRisk:       derefBool(options.NegRisk),
		SignatureType: c.signatureType,
		Metadata:      userOrder.Metadata,
		BuilderCode:   userOrder.BuilderCode,
		DeferExec:     userOrder.DeferExec,
	})
}

type orderBuildInput struct {
	TokenID       string
	MakerAmount   udecimal.Decimal
	TakerAmount   udecimal.Decimal
	Side          Side
	Expiration    uint64
	NegRisk       bool
	SignatureType SignatureType
	Metadata      string
	BuilderCode   string
	DeferExec     bool
}

func (c *SignerClient) signOrder(ctx context.Context, input orderBuildInput) (*SignedOrder, error) {
	contracts, err := getContractConfig(c.chainID)
	if err != nil {
		return nil, err
	}

	signerAddress := c.signer.Address().Hex()
	maker := signerAddress
	if c.funderAddress != "" {
		maker = c.funderAddress
	}

	salt, err := c.saltGenerator()
	if err != nil {
		return nil, fmt.Errorf("generate order salt: %w", err)
	}

	verifyingContract := contracts.Exchange
	if input.NegRisk {
		verifyingContract = contracts.NegRiskExchange
	}
	if verifyingContract == "" {
		return nil, fmt.Errorf("exchange contract not configured for chain %d", c.chainID)
	}

	timestampMs := time.Now().UnixMilli()

	metadata := input.Metadata
	if metadata == "" {
		metadata = zeroBytes32
	}
	builderCode := input.BuilderCode
	if builderCode == "" {
		builderCode = zeroBytes32
	}

	order := SignedOrder{
		Order: Order{
			Salt:          strconv.FormatUint(salt, 10),
			Maker:         maker,
			Signer:        signerAddress,
			TokenID:       input.TokenID,
			MakerAmount:   input.MakerAmount.StringFixed(0),
			TakerAmount:   input.TakerAmount.StringFixed(0),
			Side:          input.Side,
			SignatureType: input.SignatureType,
			Timestamp:     strconv.FormatInt(timestampMs, 10),
			Metadata:      metadata,
			Builder:       builderCode,
		},
		Expiration: strconv.FormatUint(input.Expiration, 10),
	}

	typedData := buildOrderTypedData(c.chainID, verifyingContract, order)

	var signature string
	if input.SignatureType == SignatureTypePoly1271 {
		signature, err = signPoly1271Order(c.signer, typedData, c.chainID)
	} else {
		signature, err = polyauth.SignTypedData(c.signer, typedData)
	}
	if err != nil {
		return nil, err
	}
	order.Signature = signature

	return &order, nil
}

// signPoly1271Order produces a Solady-style EIP-1271 wrapped signature for
// deposit wallet orders. The inner ECDSA signature is wrapped with the app
// domain separator, contents hash, and the EIP-712 type string so the deposit
// wallet's isValidSignature check can reconstruct the original typed-data digest.
func signPoly1271Order(
	signer *polyauth.Signer,
	typedData apitypes.TypedData,
	chainID int64,
) (string, error) {
	// Compute the domain separator hash.
	domainSeparator, _, err := apitypes.TypedDataAndHash(apitypes.TypedData{
		Types:       typedData.Types,
		PrimaryType: "EIP712Domain",
		Domain:      typedData.Domain,
		Message:     apitypes.TypedDataMessage{},
	})
	if err != nil {
		return "", fmt.Errorf("hash domain separator: %w", err)
	}

	// Compute the contents hash (hashStruct of the Order).
	contentsHash, _, err := apitypes.TypedDataAndHash(apitypes.TypedData{
		Types:       typedData.Types,
		PrimaryType: "Order",
		Domain:      typedData.Domain,
		Message:     typedData.Message,
	})
	if err != nil {
		return "", fmt.Errorf("hash order contents: %w", err)
	}

	// ABI-encode the TypedDataSign struct fields and hash.
	typedDataSignStructHash := crypto.Keccak256(
		abiEncodeTypedDataSign(
			contentsHash,
			chainID,
			signer.Address(),
		),
	)

	// EIP-712 final digest: 0x19 || 0x01 || domainSeparator || typedDataSignStructHash.
	var digestInput [66]byte
	digestInput[0] = 0x19
	digestInput[1] = 0x01
	copy(digestInput[2:34], domainSeparator)
	copy(digestInput[34:66], typedDataSignStructHash)
	digest := crypto.Keccak256(digestInput[:])

	// Sign the digest.
	sig, err := crypto.Sign(digest, signer.PrivateKey())
	if err != nil {
		return "", fmt.Errorf("sign poly1271 digest: %w", err)
	}
	sig[64] += 27 // EIP-155 recovery ID

	// Build the wrapped signature: 0x || innerSig || domainSep || contentsHash || typeString || typeLen(u16 BE).
	orderTypeBytes := []byte(orderTypeString)
	typeLen := uint16(len(orderTypeBytes))
	wrapped := make([]byte, 0, 2+130+64+64+len(orderTypeBytes)*2+4)
	wrapped = append(wrapped, "0x"...)
	wrapped = appendHex(wrapped, sig)
	wrapped = appendHex(wrapped, domainSeparator)
	wrapped = appendHex(wrapped, contentsHash)
	wrapped = appendHex(wrapped, orderTypeBytes)
	wrapped = appendHex(wrapped, []byte{byte(typeLen >> 8), byte(typeLen)})

	return string(wrapped), nil
}

// abiEncodeTypedDataSign ABI-encodes the fields of the Solady TypedDataSign
// struct: (bytes32 contents, string name, string version, uint256 chainId,
//
//	address verifyingContract, bytes32 salt).
//
// Each field is padded to 32 bytes. The tuple has 7 elements:
//   - keccak256(soladyTypeString) (type hash)
//   - contentsHash
//   - keccak256(depositWalletName)
//   - keccak256(depositWalletVersion)
//   - chainId
//   - signer address
//   - salt (zero)
func abiEncodeTypedDataSign(contentsHash []byte, chainID int64, signer common.Address) []byte {
	buf := make([]byte, 32*7)
	// [0:32] keccak256(soladyTypeString) — the EIP-712 type hash
	copy(buf[0:32], crypto.Keccak256([]byte(soladyTypeString)))
	// [32:64] contents hash (hashStruct of the Order)
	copy(buf[32:64], contentsHash)
	// [64:96] keccak256(depositWalletName)
	copy(buf[64:96], crypto.Keccak256([]byte(depositWalletName)))
	// [96:128] keccak256(depositWalletVersion)
	copy(buf[96:128], crypto.Keccak256([]byte(depositWalletVersion)))
	// [128:160] chainId (uint256, right-aligned)
	chainIDBytes := new(big.Int).SetInt64(chainID).Bytes()
	copy(buf[160-len(chainIDBytes):160], chainIDBytes)
	// [160:192] signer address (20 bytes, left-padded to 32)
	copy(buf[192-20:192], signer.Bytes())
	// [192:224] salt (bytes32) = zero
	return buf
}

// appendHex appends the hex encoding of data (no 0x prefix) to dst.
func appendHex(dst, data []byte) []byte {
	const hexChars = "0123456789abcdef"
	for _, b := range data {
		dst = append(dst, hexChars[b>>4], hexChars[b&0x0f])
	}
	return dst
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
				{Name: "tokenId", Type: "uint256"},
				{Name: "makerAmount", Type: "uint256"},
				{Name: "takerAmount", Type: "uint256"},
				{Name: "side", Type: "uint8"},
				{Name: "signatureType", Type: "uint8"},
				{Name: "timestamp", Type: "uint256"},
				{Name: "metadata", Type: "bytes32"},
				{Name: "builder", Type: "bytes32"},
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
			"salt":          order.Order.Salt,
			"maker":         order.Order.Maker,
			"signer":        order.Order.Signer,
			"tokenId":       order.Order.TokenID,
			"makerAmount":   order.Order.MakerAmount,
			"takerAmount":   order.Order.TakerAmount,
			"side":          strconv.Itoa(sideValue(order.Order.Side)),
			"signatureType": strconv.Itoa(int(order.Order.SignatureType)),
			"timestamp":     order.Order.Timestamp,
			"metadata":      order.Order.Metadata,
			"builder":       order.Order.Builder,
		},
	}
}

// minGTDExpirationSeconds is the minimum lead time a GTD (Good-Til-Date)
// limit order expiration must have. The CLOB rejects GTD orders expiring
// sooner, so this fails fast before signing. Mirrors the ts-sdk and py-sdk
// 3-minute client guard (not enforced by the Rust SDK, which is permissive).
const minGTDExpirationSeconds = 180

// validateGTDExpiration rejects GTD expirations closer than
// minGTDExpirationSeconds to now. A zero expiration (GTC semantics) is
// always valid. now is the current Unix time in seconds.
func validateGTDExpiration(expiration uint64, now int64) error {
	if expiration == 0 {
		return nil
	}
	minimum := uint64(now) + minGTDExpirationSeconds
	if expiration < minimum {
		return fmt.Errorf(
			"GTD expiration %d must be at least %d seconds (%d minutes) in the future",
			expiration, minGTDExpirationSeconds, minGTDExpirationSeconds/60,
		)
	}
	return nil
}

func validateLimitOrderArgs(order OrderArgs) error {
	if order.TokenID == "" {
		return fmt.Errorf("token id is required")
	}
	if order.Size.Cmp(udecimal.Zero) <= 0 {
		return fmt.Errorf("size must be positive")
	}
	if order.Price.Cmp(udecimal.Zero) <= 0 {
		return fmt.Errorf("price must be positive")
	}
	if order.Side != SideBuy && order.Side != SideSell {
		return fmt.Errorf("invalid side %q", order.Side)
	}
	if err := validateGTDExpiration(order.Expiration, time.Now().Unix()); err != nil {
		return err
	}
	return nil
}

func validateMarketOrderArgs(order MarketOrderArgs) error {
	if order.TokenID == "" {
		return fmt.Errorf("token id is required")
	}
	if order.Amount.Cmp(udecimal.Zero) <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	if order.Price.Cmp(udecimal.Zero) < 0 {
		return fmt.Errorf("price cannot be negative")
	}
	if order.Side != SideBuy && order.Side != SideSell {
		return fmt.Errorf("invalid side %q", order.Side)
	}
	return nil
}

func validatePrice(price udecimal.Decimal, tickSize TickSize) error {
	value := price
	minimum, err := parseTickSize(tickSize)
	if err != nil {
		return err
	}
	maximum := udecimal.MustFromInt64(1, 0).Sub(minimum)
	if value.Cmp(minimum) >= 0 && value.Cmp(maximum) <= 0 {
		return nil
	}
	return fmt.Errorf("invalid price (%s), min: %s - max: %s", price, minimum, maximum)
}

func validateLimitPricePrecision(price udecimal.Decimal, tickSize TickSize) error {
	minimum, err := parseTickSize(tickSize)
	if err != nil {
		return err
	}
	if decimalPlaces(price) <= decimalPlaces(minimum) {
		return nil
	}
	return fmt.Errorf(
		"price %s exceeds maximum %d decimal places for tick size %q",
		price,
		decimalPlaces(minimum),
		tickSize,
	)
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
	c.tickSizeMu.RLock()
	cached, ok := c.tickSizeCache[tokenID]
	ts := c.tickSizeTimestamps[tokenID]
	c.tickSizeMu.RUnlock()

	var marketTickSize TickSize
	if ok && (c.cacheTTL == 0 || time.Since(ts) < c.cacheTTL) {
		marketTickSize = cached
	} else {
		response, err := c.GetTickSize(ctx, tokenID)
		if err != nil {
			return "", err
		}
		marketTickSize = response.MinimumTickSize

		c.tickSizeMu.Lock()
		c.tickSizeCache[tokenID] = marketTickSize
		c.tickSizeTimestamps[tokenID] = time.Now()
		c.tickSizeMu.Unlock()
	}

	if options != nil && options.TickSize != "" {
		smaller, err := isTickSizeSmaller(options.TickSize, marketTickSize)
		if err != nil {
			return "", fmt.Errorf("invalid tick size option: %w", err)
		}
		if smaller {
			return "", fmt.Errorf(
				"invalid tick size %q, minimum for market is %q",
				options.TickSize,
				marketTickSize,
			)
		}
		return options.TickSize, nil
	}

	return marketTickSize, nil
}

func (c *Client) resolveNegRisk(
	ctx context.Context,
	tokenID string,
	options *CreateOrderOptions,
) (bool, error) {
	if options != nil && options.NegRisk != nil {
		return *options.NegRisk, nil
	}

	c.negRiskMu.RLock()
	cached, ok := c.negRiskCache[tokenID]
	ts := c.negRiskTimestamps[tokenID]
	c.negRiskMu.RUnlock()

	if ok && (c.cacheTTL == 0 || time.Since(ts) < c.cacheTTL) {
		return cached, nil
	}

	response, err := c.GetNegRisk(ctx, tokenID)
	if err != nil {
		return false, err
	}

	c.negRiskMu.Lock()
	c.negRiskCache[tokenID] = response.NegRisk
	c.negRiskTimestamps[tokenID] = time.Now()
	c.negRiskMu.Unlock()

	return response.NegRisk, nil
}

func roundDown(value udecimal.Decimal, places uint8) udecimal.Decimal {
	return value.Trunc(places)
}

func roundNormal(value udecimal.Decimal, places uint8) udecimal.Decimal {
	return value.RoundHAZ(places)
}

func roundUp(value udecimal.Decimal, places uint8) udecimal.Decimal {
	return value.RoundAwayFromZero(places)
}

// roundToAmount quantizes a computed maker/taker amount to the allowed precision.
// The two-pass approach (round-up with 4 extra digits, then round-down if still too long)
// mirrors the Rust SDK's order_builder.rs behavior: prefer rounding up to avoid
// underpaying, but clamp back down if the extra digits don't resolve cleanly.
func roundToAmount(value udecimal.Decimal, cfg roundConfig) udecimal.Decimal {
	if decimalPlaces(value) <= cfg.Amount {
		return value
	}
	v := roundUp(value, cfg.Amount+4)
	if decimalPlaces(v) > cfg.Amount {
		v = roundDown(v, cfg.Amount)
	}
	return v
}

func decimalPlaces(value udecimal.Decimal) uint8 {
	return value.PrecUint()
}

func toTokenDecimals(value udecimal.Decimal) udecimal.Decimal {
	return value.Mul(tokenScaleFactor).Trunc(0)
}

// adjustMarketBuyAmount shrinks a USDC buy amount to fit within the user's
// balance after accounting for platform and builder taker fees.
// Fee formula: platform_fee_rate = rate * (price * (1 - price))^exponent.
// This matches the Rust SDK's adjust_market_buy_amount.
func adjustMarketBuyAmount(
	amount udecimal.Decimal,
	userBalance udecimal.Decimal,
	price udecimal.Decimal,
	feeRate float64,
	feeExponent uint32,
	builderTakerFeeRate float64,
) (udecimal.Decimal, error) {
	one := udecimal.MustFromInt64(1, 0)
	base := price.Mul(one.Sub(price))

	// platform_fee_rate = rate * base^exponent
	baseF64 := base.InexactFloat64()
	expF64 := float64(feeExponent)
	platformFeeRateVal := feeRate * math.Pow(baseF64, expF64)
	platformFeeRate := udecimal.MustFromFloat64(platformFeeRateVal)

	// platform_fee = amount / price * platform_fee_rate
	amountDivPrice, err := amount.Div(price)
	if err != nil {
		return udecimal.Zero, err
	}
	platformFee := amountDivPrice.Mul(platformFeeRate)

	// total_cost = amount + platform_fee + amount * builder_taker_fee_rate
	builderFee := amount.Mul(udecimal.MustFromFloat64(builderTakerFeeRate))
	totalCost := amount.Add(platformFee).Add(builderFee)

	var raw udecimal.Decimal
	if userBalance.Cmp(totalCost) <= 0 {
		// Balance insufficient: shrink amount to fit
		// divisor = 1 + platform_fee_rate / price + builder_taker_fee_rate
		feeRateDivPrice, err := platformFeeRate.Div(price)
		if err != nil {
			return udecimal.Zero, err
		}
		divisor := one.Add(feeRateDivPrice).Add(udecimal.MustFromFloat64(builderTakerFeeRate))
		val, err := userBalance.Div(divisor)
		if err != nil {
			return udecimal.Zero, err
		}
		raw = val
	} else {
		raw = amount
	}

	adjusted := raw.Trunc(6) // USDC_DECIMALS = 6
	if adjusted.IsZero() {
		return udecimal.Zero, fmt.Errorf(
			"user balance %s too small to cover fees at price %s",
			userBalance, price,
		)
	}
	return adjusted, nil
}

func parseTickSize(value TickSize) (udecimal.Decimal, error) {
	parsed, err := udecimal.Parse(string(value))
	if err != nil {
		return udecimal.Zero, fmt.Errorf("parse tick size %q: %w", value, err)
	}
	return parsed, nil
}

func generateSalt() (uint64, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return 0, err
	}
	// Mask to 53 bits so the salt survives JSON encoding as a JS Number without
	// precision loss (JavaScript float64 values can only represent integers exactly
	// up to 2^53-1).
	return binary.BigEndian.Uint64(raw[:]) & ((1 << 53) - 1), nil
}

func derefBool(value *bool) bool {
	return value != nil && *value
}

// validateTickAlignment checks that price is aligned to the minimum tick size grid.
// This mirrors the Rust SDK's price_aligned_to_tick_size check.
func validateTickAlignment(price udecimal.Decimal, tickSize TickSize) error {
	minimum, err := parseTickSize(tickSize)
	if err != nil {
		return err
	}
	rem, err := price.Mod(minimum)
	if err != nil {
		return fmt.Errorf("tick alignment check: %w", err)
	}
	if !rem.IsZero() {
		return fmt.Errorf(
			"price %s is not aligned to the minimum tick size %s",
			price, tickSize,
		)
	}
	return nil
}

func isTickSizeSmaller(a, b TickSize) (bool, error) {
	aParsed, err := udecimal.Parse(string(a))
	if err != nil {
		return false, fmt.Errorf("parse tick size %q: %w", a, err)
	}
	bParsed, err := udecimal.Parse(string(b))
	if err != nil {
		return false, fmt.Errorf("parse tick size %q: %w", b, err)
	}
	return aParsed.Cmp(bParsed) < 0, nil
}

// PostOrdersBatchLimit is the maximum number of orders allowed in a single batch.
const PostOrdersBatchLimit = 15
