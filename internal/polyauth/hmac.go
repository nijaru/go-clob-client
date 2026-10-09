package polyauth

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
)

// normalizeSignatureBody applies the CLOB L2 quote rewrite (single → double)
// that the CLOB server performs before recomputing an L2 HMAC. Builder/relayer
// auth must NOT use this — those servers sign the body verbatim.
func normalizeSignatureBody(body []byte) []byte {
	if len(body) == 0 {
		return nil
	}
	return bytes.ReplaceAll(body, []byte("'"), []byte("\""))
}

func DecodeAPISecret(secret string) ([]byte, error) {
	normalized, err := normalizeBase64URL(secret)
	if err != nil {
		return nil, err
	}
	decoded, err := base64.URLEncoding.DecodeString(normalized)
	if err != nil {
		// The secret uses standard base64 encoding instead of URL-safe encoding.
		// This is technically misconfigured — Polymarket issues URL-safe secrets.
		// We accept it for backward compatibility but warn so the caller can fix it.
		slog.Warn("API secret uses standard base64 encoding; URL-safe encoding is expected")
		std := strings.NewReplacer("-", "+", "_", "/").Replace(strings.TrimSpace(secret))
		std, perr := padBase64(std)
		if perr != nil {
			return nil, fmt.Errorf("decode API secret: %w", perr)
		}
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
	// HMACSignature is the CLOB L2 convenience, so apply L2 quote normalization.
	return HMACSignatureBytes(
		decoded,
		timestamp,
		method,
		requestPath,
		normalizeSignatureBody(body),
	), nil
}

// HMACSignatureBytes computes the raw HMAC-SHA256 signature over
// timestamp+method+requestPath+body, encoded as padded URL-safe base64. The body
// is signed verbatim. CLOB L2 callers must apply quote normalization first (the
// server rewrites quotes); builder/relayer auth signs the body as-is.
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
		mac.Write(body)
	}

	return base64.URLEncoding.EncodeToString(mac.Sum(nil))
}

func normalizeBase64URL(value string) (string, error) {
	return padBase64(strings.TrimSpace(value))
}

// padBase64 adds the base64 padding implied by value's length, returning an
// error for the one length (mod 4 == 1) that can never be valid base64.
func padBase64(value string) (string, error) {
	switch len(value) % 4 {
	case 1:
		return "", fmt.Errorf("invalid base64 secret: length mod 4 == 1 is never valid")
	case 2:
		return value + "==", nil
	case 3:
		return value + "=", nil
	}
	return value, nil
}
