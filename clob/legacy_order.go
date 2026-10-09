package clob

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"

	ethmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

// LegacyOrderFields contains the signed fields specific to protocol V1.
type LegacyOrderFields struct {
	Taker      string `json:"taker"`
	Nonce      string `json:"nonce"`
	FeeRateBps string `json:"feeRateBps"`
}

func legacyExchangeAddress(chainID int64, negRisk bool) (string, error) {
	switch chainID {
	case PolygonChainID:
		if negRisk {
			return "0xC5d563A36AE78145C45a50134d48A1215220f80a", nil
		}
		return "0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E", nil
	case AmoyChainID:
		if negRisk {
			return "0xC5d563A36AE78145C45a50134d48A1215220f80a", nil
		}
		return "0xdFE02Eb6733538f8Ea35D585af8DE5958AD99E40", nil
	default:
		return "", fmt.Errorf("V1 exchange unsupported on chain %d", chainID)
	}
}

func (c *SignerClient) signLegacyOrder(
	ctx context.Context,
	input orderBuildInput,
	exchange orderExchange,
	order SignedOrder,
) (*SignedOrder, error) {
	fee, err := c.GetFeeRate(ctx, order.Order.TokenID)
	if err != nil {
		return nil, err
	}
	if input.FeeRateBps != nil && fee.BaseFee > 0 && *input.FeeRateBps != fee.BaseFee {
		return nil, fmt.Errorf(
			"user fee rate %d does not match market fee rate %d",
			*input.FeeRateBps,
			fee.BaseFee,
		)
	}
	order.Legacy = &LegacyOrderFields{
		Taker:      normalizeTaker(input.Taker),
		Nonce:      strconv.FormatUint(input.Nonce, 10),
		FeeRateBps: strconv.FormatUint(uint64(fee.BaseFee), 10),
	}
	order.Order.Timestamp, order.Order.Metadata, order.Order.Builder = "", "", ""
	signature, err := c.signer.SignTypedData(
		ctx,
		buildLegacyOrderTypedData(c.chainID, exchange.VerifyingContract, order),
	)
	if err != nil {
		return nil, err
	}
	order.Signature = "0x" + hex.EncodeToString(signature)
	return &order, nil
}

func buildLegacyOrderTypedData(
	chainID int64,
	exchange string,
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
			Version:           "1",
			ChainId:           ethmath.NewHexOrDecimal256(chainID),
			VerifyingContract: exchange,
		},
		Message: apitypes.TypedDataMessage{
			"salt": order.Order.Salt, "maker": order.Order.Maker, "signer": order.Order.Signer, "taker": order.Legacy.Taker, "tokenId": order.Order.TokenID,
			"makerAmount": order.Order.MakerAmount, "takerAmount": order.Order.TakerAmount, "expiration": order.Expiration, "nonce": order.Legacy.Nonce,
			"feeRateBps": order.Legacy.FeeRateBps, "side": strconv.Itoa(sideValue(order.Order.Side)), "signatureType": strconv.Itoa(int(order.Order.SignatureType)),
		},
	}
}
