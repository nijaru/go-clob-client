package polyhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-json-experiment/json/jsontext"
)

func TestGetJSONDecodesInt64Response(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("1710000000"))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client(), UserAgent: "test"}
	var out int64
	if err := client.GetJSON(context.Background(), "/", nil, AuthNone, &out); err != nil {
		t.Fatalf("get json int64: %v", err)
	}
	if out != 1710000000 {
		t.Fatalf("out = %d", out)
	}
}

func TestGetJSONDecodesStringResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`"hello"`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client(), UserAgent: "test"}
	var out string
	if err := client.GetJSON(context.Background(), "/", nil, AuthNone, &out); err != nil {
		t.Fatalf("get json string: %v", err)
	}
	if out != "hello" {
		t.Fatalf("out = %q", out)
	}
}

func TestGetJSONDecodesJSONTextValue(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client(), UserAgent: "test"}
	var out jsontext.Value
	if err := client.GetJSON(context.Background(), "/", nil, AuthNone, &out); err != nil {
		t.Fatalf("get json value: %v", err)
	}
	if string(out) != `{"ok":true}` {
		t.Fatalf("out = %s", out)
	}
}

func TestGetJSONReturnsStructuredAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"bad request"}}`, http.StatusBadRequest)
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client(), UserAgent: "test"}
	err := client.GetJSON(context.Background(), "/", url.Values{"x": {"1"}}, AuthNone, nil)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T (%v)", err, err)
	}
	if apiErr.StatusCode != http.StatusBadRequest || apiErr.Message != "bad request" {
		t.Fatalf("unexpected api error: %+v", apiErr)
	}
}

func TestGetJSONWrapsTransportError(t *testing.T) {
	t.Parallel()

	client := &Client{
		BaseURL:    "http://127.0.0.1:1",
		HTTPClient: &http.Client{},
		UserAgent:  "test",
	}
	err := client.GetJSON(context.Background(), "/", nil, AuthNone, nil)
	if err == nil || !strings.Contains(err.Error(), "perform request") {
		t.Fatalf("unexpected error: %v", err)
	}
}
