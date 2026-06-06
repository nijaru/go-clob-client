package gamma

import (
	"net/url"
	"testing"
)

func TestGammaQuery(t *testing.T) {
	q := gammaQuery(10, 5)
	if q.Get("limit") != "10" {
		t.Errorf("limit = %q", q.Get("limit"))
	}
	if q.Get("offset") != "5" {
		t.Errorf("offset = %q", q.Get("offset"))
	}

	// Zero values should not be set
	q = gammaQuery(0, 0)
	if q.Has("limit") || q.Has("offset") {
		t.Errorf("expected empty query, got %v", q)
	}
}

func TestSetBool(t *testing.T) {
	q := url.Values{}
	setBool(q, "active", nil)
	if q.Has("active") {
		t.Error("nil should not set key")
	}

	v := true
	setBool(q, "active", &v)
	if q.Get("active") != "true" {
		t.Errorf("active = %q", q.Get("active"))
	}

	f := false
	setBool(q, "closed", &f)
	if q.Get("closed") != "false" {
		t.Errorf("closed = %q", q.Get("closed"))
	}
}

func TestSetString(t *testing.T) {
	q := url.Values{}
	setString(q, "slug", "")
	if q.Has("slug") {
		t.Error("empty should not set key")
	}

	setString(q, "slug", "test-slug")
	if q.Get("slug") != "test-slug" {
		t.Errorf("slug = %q", q.Get("slug"))
	}
}

func TestSetInt(t *testing.T) {
	q := url.Values{}
	setInt(q, "providerId", 0)
	if q.Has("providerId") {
		t.Error("zero should not set key")
	}

	setInt(q, "providerId", 42)
	if q.Get("providerId") != "42" {
		t.Errorf("providerId = %q", q.Get("providerId"))
	}
}

func TestIteratorLimit(t *testing.T) {
	tests := []struct {
		limit, defaultMax, max, want int
	}{
		{0, 20, 100, 20},
		{50, 20, 100, 50},
		{200, 20, 100, 100},
		{-1, 20, 100, 20},
	}
	for _, tt := range tests {
		got := iteratorLimit(tt.limit, tt.defaultMax, tt.max)
		if got != tt.want {
			t.Errorf("iteratorLimit(%d, %d, %d) = %d, want %d",
				tt.limit, tt.defaultMax, tt.max, got, tt.want)
		}
	}
}
