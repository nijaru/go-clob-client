package data

import (
	"net/url"
	"testing"
)

func TestBoundedLimit(t *testing.T) {
	tests := []struct {
		limit, max, want int
	}{
		{0, 500, 0},
		{-1, 500, 0},
		{100, 500, 100},
		{600, 500, 500},
	}
	for _, tt := range tests {
		got := boundedLimit(tt.limit, tt.max)
		if got != tt.want {
			t.Errorf("boundedLimit(%d, %d) = %d, want %d", tt.limit, tt.max, got, tt.want)
		}
	}
}

func TestParameterBoundsError(t *testing.T) {
	t.Parallel()

	err := validateBound("positions.offset", 10_001, 0, 10_000)
	boundsErr, ok := err.(*ParameterBoundsError)
	if !ok {
		t.Fatalf("error = %T, want *ParameterBoundsError", err)
	}
	if boundsErr.Parameter != "positions.offset" || boundsErr.Value != 10_001 {
		t.Fatalf("bounds error = %+v", boundsErr)
	}
}

func TestIteratorLimit(t *testing.T) {
	tests := []struct {
		limit, defaultLimit, max, want int
	}{
		{0, 100, 500, 100},
		{-1, 100, 500, 100},
		{50, 100, 500, 50},
		{600, 100, 500, 500},
	}
	for _, tt := range tests {
		got := iteratorLimit(tt.limit, tt.defaultLimit, tt.max)
		if got != tt.want {
			t.Errorf("iteratorLimit(%d, %d, %d) = %d, want %d",
				tt.limit, tt.defaultLimit, tt.max, got, tt.want)
		}
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
}

func TestSetString(t *testing.T) {
	q := url.Values{}
	setString(q, "slug", "")
	if q.Has("slug") {
		t.Error("empty should not set key")
	}
	setString(q, "slug", "test")
	if q.Get("slug") != "test" {
		t.Errorf("slug = %q", q.Get("slug"))
	}
}

func TestSetInt(t *testing.T) {
	q := url.Values{}
	setInt(q, "limit", 0)
	if q.Has("limit") {
		t.Error("zero should not set key")
	}
	setInt(q, "limit", 10)
	if q.Get("limit") != "10" {
		t.Errorf("limit = %q", q.Get("limit"))
	}
}

func TestSetInt64(t *testing.T) {
	q := url.Values{}
	setInt64(q, "id", 0)
	if q.Has("id") {
		t.Error("zero should not set key")
	}
	setInt64(q, "id", 12345)
	if q.Get("id") != "12345" {
		t.Errorf("id = %q", q.Get("id"))
	}
}

func TestSetCommaList(t *testing.T) {
	q := url.Values{}
	setCommaList[string](q, "market", nil)
	if q.Has("market") {
		t.Error("nil should not set key")
	}
	setCommaList(q, "market", []string{"a", "b", "c"})
	if q.Get("market") != "a,b,c" {
		t.Errorf("market = %q", q.Get("market"))
	}
}

func TestMarketsFilter(t *testing.T) {
	q := url.Values{}
	MarketsFilter("m1", "m2").appendQuery(q)
	if q.Get("market") != "m1,m2" {
		t.Errorf("market = %q", q.Get("market"))
	}
	if q.Has("eventId") {
		t.Error("eventId should not be set")
	}
}

func TestEventIDsFilter(t *testing.T) {
	q := url.Values{}
	EventIDsFilter("e1", "e2").appendQuery(q)
	if q.Get("eventId") != "e1,e2" {
		t.Errorf("eventId = %q", q.Get("eventId"))
	}
	if q.Has("market") {
		t.Error("market should not be set")
	}
}

func TestZeroValueFilter(t *testing.T) {
	q := url.Values{}
	(MarketFilter{}).appendQuery(q)
	if q.Has("market") || q.Has("eventId") {
		t.Error("zero filter should not set any keys")
	}
}
