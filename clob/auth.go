package clob

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/nijaru/go-clob-client/internal/polyauth"
	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

func decodeCredentials(creds Credentials) ([]byte, error) {
	if strings.TrimSpace(creds.Key) == "" || strings.TrimSpace(creds.Passphrase) == "" {
		return nil, fmt.Errorf("API key and passphrase must not be empty")
	}
	secret, err := polyauth.DecodeAPISecret(creds.Secret)
	if err != nil {
		return nil, err
	}
	if len(secret) == 0 {
		return nil, fmt.Errorf("API secret must not be empty")
	}
	return secret, nil
}

// Credentials returns the current API credentials.
func (c *AuthenticatedClient) Credentials() *Credentials {
	c.authMu.RLock()
	defer c.authMu.RUnlock()
	if c.creds == nil {
		return nil
	}
	creds := *c.creds
	return &creds
}

// PromoteToBuilder upgrades the client with builder credentials, enabling builder-authenticated requests.
func (c *SignerClient) PromoteToBuilder(auth BuilderAuth) {
	c.builderMu.Lock()
	defer c.builderMu.Unlock()
	c.builderAuth = auth
}

// SetCredentials updates the API credentials used for authenticated requests.
// It re-derives the decoded HMAC secret from creds.Secret so subsequent CLOB L2
// requests sign with the new key. Relayer and builder authentication are independent.
// If the secret is not valid base64, existing credentials are left unchanged and
// an error is returned. Safe to
// call concurrently with in-flight requests.
func (c *AuthenticatedClient) SetCredentials(creds Credentials) error {
	decodedSecret, err := decodeCredentials(creds)
	if err != nil {
		return fmt.Errorf("set credentials: %w", err)
	}
	c.authMu.Lock()
	c.creds = &creds
	c.decodedSecret = decodedSecret
	c.authMu.Unlock()
	return nil
}

// Address returns the signer address backing the client.
func (c *SignerClient) Address() string {
	if c.signer == nil {
		return ""
	}
	return c.signer.Address().Hex()
}

// credentials returns the current credentials under a read lock.
func (c *AuthenticatedClient) credentials() *Credentials {
	return c.Credentials()
}

// authSnapshot reads one immutable credential generation. The decoded secret
// is replaced, never mutated, so no lock is held during signing or callbacks.
func (c *AuthenticatedClient) authSnapshot() (*Credentials, []byte, BuilderAuth) {
	c.authMu.RLock()
	defer c.authMu.RUnlock()
	return c.creds, c.decodedSecret, c.getBuilderAuth()
}

func (c *SignerClient) getBuilderAuth() BuilderAuth {
	c.builderMu.RLock()
	defer c.builderMu.RUnlock()
	return c.builderAuth
}

func (c *SignerClient) addAuthHeaders(
	ctx context.Context,
	method, path string,
	body []byte,
	level polyhttp.AuthLevel,
	nonce *int64,
) (map[string]string, error) {
	switch level {
	case polyhttp.AuthNone:
		return nil, nil
	case polyhttp.AuthL1:
		timestamp, err := c.timestamp(ctx)
		if err != nil {
			return nil, err
		}
		value := int64(0)
		if nonce != nil {
			value = *nonce
		}
		return polyauth.L1Headers(ctx, c.signer, c.chainID, timestamp, value)
	default:
		return nil, fmt.Errorf(
			"this client only supports L1 auth, please upgrade to an AuthenticatedClient",
		)
	}
}

func (c *AuthenticatedClient) addAuthHeaders(
	ctx context.Context, method, path string, body []byte,
	level polyhttp.AuthLevel, nonce *int64,
) (map[string]string, error) {
	switch level {
	case polyhttp.AuthNone, polyhttp.AuthL1:
		return c.SignerClient.addAuthHeaders(ctx, method, path, body, level, nonce)
	case polyhttp.AuthL2, polyhttp.AuthL2Builder:
		creds, secret, builder := c.authSnapshot()
		if creds == nil {
			return nil, fmt.Errorf("level 2 auth requires API credentials")
		}
		timestamp, err := c.timestamp(ctx)
		if err != nil {
			return nil, err
		}
		headers, err := polyauth.L2Headers(
			c.signer,
			creds.Key,
			secret,
			creds.Passphrase,
			timestamp,
			method,
			path,
			body,
		)
		if err != nil {
			return nil, err
		}
		if level == polyhttp.AuthL2Builder && builder != nil {
			builderHeaders, err := builder.Headers(
				ctx,
				BuilderHeaderRequest{Method: method, Path: path, Body: body, Timestamp: timestamp},
			)
			if err != nil {
				return nil, err
			}
			maps.Copy(headers, builderHeaders)
		}
		return headers, nil
	default:
		return nil, fmt.Errorf("unknown auth level %d", level)
	}
}

func (c *SignerClient) builderHeaders(
	ctx context.Context,
	method, path string,
	body []byte,
	timestamp int64,
) (map[string]string, error) {
	builder := c.getBuilderAuth()
	if builder == nil {
		return nil, fmt.Errorf("builder auth requires Config.BuilderAuth")
	}

	return builder.Headers(ctx, BuilderHeaderRequest{
		Method:    method,
		Path:      path,
		Body:      body,
		Timestamp: timestamp,
	})
}

func (c *SignerClient) builderOnlyHeaders(
	ctx context.Context,
	method, path string,
	body []byte,
) (map[string]string, error) {
	timestamp, err := c.timestamp(ctx)
	if err != nil {
		return nil, err
	}
	return c.builderHeaders(ctx, method, path, body, timestamp)
}

// DeriveWSAuth returns the raw credentials required by the authenticated
// CLOB websocket user channel.
func (c *AuthenticatedClient) DeriveWSAuth(_ context.Context) (WSAuth, error) {
	creds := c.credentials()
	if creds == nil {
		return WSAuth{}, fmt.Errorf("derive ws auth requires API credentials")
	}

	return WSAuth{
		Key:        creds.Key,
		Secret:     creds.Secret,
		Passphrase: creds.Passphrase,
	}, nil
}
