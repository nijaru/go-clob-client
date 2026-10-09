package signing

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

// LocalSigner implements every signing capability using an in-memory private key.
// It intentionally provides no key-export method.
type LocalSigner struct {
	key     *ecdsa.PrivateKey
	address common.Address
}

// NewLocalSigner parses a hex-encoded private key, with an optional 0x prefix.
func NewLocalSigner(raw string) (*LocalSigner, error) {
	key, err := crypto.HexToECDSA(strings.TrimPrefix(raw, "0x"))
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	return &LocalSigner{key: key, address: crypto.PubkeyToAddress(key.PublicKey)}, nil
}

func (s *LocalSigner) Address() common.Address { return s.address }
func (s *LocalSigner) SignTypedData(ctx context.Context, data apitypes.TypedData) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	digest, _, err := apitypes.TypedDataAndHash(data)
	if err != nil {
		return nil, err
	}
	return crypto.Sign(digest, s.key)
}

func (s *LocalSigner) SignMessage(ctx context.Context, message []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return crypto.Sign(accounts.TextHash(message), s.key)
}

func (s *LocalSigner) SignTransaction(
	ctx context.Context,
	chainID *big.Int,
	tx *types.Transaction,
) (*types.Transaction, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if chainID == nil || chainID.Sign() <= 0 || tx == nil {
		return nil, fmt.Errorf("signing: invalid transaction or chain")
	}
	return types.SignTx(tx, types.LatestSignerForChainID(chainID), s.key)
}
