package clob

import (
	"context"
	"encoding/hex"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/nijaru/go-clob-client/signing"
)

// Deliberately exposes neither a key nor the optional signing capabilities.
type externalTestSigner struct {
	address common.Address
	sign    func(context.Context, apitypes.TypedData) ([]byte, error)
}

func (s *externalTestSigner) Address() common.Address { return s.address }

func (s *externalTestSigner) SignTypedData(
	ctx context.Context,
	data apitypes.TypedData,
) ([]byte, error) {
	return s.sign(ctx, data)
}

func TestExternalSignerAuthAndOrderContext(t *testing.T) {
	t.Parallel()
	local, err := signing.NewLocalSigner(
		"ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
	)
	if err != nil {
		t.Fatal(err)
	}
	type marker struct{}
	ctx := context.WithValue(t.Context(), marker{}, "request")
	var seen []string
	source := &externalTestSigner{
		address: local.Address(),
		sign: func(got context.Context, data apitypes.TypedData) ([]byte, error) {
			if got.Value(marker{}) != "request" {
				t.Fatal("request context lost")
			}
			seen = append(seen, data.PrimaryType)
			if data.PrimaryType == "TypedDataSign" {
				if data.Message["verifyingContract"] != "0x1111111111111111111111111111111111111111" {
					t.Fatal("nested wallet identity lost")
				}
				if _, ok := data.Message["contents"].(apitypes.TypedDataMessage); !ok {
					t.Fatal("wallet received an opaque digest instead of nested Order")
				}
			}
			return local.SignTypedData(got, data)
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/api-key" || r.Header.Get("POLY_ADDRESS") != local.Address().Hex() {
			t.Error("L1 identity changed")
		}
		sig, err := hex.DecodeString(r.Header.Get("POLY_SIGNATURE")[2:])
		if err != nil || len(sig) != 65 || sig[64] < 27 || sig[64] > 28 {
			t.Error("invalid L1 wire signature")
		}
		_, _ = w.Write([]byte(`{"apiKey":"key","secret":"YQ==","passphrase":"pass"}`))
	}))
	defer server.Close()
	client, err := NewSignerClient(
		Config{
			Host:          server.URL,
			Signer:        source,
			SignatureType: SignatureTypePoly1271,
			FunderAddress: "0x1111111111111111111111111111111111111111",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	source.address = common.HexToAddress("0x2222222222222222222222222222222222222222")
	if client.Address() != local.Address().Hex() {
		t.Fatal("client address was not pinned")
	}
	if _, err := client.CreateAPIKey(ctx, 7); err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateExchangeV3OrderFromAmounts(ctx, OrderAmountsArgs{PositionID: nativeTestPosition(1, 0).String(), MakerAmount: big.NewInt(1), TakerAmount: big.NewInt(2), Side: SideBuy}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen[0] != "ClobAuth" || seen[1] != "TypedDataSign" {
		t.Fatalf("typed requests = %v", seen)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := client.CreateAPIKey(cancelled, 0); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled auth: %v", err)
	}
	if _, err := NewSignerClient(Config{Signer: source, PrivateKey: "invalid"}); err == nil {
		t.Fatal("accepted two signer sources")
	}
}

func TestExternalSignerRejectsWrongEOABeforeOrderWrapping(t *testing.T) {
	t.Parallel()
	local, _ := signing.NewLocalSigner(
		"ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
	)
	source := &externalTestSigner{
		address: common.HexToAddress("0x2222222222222222222222222222222222222222"),
		sign:    local.SignTypedData,
	}
	client, err := NewSignerClient(
		Config{
			Signer:        source,
			SignatureType: SignatureTypePoly1271,
			FunderAddress: "0x1111111111111111111111111111111111111111",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.CreateExchangeV3OrderFromAmounts(
		t.Context(),
		OrderAmountsArgs{
			PositionID:  nativeTestPosition(1, 0).String(),
			MakerAmount: big.NewInt(1),
			TakerAmount: big.NewInt(2),
			Side:        SideBuy,
		},
	)
	if !errors.Is(err, signing.ErrInvalidSignature) {
		t.Fatalf("error = %v", err)
	}
}
