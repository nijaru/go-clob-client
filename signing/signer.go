// Package signing defines context-aware Ethereum signer capabilities. Implementations
// may use a local key, hardware wallet, or remote service; no key export is required.
package signing

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"reflect"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

// Signer signs EIP-712 data with an EOA. Signatures must be low-S, recoverable
// 65-byte R || S || V values, with V in {0,1,27,28}. Implementations must honor
// cancellation and support concurrent calls or serialize device access themselves.
type Signer interface {
	Address() common.Address
	SignTypedData(context.Context, apitypes.TypedData) ([]byte, error)
}

// MessageSigner optionally supports EIP-191 personal signing of raw bytes.
// It must add the Ethereum message prefix, not sign the raw message hash.
type MessageSigner interface {
	SignMessage(context.Context, []byte) ([]byte, error)
}

// TransactionSigner optionally signs, but does not broadcast, an Ethereum
// transaction. It must preserve the supplied payload and use the supplied chain.
type TransactionSigner interface {
	SignTransaction(context.Context, *big.Int, *types.Transaction) (*types.Transaction, error)
}

var (
	ErrInvalidSigner                 = errors.New("signing: invalid signer")
	ErrInvalidSignature              = errors.New("signing: invalid signature")
	ErrMessageSigningUnsupported     = errors.New("signing: personal-message signing unsupported")
	ErrTransactionSigningUnsupported = errors.New("signing: transaction signing unsupported")
)

// Wallet pins an EOA address once and verifies signatures returned by signing
// methods. Opaque TransactionSender broadcasts cannot be signature-verified.
// Its optional methods return capability errors when unsupported.
type Wallet struct {
	signer  Signer
	address common.Address
}

// NewWallet pins the supplied signer's nonzero EOA address. Use this constructor,
// not a zero Wallet, to obtain the verified signing boundary.
func NewWallet(signer Signer) (*Wallet, error) {
	if signer == nil {
		return nil, ErrInvalidSigner
	}
	value := reflect.ValueOf(signer)
	switch value.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan, reflect.Interface:
		if value.IsNil() {
			return nil, ErrInvalidSigner
		}
	}
	// Already-verified wallets own their capability selection. Wrapping one
	// would falsely advertise optional methods even when its backend lacks them.
	if wallet, ok := signer.(*Wallet); ok {
		return wallet, nil
	}
	address := signer.Address()
	if address == (common.Address{}) {
		return nil, ErrInvalidSigner
	}
	return &Wallet{signer: signer, address: address}, nil
}

func (w *Wallet) Address() common.Address { return w.address }

func (w *Wallet) SignTypedData(ctx context.Context, data apitypes.TypedData) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	digest, _, err := apitypes.TypedDataAndHash(data)
	if err != nil {
		return nil, fmt.Errorf("signing: typed data: %w", err)
	}
	sig, err := w.signer.SignTypedData(ctx, data)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return verifySignature(w.address, digest, sig)
}

func (w *Wallet) SignMessage(ctx context.Context, message []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	signer, ok := w.signer.(MessageSigner)
	if !ok {
		return nil, ErrMessageSigningUnsupported
	}
	digest := accounts.TextHash(message)
	sig, err := signer.SignMessage(ctx, message)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return verifySignature(w.address, digest, sig)
}

func verifySignature(address common.Address, digest, signature []byte) ([]byte, error) {
	if len(signature) != crypto.SignatureLength {
		return nil, ErrInvalidSignature
	}
	sig := append([]byte(nil), signature...)
	switch sig[64] {
	case 27, 28:
		sig[64] -= 27
	case 0, 1:
	default:
		return nil, ErrInvalidSignature
	}
	if !crypto.ValidateSignatureValues(
		sig[64],
		new(big.Int).SetBytes(sig[:32]),
		new(big.Int).SetBytes(sig[32:64]),
		true,
	) {
		return nil, ErrInvalidSignature
	}
	pub, err := crypto.SigToPub(digest, sig)
	if err != nil || crypto.PubkeyToAddress(*pub) != address {
		return nil, ErrInvalidSignature
	}
	sig[64] += 27
	return sig, nil
}

func (w *Wallet) SignTransaction(
	ctx context.Context,
	chainID *big.Int,
	tx *types.Transaction,
) (*types.Transaction, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	signer, ok := w.signer.(TransactionSigner)
	if !ok {
		return nil, ErrTransactionSigningUnsupported
	}
	if chainID == nil || chainID.Sign() <= 0 || tx == nil {
		return nil, fmt.Errorf("signing: invalid transaction or chain")
	}
	chain := new(big.Int).Set(chainID)
	verifier := types.LatestSignerForChainID(chain)
	expected := verifier.Hash(tx)
	signed, err := signer.SignTransaction(ctx, new(big.Int).Set(chain), tx)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if signed == nil || signed.ChainId().Cmp(chain) != 0 || signed.Type() != tx.Type() ||
		verifier.Hash(signed) != expected {
		return nil, fmt.Errorf("%w: transaction payload or chain changed", ErrInvalidSignature)
	}
	from, err := types.Sender(verifier, signed)
	if err != nil || from != w.address {
		return nil, fmt.Errorf("%w: transaction sender", ErrInvalidSignature)
	}
	return signed, nil
}
