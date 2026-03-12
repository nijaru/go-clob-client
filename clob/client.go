package clob

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyauth"
	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

type Client struct {
	host          string
	chainID       int64
	useServerTime bool
	http          *polyhttp.Client
	signer        *polyauth.Signer
	creds         *Credentials
	signatureType SignatureType
	funderAddress string
	saltGenerator func() uint64
}

func New(config Config) (*Client, error) {
	config = config.normalized()

	client := &Client{
		host:          config.Host,
		chainID:       config.ChainID,
		useServerTime: config.UseServerTime,
		creds:         config.Credentials,
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

	return client, nil
}

func (c *Client) Host() string {
	return c.host
}

func (c *Client) SetCredentials(creds Credentials) {
	c.creds = &creds
}

func (c *Client) Address() string {
	if c.signer == nil {
		return ""
	}
	return c.signer.Address().Hex()
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
		if c.signer == nil {
			return nil, fmt.Errorf("level 2 auth requires a private key")
		}
		if c.creds == nil {
			return nil, fmt.Errorf("level 2 auth requires API credentials")
		}
		timestamp, err := c.timestamp(ctx)
		if err != nil {
			return nil, err
		}
		return polyauth.L2Headers(
			c.signer,
			c.creds.Key,
			c.creds.Secret,
			c.creds.Passphrase,
			timestamp,
			method,
			path,
			body,
		)
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
