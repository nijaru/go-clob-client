package clob

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyauth"
	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// Client is the public Polymarket CLOB HTTP client.
type Client struct {
	host          string
	chainID       int64
	useServerTime bool
	http          *polyhttp.Client
	geoblockHTTP  *polyhttp.Client
	signer        *polyauth.Signer
	credsMu       sync.RWMutex
	creds         *Credentials
	builderAuth   BuilderAuth
	signatureType SignatureType
	funderAddress string
	saltGenerator func() (uint64, error)
}

// New constructs a new Polymarket CLOB client from the provided config.
func New(config Config) (*Client, error) {
	config = config.normalized()

	client := &Client{
		host:          config.Host,
		chainID:       config.ChainID,
		useServerTime: config.UseServerTime,
		creds:         config.Credentials,
		builderAuth:   config.BuilderAuth,
		signatureType: config.SignatureType,
		saltGenerator: generateSalt,
	}

	if config.PrivateKey != "" {
		signer, err := polyauth.ParsePrivateKey(config.PrivateKey)
		if err != nil {
			return nil, err
		}
		client.signer = signer

		funderAddress, err := normalizeFunderAddress(
			config.ChainID,
			signer.Address().Hex(),
			config.SignatureType,
			config.FunderAddress,
		)
		if err != nil {
			return nil, err
		}
		client.funderAddress = funderAddress
	} else {
		client.funderAddress = config.FunderAddress
	}

	client.http = &polyhttp.Client{
		BaseURL:    config.Host,
		HTTPClient: config.HTTPClient,
		UserAgent:  config.UserAgent,
		Headers:    client.addAuthHeaders,
	}
	client.geoblockHTTP = &polyhttp.Client{
		BaseURL:    config.GeoblockHost,
		HTTPClient: config.HTTPClient,
		UserAgent:  config.UserAgent,
	}

	return client, nil
}

// Host returns the base CLOB API host for the client.
func (c *Client) Host() string {
	return c.host
}

// SetCredentials updates the API credentials used for authenticated requests.
// Safe to call concurrently with in-flight requests.
func (c *Client) SetCredentials(creds Credentials) {
	c.credsMu.Lock()
	c.creds = &creds
	c.credsMu.Unlock()
}

// Address returns the signer address backing the client, if configured.
func (c *Client) Address() string {
	if c.signer == nil {
		return ""
	}
	return c.signer.Address().Hex()
}

// credentials returns the current credentials under a read lock.
func (c *Client) credentials() *Credentials {
	c.credsMu.RLock()
	creds := c.creds
	c.credsMu.RUnlock()
	return creds
}

func (c *Client) addAuthHeaders(
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
		if c.signer == nil {
			return nil, fmt.Errorf("level 1 auth requires a private key")
		}
		timestamp, err := c.timestamp(ctx)
		if err != nil {
			return nil, err
		}
		value := int64(0)
		if nonce != nil {
			value = *nonce
		}
		return polyauth.L1Headers(c.signer, c.chainID, timestamp, value)
	case polyhttp.AuthL2:
		creds := c.credentials()
		if c.signer == nil {
			return nil, fmt.Errorf("level 2 auth requires a private key")
		}
		if creds == nil {
			return nil, fmt.Errorf("level 2 auth requires API credentials")
		}
		timestamp, err := c.timestamp(ctx)
		if err != nil {
			return nil, err
		}
		return polyauth.L2Headers(
			c.signer,
			creds.Key,
			creds.Secret,
			creds.Passphrase,
			timestamp,
			method,
			path,
			body,
		)
	case polyhttp.AuthL2Builder:
		creds := c.credentials()
		if c.signer == nil {
			return nil, fmt.Errorf("level 2 auth requires a private key")
		}
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
			creds.Secret,
			creds.Passphrase,
			timestamp,
			method,
			path,
			body,
		)
		if err != nil {
			return nil, err
		}
		if c.builderAuth == nil {
			return headers, nil
		}
		builderHeaders, err := c.builderHeaders(ctx, method, path, body, timestamp)
		if err != nil {
			return nil, err
		}
		for key, value := range builderHeaders {
			headers[key] = value
		}
		return headers, nil
	default:
		return nil, fmt.Errorf("unknown auth level %d", level)
	}
}

func (c *Client) timestamp(ctx context.Context) (int64, error) {
	if !c.useServerTime {
		return time.Now().Unix(), nil
	}

	var serverTime int64
	if err := c.http.GetJSON(ctx, timeEndpoint, nil, polyhttp.AuthNone, &serverTime); err != nil {
		return 0, err
	}
	return serverTime, nil
}

func (c *Client) getJSON(
	ctx context.Context,
	path string,
	query url.Values,
	auth polyhttp.AuthLevel,
	out any,
) error {
	return c.http.GetJSON(ctx, path, query, auth, out)
}

func (c *Client) getGeoblockJSON(
	ctx context.Context,
	path string,
	query url.Values,
	out any,
) error {
	return c.geoblockHTTP.GetJSON(ctx, path, query, polyhttp.AuthNone, out)
}

func (c *Client) postJSON(
	ctx context.Context,
	path string,
	body any,
	auth polyhttp.AuthLevel,
	out any,
) error {
	return c.http.PostJSON(ctx, path, body, auth, out)
}

func (c *Client) deleteJSON(
	ctx context.Context,
	path string,
	body any,
	auth polyhttp.AuthLevel,
	out any,
) error {
	return c.http.DeleteJSON(ctx, path, body, auth, out)
}

func (c *Client) deleteJSONQuery(
	ctx context.Context,
	path string,
	query url.Values,
	body any,
	auth polyhttp.AuthLevel,
	out any,
) error {
	return c.http.DeleteJSONQuery(ctx, path, query, body, auth, out)
}

func (c *Client) getJSONWithNonce(
	ctx context.Context,
	path string,
	query url.Values,
	auth polyhttp.AuthLevel,
	nonce int64,
	out any,
) error {
	return c.http.GetJSONWithNonce(ctx, path, query, auth, nonce, out)
}

func (c *Client) postJSONWithNonce(
	ctx context.Context,
	path string,
	body any,
	auth polyhttp.AuthLevel,
	nonce int64,
	out any,
) error {
	return c.http.PostJSONWithNonce(ctx, path, body, auth, nonce, out)
}

func (c *Client) doJSON(
	ctx context.Context,
	method, path string,
	query url.Values,
	body any,
	auth polyhttp.AuthLevel,
	out any,
	extraHeaders map[string]string,
) error {
	return c.http.DoJSON(ctx, method, path, query, body, auth, nil, extraHeaders, out)
}

func (c *Client) builderHeaders(
	ctx context.Context,
	method, path string,
	body []byte,
	timestamp int64,
) (map[string]string, error) {
	if c.builderAuth == nil {
		return nil, fmt.Errorf("builder auth requires Config.BuilderAuth")
	}

	return c.builderAuth.Headers(ctx, BuilderHeaderRequest{
		Method:    method,
		Path:      path,
		Body:      body,
		Timestamp: timestamp,
	})
}

func (c *Client) builderOnlyHeaders(
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
