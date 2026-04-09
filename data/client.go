package data

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

const DefaultHost = "https://data-api.polymarket.com"

type Client struct {
	host string
	http *polyhttp.Client
}

type Config struct {
	Host       string
	HTTPClient *http.Client
	UserAgent  string
}

func New(config Config) *Client {
	if config.Host == "" {
		config.Host = DefaultHost
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}
	if config.UserAgent == "" {
		config.UserAgent = "go-clob-client/data"
	}
	return &Client{
		host: config.Host,
		http: &polyhttp.Client{
			BaseURL:    config.Host,
			HTTPClient: config.HTTPClient,
			UserAgent:  config.UserAgent,
		},
	}
}

func (c *Client) getJSON(ctx context.Context, path string, query url.Values, out any) error {
	return c.http.GetJSON(ctx, path, query, polyhttp.AuthNone, out)
}

type APIError = polyhttp.APIError

func setBool(query url.Values, key string, val *bool) {
	if val != nil {
		query.Set(key, strconv.FormatBool(*val))
	}
}

func setInt(query url.Values, key string, val int) {
	if val > 0 {
		query.Set(key, strconv.Itoa(val))
	}
}

func setInt64(query url.Values, key string, val int64) {
	if val > 0 {
		query.Set(key, strconv.FormatInt(val, 10))
	}
}

func setString(query url.Values, key, val string) {
	if val != "" {
		query.Set(key, val)
	}
}

func setCommaList(query url.Values, key string, vals []string) {
	if len(vals) > 0 {
		query.Set(key, strings.Join(vals, ","))
	}
}
