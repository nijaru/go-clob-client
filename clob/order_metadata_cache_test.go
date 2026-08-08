package clob

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	json "github.com/go-json-experiment/json"
	"github.com/quagmt/udecimal"
)

func TestOrderMetadataCacheWarmsSiblingTokensAndDeduplicatesLoads(t *testing.T) {
	var conditionCalls atomic.Int32
	var marketCalls atomic.Int32
	var releaseMarket = make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case marketsByTokenEndpoint + "yes":
			conditionCalls.Add(1)
			_, _ = w.Write([]byte(`{"condition_id":"cid"}`))
		case clobMarketEndpoint + "/cid":
			marketCalls.Add(1)
			select {
			case <-releaseMarket:
			case <-time.After(time.Second):
				t.Error("timed out waiting to release market response")
			}
			_, _ = w.Write([]byte(`{"c":"cid","mts":"0.01","nr":true,"fd":{"r":"0.02","e":1},"t":[{"t":"yes"},{"t":"no"}]}`))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(Config{Host: server.URL, RetryMax: 0})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	const callers = 8
	results := make(chan orderMarketMetadata, callers)
	errs := make(chan error, callers)
	for range callers {
		go func() {
			value, err := client.resolveOrderMarketMetadata(t.Context(), "yes", false)
			results <- value
			errs <- err
		}()
	}
	for marketCalls.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	close(releaseMarket)
	for range callers {
		if err := <-errs; err != nil {
			t.Fatalf("resolve metadata: %v", err)
		}
		value := <-results
		if value.TickSize != TickSizeHundredth || !value.NegRisk || value.FeeInfo.Exponent != 1 {
			t.Fatalf("metadata = %+v", value)
		}
	}

	value, err := client.resolveOrderMarketMetadata(t.Context(), "no", false)
	if err != nil {
		t.Fatalf("resolve sibling metadata: %v", err)
	}
	if _, ok := value.TokenIDs["no"]; !ok {
		t.Fatalf("sibling metadata token IDs = %v", value.TokenIDs)
	}
	if got := conditionCalls.Load(); got != 1 {
		t.Fatalf("condition requests = %d, want 1", got)
	}
	if got := marketCalls.Load(); got != 1 {
		t.Fatalf("market requests = %d, want 1", got)
	}
}

func TestOrderMetadataCacheRefreshesMarketWithoutResolvingCondition(t *testing.T) {
	var conditionCalls atomic.Int32
	var marketCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case marketsByTokenEndpoint + "yes":
			conditionCalls.Add(1)
			_, _ = w.Write([]byte(`{"condition_id":"cid"}`))
		case clobMarketEndpoint + "/cid":
			tickSize := TickSizeHundredth
			if marketCalls.Add(1) > 1 {
				tickSize = TickSizeThousandth
			}
			data, _ := json.Marshal(ClobMarketInfoResponse{
				ConditionID: "cid",
				MinTickSize: string(tickSize),
				Tokens:      []*ClobMarketToken{{TokenID: "yes"}},
			})
			_, _ = w.Write(data)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(Config{Host: server.URL, RetryMax: 0})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := client.resolveOrderMarketMetadata(t.Context(), "yes", false); err != nil {
		t.Fatalf("initial metadata: %v", err)
	}
	value, err := client.resolveOrderMarketMetadata(t.Context(), "yes", true)
	if err != nil {
		t.Fatalf("refreshed metadata: %v", err)
	}
	if value.TickSize != TickSizeThousandth {
		t.Fatalf("refreshed tick size = %q", value.TickSize)
	}
	if got := conditionCalls.Load(); got != 1 {
		t.Fatalf("condition requests = %d, want 1", got)
	}
	if got := marketCalls.Load(); got != 2 {
		t.Fatalf("market requests = %d, want 2", got)
	}
}

