package gamma

import (
	"context"
	"net/http"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

const (
	DefaultHost = "https://gamma-api.polymarket.com"

	marketsEndpoint              = "/markets"
	eventsEndpoint               = "/events"
	seriesEndpoint               = "/series"
	tagsEndpoint                 = "/tags"
	sportsEndpoint               = "/sports"
	teamsEndpoint                = "/teams"
	commentsEndpoint             = "/comments"
	profileEndpoint              = "/public-profile"
	searchEndpoint               = "/public-search"
	statusEndpoint               = "/status"
	marketClarificationsEndpoint = "/market-clarifications"
)

// Client is a read-only client for the Polymarket Gamma API.
type Client struct {
	host string
	http *polyhttp.Client
}

// Config defines the configuration for a Gamma client.
type Config struct {
	Host       string
	HTTPClient *http.Client
	UserAgent  string
}

// New creates a new Gamma API client.
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

// GetStatus returns the raw health/status string from the Gamma API.
// It mirrors the Rust SDK's gamma `status()` and errors on a non-2xx response.
func (c *Client) GetStatus(ctx context.Context) (string, error) {
	var out string
	err := c.http.GetJSON(ctx, statusEndpoint, nil, polyhttp.AuthNone, &out)
	return out, err
}

func (c Config) normalized() Config {
	if c.Host == "" {
		c.Host = DefaultHost
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}
	if c.UserAgent == "" {
		c.UserAgent = "go-clob-client/gamma"
	}
	return c
}
