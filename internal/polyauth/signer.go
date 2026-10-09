package polyauth

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	ethmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

const clobAuthMessage = "This message attests that I control the given wallet"

type Signer struct {
	key     *ecdsa.PrivateKey
	address common.Address
}

func GenerateKey() (string, error) {
	key, err := crypto.GenerateKey()
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(crypto.FromECDSA(key)), nil
}

func ParsePrivateKey(raw string) (*Signer, error) {
	raw = strings.TrimPrefix(raw, "0x")
	key, err := crypto.HexToECDSA(raw)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	return &Signer{
		key:     key,
		address: crypto.PubkeyToAddress(key.PublicKey),
	}, nil
}

func (s *Signer) Address() common.Address {
	return s.address
}

func (s *Signer) PrivateKey() *ecdsa.PrivateKey {
	return s.key
}

func SignTypedData(signer *Signer, typedData apitypes.TypedData) (string, error) {
	digest, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		return "", fmt.Errorf("build typed data digest: %w", err)
	}

	signature, err := crypto.Sign(digest, signer.key)
	if err != nil {
		return "", fmt.Errorf("sign typed data: %w", err)
	}
	signature[64] += 27

	return "0x" + hex.EncodeToString(signature), nil
}

func (s *Signer) signClobAuth(chainID, timestamp, nonce int64) (string, error) {
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
			Name:    "ClobAuthDomain",
			Version: "1",
			ChainId: ethmath.NewHexOrDecimal256(chainID),
		},
		Message: apitypes.TypedDataMessage{
			"address":   s.address.Hex(),
			"timestamp": strconv.FormatInt(timestamp, 10),
			"nonce":     strconv.FormatInt(nonce, 10),
			"message":   clobAuthMessage,
		},
	}

	return SignTypedData(s, typedData)
}
