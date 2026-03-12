package clob

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type WalletContractConfig struct {
	ProxyFactory string
	SafeFactory  string
}

var walletConfigs = map[int64]WalletContractConfig{
	137: {
		ProxyFactory: "0xaB45c5A4B0c941a2F231C04C3f49182e1A254052",
		SafeFactory:  "0xaacFeEa03eb1561C4e67d661e40682Bd20E3541b",
	},
	80002: {
		SafeFactory: "0xaacFeEa03eb1561C4e67d661e40682Bd20E3541b",
	},
}

var (
	proxyInitCodeHash = common.HexToHash(
		"0xd21df8dc65880a8606f09fe0ce3df9b8869287ab0b058be05aa9e8af6330a00b",
	)
	safeInitCodeHash = common.HexToHash(
		"0x2bce2127ff07fb632d16c8347c4ebf501f4841168bed00d9e6ef715ddb6fcecf",
	)
)

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

func deriveProxyWallet(chainID int64, signerAddress string) (string, error) {
	config, ok := walletConfigs[chainID]
	if !ok || config.ProxyFactory == "" {
		return "", fmt.Errorf(
			"proxy wallet derivation not supported on chain %d; provide an explicit funder address",
			chainID,
		)
	}

	signer := common.HexToAddress(signerAddress)
	var packed [20]byte
	copy(packed[:], signer.Bytes())
	salt := crypto.Keccak256Hash(packed[:])
	return create2Address(config.ProxyFactory, salt, proxyInitCodeHash), nil
}

func deriveSafeWallet(chainID int64, signerAddress string) (string, error) {
	config, ok := walletConfigs[chainID]
	if !ok || config.SafeFactory == "" {
		return "", fmt.Errorf(
			"safe wallet derivation not supported on chain %d; provide an explicit funder address",
			chainID,
		)
	}

	signer := common.HexToAddress(signerAddress)
	var padded [32]byte
	copy(padded[12:], signer.Bytes())
	salt := crypto.Keccak256Hash(padded[:])
	return create2Address(config.SafeFactory, salt, safeInitCodeHash), nil
}

func create2Address(factory string, salt, initCodeHash common.Hash) string {
	var saltBytes [32]byte
	copy(saltBytes[:], salt.Bytes())
	return crypto.CreateAddress2(common.HexToAddress(factory), saltBytes, initCodeHash.Bytes()).
		Hex()
}

func isZeroAddress(address string) bool {
	return common.HexToAddress(address) == common.Address{}
}
