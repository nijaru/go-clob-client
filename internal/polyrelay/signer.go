package polyrelay

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

// Each scheme produces a 65-byte secp256k1 signature deterministically
// (RFC 6979), matching eth-account/coincurve and ts-sdk's ox output for
// identical inputs. See signer_test.go for byte-exact parity vectors.
const (
	proxyPrefix    = "rlx:"          // legacy proxy relay preimage prefix
	depositDomName = "DepositWallet" // Solady deposit-wallet EIP-712 domain
	depositDomVer  = "1"
)

// scheme describes how a wallet family turns a request into a signature:
// the digest to sign, whether the digest is EIP-191-wrapped (personal-signed)
// before signing, and an optional recovery-byte repack.
type scheme struct {
	digest func(*RelayRequest) ([]byte, error)
	wrap   bool // EIP-191 personal-sign over the digest (double-hash)
	pack   func(sig []byte)
}

// schemes is the single dispatch table. Adding a wallet family means adding a
// row here — callers never switch on transaction type.
var schemes = map[RelayerTransactionType]scheme{
	TransactionTypeProxy:  {digest: proxyDigest, wrap: true},
	TransactionTypeSafe:   {digest: safeDigest, wrap: true, pack: packSafeSignature},
	TransactionTypeWallet: {digest: depositDigest, wrap: false},
}

// Sign produces the relayer signature for req under the given wallet type.
// The returned signature is a raw 65-byte value (recovery byte in {27,28} or
// its scheme-specific repacked form); use HexSignature at the JSON boundary.
func Sign(txType RelayerTransactionType, key *ecdsa.PrivateKey, req RelayRequest) ([]byte, error) {
	if key == nil {
		return nil, ErrNilKey
	}
	sch, ok := schemes[txType]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownType, txType)
	}
	digest, err := sch.digest(&req)
	if err != nil {
		return nil, err
	}
	if sch.wrap {
		digest = personalHash(digest)
	}
	sig, err := signHash(key, digest)
	if err != nil {
		return nil, err
	}
	if sch.pack != nil {
		sch.pack(sig)
	}
	return sig, nil
}

// HexSignature formats a raw 65-byte signature as 0x-prefixed hex for the
// relayer's JSON submit payload.
func HexSignature(sig []byte) (string, error) {
	if len(sig) != 65 {
		return "", fmt.Errorf("polyrelay: signature must be 65 bytes, got %d", len(sig))
	}
	return "0x" + common.Bytes2Hex(sig), nil
}

// ---------------------------------------------------------------------------
// Shared primitives
// ---------------------------------------------------------------------------

// personalHash returns the EIP-191 "personal sign" digest of msg
// (keccak256 of "\x19Ethereum Signed Message:\n<len>" + msg). It builds one
// buffer sized to header + decimal length + message.
func personalHash(msg []byte) []byte {
	buf := make([]byte, 0, len(eip191Header)+4+len(msg))
	buf = append(buf, eip191Header...)
	buf = strconv.AppendInt(buf, int64(len(msg)), 10)
	buf = append(buf, msg...)
	return crypto.Keccak256(buf)
}

const eip191Header = "\x19Ethereum Signed Message:\n"

// signHash signs a 32-byte digest with key, returning a 65-byte signature with
// the recovery byte normalized to {27,28}.
func signHash(key *ecdsa.PrivateKey, digest []byte) ([]byte, error) {
	sig, err := crypto.Sign(digest, key)
	if err != nil {
		return nil, fmt.Errorf("polyrelay: sign: %w", err)
	}
	sig[64] += 27
	return sig, nil
}

// pad32 writes v into a 32-byte big-endian buffer (right-justified via
// FillBytes). Returns a typed error for out-of-range caller input.
func pad32(v *big.Int) ([]byte, error) {
	if v == nil {
		return nil, ErrNilValue
	}
	if v.Sign() < 0 {
		return nil, fmt.Errorf("%w: %s", ErrNegativeValue, v.String())
	}
	if v.BitLen() > 256 {
		return nil, fmt.Errorf("%w: %s", ErrOverflow, v.String())
	}
	out := make([]byte, 32)
	v.FillBytes(out)
	return out, nil
}

