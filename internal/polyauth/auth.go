package polyauth

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
		"POLY_TIMESTAMP": strconv.FormatInt(timestamp, 10),
		"POLY_NONCE":     strconv.FormatInt(nonce, 10),
	}, nil
}

func L2Headers(
	signer *Signer,
	key, secret, passphrase string,
	timestamp int64,
	method, path string,
	body []byte,
) (map[string]string, error) {
	signature, err := HMACSignature(secret, timestamp, method, path, body)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"POLY_ADDRESS":    signer.address.Hex(),
		"POLY_SIGNATURE":  signature,
		"POLY_TIMESTAMP":  strconv.FormatInt(timestamp, 10),
		"POLY_API_KEY":    key,
		"POLY_PASSPHRASE": passphrase,
	}, nil
}

func BuilderHeaders(
	key, secret, passphrase string,
	timestamp int64,
	method, path string,
	body []byte,
) (map[string]string, error) {
	signature, err := HMACSignature(secret, timestamp, method, path, body)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"POLY_BUILDER_API_KEY":    key,
		"POLY_BUILDER_SIGNATURE":  signature,
		"POLY_BUILDER_TIMESTAMP":  strconv.FormatInt(timestamp, 10),
		"POLY_BUILDER_PASSPHRASE": passphrase,
	}, nil
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

func HMACSignature(
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

	mac := hmac.New(sha256.New, decoded)
	mac.Write(strconv.AppendInt(nil, timestamp, 10))
	mac.Write([]byte(method))
	mac.Write([]byte(requestPath))
	if len(body) > 0 {
		mac.Write(body)
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
