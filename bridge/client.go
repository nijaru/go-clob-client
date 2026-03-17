package bridge

import (
	"context"
	"net/http"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

const (
	DefaultHost = "https://bridge.polymarket.com"

	supportedAssetsEndpoint = "/supported-assets"
	depositEndpoint         = "/deposit"
	statusEndpoint          = "/status"
	quoteEndpoint           = "/quote"
	withdrawEndpoint        = "/withdraw"
)

// Client is a client for the Polymarket Bridge API.
type Client struct {
	host string
	http *polyhttp.Client
}

// Config defines the configuration for a Bridge client.
type Config struct {
	Host       string
	HTTPClient *http.Client
	UserAgent  string
}

// New creates a new Bridge API client.
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

func (c Config) normalized() Config {
	if c.Host == "" {
		c.Host = DefaultHost
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}
	if c.UserAgent == "" {
		c.UserAgent = "go-clob-client/bridge"
	}
	return c
}

// GetSupportedAssets returns all chains and tokens supported by the bridge.
func (c *Client) GetSupportedAssets(ctx context.Context) ([]SupportedAsset, error) {
	var out []SupportedAsset
	err := c.http.GetJSON(ctx, supportedAssetsEndpoint, nil, polyhttp.AuthNone, &out)
	return out, err
}

// CreateDepositAddress generates unique deposit addresses for the given Polymarket wallet.
func (c *Client) CreateDepositAddress(
	ctx context.Context,
	address string,
) (*DepositResponse, error) {
	req := DepositRequest{Address: address}
	var out DepositResponse
	err := c.http.PostJSON(ctx, depositEndpoint, req, polyhttp.AuthNone, &out)
	return &out, err
}

// GetStatus checks the status of bridge transactions for an address (2026 standard).
func (c *Client) GetStatus(
	ctx context.Context,
	address string,
) (*StatusResponse, error) {
	var out StatusResponse
	err := c.http.GetJSON(ctx, statusEndpoint+"/"+address, nil, polyhttp.AuthNone, &out)
	return &out, err
}

// GetQuote gets a quote for a bridge transfer.
func (c *Client) GetQuote(ctx context.Context, req QuoteRequest) (*QuoteResponse, error) {
	var out QuoteResponse
	err := c.http.PostJSON(ctx, quoteEndpoint, req, polyhttp.AuthNone, &out)
	return &out, err
}

// Withdraw initiates a withdrawal via the bridge (2026 standard).
func (c *Client) Withdraw(ctx context.Context, req WithdrawRequest) (*WithdrawResponse, error) {
	var out WithdrawResponse
	err := c.http.PostJSON(ctx, withdrawEndpoint, req, polyhttp.AuthNone, &out)
	return &out, err
}