// ---------------------------------------------------------------------------
// PROXY scheme: keccak256 of a packed preimage, personal-signed
// ---------------------------------------------------------------------------

func proxyDigest(req *RelayRequest) ([]byte, error) {
	fee, err := pad32(req.GasFee)
	if err != nil {
		return nil, err
	}
	price, err := pad32(req.GasPrice)
	if err != nil {
		return nil, err
	}
	limit, err := pad32(req.GasLimit)
	if err != nil {
		return nil, err
	}
	nonce, err := pad32(req.Nonce)
	if err != nil {
		return nil, err
	}
	// Pre-size the buffer: prefix(4) + 4 addresses(80) + 4 uint256(128) + data.
	buf := make([]byte, 0, len(proxyPrefix)+80+128+len(req.Data))
	buf = append(buf, proxyPrefix...)
	buf = append(buf, req.Signer.Bytes()...)
	buf = append(buf, req.To.Bytes()...)
	buf = append(buf, req.Data...)
	buf = append(buf, fee...)
	buf = append(buf, price...)
	buf = append(buf, limit...)
	buf = append(buf, nonce...)
	buf = append(buf, req.RelayHub.Bytes()...)
	buf = append(buf, req.Relay.Bytes()...)
	return crypto.Keccak256(buf), nil
}

// ---------------------------------------------------------------------------
// SAFE scheme: EIP-712 SafeTx, EIP-191 double-hashed, recovery byte repacked
// ---------------------------------------------------------------------------

func safeDigest(req *RelayRequest) ([]byte, error) {
	td := apitypes.TypedData{
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
			ChainId:           (*math.HexOrDecimal256)(req.ChainID),
			VerifyingContract: req.Wallet.Hex(),
		},
		Message: apitypes.TypedDataMessage{
			"to":             req.To.Hex(),
			"value":          req.Value.String(),
			"data":           req.Data,
			"operation":      strconv.FormatUint(uint64(req.Operation), 10),
			"safeTxGas":      "0",
			"baseGas":        "0",
			"gasPrice":       "0",
			"gasToken":       common.Address{}.Hex(),
			"refundReceiver": common.Address{}.Hex(),
			"nonce":          req.Nonce.String(),
		},
	}
	digest, _, err := apitypes.TypedDataAndHash(td)
	if err != nil {
		return nil, fmt.Errorf("polyrelay: safe digest: %w", err)
	}
	return digest, nil
}

// packSafeSignature rewrites the recovery byte to the Safe signature-type
// encoding consumed by the relayer: v in {0,1} -> +31, v in {27,28} -> +4.
func packSafeSignature(sig []byte) {
	switch sig[64] {
	case 0, 1:
		sig[64] += 31
	case 27, 28:
		sig[64] += 4
	}
}

// ---------------------------------------------------------------------------
// WALLET (deposit) scheme: EIP-712 Batch, signed directly (no double-hash)
// ---------------------------------------------------------------------------

func depositDigest(req *RelayRequest) ([]byte, error) {
	if len(req.Calls) == 0 {
		return nil, ErrEmptyBatch
	}
	calls := make([]apitypes.TypedDataMessage, len(req.Calls))
	for i, c := range req.Calls {
		calls[i] = apitypes.TypedDataMessage{
			"target": c.To.Hex(),
			"value":  c.Value.String(),
			"data":   c.Data,
		}
	}
	td := apitypes.TypedData{
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
			Name:              depositDomName,
			Version:           depositDomVer,
			ChainId:           (*math.HexOrDecimal256)(req.ChainID),
			VerifyingContract: req.Wallet.Hex(),
		},
		Message: apitypes.TypedDataMessage{
			"wallet":   req.Wallet.Hex(),
			"nonce":    req.Nonce.String(),
			"deadline": req.Deadline.String(),
			"calls":    calls,
		},
	}
	digest, _, err := apitypes.TypedDataAndHash(td)
	if err != nil {
		return nil, fmt.Errorf("polyrelay: deposit digest: %w", err)
	}
	return digest, nil
}
