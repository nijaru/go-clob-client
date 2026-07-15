package perps

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/nijaru/go-clob-client/internal/polyauth"
	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// DefaultHost is the production Perps REST API host.
const DefaultHost = "https://api.perpetuals.polymarket.com"

// DefaultWebSocketHost is the production authenticated Perps WebSocket URL.
const DefaultWebSocketHost = "wss://ws.perpetuals.polymarket.com/v1/ws"

// Client is a read-only client for the Polymarket Perps API.
type Client struct {
	host string
	http *polyhttp.Client
}

// Config configures a Perps client.
type Config struct {
	// Host overrides the API host. Defaults to DefaultHost.
	Host string
	// WebSocketHost overrides the authenticated session URL. Defaults to
	// DefaultWebSocketHost.
	WebSocketHost string
	// ChainID identifies the EIP-712 domain used by signed session commands.
	// Defaults to Polygon mainnet (137).
	ChainID int64
	// HTTPClient is the underlying HTTP client. Defaults to a 15s-timeout client.
	HTTPClient *http.Client
	// UserAgent sets the User-Agent header. Defaults to "go-clob-client/perps".
	UserAgent string
}

func (c Config) normalized() Config {
	if c.Host == "" {
		c.Host = DefaultHost
	}
	if c.WebSocketHost == "" {
		c.WebSocketHost = DefaultWebSocketHost
	}
	if c.ChainID == 0 {
		c.ChainID = 137
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}
	if c.UserAgent == "" {
		c.UserAgent = "go-clob-client/perps"
	}
	return c
}

// New creates a new Perps API client.
func New(config Config) *Client {
	config = config.normalized()
	return &Client{
		host: config.Host,
		http: &polyhttp.Client{
			BaseURL:    config.Host,
			HTTPClient: config.HTTPClient,
			UserAgent:  config.UserAgent,
		},
	}
}

// AuthenticatedConfig configures an authenticated Perps client with an
// existing delegated proxy credential.
type AuthenticatedConfig struct {
	Config
	Credentials PerpsCredentials
}

// AuthenticatedClient reads account data and opens delegated Perps sessions.
type AuthenticatedClient struct {
	*Client
	webSocketHost string
	chainID       int64
	credentials   PerpsCredentials
}

// NewAuthenticated creates an authenticated Perps client from delegated
// credentials. Credential creation and revocation remain explicit owner-signed
// operations; this constructor is the safe resume path for stored credentials.
func NewAuthenticated(config AuthenticatedConfig) (*AuthenticatedClient, error) {
	baseConfig := config.Config.normalized()
	if !common.IsHexAddress(config.Credentials.Proxy) ||
		common.HexToAddress(config.Credentials.Proxy) == (common.Address{}) {
		return nil, fmt.Errorf("perps: invalid delegated proxy address")
	}
	if config.Credentials.Secret == "" {
		return nil, fmt.Errorf("perps: delegated credential secret is required")
	}
	return &AuthenticatedClient{
		Client:        New(baseConfig),
		webSocketHost: baseConfig.WebSocketHost,
		chainID:       baseConfig.ChainID,
		credentials:   config.Credentials,
	}, nil
}

// Credentials returns a copy of the delegated credentials used by the client.
func (c *AuthenticatedClient) Credentials() PerpsCredentials {
	return c.credentials
}

func (c *AuthenticatedClient) delegatedSigner() (*polyauth.Signer, error) {
	if c.credentials.PrivateKey == "" {
		return nil, nil
	}
	signer, err := polyauth.ParsePrivateKey(c.credentials.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("perps: parse delegated signing key: %w", err)
	}
	if signer.Address() != common.HexToAddress(c.credentials.Proxy) {
		return nil, fmt.Errorf("perps: delegated signing key does not match proxy")
	}
	return signer, nil
}

func (c *AuthenticatedClient) getAuthenticatedJSON(
	ctx context.Context,
	path string,
	query url.Values,
	out any,
) error {
	return c.http.DoJSON(
		ctx,
		http.MethodGet,
		path,
		query,
		nil,
		polyhttp.AuthNone,
		nil,
		map[string]string{
			"POLYMARKET-PROXY":  c.credentials.Proxy,
			"POLYMARKET-SECRET": c.credentials.Secret,
		},
		out,
	)
}

// Host returns the configured API host.
func (c *Client) Host() string { return c.host }

func (c *Client) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	return c.http.GetJSON(ctx, path, query, polyhttp.AuthNone, out)
}
