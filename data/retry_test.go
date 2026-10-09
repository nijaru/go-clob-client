package data

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestReadRetryPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, header, body string
		status, calls      int
	}{
		{"exhausted", "0", `{"error":"limited","code":"busy"}`, 429, 3},
		{"long delay", "5.01", `{"retry_after_seconds":0}`, 429, 1},
		{"body delay", "invalid", `{"retry_after_seconds":0}`, 429, 3},
		{"header precedence", "0", `{"retry_after_seconds":9}`, 429, 3},
		{"long body delay", "", `{"retry_after_seconds":6}`, 429, 1},
		{"past date", "Thu, 10 Sep 2026 06:00:00 GMT", `{}`, 429, 3},
		{"future date", "Thu, 10 Sep 2026 06:00:06 GMT", `{}`, 429, 1},
		{"server error", "0", `{"error":"unavailable"}`, 503, 1},
		{"client error", "0", `{"error":"invalid"}`, 400, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Date", "Thu, 10 Sep 2026 06:00:00 GMT")
				w.Header().Set("Retry-After", tc.header)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			})
			_, err := client.GetUserStats(t.Context(), "wallet")
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != tc.status ||
				string(apiErr.Body) != tc.body ||
				int(calls.Load()) != tc.calls {
				t.Fatalf("calls=%d error=%#v", calls.Load(), err)
			}
			if tc.name == "exhausted" && (apiErr.Code == nil || *apiErr.Code != "busy") {
				t.Fatal("retry lost error metadata")
			}
		})
	}
}

func TestReadRetryPreservesPageAndClosesResponses(t *testing.T) {
	var calls atomic.Int32
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.Method != http.MethodGet || r.URL.Path != "/v2/trades" || q.Get("start") != "1" ||
			q.Get("taker_only") != "false" ||
			q.Get("limit") != "3" {
			t.Errorf("changed request: %s %s", r.Method, r.URL)
		}
		n := calls.Add(1)
		wantCursor := "resume/opaque+1"
		if n > 3 {
			wantCursor = "next/opaque+2"
		}
		if q.Get("cursor") != wantCursor {
			t.Errorf("changed retry cursor: %s", r.URL)
		}
		if n%3 != 0 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":"limited"}`)
			return
		}
		if n == 3 {
			fmt.Fprintf(
				w,
				`{"data":[%s],"pagination":{"has_more":true,"next_cursor":"next/opaque+2"}}`,
				tradeFixture("a", "1"),
			)
		} else {
			fmt.Fprint(w, `{"data":[],"pagination":{"has_more":false,"next_cursor":null}}`)
		}
	})
	var open atomic.Int32
	client.http.HTTPClient.Transport = responseTracker{
		base: client.http.HTTPClient.Transport,
		open: &open,
		t:    t,
	}
	no := false
	params := TradesParams{
		Window:    TimeWindow{FullHistory: true},
		TakerOnly: &no,
		Page:      PageParams{Limit: 3, Cursor: "resume/opaque+1"},
	}
	var ids []string
	for trade, err := range client.IterTrades(t.Context(), params) {
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, trade.AssetID)
	}
	if strings.Join(ids, ",") != "a" || calls.Load() != 6 || open.Load() != 0 {
		t.Fatalf("rows=%v calls=%d unclosed=%d", ids, calls.Load(), open.Load())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type responseTracker struct {
	base http.RoundTripper
	open *atomic.Int32
	t    *testing.T
}

func (r responseTracker) RoundTrip(req *http.Request) (*http.Response, error) {
	if r.open.Load() != 0 {
		r.t.Error("previous response remains open on retry")
	}
	resp, err := r.base.RoundTrip(req)
	if err == nil {
		r.open.Add(1)
		resp.Body = &trackedBody{ReadCloser: resp.Body, open: r.open}
	}
	return resp, err
}

type trackedBody struct {
	io.ReadCloser
	open *atomic.Int32
}

func (b *trackedBody) Close() error {
	b.open.Add(-1)
	return b.ReadCloser.Close()
}

func TestReadRetryDelay(t *testing.T) {
	for _, tc := range []struct {
		name, header, body string
		delay              time.Duration
	}{
		{"default", "", `{}`, time.Second},
		{"fractional header", "0.025", `{"retry_after_seconds":9}`, 25 * time.Millisecond},
		{"fractional body", "invalid", `{"retry_after_seconds":0.025}`, 25 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			var first atomic.Int64
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 {
					first.Store(time.Now().UnixNano())
					w.Header().Set("Retry-After", tc.header)
					w.WriteHeader(http.StatusTooManyRequests)
					fmt.Fprint(w, tc.body)
					return
				}
				if time.Since(time.Unix(0, first.Load())) < tc.delay {
					t.Error("retried before the requested delay")
				}
				fmt.Fprint(w, `{"data":null}`)
			})
			stats, err := client.GetUserStats(t.Context(), "wallet")
			if err != nil || stats != nil || calls.Load() != 2 {
				t.Fatalf("calls=%d stats=%v error=%v", calls.Load(), stats, err)
			}
		})
	}
}

func TestReadRetryCancellation(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(fmt.Sprint(deadline), func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			if deadline {
				cancel()
				ctx, cancel = context.WithTimeout(t.Context(), 50*time.Millisecond)
			}
			defer cancel()
			var calls atomic.Int32
			var open atomic.Int32
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Retry-After", "5") // Inclusive ceiling: wait, do not propagate.
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprint(w, `{}`)
				if !deadline {
					cancel()
				}
			})
			client.http.HTTPClient.Transport = responseTracker{
				base: client.http.HTTPClient.Transport,
				open: &open,
				t:    t,
			}
			_, err := client.GetUserStats(ctx, "wallet")
			want := context.Canceled
			if deadline {
				want = context.DeadlineExceeded
			}
			if !errors.Is(err, want) || calls.Load() != 1 || open.Load() != 0 {
				t.Fatalf("calls=%d unclosed=%d error=%v", calls.Load(), open.Load(), err)
			}
			_, err = client.GetUserStats(ctx, "wallet")
			if !errors.Is(err, want) || calls.Load() != 1 {
				t.Fatal("canceled read issued another request")
			}
		})
	}
}

func TestReadErrorsAndAccountingAreNotRetried(t *testing.T) {
	for _, reply := range []string{`invalid json`, `{"data":{}}`, `{"data":null}`} {
		t.Run(reply, func(t *testing.T) {
			var calls atomic.Int32
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				fmt.Fprint(w, reply)
			})
			_, err := client.GetValue(t.Context(), ValueParams{User: "wallet"})
			if err == nil || calls.Load() != 1 {
				t.Fatalf("calls=%d error=%v", calls.Load(), err)
			}
		})
	}
	t.Run("transport error", func(t *testing.T) {
		failure := errors.New("transport failed")
		calls := 0
		client, err := NewClient(Config{HTTPClient: &http.Client{
			Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return nil, failure
			}),
		}})
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.GetUserStats(t.Context(), "wallet")
		if !errors.Is(err, failure) || calls != 1 {
			t.Fatalf("calls=%d error=%v", calls, err)
		}
	})
	var calls atomic.Int32
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	var dst strings.Builder
	err := client.WriteAccountingSnapshot(t.Context(), "wallet", &dst)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 429 || calls.Load() != 1 || dst.Len() != 0 {
		t.Fatalf(
			"accounting download retried or wrote an error: calls=%d error=%v",
			calls.Load(),
			err,
		)
	}
}
