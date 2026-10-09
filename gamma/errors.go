package gamma

import "github.com/nijaru/go-clob-client/internal/polyhttp"

// APIError preserves Gamma HTTP status, response body, machine code and retry
// details. Use errors.As to inspect it without importing shared internals.
type APIError = polyhttp.APIError
