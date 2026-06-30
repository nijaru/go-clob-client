package clob

import (
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
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

// EIP-1967 proxy constants shared with the Python and TypeScript SDKs.
const (
	erc1967Const1 = "0xcc3735a920a3ca505d382bbc545af43d6000803e6038573d6000fd5b3d6000f3"
	erc1967Const2 = "0x5155f3363d3d373d3d363d7f360894a13ba1a3210667c828492db98dca3e2076"

	erc1967BeaconConst1 = "0xb3582b35133d50545afa5036515af43d6000803e604d573d6000fd5b3d6000f3"
	erc1967BeaconConst2 = "0x1b60e01b36527fa3f0ad74e5423aebfd80d3ef4346578335a9a72aeaee59ff6c"
	erc1967BeaconConst3 = "0x60195155f3363d3d373d3d363d602036600436635c60da"

	factoryBeaconSelector = "0x49493a4d"
)

// depositWalletArgs encodes (depositWalletFactory, walletId) where walletId
// is the signer address right-padded to 32 bytes.
func depositWalletArgs(signer common.Address, factory common.Address) ([]byte, error) {
	addressType, err := abi.NewType("address", "", nil)
	if err != nil {
		return nil, fmt.Errorf("deposit wallet: create address type: %w", err)
	}
	bytes32Type, err := abi.NewType("bytes32", "", nil)
	if err != nil {
		return nil, fmt.Errorf("deposit wallet: create bytes32 type: %w", err)
	}

	args := abi.Arguments{
		{Type: addressType},
		{Type: bytes32Type},
	}

	var walletID [32]byte
	copy(walletID[12:], signer.Bytes())

	return args.Pack(factory, walletID)
}

// makeERC1967Prefix constructs the 10-byte EIP-1967 proxy prefix. The args
// byte count is added at position 2 (0-indexed), matching the upstream
// Python/TS prefix = BASE + (argsByteLength << 56).
func makeERC1967Prefix(baseHex string, argsLen int) []byte {
	prefix, _ := hex.DecodeString(baseHex)
	// argsLen << 56 shifts to byte position 2 (big-endian).
	// argsLen is at most 255 for deposit wallet args (address + bytes32 = 64 bytes).
	prefix[2] += byte(argsLen)
	return prefix
}

// uupsDepositInitCodeHash computes keccak256 of the UUPS deposit wallet init
// code using EIP-1967 proxy bytecode construction.
func uupsDepositInitCodeHash(implementation common.Address, args []byte) common.Hash {
	prefix := makeERC1967Prefix("61003d3d8160233d3973", len(args))

	bytecode := make([]byte, 0, 10+20+2+25+25+len(args))
	bytecode = append(bytecode, prefix...)
	bytecode = append(bytecode, implementation.Bytes()...)
	bytecode = append(bytecode, common.FromHex("0x6009")...)
	bytecode = append(bytecode, common.FromHex(erc1967Const2)...)
	bytecode = append(bytecode, common.FromHex(erc1967Const1)...)
	bytecode = append(bytecode, args...)

	return crypto.Keccak256Hash(bytecode)
}

// beaconDepositInitCodeHash computes keccak256 of the beacon deposit wallet
// init code using EIP-1967 beacon proxy bytecode construction.
func beaconDepositInitCodeHash(beacon common.Address, args []byte) common.Hash {
	prefix := makeERC1967Prefix("6100523d8160233d3973", len(args))

	bytecode := make([]byte, 0, 10+20+24+25+25+len(args))
	bytecode = append(bytecode, prefix...)
	bytecode = append(bytecode, beacon.Bytes()...)
	bytecode = append(bytecode, common.FromHex(erc1967BeaconConst3)...)
	bytecode = append(bytecode, common.FromHex(erc1967BeaconConst2)...)
	bytecode = append(bytecode, common.FromHex(erc1967BeaconConst1)...)
	bytecode = append(bytecode, args...)

	return crypto.Keccak256Hash(bytecode)
}

// DeriveUUPSDepositWallet computes the deterministic UUPS (legacy) deposit
// wallet address for a signer on the given chain using CREATE2. Returns a
// zero address if the chain does not support deposit wallets.
func DeriveUUPSDepositWallet(signer common.Address, chainID int64) (common.Address, error) {
	wc, err := getWalletConfig(chainID)
	if err != nil {
		return common.Address{}, err
	}
	if wc.DepositWalletFactory == "" || wc.DepositWalletImplementation == "" {
		return common.Address{}, nil
	}

	factory := common.HexToAddress(wc.DepositWalletFactory)
	impl := common.HexToAddress(wc.DepositWalletImplementation)

	args, err := depositWalletArgs(signer, factory)
	if err != nil {
		return common.Address{}, fmt.Errorf("derive uups deposit wallet: %w", err)
	}

	salt := crypto.Keccak256Hash(args)
	initCodeHash := uupsDepositInitCodeHash(impl, args)
	return computeCreate2(factory, salt.Bytes(), initCodeHash.Hex())
}

// DeriveBeaconDepositWallet computes the deterministic beacon deposit wallet
// address for a signer on the given chain using CREATE2. Returns a zero
// address if the chain does not support deposit wallets.
func DeriveBeaconDepositWallet(signer common.Address, chainID int64) (common.Address, error) {
	wc, err := getWalletConfig(chainID)
	if err != nil {
		return common.Address{}, err
	}
	if wc.DepositWalletFactory == "" || wc.DepositWalletBeacon == "" {
		return common.Address{}, nil
	}

	factory := common.HexToAddress(wc.DepositWalletFactory)
	beacon := common.HexToAddress(wc.DepositWalletBeacon)

	args, err := depositWalletArgs(signer, factory)
	if err != nil {
		return common.Address{}, fmt.Errorf("derive beacon deposit wallet: %w", err)
	}

	salt := crypto.Keccak256Hash(args)
	initCodeHash := beaconDepositInitCodeHash(beacon, args)
	return computeCreate2(factory, salt.Bytes(), initCodeHash.Hex())
}
