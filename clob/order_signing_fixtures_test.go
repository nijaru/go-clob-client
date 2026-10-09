package clob

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/nijaru/go-clob-client/internal/polyauth"
)

// Independent eth-account 0.13.x EIP-712 fixtures, with the nested Solady
// TypedDataSign schema from rs-clob-client-v2 561830b client.rs and standalone
// clob-client-v2 8046a89. Fixed public Hardhat key; no live credentials.
// Covers both structure hashes, ECDSA, and the complete Poly1271 wire wrapper.
func TestCrossSDKOrderSigningFixtures(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Version      int
		Kind         string
		Exchange     string
		DomainHash   string
		ContentsHash string
		Signature    string
	}
	data, err := os.ReadFile("testdata/order_signatures.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	signer, err := polyauth.ParsePrivateKey(
		"0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Kind+f.Exchange, func(t *testing.T) {
			address := signer.Address().Hex()
			signatureType := SignatureTypeEOA
			if f.Kind == "deposit" {
				address = "0x1111111111111111111111111111111111111111"
				signatureType = SignatureTypePoly1271
			}
			order := SignedOrder{Order: Order{
				Salt: "42", Maker: address, Signer: address, TokenID: "123", MakerAmount: "100000000", TakerAmount: "200000000",
				Side: SideBuy, SignatureType: signatureType, Timestamp: "1700000000000", Metadata: zeroBytes32, Builder: zeroBytes32,
			}, Expiration: "0"}
			typed := buildOrderTypedData(137, string(rune('0'+f.Version)), f.Exchange, order)
			if f.Version == 1 {
				order.Legacy = &LegacyOrderFields{Taker: zeroAddress, Nonce: "7", FeeRateBps: "10"}
				typed = buildLegacyOrderTypedData(137, f.Exchange, order)
			}
			domain, err := typed.HashStruct("EIP712Domain", typed.Domain.Map())
			if err != nil {
				t.Fatal(err)
			}
			contents, err := typed.HashStruct("Order", typed.Message)
			if err != nil {
				t.Fatal(err)
			}
			if common.Bytes2Hex(domain) != f.DomainHash[2:] ||
				common.Bytes2Hex(contents) != f.ContentsHash[2:] {
				t.Fatal("EIP-712 structure hashes disagree with independent oracle")
			}
			var signature string
			if f.Kind == "deposit" {
				signature, err = signPoly1271Order(signer, typed, 137)
			} else {
				signature, err = polyauth.SignTypedData(signer, typed)
			}
			if err != nil {
				t.Fatal(err)
			}
			if signature != f.Signature {
				t.Fatalf("signature mismatch\ngot  %s\nwant %s", signature, f.Signature)
			}
			if _, err := hex.DecodeString(signature[2:]); err != nil {
				t.Fatal(err)
			}
		})
	}
}
