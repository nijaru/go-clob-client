package clob

import (
	"errors"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// TradingRestriction identifies the trading restriction that caused a
// rejection. It mirrors the upstream TS/Python classification.
type TradingRestriction = polyhttp.TradingRestriction

const (
	// TradingRestrictionRestarting means the matching engine is restarting
	// and rejects order requests until it is back.
	TradingRestrictionRestarting = polyhttp.TradingRestrictionRestarting
	// TradingRestrictionCancelOnly means cancels are accepted but new orders
	// are rejected.
	TradingRestrictionCancelOnly = polyhttp.TradingRestrictionCancelOnly
	// TradingRestrictionPostOnly means cancels and post-only orders are
	// accepted while other orders are rejected.
	TradingRestrictionPostOnly = polyhttp.TradingRestrictionPostOnly
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

	// ErrInvalidSettlementOptions indicates an invalid settlement wait option.
	ErrInvalidSettlementOptions = errors.New("invalid settlement options")
	// ErrSettlementTimeout indicates that one or more order fills did not settle
	// before the configured timeout.
	ErrSettlementTimeout = errors.New("order fill settlement timed out")
	// ErrSettlementFailed indicates that every fill in an order failed execution.
	ErrSettlementFailed = errors.New("every order fill failed execution")
	// ErrCollateralReturnUnsupportedWallet indicates that collateral return is
	// unavailable for the configured EOA wallet model.
	ErrCollateralReturnUnsupportedWallet = errors.New("collateral return requires a smart wallet")
	// ErrCollateralReturnPlanMismatch indicates that a plan cannot be executed
	// by this client because its wallet or chain does not match.
	ErrCollateralReturnPlanMismatch = errors.New("collateral return plan does not match client")
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

// IsTradingRestriction reports whether err is an APIError carrying a typed
// trading restriction.
func IsTradingRestriction(err error) bool {
	apiErr, ok := errors.AsType[*polyhttp.APIError](err)
	if !ok {
		return false
	}
	return apiErr.TradingRestriction != nil
}

// AsTradingRestriction returns the trading restriction carried by err, or nil
// when err is not a trading-restriction error.
func AsTradingRestriction(err error) *TradingRestriction {
	apiErr, ok := errors.AsType[*polyhttp.APIError](err)
	if !ok {
		return nil
	}
	return apiErr.TradingRestriction
}

// IsGeoBlocked reports whether err is a 451 geo-block error.
func IsGeoBlocked(err error) bool { return errors.Is(err, ErrGeoBlock) }

// IsUnauthorized reports whether err is a 401 or 403 auth error.
func IsUnauthorized(err error) bool {
	return errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrForbidden)
}

var errRemoteBuilderURLRequired = errors.New("remote builder auth requires a URL")
