package polyrelay

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

// WrapSessionSignature encodes the Deposit Wallet session-signer envelope:
// abi.encode(bytes32(signer), bytes32(0), signature) || ERC-6492 magic bytes.
// The inner signature can itself be an ERC-1271 signature.
func WrapSessionSignature(signer common.Address, signature []byte) ([]byte, error) {
	if signer == (common.Address{}) || len(signature) == 0 {
		return nil, fmt.Errorf("polyrelay: session signature requires signer and signature")
	}
	b32, err := abi.NewType("bytes32", "", nil)
	if err != nil {
		return nil, err
	}
	bytesType, err := abi.NewType("bytes", "", nil)
	if err != nil {
		return nil, err
	}
	var signerID [32]byte
	copy(signerID[12:], signer.Bytes())
	payload, err := (abi.Arguments{{Type: b32}, {Type: b32}, {Type: bytesType}}).Pack(
		signerID,
		[32]byte{},
		signature,
	)
	if err != nil {
		return nil, err
	}
	return append(
		payload,
		common.FromHex(
			"0x6492649264926492649264926492649264926492649264926492649264926492",
		)...), nil
}
