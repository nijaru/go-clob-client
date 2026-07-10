package clob

import (
	"bytes"
	"io"
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
	authClient, err := client.AsAuthenticated(Credentials{
		Key:        "new-key",
		Secret:     "c2VjcmV0",
		Passphrase: "new-pass",
	}, nil)
	if err != nil {
		t.Fatalf("as authenticated: %v", err)
	}

	if authClient == nil {
		t.Fatal("expected client to be authenticated")
	}
}

func TestSetCredentialsRotatesDecodedSecret(t *testing.T) {
	t.Parallel()
	// SetCredentials must re-derive the decoded HMAC secret on rotation, not
	// just swap the Credentials struct (regression: it previously left
	// decodedSecret stale, so every signed request after rotation used the OLD
	// secret with the NEW key/passphrase and was rejected).
	const (
		privKey = "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae1a40cf83f4a2f9c"
		secret1 = "YWFhYQ==" // "aaaa"
		secret2 = "YmJiYg==" // "bbbb"
		invalid = "!!!!"     // valid length, invalid base64 chars -> decode error
	)
	sc, err := NewSignerClient(Config{Host: "http://example.com", PrivateKey: privKey})
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	client, err := sc.AsAuthenticated(
		Credentials{Key: "k1", Secret: secret1, Passphrase: "p1"},
		nil,
	)
	if err != nil {
		t.Fatalf("as authenticated: %v", err)
	}

	before := append([]byte(nil), client.decodedSecret...)
	if err := client.SetCredentials(Credentials{Key: "k2", Secret: secret2, Passphrase: "p2"}); err != nil {
		t.Fatalf("SetCredentials (valid): %v", err)
	}
	if got := client.credentials().Key; got != "k2" {
		t.Fatalf("key = %s, want k2", got)
	}
	if bytes.Equal(client.decodedSecret, before) {
		t.Fatal("decodedSecret unchanged after rotation (stale-secret bug regressed)")
	}

	// An invalid secret must error and leave the current credentials untouched.
	if err := client.SetCredentials(Credentials{Key: "k3", Secret: invalid, Passphrase: "p3"}); err == nil {
		t.Fatal("expected error for invalid base64 secret, got nil")
	}
	if got := client.credentials().Key; got != "k2" {
		t.Fatalf("key = %s after failed rotation, want k2 (credentials must be unchanged)", got)
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

func TestInvalidateCaches(t *testing.T) {
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

	// Clear all
	client.InvalidateCaches()

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
}

func TestManualCacheSettersHonorTTL(t *testing.T) {
	t.Parallel()

	client, err := NewClient(Config{TickSizeCacheTTL: time.Minute})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	client.SetTickSize("token-1", TickSizeHundredth)
	client.SetNegRisk("token-1", true)

	if client.tickSizeTimestamps["token-1"].IsZero() {
		t.Fatal("expected tick size timestamp to be populated")
	}
	if client.negRiskTimestamps["token-1"].IsZero() {
		t.Fatal("expected neg risk timestamp to be populated")
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
}

func TestDeleteNotifications(t *testing.T) {
	t.Parallel()

	var receivedIDs string
	var requestBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != notificationsEndpoint {
			http.NotFound(w, r)
			return
		}
		receivedIDs = r.URL.Query().Get("ids")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		requestBody = string(body)
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

	err = client.DeleteNotifications(t.Context(), DeleteNotificationsParams{
		IDs: []string{"n1", "n2"},
	})
	if err != nil {
		t.Fatalf("drop notifications: %v", err)
	}

	if receivedIDs != "n1,n2" {
		t.Errorf("received ids = %q, want %q", receivedIDs, "n1,n2")
	}
	if requestBody != `["n1","n2"]` {
		t.Errorf("request body = %q, want %q", requestBody, `["n1","n2"]`)
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
