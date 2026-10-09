package clob

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/nijaru/go-clob-client/internal/polyauth"
	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

func TestCredentialSnapshotIsolationAndConcurrentRotation(t *testing.T) {
	t.Parallel()
	creds := Credentials{Key: "k1", Secret: "YWFhYQ==", Passphrase: "p1"}
	client, err := NewAuthenticatedClient(
		Config{
			PrivateKey:  "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
			Credentials: &creds,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	creds.Key, creds.Secret, creds.Passphrase = "outside", "YmJiYg==", "outside"
	copy := client.Credentials()
	copy.Key = "mutated returned copy"
	if client.Credentials().Key != "k1" {
		t.Fatal("constructor or returned credentials alias caller memory")
	}
	sets := []Credentials{
		{Key: "k1", Secret: "YWFhYQ==", Passphrase: "p1"},
		{Key: "k2", Secret: "YmJiYg==", Passphrase: "p2"},
	}
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := 0; i < 200; i++ {
			if err := client.SetCredentials(sets[i%2]); err != nil {
				t.Error(err)
				return
			}
		}
	})
	for range 4 {
		wg.Go(func() {
			for range 200 {
				headers, err := client.addAuthHeaders(
					t.Context(),
					http.MethodPost,
					"/order",
					[]byte(`{"x":"a'b"}`),
					polyhttp.AuthL2,
					nil,
				)
				if err != nil {
					t.Error(err)
					return
				}
				index := 0
				if headers["POLY_API_KEY"] == "k2" {
					index = 1
				} else if headers["POLY_API_KEY"] != "k1" {
					t.Error("unknown credential key")
					return
				}
				stamp, err := strconv.ParseInt(headers["POLY_TIMESTAMP"], 10, 64)
				if err != nil {
					t.Error(err)
					return
				}
				want, err := polyauth.HMACSignature(
					sets[index].Secret,
					stamp,
					http.MethodPost,
					"/order",
					[]byte(`{"x":"a'b"}`),
				)
				if err != nil {
					t.Error(err)
					return
				}
				if headers["POLY_SIGNATURE"] != want ||
					headers["POLY_PASSPHRASE"] != sets[index].Passphrase {
					t.Error("mixed credential generations")
					return
				}
			}
		})
	}
	wg.Wait()
	before := *client.Credentials()
	for _, invalid := range []Credentials{{Key: "", Secret: "YQ==", Passphrase: "p"}, {Key: "k", Secret: "", Passphrase: "p"}, {Key: "k", Secret: "YQ==", Passphrase: ""}} {
		if err := client.SetCredentials(invalid); err == nil {
			t.Fatal("accepted incomplete credentials")
		}
		if *client.Credentials() != before {
			t.Fatal("invalid rotation changed active credentials")
		}
	}
}

func TestAuthPromotionSharesCacheAndPreservesResponseCallbacks(t *testing.T) {
	t.Parallel()
	calls := make(chan struct{}, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Poly-RateLimit-Tier", "test")
		_, _ = w.Write([]byte(`{"version":2}`))
	}))
	defer server.Close()
	base, err := NewClient(
		Config{
			Host:              server.URL,
			OnRateLimitUpdate: func(*polyhttp.RateLimitUpdate) { calls <- struct{}{} },
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	local, err := polyauth.ParsePrivateKey(
		"0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
	)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := base.AsSigner(
		local,
		SignatureTypeEOA,
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	auth, err := signer.AsAuthenticated(Credentials{Key: "k", Secret: "YQ==", Passphrase: "p"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	base.SetTickSize("123", TickSizeHundredth)
	if tick, ok := auth.cachedTickSize("123"); !ok || tick != TickSizeHundredth {
		t.Fatal("promotion lost shared cache")
	}
	auth.InvalidateCaches()
	if _, ok := base.cachedTickSize("123"); ok {
		t.Fatal("invalidation forked cache state")
	}
	if _, err := auth.resolveServerVersion(t.Context(), false); err != nil {
		t.Fatal(err)
	}
	if _, err := base.resolveServerVersion(t.Context(), false); err != nil {
		t.Fatal(err)
	}
	select {
	case <-calls:
	default:
		t.Fatal("promotion dropped response callback")
	}
	select {
	case <-calls:
		t.Fatal("promotion forked protocol-version cache")
	default:
	}
}

func TestBuilderAuthRotationConcurrentWithRevoke(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Poly-Builder-Key") != "test" {
			t.Error("missing builder authorization")
		}
		_, _ = w.Write([]byte(`"OK"`))
	}))
	defer server.Close()
	client := newComboTestClient(t, server.URL)
	client.http.BaseURL = server.URL
	client.rateLimiter = nil
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case <-done:
				return
			default:
				client.PromoteToBuilder(staticBuilderAuth{})
			}
		}
	})
	for range 20 {
		if err := client.RevokeBuilderAPIKey(t.Context()); err != nil {
			t.Error(err)
			break
		}
	}
	close(done)
	wg.Wait()
}

func TestCanceledContextDoesNotStartHeartbeatLoop(t *testing.T) {
	t.Parallel()
	client, err := NewAuthenticatedClient(
		Config{
			PrivateKey:  "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
			Credentials: &Credentials{Key: "k", Secret: "YQ==", Passphrase: "p"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := client.StartHeartbeats(ctx); err != context.Canceled {
		t.Fatalf("start = %v", err)
	}
	if client.HeartbeatsActive() {
		t.Fatal("canceled start allocated background work")
	}
}
