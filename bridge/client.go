package bridge

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

const (
	DefaultHost = "https://bridge.polymarket.com"

	supportedAssetsEndpoint = "/supported-assets"
	depositEndpoint         = "/deposit"
	depositStatusEndpoint   = "/deposit-status"
	quoteEndpoint           = "/quote"
	withdrawalEndpoint      = "/withdrawal"
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
) ([]DepositAddress, error) {
	req := DepositRequest{Address: address}
	var out []DepositAddress
	err := c.http.PostJSON(ctx, depositEndpoint, req, polyhttp.AuthNone, &out)
	return out, err
}

// GetDepositStatus checks the status of a bridge deposit transaction.
func (c *Client) GetDepositStatus(
	ctx context.Context,
	transactionID string,
) (*DepositStatusResponse, error) {
	query := url.Values{}
	query.Set("transactionId", transactionID)

	var out DepositStatusResponse
	err := c.http.GetJSON(ctx, depositStatusEndpoint, query, polyhttp.AuthNone, &out)
	return &out, err
}

// GetQuote gets a quote for a bridge transfer.
func (c *Client) GetQuote(ctx context.Context, req QuoteRequest) (*QuoteResponse, error) {
	var out QuoteResponse
	err := c.http.PostJSON(ctx, quoteEndpoint, req, polyhttp.AuthNone, &out)
	return &out, err
}

// Withdraw initiates a withdrawal from Polygon via the bridge.
func (c *Client) Withdraw(ctx context.Context, req WithdrawalRequest) (*WithdrawalResponse, error) {
	var out WithdrawalResponse
	err := c.http.PostJSON(ctx, withdrawalEndpoint, req, polyhttp.AuthNone, &out)
	return &out, err
}
