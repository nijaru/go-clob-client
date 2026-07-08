package polyrelay

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

const (
	proxyPrefix       = "rlx:" // legacy proxy relay preimage prefix
	depositDomainName = "DepositWallet"
	depositDomainVer  = "1"
)

var zeroAddress = common.Address{}

// ---------------------------------------------------------------------------
// Shared primitives
// ---------------------------------------------------------------------------

// personalHash returns the EIP-191 "personal sign" digest of msg
// (keccak256 of "\x19Ethereum Signed Message:\n<len>" + msg).
func personalHash(msg []byte) []byte {
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(msg))
	return crypto.Keccak256(append([]byte(prefix), msg...))
}

// signHash produces an EIP-191-style 65-byte signature (recovery byte in
// {27,28}) over an already-computed 32-byte digest. It is deterministic
// (RFC 6979), matching eth-account/coincurve output for identical inputs.
func signHash(key *ecdsa.PrivateKey, digest []byte) ([]byte, error) {
	if len(digest) != 32 {
		return nil, fmt.Errorf("polyrelay: digest must be 32 bytes, got %d", len(digest))
	}
	sig, err := crypto.Sign(digest, key)
	if err != nil {
		return nil, fmt.Errorf("polyrelay: sign: %w", err)
	}
	sig[64] += 27 // EIP-191 recovery convention
	return sig, nil
}

// toBytes32 left-pads v to 32 bytes (big-endian), panicking if it overflows —
// the relayer fee/gas/nonce fields are bounded uint256s.
func toBytes32(v *big.Int) []byte {
	if v.Sign() < 0 {
		panic("polyrelay: negative value in 32-byte field")
	}
	out := make([]byte, 32)
	if v.BitLen() > 256 {
		panic("polyrelay: value exceeds uint256")
	}
	v.FillBytes(out)
	return out
}

// ---------------------------------------------------------------------------
// Proxy scheme (POLY_PROXY)
// ---------------------------------------------------------------------------

// ProxyHash computes keccak256 over the packed proxy relay preimage
// ("rlx:" + from + to + data + fee + gasPrice + gasLimit + nonce + hub + relay).
func ProxyHash(
	from, to common.Address,
	data []byte,
	relayerFee, gasPrice, gasLimit, nonce *big.Int,
	relayHub, relay common.Address,
) common.Hash {
	var buf []byte
	buf = append(buf, proxyPrefix...)
	buf = append(buf, from.Bytes()...)
	buf = append(buf, to.Bytes()...)
	buf = append(buf, data...)
	buf = append(buf, toBytes32(relayerFee)...)
	buf = append(buf, toBytes32(gasPrice)...)
	buf = append(buf, toBytes32(gasLimit)...)
	buf = append(buf, toBytes32(nonce)...)
	buf = append(buf, relayHub.Bytes()...)
	buf = append(buf, relay.Bytes()...)
	return crypto.Keccak256Hash(buf)
}

// SignProxy signs the proxy transaction hash via EIP-191 personal sign.
// Returns a 0x-prefixed 65-byte signature.
func SignProxy(
	key *ecdsa.PrivateKey,
	from, to common.Address,
	data []byte,
	relayerFee, gasPrice, gasLimit, nonce *big.Int,
	relayHub, relay common.Address,
) (string, error) {
	if key == nil {
		return "", errors.New("polyrelay: nil private key")
	}
	h := ProxyHash(from, to, data, relayerFee, gasPrice, gasLimit, nonce, relayHub, relay)
	sig, err := signHash(key, personalHash(h.Bytes()))
	if err != nil {
		return "", err
	}
	return "0x" + common.Bytes2Hex(sig), nil
}

// ---------------------------------------------------------------------------
// Safe scheme (POLY_GNOSIS_SAFE)
// ---------------------------------------------------------------------------

// safeTypedData builds the EIP-712 SafeTx typed data. Gas fields are zeroed and
// gas/refund tokens are the zero address, matching the reference SDK exactly.
func safeTypedData(
	safeAddress, to common.Address,
	data []byte,
	value *big.Int,
	operation uint8,
	nonce *big.Int,
	chainID *big.Int,
) apitypes.TypedData {
	return apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": {
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
			"SafeTx": {
				{Name: "to", Type: "address"},
				{Name: "value", Type: "uint256"},
				{Name: "data", Type: "bytes"},
				{Name: "operation", Type: "uint8"},
				{Name: "safeTxGas", Type: "uint256"},
				{Name: "baseGas", Type: "uint256"},
				{Name: "gasPrice", Type: "uint256"},
				{Name: "gasToken", Type: "address"},
				{Name: "refundReceiver", Type: "address"},
				{Name: "nonce", Type: "uint256"},
			},
		},
		PrimaryType: "SafeTx",
		Domain: apitypes.TypedDataDomain{
			ChainId:           math.NewHexOrDecimal256(chainID.Int64()),
			VerifyingContract: safeAddress.Hex(),
		},
		Message: apitypes.TypedDataMessage{
			"to":             to.Hex(),
			"value":          value.String(),
			"data":           data,
			"operation":      fmt.Sprintf("%d", operation),
			"safeTxGas":      "0",
			"baseGas":        "0",
			"gasPrice":       "0",
			"gasToken":       zeroAddress.Hex(),
			"refundReceiver": zeroAddress.Hex(),
			"nonce":          nonce.String(),
		},
	}
}

