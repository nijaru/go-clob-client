package polyauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHMACSignatureReplacesSingleQuotes(t *testing.T) {
	t.Parallel()

	secret := "c2VjcmV0"
	withSingle, err := HMACSignature(secret, 1710000000, "POST", "/order", []byte("{'a':'b'}"))
	if err != nil {
		t.Fatalf("signature with single quotes: %v", err)
	}
	withDouble, err := HMACSignature(secret, 1710000000, "POST", "/order", []byte(`{"a":"b"}`))
	if err != nil {
		t.Fatalf("signature with double quotes: %v", err)
	}

	if withSingle != withDouble {
		t.Fatalf("signatures differ: %q vs %q", withSingle, withDouble)
	}
}

func TestFetchRemoteBuilderHeaders(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Fatalf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"poly_builder_api_key":"key",
			"poly_builder_timestamp":"1710000000",
			"poly_builder_passphrase":"pass",
			"poly_builder_signature":"sig"
		}`))
	}))
	defer server.Close()

	headers, err := FetchRemoteBuilderHeaders(
		context.Background(),
		server.Client(),
		server.URL,
		"token",
		RemoteBuilderHeaderRequest{
			Method:    http.MethodPost,
			Path:      "/builder",
			Body:      "{}",
			Timestamp: 1710000000,
		},
	)
	if err != nil {
		t.Fatalf("fetch remote builder headers: %v", err)
	}

	if headers["POLY_BUILDER_API_KEY"] != "key" || headers["POLY_BUILDER_SIGNATURE"] != "sig" {
		t.Fatalf("unexpected headers: %#v", headers)
	}
}

func TestFetchRemoteBuilderHeadersReturnsStatusError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	defer server.Close()

	_, err := FetchRemoteBuilderHeaders(
		context.Background(),
		server.Client(),
		server.URL,
		"",
		RemoteBuilderHeaderRequest{},
	)
	if err == nil || !strings.Contains(err.Error(), "status 502") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestHMACSignatureBytesMatchesBuilderSigningSDK pins HMACSignatureBytes (the raw
// primitive shared by builder/relayer auth) against golden vectors computed with
// the official go-builder-signing-sdk GenSignature. The body deliberately contains
// an apostrophe to prove builder/relayer auth signs the body verbatim and does NOT
// apply the CLOB-L2 single→double quote rewrite that the CLOB server performs.
func TestHMACSignatureBytesMatchesBuilderSigningSDK(t *testing.T) {
	t.Parallel()

	secret := []byte("super-secret-key")

	got := HMACSignatureBytes(
		secret,
		1718000000,
		"POST",
		"/submit",
		[]byte(`{"metadata":"President's choice"}`),
	)
	want := "zw3IAVBBVpn31T-3bSHNMSaI-_pIPnhhU2hzWI2leog="
	if got != want {
		t.Fatalf("body signature: got %q want %q (official builder-signing-sdk)", got, want)
	}

	gotNone := HMACSignatureBytes(secret, 1718000000, "GET", "/deployed", nil)
	wantNone := "tBklY9vATHJdKKMAETsCPbPrtyiUtVZZKOsHq3qxKZU="
	if gotNone != wantNone {
		t.Fatalf(
			"nil-body signature: got %q want %q (official builder-signing-sdk)",
			gotNone,
			wantNone,
		)
	}
}

// TestBuilderHeadersDoNotNormalizeQuotes verifies builder/relayer auth (POLY_BUILDER_*)
// signs the raw body, diverging from CLOB L2 auth (which rewrites quotes). An
// apostrophe in the body must reach the HMAC untouched.
func TestBuilderHeadersDoNotNormalizeQuotes(t *testing.T) {
	t.Parallel()

	secret := []byte("super-secret-key")
	body := []byte(`{"metadata":"President's choice"}`)

	rawSig := HMACSignatureBytes(secret, 1718000000, "POST", "/submit", body)
	headers, err := BuilderHeaders("key", secret, "pass", 1718000000, "POST", "/submit", body)
	if err != nil {
		t.Fatalf("BuilderHeaders: %v", err)
	}
	if headers["POLY_BUILDER_SIGNATURE"] != rawSig {
		t.Fatalf("builder signature %q != raw %q (builder auth must not normalize quotes)",
			headers["POLY_BUILDER_SIGNATURE"], rawSig)
	}

	// It must differ from the L2-normalized signature the CLOB server expects.
	l2Sig := HMACSignatureBytes(secret, 1718000000, "POST", "/submit", normalizeSignatureBody(body))
	if headers["POLY_BUILDER_SIGNATURE"] == l2Sig {
		t.Fatal(
			"builder signature matched the L2-normalized signature; builder auth should sign verbatim",
		)
	}
}
