package clob

import (
	"context"
	"net/http"
	neturl "net/url"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyauth"
)

type localBuilderAuth struct {
	creds Credentials
}

type remoteBuilderAuth struct {
	url         string
	bearerToken string
	httpClient  *http.Client
}

// NewLocalBuilderAuth creates a builder auth provider that signs headers locally.
func NewLocalBuilderAuth(creds Credentials) BuilderAuth {
	return &localBuilderAuth{creds: creds}
}

// NewRemoteBuilderAuth creates a builder auth provider backed by a remote signing service.
func NewRemoteBuilderAuth(cfg RemoteBuilderAuthConfig) (BuilderAuth, error) {
	if cfg.URL == "" {
		return nil, errRemoteBuilderURLRequired
	}
	parsedURL, err := neturl.ParseRequestURI(cfg.URL)
	if err != nil {
		return nil, err
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	return &remoteBuilderAuth{
		url:         parsedURL.String(),
		bearerToken: cfg.BearerToken,
		httpClient:  httpClient,
	}, nil
}

func (a *localBuilderAuth) Headers(
	_ context.Context,
	req BuilderHeaderRequest,
) (map[string]string, error) {
	return polyauth.BuilderHeaders(
		a.creds.Key,
		a.creds.Secret,
		a.creds.Passphrase,
		req.Timestamp,
		req.Method,
		req.Path,
		req.Body,
	)
}

func (a *remoteBuilderAuth) Headers(
	ctx context.Context,
	req BuilderHeaderRequest,
) (map[string]string, error) {
	return polyauth.FetchRemoteBuilderHeaders(
		ctx,
		a.httpClient,
		a.url,
		a.bearerToken,
		polyauth.RemoteBuilderHeaderRequest{
			Method:    req.Method,
			Path:      req.Path,
			Body:      string(req.Body),
			Timestamp: req.Timestamp,
		},
	)
}
