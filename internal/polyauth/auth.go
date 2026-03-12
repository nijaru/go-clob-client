package polyauth

import (
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
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

func L1Headers(signer *Signer, chainID, timestamp, nonce int64) (map[string]string, error) {
	signature, err := signer.signClobAuth(chainID, timestamp, nonce)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"POLY_ADDRESS":   signer.address.Hex(),
		"POLY_SIGNATURE": signature,
		"POLY_TIMESTAMP": fmt.Sprintf("%d", timestamp),
		"POLY_NONCE":     fmt.Sprintf("%d", nonce),
	}, nil
}

func L2Headers(
	signer *Signer,
	key, secret, passphrase string,
	timestamp int64,
	method, path string,
	body []byte,
) (map[string]string, error) {
	signature, err := buildHMACSignature(secret, timestamp, method, path, body)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"POLY_ADDRESS":    signer.address.Hex(),
		"POLY_SIGNATURE":  signature,
		"POLY_TIMESTAMP":  fmt.Sprintf("%d", timestamp),
		"POLY_API_KEY":    key,
		"POLY_PASSPHRASE": passphrase,
	}, nil
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
			"timestamp": fmt.Sprintf("%d", timestamp),
			"nonce":     fmt.Sprintf("%d", nonce),
			"message":   clobAuthMessage,
		},
	}

	return SignTypedData(s, typedData)
}

func buildHMACSignature(
	secret string,
	timestamp int64,
	method, requestPath string,
	body []byte,
) (string, error) {
	secret = normalizeBase64URL(secret)
	decoded, err := base64.URLEncoding.DecodeString(secret)
	if err != nil {
		std := strings.NewReplacer("-", "+", "_", "/").Replace(secret)
		decoded, err = base64.StdEncoding.DecodeString(std)
		if err != nil {
			return "", fmt.Errorf("decode API secret: %w", err)
		}
	}

	message := fmt.Sprintf("%d%s%s", timestamp, method, requestPath)
	if len(body) > 0 {
		message += string(body)
	}

	mac := hmac.New(sha256.New, decoded)
	if _, err := mac.Write([]byte(message)); err != nil {
		return "", fmt.Errorf("write HMAC payload: %w", err)
	}

	return base64.URLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func normalizeBase64URL(value string) string {
	value = strings.TrimSpace(value)
	switch len(value) % 4 {
	case 2:
		value += "=="
	case 3:
		value += "="
	}
	return value
}
