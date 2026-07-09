package polyrelay

import (
	"crypto/ecdsa"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Shared test helpers used by signer_test, encode_test, and payload_test.

// vectorKeyHex is the canonical go-ethereum example key; its deterministic
// (RFC 6979) signatures are the parity vectors in signer_test.go.
const vectorKeyHex = "4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c"

func mustKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := crypto.HexToECDSA(vectorKeyHex)
	if err != nil {
		t.Fatalf("parse key: %v", err)
	}
	return key
}

// addrRepeat returns an address filled with a single repeated byte.
func addrRepeat(b byte) common.Address {
	var a [20]byte
	for i := range a {
		a[i] = b
	}
	return common.Address(a)
}

// sigHex formats a raw signature as 0x-prefixed hex.
func sigHex(sig []byte) string { return "0x" + common.Bytes2Hex(sig) }
