package polyrelay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/nijaru/go-clob-client/signing"
)

type externalRelaySigner struct {
	address common.Address
	typed   func(context.Context, apitypes.TypedData) ([]byte, error)
}

func (s externalRelaySigner) Address() common.Address { return s.address }

func (s externalRelaySigner) SignTypedData(
	ctx context.Context,
	data apitypes.TypedData,
) ([]byte, error) {
	return s.typed(ctx, data)
}

func TestExternalDepositSignerNonceCorrectionContext(t *testing.T) {
	t.Parallel()
	local, err := signing.NewLocalSigner(vectorKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	type marker struct{}
	ctx := context.WithValue(t.Context(), marker{}, "request")
	var signedNonces []string
	source := externalRelaySigner{
		address: local.Address(),
		typed: func(ctx context.Context, data apitypes.TypedData) ([]byte, error) {
			if ctx.Value(marker{}) != "request" {
				t.Error("nonce re-signing lost context")
			}
			if data.PrimaryType != "Batch" {
				t.Error("expected deposit EIP-712, not opaque hash")
			}
			signedNonces = append(signedNonces, data.Message["nonce"].(string))
			return local.SignTypedData(ctx, data)
		},
	}
	wallet, err := signing.NewWallet(source)
	if err != nil {
		t.Fatal(err)
	}
	submissions := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == executeParamsPath {
			fmt.Fprint(w, `{"nonce":"1"}`)
			return
		}
		submissions++
		var body SubmitRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		if submissions == 1 {
			http.Error(w, `{"error":"batch nonce 1 does not match on-chain nonce 2"}`, 400)
			return
		}
		if body.Nonce != "2" {
			t.Error("corrected nonce not submitted")
		}
		fmt.Fprint(w, `{"transactionID":"external"}`)
	}))
	defer server.Close()
	_, err = PrepareGasless(
		ctx,
		testTransport(t, server),
		testGaslessConfig(TransactionTypeWallet),
		wallet,
		[]TransactionCall{{To: addrRepeat(0x20), Value: big.NewInt(0)}},
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(signedNonces) != 2 || signedNonces[0] != "1" || signedNonces[1] != "2" ||
		submissions != 2 {
		t.Fatalf("nonces = %v, submits = %d", signedNonces, submissions)
	}
}

func TestRelayerRequiresPersonalSigningCapability(t *testing.T) {
	t.Parallel()
	local, _ := signing.NewLocalSigner(vectorKeyHex)
	// Only typed signing is exposed, even though the underlying test key could
	// sign messages. The relayer must not substitute typed/raw-hash signing.
	source := externalRelaySigner{address: local.Address(), typed: local.SignTypedData}
	wallet, err := signing.NewWallet(source)
	if err != nil {
		t.Fatal(err)
	}
	req := RelayRequest{
		Signer:   local.Address(),
		Wallet:   addrRepeat(0x20),
		To:       addrRepeat(0x21),
		ChainID:  big.NewInt(137),
		Value:    big.NewInt(0),
		Nonce:    big.NewInt(0),
		GasFee:   big.NewInt(0),
		GasPrice: big.NewInt(0),
		GasLimit: big.NewInt(21000),
	}
	for _, typ := range []RelayerTransactionType{TransactionTypeSafe, TransactionTypeProxy} {
		if _, err := Sign(t.Context(), typ, wallet, req); !errors.Is(
			err,
			signing.ErrMessageSigningUnsupported,
		) {
			t.Fatalf("%s: %v", typ, err)
		}
	}
	cfg := testGaslessConfig(TransactionTypeWallet)
	cfg.Signer = addrRepeat(0x99)
	if _, err := BuildGaslessSubmit(t.Context(), nil, cfg, wallet, []TransactionCall{{To: addrRepeat(0x20), Value: big.NewInt(0)}}, ""); err == nil {
		t.Fatal("configured EOA differs from pinned EOA")
	}
}
