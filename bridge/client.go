package bridge

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

const (
	DefaultHost             = "https://bridge.polymarket.com"
	supportedAssetsEndpoint = "/supported-assets"
	depositEndpoint         = "/deposit"
	statusEndpoint          = "/status"
	quoteEndpoint           = "/quote"
	withdrawEndpoint        = "/withdraw"
)

// Client reads bridge quotes/status and explicitly creates routing addresses.
// It never signs, broadcasts or waits for a token transfer.
type Client struct{ http *polyhttp.Client }

type Config struct {
	Host       string
	HTTPClient *http.Client
	UserAgent  string
}

type APIError = polyhttp.APIError

type InputError struct{ Field, Message string }

func (e *InputError) Error() string { return "bridge: " + e.Field + ": " + e.Message }

func NewClient(config Config) (*Client, error) {
	if config.Host == "" {
		config.Host = DefaultHost
	}
	parsed, err := url.Parse(config.Host)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") ||
		parsed.User != nil ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" {
		return nil, &InputError{
			Field:   "host",
			Message: "must be an HTTP(S) URL without credentials, query, or fragment",
		}
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}
	if config.UserAgent == "" {
		config.UserAgent = "go-clob-client/bridge"
	}
	return &Client{
		http: &polyhttp.Client{
			BaseURL:    strings.TrimRight(config.Host, "/"),
			HTTPClient: config.HTTPClient,
			UserAgent:  config.UserAgent,
		},
	}, nil
}

func (c *Client) GetSupportedAssets(ctx context.Context) (*SupportedAssetsResponse, error) {
	var out SupportedAssetsResponse
	if err := c.http.GetJSON(ctx, supportedAssetsEndpoint, nil, polyhttp.AuthNone, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateDepositAddress registers routing addresses for a Polymarket wallet.
// No tokens move until the caller separately sends funds to those addresses.
func (c *Client) CreateDepositAddress(
	ctx context.Context,
	address common.Address,
) (*DepositResponse, error) {
	var out DepositResponse
	if err := c.http.PostJSON(ctx, depositEndpoint, DepositRequest{Address: address}, polyhttp.AuthNone, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetStatus accepts an address from any supported network, not just EVM.
func (c *Client) GetStatus(ctx context.Context, address string) (*StatusResponse, error) {
	if strings.TrimSpace(address) == "" || address == "." || address == ".." {
		return nil, &InputError{Field: "address", Message: "is required"}
	}
	var out StatusResponse
	if err := c.http.GetJSON(ctx, statusEndpoint+"/"+url.PathEscape(address), nil, polyhttp.AuthNone, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetQuote(ctx context.Context, request QuoteRequest) (*QuoteResponse, error) {
	if _, err := ParseBaseUnits(string(request.FromAmountBaseUnit)); err != nil {
		return nil, &InputError{Field: "from_amount_base_unit", Message: err.Error()}
	}
	var out QuoteResponse
	if err := c.http.PostJSON(ctx, quoteEndpoint, request, polyhttp.AuthNone, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateWithdrawalAddress registers routing addresses for the requested
// destination. Despite the service's /withdraw path, this does not itself
// withdraw, sign or transfer funds from the Polymarket wallet.
func (c *Client) CreateWithdrawalAddress(
	ctx context.Context,
	request WithdrawRequest,
) (*WithdrawResponse, error) {
	var out WithdrawResponse
	if err := c.http.PostJSON(ctx, withdrawEndpoint, request, polyhttp.AuthNone, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
