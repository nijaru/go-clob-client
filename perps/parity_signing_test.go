package perps

import (
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

// Entry/approval fixtures are backend fixtures in TS 087f9443,
// websockets/perps/actions/trading.test.ts. The additional compact rows follow
// that source's grammar; hashes were independently calculated using Python
// msgpack 1.2.3 and eth-account 0.14.0 (chain 31337, salt 1, ts 1739491200000).
// They protect optional-field compaction, nesting, and the complete EIP-712 digest.
func TestPerpsSigningParity(t *testing.T) {
	const builder = "0x0000000000000000000000000000000000001234"
	gtd, body, err := perpsOrderWire(
		PerpsOrderRequest{
			InstrumentID: 1,
			Side:         PerpsOrderBuy,
			Price:        "100.50",
			Quantity:     "10",
			TimeInForce:  PerpsTIFGTD,
			PostOnly:     true,
			GTDExpiry:    1893456000123,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	appendBuilder(&gtd, body, &PerpsBuilderTerms{Address: builder, FeeRate: "0.002"})
	trailing, _, err := (triggerOrder{instrumentID: 1, buy: false, quantity: "0.25", kind: "sl", trigger: TPSLTrigger{TrailingBps: 100, ActivationPrice: "110"}}).wire()
	if err != nil {
		t.Fatal(err)
	}
	entry, _, err := perpsOrderWire(
		PerpsOrderRequest{
			InstrumentID: 1,
			Side:         PerpsOrderBuy,
			Price:        "100",
			Quantity:     "1",
			TimeInForce:  PerpsTIFGTC,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	tp, _, err := (triggerOrder{instrumentID: 1, buy: false, quantity: "1", kind: "tp", trigger: TPSLTrigger{TriggerPrice: "120"}}).wire()
	if err != nil {
		t.Fatal(err)
	}
	sl, _, err := (triggerOrder{instrumentID: 1, buy: false, quantity: "1", kind: "sl", trigger: TPSLTrigger{TriggerPrice: "80"}}).wire()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name         string
		op           []any
		data, digest string
	}{
		{"gtd_builder", []any{"createOrders", []any{gtd}}, "8681bc655fedd1171421496ebd2e56e917ef69860341b44d284b01d45e2afb0f", "ca5c08039674b768e32d757f55ee8c8561dfa9568fff1b9a1d537f5ba1c0dc92"},
		{"partial_trailing", []any{"createOrders", []any{trailing}, "position"}, "577c63ac49255c86d5765ae083f2772370fc09769d9f0ba5e4f4c4a95ff83e54", "6399abbe61542379f5af7ae6cf2056bc111b0fafba276819b940da484a413901"},
		{"entry_bracket", []any{"createOrders", []any{entry, tp, sl}, "order"}, "f88e8d9343a53c2727f4c940a35d9163b36044322270d0eab9f2fd345edef60c", "8b404e2610ddea697110263d99fb6a9cb155c52db678f91ca5183e544ae9cd73"},
		{"batch_leverage", []any{"updateLeverages", []any{[]any{1, 5, true}, []any{2, 10, false}}}, "3f0ae6c38eea424e87491e037d19934f96a3465ee5de5d92ef9ec6e8bd73a642", "a8e7efba81c60389ccc1dd9c1bce8567ce1307b88316162772d881fc408b5e64"},
		{"builder_approval", []any{"approveBuilder", []any{builder, "0.002", 1}}, "3f1cc1f398302e8b00fa75c4f5d2ca0e8464014785a365873fd41582b7280a9b", "9107350857f4671a6c2f4f8e4ec6152bd1cf7294ca24e97a5531b6f2007d0f9e"},
		{"internal_transfer", []any{"internalTransfer", []any{"0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf", "0x0000000000000000000000000000000000000002", "100.00", "0x0000000000000000000000000000000000000003"}}, "bec3cbd9679dd71353ec802fa9c57e0d128f3c86a277d7d174665f00c13968f0", "91dd7441807556db191c62a2aa0bbd6b8ba9664e52c5757ebbc09812d7acdd00"},
		{"delete_proxy", []any{"deleteProxy", []any{"0x0000000000000000000000000000000000000003"}}, "bbebf26b8e56dd32df42c4902b70fec126449041c17b131eb075f446fb536226", "2d24373a42412cf8741fa5801ed2d379475eb0f3b8a12d940d1ae9a981667fee"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := operationTypedData(31337, tc.op, 1, 1739491200000)
			if err != nil {
				t.Fatal(err)
			}
			if data.Message["data"] != "0x"+tc.data {
				t.Fatalf("operation hash = %v", data.Message["data"])
			}
			digest, _, err := apitypes.TypedDataAndHash(data)
			if err != nil {
				t.Fatal(err)
			}
			if hex.EncodeToString(digest) != tc.digest {
				t.Fatalf("EIP-712 digest = %x", digest)
			}
		})
	}
}

func TestMarginNormalizationPreservesVenuePrecision(t *testing.T) {
	const amount = "-1234567890.1234567890123456789012345678"
	got, err := normalizePerpsDecimal(amount)
	if err != nil || got != amount {
		t.Fatalf("margin = %q, %v", got, err)
	}
}

func assertOperationSignature(t *testing.T, c fixtureCommand, op []any) {
	t.Helper()
	data, err := operationTypedData(31337, op, c.Salt, c.Timestamp)
	if err != nil {
		t.Fatal(err)
	}
	assertTypedSignature(t, c.Signature, data, fixtureProxy)
}

func assertTypedSignature(t *testing.T, signature string, data apitypes.TypedData, address string) {
	t.Helper()
	digest, _, err := apitypes.TypedDataAndHash(data)
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := hex.DecodeString(signature[2:])
	if err != nil || len(bytes) != 65 {
		t.Fatalf("signature = %q: %v", signature, err)
	}
	bytes[64] -= 27
	pub, err := crypto.SigToPub(digest, bytes)
	if err != nil {
		t.Fatal(err)
	}
	if crypto.PubkeyToAddress(*pub).Hex() != address {
		t.Fatalf("signature recovered unexpected signer %s", crypto.PubkeyToAddress(*pub))
	}
}
