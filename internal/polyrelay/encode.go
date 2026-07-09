package polyrelay

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Relay calldata is produced by bundling one or more TransactionCalls into a
// single encoded blob that the wallet's relay entrypoint (proxy factory or Safe
// multiSend) unpacks and dispatches. Two shapes mirror the reference SDKs:
//
//   - PROXY: proxy((uint8,address,uint256,bytes)[]) — each call wrapped as a
//     typed tuple with callType=1.
//   - SAFE: multiSend(bytes) — calls packed into the Safe multiSend inner form
//     (operation + to + value + length + data), then wrapped as bytes.

const proxyCallType uint8 = 1

// 4-byte selectors, stable for the lifetime of the contract signatures.
var (
	proxyFactorySel  = crypto.Keccak256([]byte("proxy((uint8,address,uint256,bytes)[])"))[:4]
	safeMultisendSel = crypto.Keccak256([]byte("multiSend(bytes)"))[:4]

	// ABI type literals are programmer-controlled invariants; a malformed
	// literal is a bug and fails fast at init (regexp.MustCompile pattern).
	proxyTupleType = mustNewType("tuple[]", "", []abi.ArgumentMarshaling{
		{Name: "callType", Type: "uint8"},
		{Name: "to", Type: "address"},
		{Name: "value", Type: "uint256"},
		{Name: "data", Type: "bytes"},
	})
	safeBytesType = mustNewType("bytes", "", nil)
)

func mustNewType(name, raw string, components []abi.ArgumentMarshaling) abi.Type {
	t, err := abi.NewType(name, raw, components)
	if err != nil {
		panic("polyrelay: abi.NewType(" + name + "): " + err.Error())
	}
	return t
}

// proxyTuple is the on-chain shape of a single proxied call.
type proxyTuple struct {
	CallType uint8          `abi:"callType"`
	To       common.Address `abi:"to"`
	Value    *big.Int       `abi:"value"`
	Data     []byte         `abi:"data"`
}

// EncodeProxyCall bundles calls into proxy-factory calldata:
// selector + ABI-encoded (uint8,address,uint256,bytes)[] with callType=1.
func EncodeProxyCall(calls []TransactionCall) ([]byte, error) {
	if len(calls) == 0 {
		return nil, ErrEmptyBatch
	}
	tuples := make([]proxyTuple, len(calls))
	for i, c := range calls {
		if c.Value == nil {
			return nil, fmt.Errorf("%w: call %d value", ErrNilValue, i)
		}
		tuples[i] = proxyTuple{CallType: proxyCallType, To: c.To, Value: c.Value, Data: c.Data}
	}
	body, err := abi.Arguments{{Type: proxyTupleType}}.Pack(tuples)
	if err != nil {
		return nil, fmt.Errorf("polyrelay: encode proxy call: %w", err)
	}
	return withSelector(proxyFactorySel, body), nil
}

// EncodeSafeMultisendCall bundles calls into Safe multiSend calldata:
// selector + ABI-encoded bytes, where the bytes are the multiSend inner stream
// (0x00 + to + value + length + data per call).
func EncodeSafeMultisendCall(calls []TransactionCall) ([]byte, error) {
	if len(calls) == 0 {
		return nil, ErrEmptyBatch
	}
	inner := make([]byte, 0, safeMultisendInnerSize(calls))
	var lenBuf [32]byte
	for _, c := range calls {
		value, err := pad32(c.Value)
		if err != nil {
			return nil, err
		}
		inner = append(inner, 0) // operation: CALL
		inner = append(inner, c.To.Bytes()...)
		inner = append(inner, value...)
		big.NewInt(int64(len(c.Data))).FillBytes(lenBuf[:])
		inner = append(inner, lenBuf[:]...)
		inner = append(inner, c.Data...)
	}
	body, err := abi.Arguments{{Type: safeBytesType}}.Pack(inner)
	if err != nil {
		return nil, fmt.Errorf("polyrelay: encode safe multisend: %w", err)
	}
	return withSelector(safeMultisendSel, body), nil
}

// safeMultisendInnerSize pre-sizes the buffer: 1 (op) + 20 (to) + 32 (value) +
// 32 (length) + len(data) per call.
func safeMultisendInnerSize(calls []TransactionCall) int {
	n := 0
	for _, c := range calls {
		n += 85 + len(c.Data)
	}
	return n
}

// withSelector returns a new slice = sel ++ body.
func withSelector(sel, body []byte) []byte {
	out := make([]byte, 0, len(sel)+len(body))
	out = append(out, sel...)
	out = append(out, body...)
	return out
}
