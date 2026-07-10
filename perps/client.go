package perps

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// DefaultHost is the production Perps REST API host.
const DefaultHost = "https://api.perpetuals.polymarket.com"

// Client is a read-only client for the Polymarket Perps API.
type Client struct {
	host string
	http *polyhttp.Client
}

// Config configures a Perps client.
type Config struct {
	// Host overrides the API host. Defaults to DefaultHost.
	Host string
	// HTTPClient is the underlying HTTP client. Defaults to a 15s-timeout client.
	HTTPClient *http.Client
	// UserAgent sets the User-Agent header. Defaults to "go-clob-client/perps".
	UserAgent string
}

func (c Config) normalized() Config {
	if c.Host == "" {
		c.Host = DefaultHost
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

// Host returns the configured API host.
func (c *Client) Host() string { return c.host }

func (c *Client) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	return c.http.GetJSON(ctx, path, query, polyhttp.AuthNone, out)
}
