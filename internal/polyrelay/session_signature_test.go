package polyrelay

import (
	"bytes"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestSessionSignatureReferenceEnvelope(t *testing.T) {
	t.Parallel()
	// Independent eth-abi encoding of py-sdk _internal/wallet.py's envelope.
	const golden = "00000000000000000000000011111111111111111111111111111111111111110000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000412222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222000000000000000000000000000000000000000000000000000000000000006492649264926492649264926492649264926492649264926492649264926492"
	wrapped, err := WrapSessionSignature(
		common.HexToAddress("0x1111111111111111111111111111111111111111"),
		bytes.Repeat([]byte{0x22}, 65),
	)
	if err != nil || hex.EncodeToString(wrapped) != golden {
		t.Fatalf("envelope mismatch: %x %v", wrapped, err)
	}
	body, err := BuildDepositSubmit(
		DepositSubmitInput{
			Calls:     []TransactionCall{{Value: big.NewInt(0)}},
			Nonce:     big.NewInt(1),
			Deadline:  big.NewInt(2),
			Signature: wrapped,
		},
	)
	if err != nil || body.Signature != "0x"+golden {
		t.Fatalf("session envelope not retained on wire: %v", err)
	}
}
