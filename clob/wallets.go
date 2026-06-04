package clob

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// normalizeFunderAddress resolves the funder address for the given signature type.
// For EOA, funder must be empty. For proxy/safe types, it derives the address
// via CREATE2 when no explicit funder is provided.
func normalizeFunderAddress(
	chainID int64,
	signerAddress string,
	signatureType SignatureType,
	funderAddress string,
) (string, error) {
	switch signatureType {
	case SignatureTypeEOA:
		if funderAddress != "" {
			return "", fmt.Errorf("cannot have a funder address with an EOA signature type")
		}
		return "", nil
	case SignatureTypePolyProxy:
		return resolveProxyFunder(chainID, signerAddress, funderAddress)
	case SignatureTypePolyGnosisSafe:
		return resolveSafeFunder(chainID, signerAddress, funderAddress)
	case SignatureTypePoly1271:
		if funderAddress == "" {
			return "", fmt.Errorf(
				"deposit wallet funder address is required with Poly1271 signature type",
			)
		}
		return funderAddress, nil
	default:
		return "", fmt.Errorf("unsupported signature type %d", signatureType)
	}
}

func resolveProxyFunder(chainID int64, signerAddress, funderAddress string) (string, error) {
	if funderAddress != "" {
		if isZeroAddress(funderAddress) {
			return "", fmt.Errorf(
				"cannot have a zero funder address with a POLY_PROXY signature type",
			)
		}
		return funderAddress, nil
	}

	derived, err := deriveProxyWallet(chainID, signerAddress)
	if err != nil {
		return "", err
	}
	return derived, nil
}

func resolveSafeFunder(chainID int64, signerAddress, funderAddress string) (string, error) {
	if funderAddress != "" {
		if isZeroAddress(funderAddress) {
			return "", fmt.Errorf(
				"cannot have a zero funder address with a POLY_GNOSIS_SAFE signature type",
			)
		}
		return funderAddress, nil
	}

	derived, err := deriveSafeWallet(chainID, signerAddress)
	if err != nil {
		return "", err
	}
	return derived, nil
}

// deriveProxyWallet computes the proxy wallet address for internal use (string-based).
func deriveProxyWallet(chainID int64, signerAddress string) (string, error) {
	wc, err := getWalletConfig(chainID)
	if err != nil || wc.ProxyFactory == "" {
		return "", fmt.Errorf(
			"proxy wallet derivation not supported on chain %d; provide an explicit funder address",
			chainID,
		)
	}

	signer := common.HexToAddress(signerAddress)
	var packed [20]byte
	copy(packed[:], signer.Bytes())
	salt := crypto.Keccak256Hash(packed[:])
	return create2AddressInternal(wc.ProxyFactory, salt, common.HexToHash(proxyInitCodeHash)), nil
}

// deriveSafeWallet computes the safe wallet address for internal use (string-based).
func deriveSafeWallet(chainID int64, signerAddress string) (string, error) {
	wc, err := getWalletConfig(chainID)
	if err != nil || wc.SafeFactory == "" {
		return "", fmt.Errorf(
			"safe wallet derivation not supported on chain %d; provide an explicit funder address",
			chainID,
		)
	}

	signer := common.HexToAddress(signerAddress)
	var padded [32]byte
	copy(padded[12:], signer.Bytes())
	salt := crypto.Keccak256Hash(padded[:])
	return create2AddressInternal(wc.SafeFactory, salt, common.HexToHash(safeInitCodeHash)), nil
}

// DeriveProxyWallet computes the deterministic EIP-1167 minimal proxy wallet
// address for an EOA using CREATE2. This is the wallet Polymarket deploys for
// Magic/email wallet users.
//
// Returns a zero address if the chain does not support proxy wallets.
func DeriveProxyWallet(eoa common.Address, chainID int64) (common.Address, error) {
	wc, err := getWalletConfig(chainID)
	if err != nil {
		return common.Address{}, err
	}
	if wc.ProxyFactory == "" {
		return common.Address{}, nil
	}
	// Salt is keccak256(encodePacked(address)) — raw 20 bytes, no padding
	salt := crypto.Keccak256Hash(eoa.Bytes())
	return computeCreate2(common.HexToAddress(wc.ProxyFactory), salt.Bytes(), proxyInitCodeHash)
}

// DeriveSafeWallet computes the deterministic Gnosis Safe wallet address for
// an EOA using CREATE2. This is the 1-of-1 Safe multisig that Polymarket
// deploys for browser wallet users.
func DeriveSafeWallet(eoa common.Address, chainID int64) (common.Address, error) {
	wc, err := getWalletConfig(chainID)
	if err != nil {
		return common.Address{}, err
	}
	// ABI encoding pads address to 32 bytes (left-padded with zeros)
	var padded [32]byte
	copy(padded[12:], eoa.Bytes())
	// Salt is keccak256(encodeAbiParameters(address))
	salt := crypto.Keccak256Hash(padded[:])
	return computeCreate2(common.HexToAddress(wc.SafeFactory), salt.Bytes(), safeInitCodeHash)
}

// computeCreate2 computes a CREATE2 address from components.
func computeCreate2(
	factory common.Address,
	salt []byte,
	initCodeHashHex string,
) (common.Address, error) {
	initCodeHash := common.FromHex(initCodeHashHex)
	if len(initCodeHash) != 32 {
		return common.Address{}, fmt.Errorf("invalid init code hash length: %d", len(initCodeHash))
	}

	var saltBytes [32]byte
	copy(saltBytes[:], salt)
	return crypto.CreateAddress2(factory, saltBytes, initCodeHash), nil
}

// create2AddressInternal computes a CREATE2 address and returns it as a hex string.
func create2AddressInternal(factory string, salt, initCodeHash common.Hash) string {
	var saltBytes [32]byte
	copy(saltBytes[:], salt.Bytes())
	return crypto.CreateAddress2(common.HexToAddress(factory), saltBytes, initCodeHash.Bytes()).
		Hex()
}

func isZeroAddress(address string) bool {
	return common.HexToAddress(address) == common.Address{}
}
