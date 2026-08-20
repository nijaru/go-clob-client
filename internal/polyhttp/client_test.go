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

func TestAPIErrorRetryAfterHeader(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "17.5")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"retry_after_seconds":3}`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client(), UserAgent: "test"}
	err := client.GetJSON(context.Background(), "/", nil, AuthNone, nil)
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.RetryAfterSeconds == nil || *apiErr.RetryAfterSeconds != 17.5 {
		t.Fatalf("unexpected retry metadata: %#v", err)
	}
}

func TestAPIErrorRetryAfterBodyFallback(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limited","retry_after_seconds":2.25}`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client(), UserAgent: "test"}
	err := client.GetJSON(context.Background(), "/", nil, AuthNone, nil)
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.RetryAfterSeconds == nil || *apiErr.RetryAfterSeconds != 2.25 {
		t.Fatalf("unexpected retry metadata: %#v", err)
	}
}

func TestAPIErrorRetryAfterRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		header string
		body   string
	}{
		{name: "negative header", header: "-1", body: `{}`},
		{name: "nonfinite header", header: "NaN", body: `{}`},
		{name: "string body", body: `{"retry_after_seconds":"4"}`},
		{name: "negative body", body: `{"retry_after_seconds":-1}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if tc.header != "" {
						w.Header().Set("Retry-After", tc.header)
					}
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(tc.body))
				}),
			)
			defer server.Close()

			client := &Client{BaseURL: server.URL, HTTPClient: server.Client(), UserAgent: "test"}
			err := client.GetJSON(context.Background(), "/", nil, AuthNone, nil)
			apiErr, ok := err.(*APIError)
			if !ok || apiErr.RetryAfterSeconds != nil {
				t.Fatalf("unexpected retry metadata: %#v", err)
			}
		})
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

func TestAPIErrorTradingRestrictionRestarting(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooEarly)
		_, _ = w.Write([]byte(`{"error":"engine restarting"}`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client(), UserAgent: "test"}
	err := client.GetJSON(context.Background(), "/", nil, AuthNone, nil)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.TradingRestriction == nil || *apiErr.TradingRestriction != TradingRestrictionRestarting {
		t.Fatalf("unexpected restriction: %#v", apiErr.TradingRestriction)
	}
}

func TestAPIErrorTradingRestrictionPostOnly(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"code":"post_only_mode","error":"post-only mode"}`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client(), UserAgent: "test"}
	err := client.GetJSON(context.Background(), "/", nil, AuthNone, nil)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.TradingRestriction == nil || *apiErr.TradingRestriction != TradingRestrictionPostOnly {
		t.Fatalf("unexpected restriction: %#v", apiErr.TradingRestriction)
	}
	if apiErr.Code == nil || *apiErr.Code != "post_only_mode" {
		t.Fatalf("unexpected code: %#v", apiErr.Code)
	}
}

func TestAPIErrorTradingRestrictionCancelOnly(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"cancel-only: only cancels are accepted"}`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client(), UserAgent: "test"}
	err := client.GetJSON(context.Background(), "/", nil, AuthNone, nil)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.TradingRestriction == nil || *apiErr.TradingRestriction != TradingRestrictionCancelOnly {
		t.Fatalf("unexpected restriction: %#v", apiErr.TradingRestriction)
	}
}

func TestAPIErrorNoTradingRestrictionOnOrdinary503(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":"service unavailable"}`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client(), UserAgent: "test"}
	err := client.GetJSON(context.Background(), "/", nil, AuthNone, nil)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.TradingRestriction != nil {
		t.Fatalf("unexpected restriction: %#v", apiErr.TradingRestriction)
	}
}

func TestAPIErrorCodeFromJSONBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"invalid_nonce","error":"bad nonce"}`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client(), UserAgent: "test"}
	err := client.GetJSON(context.Background(), "/", nil, AuthNone, nil)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code == nil || *apiErr.Code != "invalid_nonce" {
		t.Fatalf("unexpected code: %#v", apiErr.Code)
	}
}

func TestAPIErrorCodeAbsentWhenNotJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`bad request`))
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client(), UserAgent: "test"}
	err := client.GetJSON(context.Background(), "/", nil, AuthNone, nil)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != nil {
		t.Fatalf("unexpected code: %#v", apiErr.Code)
	}
}

func TestParseRateLimitHeadersFull(t *testing.T) {
	t.Parallel()

	headers := http.Header{}
	headers.Set("Poly-RateLimit-Remaining", "42")
	headers.Set("Poly-RateLimit-Reset", "1700000000")
	headers.Set("Poly-RateLimit-Tier", "order")
	headers.Set("Poly-RateLimit-Warning", "true")

	update := parseRateLimitHeaders(headers)
	if update == nil {
		t.Fatal("expected rate-limit update")
	}
	if update.Remaining == nil || *update.Remaining != 42 {
		t.Errorf("remaining = %#v", update.Remaining)
	}
	if update.Reset == nil || *update.Reset != 1700000000 {
		t.Errorf("reset = %#v", update.Reset)
	}
	if update.Tier == nil || *update.Tier != "order" {
		t.Errorf("tier = %#v", update.Tier)
	}
	if !update.Warning {
		t.Errorf("warning = false, want true")
	}
}

func TestParseRateLimitHeadersPartial(t *testing.T) {
	t.Parallel()

	headers := http.Header{}
	headers.Set("Poly-RateLimit-Tier", "cancel")

	update := parseRateLimitHeaders(headers)
	if update == nil {
		t.Fatal("expected rate-limit update")
	}
	if update.Tier == nil || *update.Tier != "cancel" {
		t.Errorf("tier = %#v", update.Tier)
	}
	if update.Remaining != nil {
		t.Errorf("remaining = %#v, want nil", update.Remaining)
	}
	if update.Warning {
		t.Errorf("warning = true, want false")
	}
}

func TestParseRateLimitHeadersEmpty(t *testing.T) {
	t.Parallel()

	update := parseRateLimitHeaders(http.Header{})
	if update != nil {
		t.Errorf("expected nil, got %#v", update)
	}
}

func TestParseRateLimitHeadersRejectsInvalid(t *testing.T) {
	t.Parallel()

	// All fields invalid or absent: update is nil.
	headers := http.Header{}
	headers.Set("Poly-RateLimit-Remaining", "NaN")

	update := parseRateLimitHeaders(headers)
	if update != nil {
		t.Errorf("expected nil, got %#v", update)
	}
}

func TestRateLimitCallbackOnSuccess(t *testing.T) {
	t.Parallel()

	var got *RateLimitUpdate
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Poly-RateLimit-Remaining", "10")
		w.Header().Set("Poly-RateLimit-Tier", "order")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := &Client{
		BaseURL:           server.URL,
		HTTPClient:        server.Client(),
		UserAgent:         "test",
		OnRateLimitUpdate: func(u *RateLimitUpdate) { got = u },
	}

	var out struct {
		OK bool `json:"ok"`
	}
	if err := client.GetJSON(context.Background(), "/", nil, AuthNone, &out); err != nil {
		t.Fatalf("get json: %v", err)
	}
	if got == nil {
		t.Fatal("expected rate-limit callback")
	}
	if got.Remaining == nil || *got.Remaining != 10 {
		t.Errorf("remaining = %#v", got.Remaining)
	}
	if got.Tier == nil || *got.Tier != "order" {
		t.Errorf("tier = %#v", got.Tier)
	}
}

func TestRateLimitCallbackDoesNotAffectRequestHandling(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Poly-RateLimit-Remaining", "10")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := &Client{
		BaseURL:   server.URL,
		HTTPClient: server.Client(),
		UserAgent: "test",
		OnRateLimitUpdate: func(u *RateLimitUpdate) {
			panic("listener must not affect request handling")
		},
	}

	var out struct {
		OK bool `json:"ok"`
	}
	if err := client.GetJSON(context.Background(), "/", nil, AuthNone, &out); err != nil {
		t.Fatalf("get json: %v", err)
	}
}

func TestRateLimitCallbackOnRateLimitError(t *testing.T) {
	t.Parallel()

	var got *RateLimitUpdate
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Poly-RateLimit-Remaining", "0")
		w.Header().Set("Poly-RateLimit-Reset", "1700000000")
		w.Header().Set("Poly-RateLimit-Tier", "order")
		w.Header().Set("Poly-RateLimit-Warning", "true")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer server.Close()

	client := &Client{
		BaseURL:           server.URL,
		HTTPClient:        server.Client(),
		UserAgent:         "test",
		OnRateLimitUpdate: func(u *RateLimitUpdate) { got = u },
	}

	err := client.GetJSON(context.Background(), "/", nil, AuthNone, nil)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.RateLimit == nil {
		t.Fatal("expected rate-limit state on error")
	}
	if got == nil {
		t.Fatal("expected rate-limit callback on error")
	}
	if got.Remaining == nil || *got.Remaining != 0 {
		t.Errorf("remaining = %#v", got.Remaining)
	}
	if got.Reset == nil || *got.Reset != 1700000000 {
		t.Errorf("reset = %#v", got.Reset)
	}
	if !got.Warning {
		t.Errorf("warning = false, want true")
	}
}
