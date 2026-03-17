package clob

import (
	"errors"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// APIError is the typed error returned for non-successful Polymarket API responses.
type APIError = polyhttp.APIError

// Sentinel errors for common API failure cases.
// Use with errors.Is:
//
//	if errors.Is(err, clob.ErrNotFound) { ... }
var (
	ErrNotFound     = httpSentinel{404}
	ErrUnauthorized = httpSentinel{401}
	ErrForbidden    = httpSentinel{403}
	ErrRateLimit    = httpSentinel{429}
	ErrGeoBlock     = httpSentinel{451}
	ErrBadRequest   = httpSentinel{400}
)

// httpSentinel is a sentinel error that matches APIErrors by status code.
type httpSentinel struct{ status int }

func (s httpSentinel) Error() string   { return httpStatusText(s.status) }
func (s httpSentinel) HTTPStatus() int { return s.status }

func httpStatusText(code int) string {
	switch code {
	case 400:
		return "bad request"
	case 401:
		return "unauthorized"
	case 403:
		return "forbidden"
	case 404:
		return "not found"
	case 429:
		return "rate limited"
	case 451:
		return "geo blocked"
	default:
		return "HTTP error"
	}
}

// IsNotFound reports whether err is a 404 API error.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// IsRateLimit reports whether err is a 429 rate-limit error.
func IsRateLimit(err error) bool { return errors.Is(err, ErrRateLimit) }

// IsGeoBlocked reports whether err is a 451 geo-block error.
func IsGeoBlocked(err error) bool { return errors.Is(err, ErrGeoBlock) }

// IsUnauthorized reports whether err is a 401 or 403 auth error.
func IsUnauthorized(err error) bool {
	return errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrForbidden)
}

var errRemoteBuilderURLRequired = errors.New("remote builder auth requires a URL")
