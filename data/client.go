package data

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

const (
	DefaultHost = "https://data-api.polymarket.com"

	positionsEndpoint       = "/positions"
	closedPositionsEndpoint = "/closed-positions"
	valueEndpoint           = "/value"
	tradesEndpoint          = "/trades"
	activityEndpoint        = "/activity"
	leaderboardEndpoint     = "/v1/leaderboard"
)

// Client is a read-only client for the Polymarket Data API.
type Client struct {
	host string
	http *polyhttp.Client
}

// Config defines the configuration for a Data client.
type Config struct {
	Host       string
	HTTPClient *http.Client
	UserAgent  string
}

// New creates a new Data API client.
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
		c.UserAgent = "go-clob-client/data"
	}
	return c
}

// GetPositions returns the open positions for a user.
func (c *Client) GetPositions(ctx context.Context, user string) ([]Position, error) {
	query := url.Values{}
	query.Set("user", user)

	var out []Position
	err := c.http.GetJSON(ctx, positionsEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// GetClosedPositions returns the closed positions for a user.
func (c *Client) GetClosedPositions(ctx context.Context, user string) ([]Position, error) {
	query := url.Values{}
	query.Set("user", user)

	var out []Position
	err := c.http.GetJSON(ctx, closedPositionsEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// GetTotalValue returns the total current value of a user's positions.
func (c *Client) GetTotalValue(ctx context.Context, user string) (float64, error) {
	query := url.Values{}
	query.Set("user", user)

	var out PositionValue
	err := c.http.GetJSON(ctx, valueEndpoint, query, polyhttp.AuthNone, &out)
	return out.Value, err
}

// GetTrades returns trade history for a user or market.
func (c *Client) GetTrades(ctx context.Context, params TradeParams) ([]DataTrade, error) {
	query := url.Values{}
	if params.User != "" {
		query.Set("user", params.User)
	}
	if params.Market != "" {
		query.Set("market", params.Market)
	}
	if params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Offset > 0 {
		query.Set("offset", strconv.Itoa(params.Offset))
	}

	var out []DataTrade
	err := c.http.GetJSON(ctx, tradesEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// GetActivity returns on-chain activity logs for a user.
func (c *Client) GetActivity(ctx context.Context, user string) ([]Activity, error) {
	query := url.Values{}
	query.Set("user", user)

	var out []Activity
	err := c.http.GetJSON(ctx, activityEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// GetLeaderboard returns rankings from the trader leaderboard.
func (c *Client) GetLeaderboard(
	ctx context.Context,
	params LeaderboardParams,
) ([]LeaderboardEntry, error) {
	query := url.Values{}
	if params.Category != "" {
		query.Set("category", string(params.Category))
	}
	if params.TimePeriod != "" {
		query.Set("timePeriod", string(params.TimePeriod))
	}
	if params.SortBy != "" {
		query.Set("orderBy", string(params.SortBy))
	}
	if params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Offset > 0 {
		query.Set("offset", strconv.Itoa(params.Offset))
	}
	if params.User != "" {
		query.Set("user", params.User)
	}
	if params.UserName != "" {
		query.Set("userName", params.UserName)
	}

	var out []LeaderboardEntry
	err := c.http.GetJSON(ctx, leaderboardEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}
