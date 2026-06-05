package clob

import (
	"errors"
	"net/http"
	"testing"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

func TestAPIError(t *testing.T) {
	t.Parallel()

	err := &polyhttp.APIError{
		StatusCode: http.StatusNotFound,
		Body:       []byte(`{"error":"not found"}`),
	}

	if err.HTTPStatus() != 404 {
		t.Errorf("expected 404, got %d", err.HTTPStatus())
	}
}

func TestHTTPStatusText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code int
		want string
	}{
		{400, "bad request"},
		{401, "unauthorized"},
		{403, "forbidden"},
		{404, "not found"},
		{429, "rate limited"},
		{451, "geo blocked"},
	}

	for _, tt := range tests {
		got := httpStatusText(tt.code)
		if got != tt.want {
			t.Errorf("status %d: got %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestIsNotFound(t *testing.T) {
	t.Parallel()

	err := &polyhttp.APIError{StatusCode: 404}
	if !errors.Is(err, ErrNotFound) {
		t.Fatal("expected IsNotFound")
	}

	err2 := &polyhttp.APIError{StatusCode: 500}
	if errors.Is(err2, ErrNotFound) {
		t.Fatal("did not expect IsNotFound for 500")
	}
}

func TestIsRateLimit(t *testing.T) {
	t.Parallel()

	err := &polyhttp.APIError{StatusCode: 429}
	if !errors.Is(err, ErrRateLimit) {
		t.Fatal("expected IsRateLimit")
	}
}

func TestIsGeoBlock(t *testing.T) {
	t.Parallel()

	err := &polyhttp.APIError{StatusCode: 451}
	if !errors.Is(err, ErrGeoBlock) {
		t.Fatal("expected IsGeoBlock")
	}
}

func TestIsUnauthorized(t *testing.T) {
	t.Parallel()

	for _, code := range []int{401, 403} {
		err := &polyhttp.APIError{StatusCode: code}
		if !IsUnauthorized(err) {
			t.Errorf("status %d: expected IsUnauthorized", code)
		}
	}

	err := &polyhttp.APIError{StatusCode: 200}
	if IsUnauthorized(err) {
		t.Fatal("did not expect IsUnauthorized for 200")
	}
}

func TestSentinelErrors(t *testing.T) {
	t.Parallel()

	if ErrNotFound.Error() != "not found" {
		t.Errorf("ErrNotFound: %q", ErrNotFound.Error())
	}
	if ErrRateLimit.Error() != "rate limited" {
		t.Errorf("ErrRateLimit: %q", ErrRateLimit.Error())
	}
	if ErrGeoBlock.Error() != "geo blocked" {
		t.Errorf("ErrGeoBlock: %q", ErrGeoBlock.Error())
	}
	if ErrUnauthorized.Error() != "unauthorized" {
		t.Errorf("ErrUnauthorized: %q", ErrUnauthorized.Error())
	}
	if ErrForbidden.Error() != "forbidden" {
		t.Errorf("ErrForbidden: %q", ErrForbidden.Error())
	}
	if ErrBadRequest.Error() != "bad request" {
		t.Errorf("ErrBadRequest: %q", ErrBadRequest.Error())
	}
}
