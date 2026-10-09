package polyhttp

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPDateRetryAfter(t *testing.T) {
	reference := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name, header string
		want         float64
	}{
		{"future date with server clock", reference.Add(83 * time.Second).Format(http.TimeFormat), 83},
		{"expired date", reference.Add(-time.Second).Format(http.TimeFormat), 0},
		{"numeric header retains precedence", "17.5", 17.5},
		{"invalid date uses JSON fallback", "not-a-date", 2.25},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Date", reference.Format(http.TimeFormat))
					w.Header().Set("Retry-After", test.header)
					w.WriteHeader(http.StatusTooManyRequests)
					io.WriteString(w, `{"error":"rate limited","retry_after_seconds":2.25}`)
				}),
			)
			defer server.Close()
			client := Client{BaseURL: server.URL, HTTPClient: server.Client()}
			err := client.GetJSON(t.Context(), "/", nil, AuthNone, nil)
			apiErr, ok := errors.AsType[*APIError](err)
			if !ok || apiErr.RetryAfterSeconds == nil || *apiErr.RetryAfterSeconds != test.want {
				t.Fatalf("Retry-After: %v", err)
			}
		})
	}
}

func TestStreamingDestinationClosesResponse(t *testing.T) {
	for _, fail := range []bool{false, true} {
		body := &trackedBody{Reader: bytes.NewReader([]byte{'P', 'K', 3, 4, 0, 255})}
		client := Client{
			BaseURL:    "https://fixture.invalid",
			HTTPClient: &http.Client{Transport: responseTransport{body: body}},
		}
		var dst bytes.Buffer
		var target io.Writer = &dst
		if fail {
			target = failingDestination{}
		}
		err := client.GetJSON(t.Context(), "/", nil, AuthNone, target)
		if fail && !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("destination error lost: %v", err)
		}
		if !fail && (err != nil || !bytes.Equal(dst.Bytes(), []byte{'P', 'K', 3, 4, 0, 255})) {
			t.Fatalf("raw stream changed: %v %v", dst.Bytes(), err)
		}
		if !body.closed {
			t.Fatal("streaming response body was leaked")
		}
	}
}

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error { b.closed = true; return nil }

type responseTransport struct{ body io.ReadCloser }

func (r responseTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: r.body}, nil
}

type failingDestination struct{}

func (failingDestination) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
