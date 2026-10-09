package polyrelay

import (
	"context"
	"fmt"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/nijaru/go-clob-client/signing"
)

// The local adapter uses RFC 6979, matching eth-account/coincurve and
// ts-sdk's ox output. External signers need not be deterministic; all signatures
// must recover the pinned EOA. See signer_test.go for byte-exact local vectors.
const (
	proxyPrefix    = "rlx:"          // legacy proxy relay preimage prefix
	depositDomName = "DepositWallet" // Solady deposit-wallet EIP-712 domain
	depositDomVer  = "1"
)

// Sign verifies the EOA signature before scheme-specific wrapping. Safe and
// Proxy require personal signing; deposit wallets require structured EIP-712.
func Sign(
	ctx context.Context,
	txType RelayerTransactionType,
	signer *signing.Wallet,
	req RelayRequest,
) ([]byte, error) {
	if signer == nil {
		return nil, ErrNilSigner
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if txType == TransactionTypeWallet {
		data, err := depositTypedData(&req)
		if err != nil {
			return nil, err
		}
		return signer.SignTypedData(ctx, data)
	}
	var digest []byte
	var err error
	switch txType {
	case TransactionTypeProxy:
		digest, err = proxyDigest(&req)
	case TransactionTypeSafe:
		digest, err = safeDigest(&req)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownType, txType)
	}
	if err != nil {
		return nil, err
	}
	if req.Signer != (common.Address{}) && req.Signer != signer.Address() {
		return nil, fmt.Errorf("polyrelay: signer address mismatch")
	}
	sig, err := signer.SignMessage(ctx, digest)
	if err != nil {
		return nil, err
	}
	if txType == TransactionTypeSafe {
		packSafeSignature(sig)
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
	if req.ChainID == nil || req.Value == nil || req.Nonce == nil {
		return nil, fmt.Errorf("%w: safe digest requires chainId, value, nonce", ErrNilValue)
	}
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

func depositTypedData(req *RelayRequest) (apitypes.TypedData, error) {
	if len(req.Calls) == 0 {
		return apitypes.TypedData{}, ErrEmptyBatch
	}
	if req.ChainID == nil || req.Nonce == nil || req.Deadline == nil {
		return apitypes.TypedData{}, fmt.Errorf(
			"%w: deposit digest requires chainId, nonce, deadline",
			ErrNilValue,
		)
	}
	calls := make([]apitypes.TypedDataMessage, len(req.Calls))
	for i, c := range req.Calls {
		if c.Value == nil {
			return apitypes.TypedData{}, fmt.Errorf("%w: call %d value", ErrNilValue, i)
		}
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
	return td, nil
}
