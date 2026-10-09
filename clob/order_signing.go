package clob

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"time"

	ethmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/nijaru/go-clob-client/internal/polyauth"
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
)

const zeroBytes32 = "0x0000000000000000000000000000000000000000000000000000000000000000"

type orderBuildInput struct {
	TokenID       string
	PositionID    string
	MakerAmount   *big.Int
	TakerAmount   *big.Int
	Side          Side
	Expiration    uint64
	NegRisk       bool
	SignatureType SignatureType
	Metadata      string
	BuilderCode   string
	Taker         string
	Nonce         uint64
	FeeRateBps    *uint32
}

// orderExchange selects the verifying contract and EIP-712 domain version for
// an order. Position-backed (V3) orders always sign against Exchange V3 with
// protocol version "3". Token-backed orders sign against the exchange the
// CLOB server's current protocol version selects: server version 3 signs
// against Exchange V3 with version "3"; older servers sign against the CTF
// Exchange (or its neg-risk variant) with version "2". The order body shape
// is identical for both — the wire field is named `tokenId` for CTF token IDs
// and PolyV2 position IDs alike.
type orderExchange struct {
	VerifyingContract string
	Version           string
}

func (c *SignerClient) resolveOrderExchange(
	ctx context.Context, input orderBuildInput, contracts contractConfig,
) (orderExchange, error) {
	version := uint32(3)
	var err error
	if input.PositionID == "" {
		version, err = c.resolveServerVersion(ctx, false)
		if err != nil {
			return orderExchange{}, err
		}
	}
	switch version {
	case 1:
		if input.SignatureType == SignatureTypePoly1271 {
			return orderExchange{}, fmt.Errorf("Poly1271 is not supported for V1 orders")
		}
		address, err := legacyExchangeAddress(c.chainID, input.NegRisk)
		if err != nil {
			return orderExchange{}, err
		}
		return orderExchange{VerifyingContract: address, Version: "1"}, nil
	case 2:
		address := contracts.Exchange
		if input.NegRisk {
			address = contracts.NegRiskExchange
		}
		if address == "" {
			return orderExchange{}, fmt.Errorf("V2 exchange not configured for chain %d", c.chainID)
		}
		return orderExchange{VerifyingContract: address, Version: "2"}, nil
	case 3:
		if contracts.ExchangeV3 == "" {
			return orderExchange{}, fmt.Errorf("V3 exchange not configured for chain %d", c.chainID)
		}
		return orderExchange{VerifyingContract: contracts.ExchangeV3, Version: "3"}, nil
	default:
		return orderExchange{}, fmt.Errorf("unsupported CLOB protocol version %d", version)
	}
}

func (c *SignerClient) signOrder(ctx context.Context, input orderBuildInput) (*SignedOrder, error) {
	contracts, err := getContractConfig(c.chainID)
	if err != nil {
		return nil, err
	}

	if input.TokenID == "" && input.PositionID == "" {
		return nil, fmt.Errorf("order requires exactly one of TokenID or PositionID")
	}
	if input.TokenID != "" && input.PositionID != "" {
		return nil, fmt.Errorf("order must not set both TokenID and PositionID")
	}

	if err := validateOrderAmounts(input.MakerAmount, input.TakerAmount); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	signerAddress := c.signer.Address().Hex()
	maker := signerAddress
	if c.funderAddress != "" {
		maker = c.funderAddress
	}
	if input.SignatureType == SignatureTypePoly1271 {
		signerAddress = maker
	}

	salt, err := c.saltGenerator()
	if err != nil {
		return nil, fmt.Errorf("generate order salt: %w", err)
	}

	exchange, err := c.resolveOrderExchange(ctx, input, contracts)
	if err != nil {
		return nil, err
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

	// The wire field is named `tokenId` for both CTF token IDs and PolyV2
	// position IDs. V3 position-backed orders carry the position ID there.
	wireTokenID := input.TokenID
	if wireTokenID == "" {
		wireTokenID = input.PositionID
	}

	order := SignedOrder{
		Order: Order{
			Salt:          strconv.FormatUint(salt, 10),
			Maker:         maker,
			Signer:        signerAddress,
			TokenID:       wireTokenID,
			MakerAmount:   input.MakerAmount.String(),
			TakerAmount:   input.TakerAmount.String(),
			Side:          input.Side,
			SignatureType: input.SignatureType,
			Timestamp:     strconv.FormatInt(timestampMs, 10),
			Metadata:      metadata,
			Builder:       builderCode,
		},
		Expiration: strconv.FormatUint(input.Expiration, 10),
	}
	if exchange.Version == "1" {
		return c.signLegacyOrder(ctx, input, exchange, order)
	}

	typedData := buildOrderTypedData(c.chainID, exchange.Version, exchange.VerifyingContract, order)

	var signature string
	if input.SignatureType == SignatureTypePoly1271 {
		signature, err = signPoly1271Order(ctx, c.signer, typedData, c.chainID)
	} else {
		var sig []byte
		sig, err = c.signer.SignTypedData(ctx, typedData)
		signature = "0x" + hex.EncodeToString(sig)
	}
	if err != nil {
		return nil, err
	}
	if input.SignatureType == SignatureTypePoly1271 {
		signature, err = c.wrapDepositWalletSignature(signature)
		if err != nil {
			return nil, err
		}
	}
	order.Signature = signature

	return &order, nil
}

// signPoly1271Order produces a Solady-style EIP-1271 wrapped signature for
// deposit wallet orders. The inner ECDSA signature is wrapped with the app
// domain separator, contents hash, and the EIP-712 type string so the deposit
// wallet's isValidSignature check can reconstruct the original typed-data digest.
func signPoly1271Order(
	ctx context.Context,
	signer *polyauth.Signer,
	typedData apitypes.TypedData,
	chainID int64,
) (string, error) {
	// Compute the domain separator hash.
	domainSeparator, err := typedData.HashStruct("EIP712Domain", typedData.Domain.Map())
	if err != nil {
		return "", fmt.Errorf("hash domain separator: %w", err)
	}

	// Compute the contents hash (hashStruct of the Order).
	contentsHash, err := typedData.HashStruct("Order", typedData.Message)
	if err != nil {
		return "", fmt.Errorf("hash order contents: %w", err)
	}

	// Present the nested EIP-712 structure to wallets, never an opaque hash.
	nestedTypes := make(apitypes.Types, len(typedData.Types)+1)
	for name, fields := range typedData.Types {
		nestedTypes[name] = fields
	}
	nestedTypes["TypedDataSign"] = []apitypes.Type{
		{Name: "contents", Type: "Order"},
		{Name: "name", Type: "string"},
		{Name: "version", Type: "string"},
		{Name: "chainId", Type: "uint256"},
		{Name: "verifyingContract", Type: "address"},
		{Name: "salt", Type: "bytes32"},
	}
	nested := apitypes.TypedData{
		Types: nestedTypes, PrimaryType: "TypedDataSign", Domain: typedData.Domain,
		Message: apitypes.TypedDataMessage{
			"contents": typedData.Message, "name": depositWalletName,
			"version": depositWalletVersion, "chainId": strconv.FormatInt(chainID, 10),
			"verifyingContract": typedData.Message["signer"], "salt": zeroBytes32,
		},
	}
	sig, err := signer.SignTypedData(ctx, nested)
	if err != nil {
		return "", fmt.Errorf("sign poly1271 typed data: %w", err)
	}

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
	protocolVersion string,
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
