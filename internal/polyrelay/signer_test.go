package polyrelay

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// Vectors are generated from py-clob-client's reference signing modules and
// cross-checked against ts-sdk's ox-based path; the two official SDKs produce
// identical typed-data structures and signing semantics. RFC 6979 deterministic
// signing makes eth-account/coincurve, ox, and go-ethereum byte-stable, so
// exact equality is required.

const vectorAddressHex = "0xF3F53fD15F3D5C773e84F1A1827c7ECdBC08ADA0"

func TestKeyDerivesExpectedAddress(t *testing.T) {
	t.Parallel()
	got := crypto.PubkeyToAddress(mustKey(t).PublicKey)
	if want := common.HexToAddress(vectorAddressHex); got != want {
		t.Fatalf("address = %s, want %s (key mismatch invalidates all vectors)", got, want)
	}
}

// --- PROXY: keccak-packed preimage, personal-signed ---

func TestProxyDigestAndSign(t *testing.T) {
	t.Parallel()
	req := RelayRequest{
		Signer:   common.HexToAddress(vectorAddressHex),
		To:       addrRepeat(0x22),
		Data:     common.FromHex("0xdeadbeef"),
		GasFee:   big.NewInt(0),
		GasPrice: big.NewInt(1_000_000_000),
		GasLimit: big.NewInt(200_000),
		Nonce:    big.NewInt(7),
		RelayHub: addrRepeat(0x33),
		Relay:    addrRepeat(0x44),
	}

	digest, err := proxyDigest(&req)
	if err != nil {
		t.Fatalf("proxyDigest: %v", err)
	}
	const wantHash = "0x724980e017283107d97bc34596507eb89f94458fd06f643c319aec16274af0e8"
	if got := common.BytesToHash(digest).Hex(); got != wantHash {
		t.Fatalf("proxy digest = %s, want %s", got, wantHash)
	}

	sig, err := Sign(TransactionTypeProxy, mustKey(t), req)
	if err != nil {
		t.Fatalf("Sign PROXY: %v", err)
	}
	const wantSig = "0x8b392491af0374167422493cdad091816a4bbdda894a8da323676acbc5c2237f5e0f91dea8ed9c33f78084ee70e012b5c6a819e718a82704adbde87338529a211c"
	if got := sigHex(sig); got != wantSig {
		t.Fatalf("Sign PROXY = %s, want %s", got, wantSig)
	}
}

// --- SAFE: EIP-712 SafeTx, double-hashed, recovery byte repacked ---

func TestSafeDigestAndSign(t *testing.T) {
	t.Parallel()
	req := RelayRequest{
		Wallet:    addrRepeat(0x55),
		To:        addrRepeat(0x66),
		Data:      common.FromHex("0xcafe"),
		Value:     big.NewInt(0),
		Operation: 0,
		Nonce:     big.NewInt(3),
		ChainID:   big.NewInt(137),
	}

	digest, err := safeDigest(&req)
	if err != nil {
		t.Fatalf("safeDigest: %v", err)
	}
	const wantDigest = "0x4a06c869a005994d20e57056788a07266b3cc2e07476abd7333ede73f3d364a1"
	if got := common.BytesToHash(digest).Hex(); got != wantDigest {
		t.Fatalf("safe digest = %s, want %s", got, wantDigest)
	}

	sig, err := Sign(TransactionTypeSafe, mustKey(t), req)
	if err != nil {
		t.Fatalf("Sign SAFE: %v", err)
	}
	const wantSig = "0xb9f8579c8dd8b4fbb80099fa1879a8ac380da9388fffc047b09ea00675a192845282b4dd8417ee3508447a33db4dd4cfbd8f060517cc5df0c9c3c350d3efbd2020"
	if got := sigHex(sig); got != wantSig {
		t.Fatalf("Sign SAFE = %s, want %s", got, wantSig)
	}
	if v := sig[64]; v != 0x20 && v != 0x1f {
		t.Fatalf("SAFE recovery byte = 0x%x, want 0x1f or 0x20 (repacked)", v)
	}
}

func TestPackSafeSignature(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want byte }{
		{0, 31}, {1, 32}, {27, 31}, {28, 32}, {5, 5}, {99, 99},
	}
	for _, c := range cases {
		sig := make([]byte, 65)
		sig[64] = c.in
		packSafeSignature(sig)
		if sig[64] != c.want {
			t.Fatalf("pack(%d) = %d, want %d", c.in, sig[64], c.want)
		}
	}
}

// --- WALLET (deposit): EIP-712 Batch, signed directly ---

func TestDepositDigestAndSign(t *testing.T) {
	t.Parallel()
	req := RelayRequest{
		Wallet: addrRepeat(0x77),
		Calls: []TransactionCall{
			{To: addrRepeat(0x88), Data: nil, Value: big.NewInt(0)},
			{To: addrRepeat(0x99), Data: common.FromHex("0xaabbcc"), Value: big.NewInt(1_000_000)},
		},
		Nonce:    big.NewInt(5),
		Deadline: big.NewInt(1_700_000_000),
		ChainID:  big.NewInt(137),
	}

	digest, err := depositDigest(&req)
	if err != nil {
		t.Fatalf("depositDigest: %v", err)
	}
	const wantDigest = "0x88e19ae7c7f5990d5152240660d62863bf9572df60e2134cf6020c4b22c5df0a"
	if got := common.BytesToHash(digest).Hex(); got != wantDigest {
		t.Fatalf("deposit digest = %s, want %s", got, wantDigest)
	}

	sig, err := Sign(TransactionTypeWallet, mustKey(t), req)
	if err != nil {
		t.Fatalf("Sign WALLET: %v", err)
	}
	const wantSig = "0x6b90fc85210127e17af28e6f48fcaf866b61d2024d8f054d04a14dc5eeb0366170b25069fd496c4a6a8aeb40d0926a29e761c311a77325503df1b815cba95a671c"
	if got := sigHex(sig); got != wantSig {
		t.Fatalf("Sign WALLET = %s, want %s", got, wantSig)
	}
}

