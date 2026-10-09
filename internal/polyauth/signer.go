package polyauth

import (
	"context"
	"encoding/hex"
	"strconv"

	ethmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/nijaru/go-clob-client/signing"
)

const clobAuthMessage = "This message attests that I control the given wallet"

type Signer = signing.Wallet

func GenerateKey() (string, error) {
	key, err := crypto.GenerateKey()
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(crypto.FromECDSA(key)), nil
}

func ParsePrivateKey(raw string) (*Signer, error) {
	local, err := signing.NewLocalSigner(raw)
	if err != nil {
		return nil, err
	}
	return signing.NewWallet(local)
}

// SignTypedData is used by the separately owned perps signing surface, whose
// context-aware migration must be completed there. CLOB uses Wallet directly.
func SignTypedData(signer *Signer, data apitypes.TypedData) (string, error) {
	sig, err := signer.SignTypedData(context.Background(), data)
	if err != nil {
		return "", err
	}
	return "0x" + hex.EncodeToString(sig), nil
}

func signClobAuth(ctx context.Context, s *Signer, chainID, timestamp, nonce int64) (string, error) {
	typedData := apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": {
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
			},
			"ClobAuth": {
				{Name: "address", Type: "address"},
				{Name: "timestamp", Type: "string"},
				{Name: "nonce", Type: "uint256"},
				{Name: "message", Type: "string"},
			},
		},
		PrimaryType: "ClobAuth",
		Domain: apitypes.TypedDataDomain{
			Name: "ClobAuthDomain", Version: "1", ChainId: ethmath.NewHexOrDecimal256(chainID),
		},
		Message: apitypes.TypedDataMessage{
			"address": s.Address().Hex(), "timestamp": strconv.FormatInt(timestamp, 10),
			"nonce": strconv.FormatInt(nonce, 10), "message": clobAuthMessage,
		},
	}
	sig, err := s.SignTypedData(ctx, typedData)
	if err != nil {
		return "", err
	}
	return "0x" + hex.EncodeToString(sig), nil
}