// packSafeSignature rewrites the recovery byte to the Safe signature-type
// encoding consumed by the relayer: v in {0,1} -> +31, v in {27,28} -> +4.
func packSafeSignature(sig []byte) {
	v := sig[64]
	switch v {
	case 0, 1:
		sig[64] = v + 31
	case 27, 28:
		sig[64] = v + 4
	}
}

// SignSafe signs a Safe transaction. The EIP-712 SafeTx digest is EIP-191
// personal-signed (double-hashed), then the recovery byte is repacked.
func SignSafe(
	key *ecdsa.PrivateKey,
	safeAddress, to common.Address,
	data []byte,
	value *big.Int,
	operation uint8,
	nonce *big.Int,
	chainID *big.Int,
) (string, error) {
	if key == nil {
		return "", errors.New("polyrelay: nil private key")
	}
	td := safeTypedData(safeAddress, to, data, value, operation, nonce, chainID)
	digest, _, err := apitypes.TypedDataAndHash(td)
	if err != nil {
		return "", fmt.Errorf("polyrelay: safe typed-data digest: %w", err)
	}
	sig, err := signHash(key, personalHash(digest))
	if err != nil {
		return "", err
	}
	packSafeSignature(sig)
	return "0x" + common.Bytes2Hex(sig), nil
}

// ---------------------------------------------------------------------------
// DepositWallet scheme (Solady-wrapped wallet)
// ---------------------------------------------------------------------------

// DepositWalletTypedData builds the EIP-712 Batch typed data for a deposit
// wallet executing one or more calls.
func DepositWalletTypedData(
	wallet common.Address,
	calls []TransactionCall,
	nonce, deadline *big.Int,
	chainID *big.Int,
) apitypes.TypedData {
	callMessages := make([]apitypes.TypedDataMessage, len(calls))
	for i, c := range calls {
		callMessages[i] = apitypes.TypedDataMessage{
			"target": c.To.Hex(),
			"value":  c.Value.String(),
			"data":   c.Data,
		}
	}
	return apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": {
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
			"Batch": {
				{Name: "wallet", Type: "address"},
				{Name: "nonce", Type: "uint256"},
				{Name: "deadline", Type: "uint256"},
				{Name: "calls", Type: "Call[]"},
			},
			"Call": {
				{Name: "target", Type: "address"},
				{Name: "value", Type: "uint256"},
				{Name: "data", Type: "bytes"},
			},
		},
		PrimaryType: "Batch",
		Domain: apitypes.TypedDataDomain{
			Name:              depositDomainName,
			Version:           depositDomainVer,
			ChainId:           math.NewHexOrDecimal256(chainID.Int64()),
			VerifyingContract: wallet.Hex(),
		},
		Message: apitypes.TypedDataMessage{
			"wallet":   wallet.Hex(),
			"nonce":    nonce.String(),
			"deadline": deadline.String(),
			"calls":    callMessages,
		},
	}
}

// SignDepositWalletBatch signs a deposit-wallet batch via standard EIP-712
// (the digest is signed directly, not EIP-191-wrapped).
func SignDepositWalletBatch(
	key *ecdsa.PrivateKey,
	wallet common.Address,
	calls []TransactionCall,
	nonce, deadline *big.Int,
	chainID *big.Int,
) (string, error) {
	if key == nil {
		return "", errors.New("polyrelay: nil private key")
	}
	if len(calls) == 0 {
		return "", errors.New("polyrelay: deposit-wallet batch requires at least one call")
	}
	td := DepositWalletTypedData(wallet, calls, nonce, deadline, chainID)
	digest, _, err := apitypes.TypedDataAndHash(td)
	if err != nil {
		return "", fmt.Errorf("polyrelay: deposit-wallet typed-data digest: %w", err)
	}
	sig, err := signHash(key, digest)
	if err != nil {
		return "", err
	}
	return "0x" + common.Bytes2Hex(sig), nil
}