// --- error paths ---

func TestSignValidatesInput(t *testing.T) {
	t.Parallel()
	key := mustKey(t)

	t.Run("nil key", func(t *testing.T) {
		t.Parallel()
		_, err := Sign(TransactionTypeWallet, nil, RelayRequest{})
		if !errors.Is(err, ErrNilKey) {
			t.Fatalf("err = %v, want ErrNilKey", err)
		}
	})

	t.Run("unknown type", func(t *testing.T) {
		t.Parallel()
		_, err := Sign("NOPE", key, RelayRequest{})
		if !errors.Is(err, ErrUnknownType) {
			t.Fatalf("err = %v, want ErrUnknownType", err)
		}
	})

	t.Run("empty deposit batch", func(t *testing.T) {
		t.Parallel()
		req := RelayRequest{
			Wallet:   addrRepeat(0x77),
			Nonce:    big.NewInt(1),
			Deadline: big.NewInt(2),
			ChainID:  big.NewInt(137),
		}
		_, err := Sign(TransactionTypeWallet, key, req)
		if !errors.Is(err, ErrEmptyBatch) {
			t.Fatalf("err = %v, want ErrEmptyBatch", err)
		}
	})

	t.Run("proxy negative gas fee", func(t *testing.T) {
		t.Parallel()
		req := RelayRequest{
			Signer: addrRepeat(0x11), To: addrRepeat(0x22),
			GasFee: big.NewInt(
				-1,
			), GasPrice: big.NewInt(0), GasLimit: big.NewInt(0), Nonce: big.NewInt(0),
			RelayHub: addrRepeat(0x33), Relay: addrRepeat(0x44),
		}
		_, err := Sign(TransactionTypeProxy, key, req)
		if !errors.Is(err, ErrNegativeValue) {
			t.Fatalf("err = %v, want ErrNegativeValue", err)
		}
	})

	t.Run("proxy overflowing nonce", func(t *testing.T) {
		t.Parallel()
		tooBig := new(big.Int).Lsh(big.NewInt(1), 257) // > uint256
		req := RelayRequest{
			Signer: addrRepeat(0x11), To: addrRepeat(0x22),
			GasFee: big.NewInt(0), GasPrice: big.NewInt(0), GasLimit: big.NewInt(0), Nonce: tooBig,
			RelayHub: addrRepeat(0x33), Relay: addrRepeat(0x44),
		}
		_, err := Sign(TransactionTypeProxy, key, req)
		if !errors.Is(err, ErrOverflow) {
			t.Fatalf("err = %v, want ErrOverflow", err)
		}
	})

	t.Run("proxy nil gas field", func(t *testing.T) {
		t.Parallel()
		req := RelayRequest{
			Signer: addrRepeat(0x11), To: addrRepeat(0x22),
			GasFee: nil, GasPrice: big.NewInt(0), GasLimit: big.NewInt(0), Nonce: big.NewInt(0),
			RelayHub: addrRepeat(0x33), Relay: addrRepeat(0x44),
		}
		_, err := Sign(TransactionTypeProxy, key, req)
		if !errors.Is(err, ErrNilValue) {
			t.Fatalf("err = %v, want ErrNilValue", err)
		}
	})

	t.Run("deposit call nil value", func(t *testing.T) {
		t.Parallel()
		// A caller passing a 0-value call (the common case for approvals/transfers)
		// without explicitly setting Value must get a typed error, not a nil-deref
		// panic. This is the WALLET equivalent of the proxy nil-field check above;
		// the SAFE path guards it in resolveSafeCall.
		req := RelayRequest{
			Wallet: addrRepeat(0x77),
			Calls: []TransactionCall{
				{To: addrRepeat(0x88), Data: []byte{0x01}}, // Value unset (nil)
				{To: addrRepeat(0x99), Data: []byte{0x02}, Value: big.NewInt(1)},
			},
			Nonce: big.NewInt(1), Deadline: big.NewInt(2), ChainID: big.NewInt(137),
		}
		_, err := Sign(TransactionTypeWallet, key, req)
		if !errors.Is(err, ErrNilValue) {
			t.Fatalf("err = %v, want ErrNilValue", err)
		}
	})
}

func TestHexSignature(t *testing.T) {
	t.Parallel()
	if _, err := HexSignature(make([]byte, 64)); err == nil {
		t.Fatal("expected error for 64-byte signature, got nil")
	}
	h, err := HexSignature(make([]byte, 65))
	if err != nil {
		t.Fatalf("HexSignature: %v", err)
	}
	if len(h) != 132 || h[:2] != "0x" {
		t.Fatalf("HexSignature = %q (len %d), want 0x + 130 hex chars (132 total)", h, len(h))
	}
}
