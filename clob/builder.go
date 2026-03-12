package clob

import (
	"context"
	"net/http"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyauth"
)

type BuilderHeaderRequest struct {
	Method    string
	Path      string
	Body      []byte
	Timestamp int64
}

type BuilderAuth interface {
	Headers(ctx context.Context, req BuilderHeaderRequest) (map[string]string, error)
}

type RemoteBuilderAuthConfig struct {
	URL         string
	BearerToken string
	HTTPClient  *http.Client
}

type localBuilderAuth struct {
	creds Credentials
}

type remoteBuilderAuth struct {
	url         string
	bearerToken string
	httpClient  *http.Client
}

func NewLocalBuilderAuth(creds Credentials) BuilderAuth {
	return &localBuilderAuth{creds: creds}
}

func NewRemoteBuilderAuth(cfg RemoteBuilderAuthConfig) BuilderAuth {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	return &remoteBuilderAuth{
		url:         cfg.URL,
		bearerToken: cfg.BearerToken,
		httpClient:  httpClient,
	}
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
