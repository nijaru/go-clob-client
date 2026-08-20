package polyrelay

import (
	"encoding/json"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

// Payload parity is functional, not byte-identical: the /submit body is a JSON
// object the relayer parses, so what matters is keys + string-typed values,
// not field order or address casing. We marshal Go's output, parse both sides
// to maps, lowercase hex leaves (EIP-55 casing is irrelevant to the server),
// and deep-compare. Signatures use a 65-byte zero placeholder.

func lowerHexLeaves(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			x[k] = lowerHexLeaves(e)
		}
		return x
	case []any:
		for i := range x {
			x[i] = lowerHexLeaves(x[i])
		}
		return x
	case string:
		if strings.HasPrefix(x, "0x") {
			return strings.ToLower(x)
		}
		return x
	default:
		return v
	}
}

func parseLower(t *testing.T, raw string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("parse json: %v\nraw: %s", err, raw)
	}
	return lowerHexLeaves(v)
}

func marshalLower(t *testing.T, req *SubmitRequest) any {
	t.Helper()
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return parseLower(t, string(raw))
}

// patchSig sets the parsed map's signature to the zero placeholder so the
// reference vector's dummy "0xsig" doesn't need to match real signing output.
func patchSig(m map[string]any) { m["signature"] = "0x" + strings.Repeat("00", 65) }

func TestBuildProxySubmit(t *testing.T) {
	t.Parallel()
	A, B, C := addrRepeat(0x11), addrRepeat(0x22), addrRepeat(0x33)
	req, err := BuildProxySubmit(ProxySubmitInput{
		Signer: A, ProxyFactory: B, Wallet: C,
		Data: common.FromHex("0xdeadbeef"), Nonce: big.NewInt(7), Signature: make([]byte, 65),
		GasLimit: big.NewInt(200_000), Relay: C, RelayHub: B, Metadata: "merge",
	})
	if err != nil {
		t.Fatalf("BuildProxySubmit: %v", err)
	}
	const wantPy = `{"type":"PROXY","from":"0x1111111111111111111111111111111111111111","to":"0x2222222222222222222222222222222222222222","proxyWallet":"0x3333333333333333333333333333333333333333","data":"0xdeadbeef","nonce":"7","signature":"0xsig","metadata":"merge","signatureParams":{"gasLimit":"200000","gasPrice":"0","relay":"0x3333333333333333333333333333333333333333","relayHub":"0x2222222222222222222222222222222222222222","relayerFee":"0"}}`
	got := marshalLower(t, req).(map[string]any)
	want := parseLower(t, wantPy).(map[string]any)
	patchSig(got)
	patchSig(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("proxy payload mismatch:\ngot  %v\nwant %v", got, want)
	}
}

func TestBuildSafeSubmit(t *testing.T) {
	t.Parallel()
	A, B, C := addrRepeat(0x11), addrRepeat(0x22), addrRepeat(0x33)
	const safeZero = `{"type":"SAFE","from":"0x1111111111111111111111111111111111111111","to":"0x2222222222222222222222222222222222222222","proxyWallet":"0x3333333333333333333333333333333333333333","data":"0xcafe","nonce":"3","signature":"0xsig","metadata":"x","signatureParams":{"baseGas":"0","gasPrice":"0","gasToken":"0x0000000000000000000000000000000000000000","operation":"0","refundReceiver":"0x0000000000000000000000000000000000000000","safeTxnGas":"0"}}`
	const safeValue = `{"type":"SAFE","from":"0x1111111111111111111111111111111111111111","to":"0x2222222222222222222222222222222222222222","proxyWallet":"0x3333333333333333333333333333333333333333","data":"0xcafe","nonce":"3","signature":"0xsig","metadata":"x","value":"1000000","signatureParams":{"baseGas":"0","gasPrice":"0","gasToken":"0x0000000000000000000000000000000000000000","operation":"1","refundReceiver":"0x0000000000000000000000000000000000000000","safeTxnGas":"0"}}`
	check := func(value *big.Int, op uint8, wantPy string) {
		t.Helper()
		req, err := BuildSafeSubmit(SafeSubmitInput{
			Signer:    A,
			Wallet:    C,
			Target:    B,
			Data:      common.FromHex("0xcafe"),
			Value:     value,
			Operation: op,
			Nonce:     big.NewInt(3),
			Signature: make([]byte, 65),
			Metadata:  "x",
		})
		if err != nil {
			t.Fatalf("BuildSafeSubmit: %v", err)
		}
		got := marshalLower(t, req).(map[string]any)
		want := parseLower(t, wantPy).(map[string]any)
		patchSig(got)
		patchSig(want)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("safe payload mismatch (value=%s):\ngot  %v\nwant %v", value, got, want)
		}
	}
	check(big.NewInt(0), 0, safeZero)
	check(big.NewInt(1_000_000), 1, safeValue)
}

