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
