package gamma

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetStatus(t *testing.T) {
	t.Parallel()

	t.Run("returns raw status body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.Path; got != statusEndpoint {
				t.Fatalf("unexpected path: %s", got)
			}
			w.Write([]byte("OK"))
		}))
		defer server.Close()

		client := New(Config{Host: server.URL})
		got, err := client.GetStatus(t.Context())
		if err != nil {
			t.Fatalf("GetStatus: %v", err)
		}
		if got != "OK" {
			t.Fatalf("expected OK, got %q", got)
		}
	})

	t.Run("propagates non-2xx as error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		client := New(Config{Host: server.URL})
		if _, err := client.GetStatus(t.Context()); err == nil {
			t.Fatal("expected error for 503")
		}
	})
}
