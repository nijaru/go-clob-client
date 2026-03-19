package clob

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAsAuthenticated(t *testing.T) {
	t.Parallel()

	privateKey := "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c"
	client, err := NewSignerClient(Config{Host: "http://example.com", PrivateKey: privateKey})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	// Upgrade to AuthenticatedClient
	authClient := client.AsAuthenticated(Credentials{
		Key:        "new-key",
		Secret:     "c2VjcmV0",
		Passphrase: "new-pass",
	}, nil)

	if authClient == nil {
		t.Fatal("expected client to be authenticated")
	}
}

func TestHost(t *testing.T) {
	t.Parallel()

	client, err := NewClient(Config{Host: "https://example.com"})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	if got := client.Host(); got != "https://example.com" {
		t.Errorf("Host() = %q, want %q", got, "https://example.com")
	}
}

func TestAddressWithSigner(t *testing.T) {
	t.Parallel()

	privateKey := "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c"
	client, err := NewSignerClient(Config{PrivateKey: privateKey})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	addr := client.Address()
	if addr == "" {
		t.Fatal("Address() should not be empty with private key")
	}
	if len(addr) != 42 || addr[:2] != "0x" {
		t.Errorf("unexpected address format: %s", addr)
	}
}

func TestClearTickSizeCache(t *testing.T) {
	t.Parallel()

	client, err := NewClient(Config{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	// Populate cache directly (cache is populated by resolveTickSize, not GetTickSize)
	client.tickSizeMu.Lock()
	client.tickSizeCache["token-1"] = "0.01"
	client.tickSizeTimestamps["token-1"] = time.Now()
	client.tickSizeMu.Unlock()

	// Verify cached
	client.tickSizeMu.RLock()
	_, ok := client.tickSizeCache["token-1"]
	client.tickSizeMu.RUnlock()
	if !ok {
		t.Fatal("expected token-1 in cache")
	}

	// Clear specific token
	client.ClearTickSizeCache("token-1")

	client.tickSizeMu.RLock()
	_, ok = client.tickSizeCache["token-1"]
	_, tsOK := client.tickSizeTimestamps["token-1"]
	client.tickSizeMu.RUnlock()
	if ok {
		t.Error("expected token-1 removed from cache after ClearTickSizeCache")
	}
	if tsOK {
		t.Error("expected token-1 timestamp removed after ClearTickSizeCache")
	}
}

func TestClearFeeRateCache(t *testing.T) {
	t.Parallel()

	client, err := NewClient(Config{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	// Populate cache directly (cache is populated by resolveFeeRateBps, not GetFeeRateBps)
	client.feeRateMu.Lock()
	client.feeRateCache["token-1"] = 50
	client.feeRateTimestamps["token-1"] = time.Now()
	client.feeRateMu.Unlock()

	client.feeRateMu.RLock()
	_, ok := client.feeRateCache["token-1"]
	client.feeRateMu.RUnlock()
	if !ok {
		t.Fatal("expected token-1 in fee rate cache")
	}

	client.ClearFeeRateCache("token-1")

	client.feeRateMu.RLock()
	_, ok = client.feeRateCache["token-1"]
	_, tsOK := client.feeRateTimestamps["token-1"]
	client.feeRateMu.RUnlock()
	if ok {
		t.Error("expected token-1 removed from fee rate cache")
	}
	if tsOK {
		t.Error("expected token-1 timestamp removed from fee rate cache")
	}
}

func TestClearTickSizeCaches(t *testing.T) {
	t.Parallel()

	client, err := NewClient(Config{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	// Populate all caches directly
	client.tickSizeMu.Lock()
	client.tickSizeCache["tok-1"] = "0.01"
	client.tickSizeTimestamps["tok-1"] = time.Now()
	client.tickSizeMu.Unlock()

	client.negRiskMu.Lock()
	client.negRiskCache["tok-1"] = true
	client.negRiskTimestamps["tok-1"] = time.Now()
	client.negRiskMu.Unlock()

	client.feeRateMu.Lock()
	client.feeRateCache["tok-1"] = 50
	client.feeRateTimestamps["tok-1"] = time.Now()
	client.feeRateMu.Unlock()

	// Clear all
	client.ClearTickSizeCaches()

	client.tickSizeMu.RLock()
	if len(client.tickSizeCache) != 0 {
		t.Error("tick size cache should be empty")
	}
	if len(client.tickSizeTimestamps) != 0 {
		t.Error("tick size timestamps should be empty")
	}
	client.tickSizeMu.RUnlock()

	client.negRiskMu.RLock()
	if len(client.negRiskCache) != 0 {
		t.Error("neg risk cache should be empty")
	}
	if len(client.negRiskTimestamps) != 0 {
		t.Error("neg risk timestamps should be empty")
	}
	client.negRiskMu.RUnlock()

	client.feeRateMu.RLock()
	if len(client.feeRateCache) != 0 {
		t.Error("fee rate cache should be empty")
	}
	if len(client.feeRateTimestamps) != 0 {
		t.Error("fee rate timestamps should be empty")
	}
	client.feeRateMu.RUnlock()
}

func TestManualCacheSettersHonorTTL(t *testing.T) {
	t.Parallel()

	client, err := NewClient(Config{TickSizeCacheTTL: time.Minute})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	client.SetTickSize("token-1", TickSizeHundredth)
	client.SetNegRisk("token-1", true)
	client.SetFeeRateBps("token-1", 50)

	if client.tickSizeTimestamps["token-1"].IsZero() {
		t.Fatal("expected tick size timestamp to be populated")
	}
	if client.negRiskTimestamps["token-1"].IsZero() {
		t.Fatal("expected neg risk timestamp to be populated")
	}
	if client.feeRateTimestamps["token-1"].IsZero() {
		t.Fatal("expected fee rate timestamp to be populated")
	}

	tickSize, err := client.resolveTickSize(t.Context(), "token-1", nil)
	if err != nil {
		t.Fatalf("resolve tick size: %v", err)
	}
	if tickSize != TickSizeHundredth {
		t.Fatalf("tick size = %q, want %q", tickSize, TickSizeHundredth)
	}

	negRisk, err := client.resolveNegRisk(t.Context(), "token-1", nil)
	if err != nil {
		t.Fatalf("resolve neg risk: %v", err)
	}
	if !negRisk {
		t.Fatal("expected neg risk cache hit")
	}

	feeRate, err := client.resolveFeeRateBps(t.Context(), "token-1", 0)
	if err != nil {
		t.Fatalf("resolve fee rate: %v", err)
	}
	if feeRate != 50 {
		t.Fatalf("fee rate = %d, want 50", feeRate)
	}
}

func TestDropNotifications(t *testing.T) {
	t.Parallel()

	var receivedIDs string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != notificationsEndpoint {
			http.NotFound(w, r)
			return
		}
		receivedIDs = r.URL.Query().Get("ids")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	privateKey := "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c"
	client, err := NewAuthenticatedClient(Config{
		Host:       server.URL,
		PrivateKey: privateKey,
		Credentials: &Credentials{
			Key:        "key",
			Secret:     "c2VjcmV0",
			Passphrase: "pass",
		},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	err = client.DropNotifications(t.Context(), DeleteNotificationsParams{
		IDs: []string{"n1", "n2"},
	})
	if err != nil {
		t.Fatalf("drop notifications: %v", err)
	}

	if receivedIDs != "n1,n2" {
		t.Errorf("received ids = %q, want %q", receivedIDs, "n1,n2")
	}
}

func TestDeriveWSAuth(t *testing.T) {
	t.Parallel()

	privateKey := "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c"
	client, err := NewAuthenticatedClient(Config{
		PrivateKey: privateKey,
		Credentials: &Credentials{
			Key:        "my-key",
			Secret:     "c2VjcmV0",
			Passphrase: "my-pass",
		},
	})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	auth, err := client.DeriveWSAuth(t.Context())
	if err != nil {
		t.Fatalf("derive ws auth: %v", err)
	}

	if auth.Key != "my-key" {
		t.Errorf("key = %q, want %q", auth.Key, "my-key")
	}
	if auth.Passphrase != "my-pass" {
		t.Errorf("passphrase = %q, want %q", auth.Passphrase, "my-pass")
	}
	if auth.Timestamp == "" {
		t.Error("timestamp should not be empty")
	}
	if auth.Signature == "" {
		t.Error("signature should not be empty")
	}
}

func TestCredentialsReturnsCopy(t *testing.T) {
	t.Parallel()

	client, err := NewAuthenticatedClient(Config{
		PrivateKey: "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c",
		Credentials: &Credentials{
			Key:        "orig-key",
			Secret:     "orig-secret",
			Passphrase: "orig-pass",
		},
		DisableAutoHeartbeat: true,
	})
	if err != nil {
		t.Fatalf("new authenticated client: %v", err)
	}

	creds := client.Credentials()
	if creds == nil {
		t.Fatal("expected credentials")
	}
	creds.Key = "mutated"
	creds.Secret = "changed"
	creds.Passphrase = "changed"

	current := client.Credentials()
	if current == nil {
		t.Fatal("expected credentials copy")
	}
	if current.Key != "orig-key" || current.Secret != "orig-secret" ||
		current.Passphrase != "orig-pass" {
		t.Fatalf("client credentials mutated through getter: %#v", current)
	}
}
