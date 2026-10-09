package polyauth

import (
	"context"
	"strconv"
)

func L1Headers(
	ctx context.Context,
	signer *Signer,
	chainID, timestamp, nonce int64,
) (map[string]string, error) {
	signature, err := signClobAuth(ctx, signer, chainID, timestamp, nonce)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"POLY_ADDRESS":   signer.Address().Hex(),
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
	// CLOB L2 auth: the server rewrites single quotes as double quotes before
	// recomputing the HMAC (matches py-clob-client-v2 and rs-clob-client-v2).
	signature := HMACSignatureBytes(secret, timestamp, method, path, normalizeSignatureBody(body))

	return map[string]string{
		"POLY_ADDRESS":    signer.Address().Hex(),
		"POLY_SIGNATURE":  signature,
		"POLY_TIMESTAMP":  strconv.FormatInt(timestamp, 10),
		"POLY_API_KEY":    key,
		"POLY_PASSPHRASE": passphrase,
	}, nil
}

// BuilderHeaders builds the POLY_BUILDER_* headers used for builder and relayer
// auth. Unlike CLOB L2 auth, builder auth signs the request body verbatim: the
// builder/relayer server does not rewrite quotes before recomputing the HMAC
// (verified against py-sdk build_hmac_signature and go-builder-signing-sdk).
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
