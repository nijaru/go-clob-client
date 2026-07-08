package polyrelay

import (
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

// Vectors below are generated from py-clob-client's reference signing
// modules (polymarket._internal.actions.relayer.signing) using the well-known
// go-ethereum test key. RFC 6979 deterministic signing makes these byte-stable
// across eth-account/coincurve and go-ethereum, so exact equality is required.

const (
	// 0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c
	vectorKeyHex     = "4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c"
	vectorAddressHex = "0xF3F53fD15F3D5C773e84F1A1827c7ECdBC08ADA0"
)

func mustKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := crypto.HexToECDSA(vectorKeyHex)
	if err != nil {
		t.Fatalf("parse key: %v", err)
	}
	return key
}

func addr(hex string) common.Address {
	return common.HexToAddress(hex)
}

func TestKeyDerivesExpectedAddress(t *testing.T) {
	t.Parallel()
	key := mustKey(t)
	got := crypto.PubkeyToAddress(key.PublicKey)
	if want := addr(vectorAddressHex); got != want {
		t.Fatalf("address = %s, want %s (key mismatch invalidates all vectors)", got, want)
	}
}

func TestProxyHashAndSignature(t *testing.T) {
	t.Parallel()
	key := mustKey(t)

	to := addr("0x" + repeat("22", 20))
	relayHub := addr("0x" + repeat("33", 20))
	relay := addr("0x" + repeat("44", 20))
	data := common.FromHex("0xdeadbeef")

	h := ProxyHash(
		addr(vectorAddressHex), to, data,
		big.NewInt(0), big.NewInt(1_000_000_000), big.NewInt(200_000), big.NewInt(7),
		relayHub, relay,
	)
	const wantHash = "0x724980e017283107d97bc34596507eb89f94458fd06f643c319aec16274af0e8"
	if got := h.Hex(); got != wantHash {
		t.Fatalf("ProxyHash = %s, want %s", got, wantHash)
	}

	sig, err := SignProxy(
		key,
		addr(vectorAddressHex), to, data,
		big.NewInt(0), big.NewInt(1_000_000_000), big.NewInt(200_000), big.NewInt(7),
		relayHub, relay,
	)
	if err != nil {
		t.Fatalf("SignProxy: %v", err)
	}
	const wantSig = "0x8b392491af0374167422493cdad091816a4bbdda894a8da323676acbc5c2237f5e0f91dea8ed9c33f78084ee70e012b5c6a819e718a82704adbde87338529a211c"
	if sig != wantSig {
		t.Fatalf("SignProxy = %s, want %s", sig, wantSig)
	}
}

func TestSafeTypedDataDigest(t *testing.T) {
	t.Parallel()
	safe := addr("0x" + repeat("55", 20))
	to := addr("0x" + repeat("66", 20))
	data := common.FromHex("0xcafe")

	td := safeTypedData(safe, to, data, big.NewInt(0), 0, big.NewInt(3), big.NewInt(137))
	digest, _, err := apitypesTypedDataAndHash(t, td)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	const wantDigest = "0x4a06c869a005994d20e57056788a07266b3cc2e07476abd7333ede73f3d364a1"
	if got := common.BytesToHash(digest).Hex(); got != wantDigest {
		t.Fatalf("safe digest = %s, want %s", got, wantDigest)
	}
}

func TestSignSafe(t *testing.T) {
	t.Parallel()
	key := mustKey(t)
	safe := addr("0x" + repeat("55", 20))
	to := addr("0x" + repeat("66", 20))
	data := common.FromHex("0xcafe")

	sig, err := SignSafe(key, safe, to, data, big.NewInt(0), 0, big.NewInt(3), big.NewInt(137))
	if err != nil {
		t.Fatalf("SignSafe: %v", err)
	}
	const want = "0xb9f8579c8dd8b4fbb80099fa1879a8ac380da9388fffc047b09ea00675a192845282b4dd8417ee3508447a33db4dd4cfbd8f060517cc5df0c9c3c350d3efbd2020"
	if sig != want {
		t.Fatalf("SignSafe = %s, want %s", sig, want)
	}
}

