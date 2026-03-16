package clob

import (
	json "encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetCredentials(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data, _ := json.Marshal([]Notification{})
		w.Write(data)
	}))
	defer server.Close()

	privateKey := "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c"
	client, err := New(Config{Host: server.URL, PrivateKey: privateKey})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	// Initially no credentials — L2 call should fail
	_, err = client.GetNotifications(t.Context())
	if err == nil {
		t.Fatal("expected error without credentials")
	}

	// Set credentials
	client.SetCredentials(Credentials{
		Key:        "new-key",
		Secret:     "c2VjcmV0",
		Passphrase: "new-pass",
	})

	// Now L2 call should succeed
	_, err = client.GetNotifications(t.Context())
	if err != nil {
		t.Fatalf("get notifications after SetCredentials: %v", err)
	}
}

func TestHost(t *testing.T) {
	t.Parallel()

	client, err := New(Config{Host: "https://example.com"})
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
	client, err := New(Config{PrivateKey: privateKey})
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

func TestAddressWithoutSigner(t *testing.T) {
	t.Parallel()

	client, err := New(Config{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	if got := client.Address(); got != "" {
		t.Errorf("Address() = %q, want empty without signer", got)
	}
}

func TestClearTickSizeCache(t *testing.T) {
	t.Parallel()

	client, err := New(Config{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	// Populate cache directly (cache is populated by resolveTickSize, not GetTickSize)
	client.tickSizeMu.Lock()
	client.tickSizeCache["token-1"] = "0.01"
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
	client.tickSizeMu.RUnlock()
	if ok {
		t.Error("expected token-1 removed from cache after ClearTickSizeCache")
	}
}

func TestClearFeeRateCache(t *testing.T) {
	t.Parallel()

	client, err := New(Config{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	// Populate cache directly (cache is populated by resolveFeeRateBps, not GetFeeRateBps)
	client.feeRateMu.Lock()
	client.feeRateCache["token-1"] = 50
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
	client.feeRateMu.RUnlock()
	if ok {
		t.Error("expected token-1 removed from fee rate cache")
	}
}

func TestClearTickSizeCaches(t *testing.T) {
	t.Parallel()

	client, err := New(Config{})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	// Populate all caches directly
	client.tickSizeMu.Lock()
	client.tickSizeCache["tok-1"] = "0.01"
	client.tickSizeMu.Unlock()

	client.negRiskMu.Lock()
	client.negRiskCache["tok-1"] = true
	client.negRiskMu.Unlock()

	client.feeRateMu.Lock()
	client.feeRateCache["tok-1"] = 50
	client.feeRateMu.Unlock()

	// Clear all
	client.ClearTickSizeCaches()

	client.tickSizeMu.RLock()
	if len(client.tickSizeCache) != 0 {
		t.Error("tick size cache should be empty")
	}
	client.tickSizeMu.RUnlock()

	client.negRiskMu.RLock()
	if len(client.negRiskCache) != 0 {
		t.Error("neg risk cache should be empty")
	}
	client.negRiskMu.RUnlock()

	client.feeRateMu.RLock()
	if len(client.feeRateCache) != 0 {
		t.Error("fee rate cache should be empty")
	}
	client.feeRateMu.RUnlock()
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
	client, err := New(Config{
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
	client, err := New(Config{
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

func TestDeriveWSAuthRequiresCredentials(t *testing.T) {
	t.Parallel()

	privateKey := "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c"
	client, err := New(Config{PrivateKey: privateKey})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.DeriveWSAuth(t.Context())
	if err == nil {
		t.Fatal("expected error without credentials")
	}
}