func TestBuilderFeeCacheDeduplicatesLoads(t *testing.T) {
	var calls atomic.Int32
	var release = make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != builderFeeRateEndpoint+"builder" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		calls.Add(1)
		<-release
		_, _ = w.Write([]byte(`{"builder_maker_fee_rate_bps":3,"builder_taker_fee_rate_bps":7}`))
	}))
	t.Cleanup(server.Close)
	client, err := NewClient(Config{Host: server.URL, RetryMax: 0})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	const callers = 4
	values := make([]*BuilderFeeRateResponse, callers)
	errs := make([]error, callers)
	var group sync.WaitGroup
	for i := range callers {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			values[index], errs[index] = client.resolveBuilderFeeRateCached(t.Context(), "builder")
		}(i)
	}
	for calls.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	close(release)
	group.Wait()
	for i := range callers {
		if errs[i] != nil {
			t.Fatalf("builder fee load %d: %v", i, errs[i])
		}
		if values[i].BuilderTakerFeeRateBps != 7 {
			t.Fatalf("builder fee %d = %+v", i, values[i])
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("builder fee requests = %d, want 1", got)
	}

	if _, err := client.resolveBuilderFeeRateCached(t.Context(), "builder"); err != nil {
		t.Fatalf("cached builder fee: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("cached builder fee requests = %d, want 1", got)
	}
}

func TestCreateOrderHonorsIndependentTickCache(t *testing.T) {
	server := newTradingTestServerWithTickSize(t, nil, TickSizeHundredth)
	client, err := NewSignerClient(Config{
		Host:       server.URL,
		PrivateKey: "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		RetryMax:   0,
	})
	if err != nil {
		t.Fatalf("NewSignerClient: %v", err)
	}
	client.SetTickSize("100", TickSizeThousandth)
	client.saltGenerator = func() (uint64, error) { return 42, nil }

	order, err := client.CreateOrder(t.Context(), OrderArgs{
		TokenID: "100",
		Price:   udecimal.MustParse("0.451"),
		Size:    udecimal.MustParse("10"),
		Side:    SideBuy,
	}, nil)
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	if order.Order.MakerAmount != "4510000" {
		t.Fatalf("maker amount = %s, want 4510000", order.Order.MakerAmount)
	}
}

func TestCreateOrderHonorsIndependentNegRiskCache(t *testing.T) {
	server := newTradingTestServerWithTickSize(t, nil, TickSizeHundredth)
	client, err := NewSignerClient(Config{
		Host:       server.URL,
		PrivateKey: "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		ChainID:    PolygonChainID,
		RetryMax:   0,
	})
	if err != nil {
		t.Fatalf("NewSignerClient: %v", err)
	}
	client.SetNegRisk("100", true)
	client.saltGenerator = func() (uint64, error) { return 42, nil }

	order, err := client.CreateOrder(t.Context(), OrderArgs{
		TokenID: "100",
		Price:   udecimal.MustParse("0.45"),
		Size:    udecimal.MustParse("10"),
		Side:    SideBuy,
	}, nil)
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	contracts, err := getContractConfig(client.chainID)
	if err != nil {
		t.Fatalf("getContractConfig: %v", err)
	}
	if !signatureUsesContract(order, client.chainID, contracts.NegRiskExchange) {
		t.Fatal("order signature did not use the cached negative-risk exchange")
	}
}

func signatureUsesContract(order *SignedOrder, chainID int64, contract string) bool {
	typedData := buildOrderTypedData(chainID, contract, *order)
	digest, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		return false
	}
	signature, err := hex.DecodeString(strings.TrimPrefix(order.Signature, "0x"))
	if err != nil || len(signature) != 65 {
		return false
	}
	if signature[64] >= 27 {
		signature[64] -= 27
	}
	publicKey, err := crypto.SigToPub(digest, signature)
	if err != nil {
		return false
	}
	return crypto.PubkeyToAddress(*publicKey).Hex() == order.Order.Signer
}

func TestCreateOrderRefreshesStaleTickMetadata(t *testing.T) {
	var tickCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case tickSizeEndpoint:
			tickCalls.Add(1)
			_, _ = w.Write([]byte(`{"minimum_tick_size":"0.01"}`))
		case negRiskEndpoint:
			_, _ = w.Write([]byte(`{"neg_risk":false}`))
		case marketsByTokenEndpoint + "100":
			_, _ = w.Write([]byte(`{"condition_id":"cid"}`))
		case clobMarketEndpoint + "/cid":
			_, _ = w.Write([]byte(`{"c":"cid","mts":"0.001","nr":false,"t":[{"t":"100"}]}`))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	client, err := NewSignerClient(Config{
		Host:       server.URL,
		PrivateKey: "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		RetryMax:   0,
	})
	if err != nil {
		t.Fatalf("NewSignerClient: %v", err)
	}
	client.saltGenerator = func() (uint64, error) { return 42, nil }

	order, err := client.CreateOrder(t.Context(), OrderArgs{
		TokenID: "100",
		Price:   udecimal.MustParse("0.451"),
		Size:    udecimal.MustParse("10"),
		Side:    SideBuy,
	}, nil)
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	if order.Order.MakerAmount != "4510000" || order.Order.TakerAmount != "10000000" {
		t.Fatalf("order amounts = %s/%s", order.Order.MakerAmount, order.Order.TakerAmount)
	}
	if tickCalls.Load() != 0 {
		t.Fatalf("tick-size requests = %d, want 0", tickCalls.Load())
	}
}
