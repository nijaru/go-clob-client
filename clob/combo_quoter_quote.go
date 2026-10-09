package clob

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

// The quoter wire uses numeric side and omits expiration, unlike the requester
// accept wire. All amounts, native position IDs, and salts remain strings.
type comboQuoterOrder struct {
	Salt          string        `json:"salt"`
	Maker         string        `json:"maker"`
	Signer        string        `json:"signer"`
	TokenID       string        `json:"tokenId"`
	MakerAmount   string        `json:"makerAmount"`
	TakerAmount   string        `json:"takerAmount"`
	Side          uint8         `json:"side"`
	SignatureType SignatureType `json:"signatureType"`
	Timestamp     string        `json:"timestamp"`
	Builder       string        `json:"builder"`
	Metadata      string        `json:"metadata"`
	Signature     string        `json:"signature"`
}

type comboQuoterQuote struct {
	Type        string           `json:"type"`
	RFQID       string           `json:"rfq_id"`
	PriceE6     string           `json:"price_e6"`
	SizeE6      string           `json:"size_e6"`
	SignedOrder comboQuoterOrder `json:"signed_order"`
}

func comboPositiveE6(value string) (*big.Int, error) {
	whole, fraction, _ := strings.Cut(value, ".")
	if !isNumericString(whole) || (fraction != "" && !isNumericString(fraction)) ||
		len(fraction) > 6 {
		return nil, fmt.Errorf("amount must be an unsigned decimal with at most six places")
	}
	result, ok := new(big.Int).SetString(whole+fraction+strings.Repeat("0", 6-len(fraction)), 10)
	if !ok || result.Sign() <= 0 || validateUint256(result, "e6 amount") != nil {
		return nil, fmt.Errorf("e6 amount must be a positive uint256")
	}
	return result, nil
}

func comboPositionValid(id string) bool {
	value, ok := new(big.Int).SetString(id, 10)
	return ok && isNumericString(id) && validateUint256(value, "position ID") == nil
}

func (c *AuthenticatedClient) buildComboQuoterQuote(
	ctx context.Context,
	request ComboRFQQuoteRequest,
	response ComboRFQQuoteResponse,
) (comboQuoterQuote, error) {
	empty := comboQuoterQuote{}
	if err := c.requireComboAccount(); err != nil {
		return empty, err
	}
	if request.RFQID == "" || request.Side != RFQSideYes ||
		(request.Direction != RFQDirectionBuy && request.Direction != RFQDirectionSell) ||
		!comboPositionValid(request.YesPositionID) ||
		!comboPositionValid(request.NoPositionID) {
		return empty, fmt.Errorf(
			"invalid combo quote request identity, direction, side or position IDs",
		)
	}
	price, err := comboPositiveE6(response.Price)
	if err != nil {
		return empty, fmt.Errorf("combo quote price: %w", err)
	}
	e6 := big.NewInt(1_000_000)
	if price.Cmp(e6) >= 0 {
		return empty, fmt.Errorf("combo quote price must be less than 1")
	}
	source := response.Source
	if source == "" {
		source = ComboQuoteCollateral
	}
	if source != ComboQuoteCollateral && source != ComboQuoteInventory {
		return empty, fmt.Errorf("combo quote source must be collateral or inventory")
	}
	var size *big.Int
	if response.Size != "" {
		size, err = comboPositiveE6(response.Size)
	} else {
		size, err = comboPositiveE6(request.RequestedSize.Value)
		if err == nil {
			switch request.RequestedSize.Unit {
			case RFQSizeUnitShares:
			case RFQSizeUnitNotional:
				size.Mul(size, e6).Quo(size, price)
			default:
				err = fmt.Errorf("unknown requested size unit")
			}
		}
	}
	if err != nil {
		return empty, fmt.Errorf("combo quote size: %w", err)
	}
	if size.Sign() <= 0 {
		return empty, fmt.Errorf("combo quote size rounds to zero")
	}
	// TS temporarily limits explicitly supplied values to MAX_SAFE_INTEGER.
	// The wire is string-valued; Go/Python retain exact integers, bounded by uint256.
	tokenID := request.YesPositionID
	complement := (request.Direction == RFQDirectionBuy && source == ComboQuoteCollateral) ||
		(request.Direction == RFQDirectionSell && source == ComboQuoteInventory)
	if complement {
		tokenID = request.NoPositionID
	}
	orderPrice := new(big.Int).Set(price)
	if complement {
		orderPrice.Sub(e6, price)
	}
	collateral := new(big.Int).Mul(orderPrice, size)
	side := SideBuy
	makerAmount, takerAmount := new(big.Int), new(big.Int)
	if source == ComboQuoteInventory {
		side = SideSell
		makerAmount.Set(size)
		takerAmount.Quo(collateral, e6)
	} else {
		makerAmount.Add(collateral, big.NewInt(999_999)).Quo(makerAmount, e6)
		takerAmount.Set(size)
	}
	if err := validateOrderAmounts(makerAmount, takerAmount); err != nil {
		return empty, err
	}
	contracts, err := getContractConfig(c.chainID)
	if err != nil {
		return empty, err
	}
	if contracts.ExchangeV3 == "" {
		return empty, fmt.Errorf("combo quote requires Exchange V3")
	}
	salt, err := c.saltGenerator()
	if err != nil {
		return empty, err
	}
	order := comboSignedOrderWire{
		Salt:          strconv.FormatUint(salt, 10),
		Maker:         c.comboMakerAddress(),
		Signer:        c.Address(),
		TokenID:       tokenID,
		MakerAmount:   makerAmount.String(),
		TakerAmount:   takerAmount.String(),
		Side:          side,
		SignatureType: c.signatureType,
		Timestamp:     strconv.FormatInt(time.Now().Unix(), 10),
		Builder:       zeroBytes32,
		Metadata:      zeroBytes32,
	}
	if c.signatureType == SignatureTypePoly1271 {
		order.Signer = order.Maker
	}
	if err := c.signComboOrder(ctx, &order, contracts.ExchangeV3); err != nil {
		return empty, err
	}
	wireSide := uint8(0)
	if side == SideSell {
		wireSide = 1
	}
	return comboQuoterQuote{
		Type:    "RFQ_QUOTE",
		RFQID:   request.RFQID,
		PriceE6: price.String(),
		SizeE6:  size.String(),
		SignedOrder: comboQuoterOrder{
			Salt:          order.Salt,
			Maker:         order.Maker,
			Signer:        order.Signer,
			TokenID:       order.TokenID,
			MakerAmount:   order.MakerAmount,
			TakerAmount:   order.TakerAmount,
			Side:          wireSide,
			SignatureType: order.SignatureType,
			Timestamp:     order.Timestamp,
			Builder:       order.Builder,
			Metadata:      order.Metadata,
			Signature:     order.Signature,
		},
	}, nil
}
