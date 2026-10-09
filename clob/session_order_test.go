package clob

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// Rust fixtures cover the inner Solady signature; TS/Python require a second
// ABI(bytes32 signerId, bytes32 zero, bytes signature) envelope for session keys.
func TestDepositOrderOwnerAndSessionSignatures(t *testing.T) {
	t.Parallel()
	for _, owner := range []bool{true, false} {
		t.Run(map[bool]string{true: "owner", false: "session"}[owner], func(t *testing.T) {
			client := newSessionOwner(t, "http://127.0.0.1:1")
			if !owner {
				client.funderAddress = "0x1111111111111111111111111111111111111111"
			}
			order, err := client.CreateExchangeV3OrderFromAmounts(t.Context(), OrderAmountsArgs{
				PositionID: nativeTestPosition(1, 0).String(), MakerAmount: big.NewInt(1_000_000),
				TakerAmount: big.NewInt(2_000_000), Side: SideBuy,
			})
			if err != nil {
				t.Fatal(err)
			}
			if order.Order.Maker != client.funderAddress ||
				order.Order.Signer != client.funderAddress {
				t.Fatal("deposit order must use the wallet for maker and signer")
			}
			cfg, _ := getContractConfig(PolygonChainID)
			inner, err := signPoly1271Order(
				t.Context(),
				client.signer,
				buildOrderTypedData(PolygonChainID, "3", cfg.ExchangeV3, *order),
				PolygonChainID,
			)
			if err != nil {
				t.Fatal(err)
			}
			if owner {
				if order.Signature != inner {
					t.Fatal("owner signature unexpectedly wrapped")
				}
				return
			}
			raw, err := hexutil.Decode(order.Signature)
			if err != nil {
				t.Fatal(err)
			}
			magic := bytes.Repeat([]byte{0x64, 0x92}, 16)
			if len(raw) < 32 || !bytes.Equal(raw[len(raw)-32:], magic) {
				t.Fatal("session signature missing magic suffix")
			}
			bytes32Type, _ := abi.NewType("bytes32", "", nil)
			bytesType, _ := abi.NewType("bytes", "", nil)
			values, err := (abi.Arguments{{Type: bytes32Type}, {Type: bytes32Type}, {Type: bytesType}}).Unpack(
				raw[:len(raw)-32],
			)
			if err != nil {
				t.Fatal(err)
			}
			var signerID [32]byte
			copy(signerID[12:], client.signer.Address().Bytes())
			if values[0].([32]byte) != signerID || values[1].([32]byte) != ([32]byte{}) ||
				hexutil.Encode(values[2].([]byte)) != inner {
				t.Fatal("session envelope changed signer identity or inner Solady signature")
			}
		})
	}
}

func TestComboAcceptDepositIdentityAndUint256(t *testing.T) {
	t.Parallel()
	mock := newComboGatewayMock(t, func(string) (int, string) {
		time.Sleep(20 * time.Millisecond)
		return http.StatusOK, `{"rfq_id":"rfq-1","status":"EXECUTING"}`
	})
	client := newSessionOwner(t, mock.server.URL)
	client.gatewayHTTP.BaseURL = mock.server.URL
	// Gateway POSTs have their own 30s hold window, not the ordinary HTTP timeout.
	client.gatewayHTTP.HTTPClient = &http.Client{Timeout: 5 * time.Millisecond}
	position := nativeTestPosition(3, 0).String()
	_, err := client.AcceptComboQuote(t.Context(), AcceptComboQuoteParams{
		RFQID: "rfq-1", Direction: SideBuy, PositionID: position, BuilderCode: zeroBytes32,
		Quote: ComboQuoteReference{QuoteID: "quote-1", MakerAmount: "1", TakerAmount: "2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if client.gatewayHTTP.HTTPClient.Timeout != 5*time.Millisecond {
		t.Fatal("combo POST mutated caller's HTTP client")
	}
	var request builderRfqAcceptRequest
	if err := json.Unmarshal([]byte(mock.body(builderRFQRequestsEndpoint+"/rfq-1/accept")), &request); err != nil {
		t.Fatal(err)
	}
	order := request.SignedOrder
	if order.TokenID != position || order.Signer != client.funderAddress ||
		order.Maker != client.funderAddress {
		t.Fatal("combo acceptance lost uint256 ID or deposit identity")
	}
	cfg, _ := getContractConfig(PolygonChainID)
	want, err := signPoly1271Order(
		t.Context(),
		client.signer,
		buildOrderTypedData(PolygonChainID, "3", cfg.ExchangeV3, order.typedOrder()),
		PolygonChainID,
	)
	if err != nil || order.Signature != want {
		t.Fatalf("combo signature: %v", err)
	}
}

func TestComboSessionAccountRejectedBeforeNetwork(t *testing.T) {
	t.Parallel()
	client := newSessionOwner(t, "http://127.0.0.1:1")
	client.funderAddress = "0x1111111111111111111111111111111111111111"
	_, err := client.RequestComboQuote(t.Context(), RequestComboQuoteParams{
		LegPositionIDs: []string{
			nativeTestPosition(1, 0).String(),
			nativeTestPosition(2, 0).String(),
		},
		Direction: RFQDirectionBuy, Amount: MustDec("1"),
	})
	if !errors.Is(err, ErrComboSessionKeyUnsupported) {
		t.Fatalf("session combo request: %v", err)
	}
	_, err = client.AcceptComboQuote(t.Context(), AcceptComboQuoteParams{
		RFQID: "rfq-1", Direction: SideBuy, PositionID: nativeTestPosition(3, 0).String(), BuilderCode: zeroBytes32,
		Quote: ComboQuoteReference{QuoteID: "q", MakerAmount: "1", TakerAmount: "2"},
	})
	if !errors.Is(err, ErrComboSessionKeyUnsupported) {
		t.Fatalf("session combo acceptance: %v", err)
	}
	if _, err := client.GetComboRFQStatus(t.Context(), "rfq-1"); !errors.Is(
		err,
		ErrComboSessionKeyUnsupported,
	) {
		t.Fatalf("session combo status: %v", err)
	}
}
