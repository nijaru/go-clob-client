package signing

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

const testKey = "4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c"

type callbackSigner struct {
	address common.Address
	typed   func(context.Context, apitypes.TypedData) ([]byte, error)
}

func (s *callbackSigner) Address() common.Address { return s.address }

func (s *callbackSigner) SignTypedData(
	ctx context.Context,
	data apitypes.TypedData,
) ([]byte, error) {
	return s.typed(ctx, data)
}

func testData() apitypes.TypedData {
	return apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": {{Name: "name", Type: "string"}},
			"Test":         {{Name: "value", Type: "uint256"}},
		},
		PrimaryType: "Test", Domain: apitypes.TypedDataDomain{Name: "test"},
		Message: apitypes.TypedDataMessage{"value": "1"},
	}
}

func local(t *testing.T) *LocalSigner {
	t.Helper()
	s, err := NewLocalSigner(testKey)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func wallet(t *testing.T, s Signer) *Wallet {
	t.Helper()
	w, err := NewWallet(s)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestTypedSignatureValidation(t *testing.T) {
	t.Parallel()
	source := local(t)
	other, err := NewLocalSigner("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func([]byte) []byte
		signer *LocalSigner
		valid  bool
	}{
		{name: "v01", signer: source, valid: true},
		{
			name:   "v2728",
			signer: source,
			valid:  true,
			mutate: func(sig []byte) []byte { sig[64] += 27; return sig },
		},
		{name: "wrong EOA", signer: other},
		{name: "short", signer: source, mutate: func(sig []byte) []byte { return sig[:64] }},
		{
			name:   "invalid V",
			signer: source,
			mutate: func(sig []byte) []byte { sig[64] = 29; return sig },
		},
		{
			name:   "zero R",
			signer: source,
			mutate: func(sig []byte) []byte { clear(sig[:32]); return sig },
		},
		{name: "high S", signer: source, mutate: func(sig []byte) []byte {
			high := new(big.Int).Sub(crypto.S256().Params().N, new(big.Int).SetBytes(sig[32:64]))
			high.FillBytes(sig[32:64])
			sig[64] ^= 1
			return sig
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var raw []byte
			s := &callbackSigner{
				address: source.Address(),
				typed: func(ctx context.Context, data apitypes.TypedData) ([]byte, error) {
					sig, err := tc.signer.SignTypedData(ctx, data)
					if tc.mutate != nil {
						sig = tc.mutate(sig)
					}
					raw = sig
					return sig, err
				},
			}
			w := wallet(t, s)
			// Address changes on a device must not silently change the client's identity.
			s.address = other.Address()
			sig, err := w.SignTypedData(t.Context(), testData())
			if !tc.valid {
				if !errors.Is(err, ErrInvalidSignature) {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil || len(sig) != 65 || (sig[64] != 27 && sig[64] != 28) {
				t.Fatalf("signature = %x, error = %v", sig, err)
			}
			if w.Address() != source.Address() {
				t.Fatal("address was not pinned")
			}
			original := sig[0]
			raw[0] ^= 1
			if sig[0] != original {
				t.Fatal("returned signature aliases device buffer")
			}
		})
	}
}

func TestCapabilitiesAndCancellation(t *testing.T) {
	t.Parallel()
	source := local(t)
	var calls int
	ctx, cancel := context.WithCancel(t.Context())
	s := &callbackSigner{
		address: source.Address(),
		typed: func(ctx context.Context, data apitypes.TypedData) ([]byte, error) {
			calls++
			sig, err := source.SignTypedData(ctx, data)
			cancel()
			return sig, err
		},
	}
	w := wallet(t, s)
	if _, err := w.SignTypedData(ctx, testData()); !errors.Is(err, context.Canceled) {
		t.Fatalf("late cancellation = %v", err)
	}
	if _, err := w.SignTypedData(ctx, testData()); !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("pre cancellation = %v, calls = %d", err, calls)
	}
	if _, err := w.SignMessage(t.Context(), []byte("hello")); !errors.Is(
		err,
		ErrMessageSigningUnsupported,
	) {
		t.Fatal(err)
	}
	if _, err := w.SignTransaction(t.Context(), big.NewInt(137), testTransaction(0)); !errors.Is(
		err,
		ErrTransactionSigningUnsupported,
	) {
		t.Fatal(err)
	}
	if _, err := NewWallet(nil); !errors.Is(err, ErrInvalidSigner) {
		t.Fatal(err)
	}
	var absent *callbackSigner
	if _, err := NewWallet(absent); !errors.Is(err, ErrInvalidSigner) {
		t.Fatal(err)
	}
	if _, err := NewWallet(&callbackSigner{}); !errors.Is(err, ErrInvalidSigner) {
		t.Fatal(err)
	}
}

type txSigner struct {
	*LocalSigner
	sign func(context.Context, *big.Int, *types.Transaction) (*types.Transaction, error)
}

func (s txSigner) SignTransaction(
	ctx context.Context,
	chain *big.Int,
	tx *types.Transaction,
) (*types.Transaction, error) {
	return s.sign(ctx, chain, tx)
}

func testTransaction(nonce uint64) *types.Transaction {
	to := common.HexToAddress("0x1111111111111111111111111111111111111111")
	return types.NewTx(
		&types.DynamicFeeTx{
			ChainID:   big.NewInt(137),
			Nonce:     nonce,
			GasTipCap: big.NewInt(1),
			GasFeeCap: big.NewInt(2),
			Gas:       21000,
			To:        &to,
			Value:     big.NewInt(10),
			Data:      []byte{1},
		},
	)
}

func TestTransactionVerification(t *testing.T) {
	t.Parallel()
	source := local(t)
	other, _ := NewLocalSigner("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	for _, name := range []string{"valid", "payload", "chain", "sender", "nil"} {
		t.Run(name, func(t *testing.T) {
			s := txSigner{
				LocalSigner: source,
				sign: func(ctx context.Context, chain *big.Int, tx *types.Transaction) (*types.Transaction, error) {
					key := source
					switch name {
					case "payload":
						tx = testTransaction(1)
					case "chain":
						chain.SetInt64(1)
					case "sender":
						key = other
					case "nil":
						return nil, nil
					}
					// SignTx refuses typed transactions for a different chain. A legacy tx
					// makes the wrong-chain case exercise our verification, not SignTx's guard.
					if name == "chain" {
						tx = types.NewTransaction(
							0,
							*tx.To(),
							tx.Value(),
							tx.Gas(),
							tx.GasPrice(),
							tx.Data(),
						)
					}
					return key.SignTransaction(ctx, chain, tx)
				},
			}
			chain := big.NewInt(137)
			signed, err := wallet(t, s).SignTransaction(t.Context(), chain, testTransaction(0))
			if name == "valid" {
				if err != nil || signed == nil {
					t.Fatal(err)
				}
			} else if !errors.Is(err, ErrInvalidSignature) {
				t.Fatalf("error = %v", err)
			}
			if chain.Int64() != 137 {
				t.Fatal("signer mutated caller chain")
			}
		})
	}
}
