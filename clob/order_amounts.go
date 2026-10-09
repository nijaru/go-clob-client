package clob

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/quagmt/udecimal"
)

// OrderAmountsArgs signs an Exchange V3 order from exact base-unit amounts.
// No order book, rounding, or fee calculation is performed. The caller is
// responsible for the economic meaning of the amounts. TokenID and PositionID
// are mutually exclusive, as with OrderArgs.
type OrderAmountsArgs struct {
	TokenID     string
	PositionID  string
	MakerAmount *big.Int
	TakerAmount *big.Int
	Side        Side
	Expiration  uint64
	Metadata    string
	BuilderCode string
}

// CreateExchangeV3OrderFromAmounts signs exact uint256 base-unit amounts.
func (c *SignerClient) CreateExchangeV3OrderFromAmounts(
	ctx context.Context,
	args OrderAmountsArgs,
) (*SignedOrder, error) {
	if (args.TokenID == "") == (args.PositionID == "") {
		return nil, fmt.Errorf("order requires exactly one of TokenID or PositionID")
	}
	if args.Side != SideBuy && args.Side != SideSell {
		return nil, fmt.Errorf("invalid side %q", args.Side)
	}
	if err := validateGTDExpiration(args.Expiration, time.Now().Unix()); err != nil {
		return nil, err
	}
	// PositionID selects V3 independently of the server version. For a CTF ID,
	// use the same signing route while retaining the wire identifier unchanged.
	return c.signOrder(ctx, orderBuildInput{
		PositionID:  orderAssetID(args.TokenID, args.PositionID),
		MakerAmount: args.MakerAmount, TakerAmount: args.TakerAmount,
		Side: args.Side, Expiration: args.Expiration, SignatureType: c.signatureType,
		Metadata: args.Metadata, BuilderCode: args.BuilderCode,
	})
}

func orderAssetID(tokenID, positionID string) string {
	if positionID != "" {
		return positionID
	}
	return tokenID
}

func validateOrderAmounts(maker, taker *big.Int) error {
	for _, amount := range []*big.Int{maker, taker} {
		if amount == nil || amount.Sign() <= 0 || amount.BitLen() > 256 {
			return fmt.Errorf("order amounts must be positive uint256 values")
		}
	}
	return nil
}

func decimalInteger(value udecimal.Decimal) *big.Int {
	integer, _ := new(big.Int).SetString(value.StringFixed(0), 10)
	return integer
}

func decimalBaseUnits(value udecimal.Decimal) *big.Int {
	return decimalInteger(toTokenDecimals(value))
}
