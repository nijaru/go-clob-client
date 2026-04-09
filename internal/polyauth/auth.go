package polyauth

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	json "github.com/go-json-experiment/json"

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

func L1Headers(signer *Signer, chainID, timestamp, nonce int64) (map[string]string, error) {
	signature, err := signer.signClobAuth(chainID, timestamp, nonce)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"POLY_ADDRESS":   signer.address.Hex(),
		"POLY_SIGNATURE": signature,
		"POLY_TIMESTAMP": strconv.FormatInt(timestamp, 10),
		"POLY_NONCE":     strconv.FormatInt(nonce, 10),
	}, nil
}

func L2Headers(
	signer *Signer,
	key string, secret []byte, passphrase string,
	timestamp int64,
	method, path string,
	body []byte,
) (map[string]string, error) {
	signature := HMACSignatureBytes(secret, timestamp, method, path, body)

	return map[string]string{
		"POLY_ADDRESS":    signer.address.Hex(),
		"POLY_SIGNATURE":  signature,
		"POLY_TIMESTAMP":  strconv.FormatInt(timestamp, 10),
		"POLY_API_KEY":    key,
		"POLY_PASSPHRASE": passphrase,
	}, nil
}

func BuilderHeaders(
	key string, secret []byte, passphrase string,
	timestamp int64,
	method, path string,
	body []byte,
) (map[string]string, error) {
	signature := HMACSignatureBytes(secret, timestamp, method, path, body)

	return map[string]string{
		"POLY_BUILDER_API_KEY":    key,
		"POLY_BUILDER_SIGNATURE":  signature,
		"POLY_BUILDER_TIMESTAMP":  strconv.FormatInt(timestamp, 10),
		"POLY_BUILDER_PASSPHRASE": passphrase,
	}, nil
}

func normalizeSignatureBody(body []byte) []byte {
	if len(body) == 0 {
		return nil
	}
	return bytes.ReplaceAll(body, []byte("'"), []byte("\""))
}

func NormalizeSignatureBodyForRemote(body []byte) []byte {
	return normalizeSignatureBody(body)
}

type RemoteBuilderHeaderRequest struct {
	Method    string `json:"method"`
	Path      string `json:"path"`
	Body      string `json:"body"`
	Timestamp int64  `json:"timestamp"`
}

type RemoteBuilderHeaderResponse struct {
	APIKey     string `json:"poly_builder_api_key"`
	Timestamp  string `json:"poly_builder_timestamp"`
	Passphrase string `json:"poly_builder_passphrase"`
	Signature  string `json:"poly_builder_signature"`
}

func FetchRemoteBuilderHeaders(
	ctx context.Context,
	client *http.Client,
	endpoint string,
	bearerToken string,
	request RemoteBuilderHeaderRequest,
) (map[string]string, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal remote builder request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("create remote builder request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform remote builder request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read remote builder response: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf(
			"remote builder signer returned status %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	var decoded RemoteBuilderHeaderResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("decode remote builder response: %w", err)
	}

	return map[string]string{
		"POLY_BUILDER_API_KEY":    decoded.APIKey,
		"POLY_BUILDER_SIGNATURE":  decoded.Signature,
		"POLY_BUILDER_TIMESTAMP":  decoded.Timestamp,
		"POLY_BUILDER_PASSPHRASE": decoded.Passphrase,
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
			"timestamp": strconv.FormatInt(timestamp, 10),
			"nonce":     strconv.FormatInt(nonce, 10),
			"message":   clobAuthMessage,
		},
	}

	return SignTypedData(s, typedData)
}

func DecodeAPISecret(secret string) ([]byte, error) {
	normalized, err := normalizeBase64URL(secret)
	if err != nil {
		return nil, err
	}
	decoded, err := base64.URLEncoding.DecodeString(normalized)
	if err != nil {
		std := strings.NewReplacer("-", "+", "_", "/").Replace(secret)
		decoded, err = base64.StdEncoding.DecodeString(std)
		if err != nil {
			return nil, fmt.Errorf("decode API secret: %w", err)
		}
	}
	return decoded, nil
}

func HMACSignature(
	secret string,
	timestamp int64,
	method, requestPath string,
	body []byte,
) (string, error) {
	decoded, err := DecodeAPISecret(secret)
	if err != nil {
		return "", err
	}
	return HMACSignatureBytes(decoded, timestamp, method, requestPath, body), nil
}

func HMACSignatureBytes(
	secret []byte,
	timestamp int64,
	method, requestPath string,
	body []byte,
) string {
	mac := hmac.New(sha256.New, secret)

	// Use a pre-allocated buffer to avoid multiple ephemeral allocations
	var buf [24]byte
	mac.Write(strconv.AppendInt(buf[:0], timestamp, 10))
	io.WriteString(mac, method)
	io.WriteString(mac, requestPath)
	if len(body) > 0 {
		// Polymarket API expects single quotes to be replaced with double quotes in the signature message
		mac.Write(normalizeSignatureBody(body))
	}

	return base64.URLEncoding.EncodeToString(mac.Sum(nil))
}

func normalizeBase64URL(value string) (string, error) {
	value = strings.TrimSpace(value)
	switch len(value) % 4 {
	case 1:
		return "", fmt.Errorf("invalid base64url secret: length mod 4 == 1 is never valid")
	case 2:
		value += "=="
	case 3:
		value += "="
	}
	return value, nil
}