func TestSignSafeRecoveryRepack(t *testing.T) {
	t.Parallel()
	// The reference Safe signature's final byte is always 0x1f (31) or 0x20 (32):
	// v in {27,28} -> +4. Assert the pack mapping directly.
	for _, in := range []byte{0, 1, 27, 28, 5, 99} {
		sig := make([]byte, 65)
		sig[64] = in
		packSafeSignature(sig)
		switch in {
		case 0, 1:
			if sig[64] != in+31 {
				t.Fatalf("pack(%d) = %d, want %d", in, sig[64], in+31)
			}
		case 27, 28:
			if sig[64] != in+4 {
				t.Fatalf("pack(%d) = %d, want %d", in, sig[64], in+4)
			}
		default:
			if sig[64] != in {
				t.Fatalf("pack(%d) = %d, want %d (unchanged)", in, sig[64], in)
			}
		}
	}
}

func TestDepositWalletTypedDataDigest(t *testing.T) {
	t.Parallel()
	wallet := addr("0x" + repeat("77", 20))
	calls := []TransactionCall{
		{To: addr("0x" + repeat("88", 20)), Data: []byte{}, Value: big.NewInt(0)},
		{To: addr("0x" + repeat("99", 20)), Data: common.FromHex("0xaabbcc"), Value: big.NewInt(1_000_000)},
	}

	td := DepositWalletTypedData(wallet, calls, big.NewInt(5), big.NewInt(1_700_000_000), big.NewInt(137))
	digest, _, err := apitypesTypedDataAndHash(t, td)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	const wantDigest = "0x88e19ae7c7f5990d5152240660d62863bf9572df60e2134cf6020c4b22c5df0a"
	if got := common.BytesToHash(digest).Hex(); got != wantDigest {
		t.Fatalf("deposit digest = %s, want %s", got, wantDigest)
	}
}

func TestSignDepositWalletBatch(t *testing.T) {
	t.Parallel()
	key := mustKey(t)
	wallet := addr("0x" + repeat("77", 20))
	calls := []TransactionCall{
		{To: addr("0x" + repeat("88", 20)), Data: []byte{}, Value: big.NewInt(0)},
		{To: addr("0x" + repeat("99", 20)), Data: common.FromHex("0xaabbcc"), Value: big.NewInt(1_000_000)},
	}

	sig, err := SignDepositWalletBatch(key, wallet, calls, big.NewInt(5), big.NewInt(1_700_000_000), big.NewInt(137))
	if err != nil {
		t.Fatalf("SignDepositWalletBatch: %v", err)
	}
	const want = "0x6b90fc85210127e17af28e6f48fcaf866b61d2024d8f054d04a14dc5eeb0366170b25069fd496c4a6a8aeb40d0926a29e761c311a77325503df1b815cba95a671c"
	if sig != want {
		t.Fatalf("SignDepositWalletBatch = %s, want %s", sig, want)
	}
}

func TestSignDepositWalletBatchRejectsEmpty(t *testing.T) {
	t.Parallel()
	key := mustKey(t)
	wallet := addr("0x" + repeat("77", 20))
	if _, err := SignDepositWalletBatch(key, wallet, nil, big.NewInt(1), big.NewInt(2), big.NewInt(137)); err == nil {
		t.Fatal("expected error for empty call batch, got nil")
	}
}

func TestNilKeyRejected(t *testing.T) {
	t.Parallel()
	wallet := addr("0x" + repeat("77", 20))
	calls := []TransactionCall{{To: wallet, Data: []byte{1}, Value: big.NewInt(0)}}
	checks := []struct {
		name string
		fn   func() (string, error)
	}{
		{"proxy", func() (string, error) {
			return SignProxy(nil, wallet, wallet, []byte{1}, big.NewInt(0), big.NewInt(0), big.NewInt(0), big.NewInt(0), wallet, wallet)
		}},
		{"safe", func() (string, error) {
			return SignSafe(nil, wallet, wallet, []byte{1}, big.NewInt(0), 0, big.NewInt(0), big.NewInt(137))
		}},
		{"deposit", func() (string, error) {
			return SignDepositWalletBatch(nil, wallet, calls, big.NewInt(0), big.NewInt(0), big.NewInt(137))
		}},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if _, err := c.fn(); err == nil {
				t.Fatalf("expected error for nil key, got nil")
			}
		})
	}
}

// repeat builds a hex string of n doubled chars (e.g. repeat("22",20) -> "22"*20).
func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

// apitypesTypedDataAndHash is a thin test wrapper around go-ethereum's
// EIP-712 digest helper, kept here to avoid importing apitypes in every digest
// test body.
func apitypesTypedDataAndHash(t *testing.T, td apitypes.TypedData) ([]byte, []byte, error) {
	t.Helper()
	digest, rawData, err := apitypes.TypedDataAndHash(td)
	return digest, []byte(rawData), err
}
