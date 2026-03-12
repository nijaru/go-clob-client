package clob

import (
	"errors"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// APIError is the typed error returned for non-successful Polymarket API responses.
type APIError = polyhttp.APIError

var errRemoteBuilderURLRequired = errors.New("remote builder auth requires a URL")