func TestBuildDepositSubmit(t *testing.T) {
	t.Parallel()
	A, B, C := addrRepeat(0x11), addrRepeat(0x22), addrRepeat(0x33)
	req, err := BuildDepositSubmit(DepositSubmitInput{
		Signer: A, Factory: B, Wallet: C,
		Calls: []TransactionCall{
			{To: B, Data: nil, Value: big.NewInt(0)},
			{To: C, Data: common.FromHex("0xaabbcc"), Value: big.NewInt(1_000_000)},
		},
		Nonce: big.NewInt(
			5,
		), Deadline: big.NewInt(1_700_000_000), Signature: make([]byte, 65), Metadata: "redeem",
	})
	if err != nil {
		t.Fatalf("BuildDepositSubmit: %v", err)
	}
	const wantPy = `{"type":"WALLET","from":"0x1111111111111111111111111111111111111111","to":"0x2222222222222222222222222222222222222222","nonce":"5","signature":"0xsig","metadata":"redeem","depositWalletParams":{"depositWallet":"0x3333333333333333333333333333333333333333","deadline":"1700000000","calls":[{"target":"0x2222222222222222222222222222222222222222","value":"0","data":"0x"},{"target":"0x3333333333333333333333333333333333333333","value":"1000000","data":"0xaabbcc"}]}}`
	got := marshalLower(t, req).(map[string]any)
	want := parseLower(t, wantPy).(map[string]any)
	patchSig(got)
	patchSig(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("deposit payload mismatch:\ngot  %v\nwant %v", got, want)
	}
}

func TestBuildSubmitEmitsEmptyMetadata(t *testing.T) {
	t.Parallel()
	// py-sdk always emits "metadata" (even ""), so Metadata has no omitempty;
	// an empty metadata must still appear as a key in every submit body.
	req, err := BuildProxySubmit(ProxySubmitInput{
		Signer: addrRepeat(1), ProxyFactory: addrRepeat(2), Wallet: addrRepeat(3),
		Data: nil, Nonce: big.NewInt(1), Signature: make([]byte, 65),
		GasLimit: big.NewInt(1), Relay: addrRepeat(4), RelayHub: addrRepeat(5), Metadata: "",
	})
	if err != nil {
		t.Fatalf("BuildProxySubmit: %v", err)
	}
	got := marshalLower(t, req).(map[string]any)
	v, ok := got["metadata"]
	if !ok {
		t.Fatal("metadata key missing for empty metadata (omitempty regressed)")
	}
	if v != "" {
		t.Fatalf("metadata = %v, want empty string", v)
	}
}

func TestBuildWalletCreate(t *testing.T) {
	t.Parallel()
	A, B := addrRepeat(0x11), addrRepeat(0x22)
	req := BuildWalletCreate(WalletCreateInput{Signer: A, Factory: B, Metadata: "deploy"})
	const wantPy = `{"type":"WALLET-CREATE","from":"0x1111111111111111111111111111111111111111","to":"0x2222222222222222222222222222222222222222","metadata":"deploy"}`
	got := marshalLower(t, req)
	if want := parseLower(t, wantPy); !reflect.DeepEqual(got, want) {
		t.Fatalf("wallet-create payload mismatch:\ngot  %v\nwant %v", got, want)
	}
}

func TestBuildSubmitRejectsBadInput(t *testing.T) {
	t.Parallel()
	sig := make([]byte, 65)
	if _, err := BuildProxySubmit(
		ProxySubmitInput{
			Signer:    addrRepeat(1),
			Nonce:     nil,
			Signature: sig,
			GasLimit:  big.NewInt(1),
		},
	); err == nil {
		t.Fatal("nil nonce: expected error")
	}
	if _, err := BuildSafeSubmit(
		SafeSubmitInput{Value: nil, Nonce: big.NewInt(1), Signature: sig},
	); err == nil {
		t.Fatal("nil value: expected error")
	}
	if _, err := BuildDepositSubmit(
		DepositSubmitInput{Nonce: big.NewInt(1), Deadline: big.NewInt(2), Signature: sig},
	); err == nil {
		t.Fatal("empty calls: expected error")
	}
	if _, err := BuildProxySubmit(
		ProxySubmitInput{
			Signer:    addrRepeat(1),
			Nonce:     big.NewInt(-1),
			Signature: sig,
			GasLimit:  big.NewInt(1),
		},
	); err == nil {
		t.Fatal("negative nonce: expected error")
	}
}
