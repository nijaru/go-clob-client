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

const (
	DefaultHost = "https://data-api.polymarket.com"

	positionsEndpoint          = "/positions"
	closedPositionsEndpoint    = "/closed-positions"
	valueEndpoint              = "/value"
	tradesEndpoint             = "/trades"
	activityEndpoint           = "/activity"
	holdersEndpoint            = "/holders"
	tradedEndpoint             = "/traded"
	oiEndpoint                 = "/oi"
	liveVolumeEndpoint         = "/live-volume"
	leaderboardEndpoint        = "/v1/leaderboard"
	builderLeaderboardEndpoint = "/v1/builders/leaderboard"
	builderVolumeEndpoint      = "/v1/builders/volume"
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
		httpClient := http.DefaultClient
		httpClient.Timeout = 15 * time.Second
		c.HTTPClient = httpClient
	}
	if c.UserAgent == "" {
		c.UserAgent = "go-clob-client/data"
	}
	return c
}

// GetPositions returns the open positions for a user.
func (c *Client) GetPositions(ctx context.Context, params PositionParams) ([]Position, error) {
	query := url.Values{}
	query.Set("user", params.User)
	if len(params.Markets) > 0 {
		query.Set("market", strings.Join(params.Markets, ","))
	}
	if len(params.EventIDs) > 0 {
		query.Set("eventID", strings.Join(params.EventIDs, ","))
	}
	if params.SizeThreshold != "" {
		query.Set("sizeThreshold", params.SizeThreshold)
	}
	if params.Redeemable != nil {
		query.Set("redeemable", strconv.FormatBool(*params.Redeemable))
	}
	if params.Mergeable != nil {
		query.Set("mergeable", strconv.FormatBool(*params.Mergeable))
	}
	if params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Offset > 0 {
		query.Set("offset", strconv.Itoa(params.Offset))
	}
	if params.SortBy != "" {
		query.Set("sortBy", params.SortBy)
	}
	if params.SortDirection != "" {
		query.Set("sortDirection", params.SortDirection)
	}
	if params.Title != "" {
		query.Set("title", params.Title)
	}

	var out []Position
	err := c.http.GetJSON(ctx, positionsEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// GetClosedPositions returns the closed positions for a user.
func (c *Client) GetClosedPositions(ctx context.Context, params ClosedPositionParams) ([]ClosedPosition, error) {
	query := url.Values{}
	query.Set("user", params.User)
	if len(params.Markets) > 0 {
		query.Set("market", strings.Join(params.Markets, ","))
	}
	if len(params.EventIDs) > 0 {
		query.Set("eventID", strings.Join(params.EventIDs, ","))
	}
	if params.Title != "" {
		query.Set("title", params.Title)
	}
	if params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Offset > 0 {
		query.Set("offset", strconv.Itoa(params.Offset))
	}
	if params.SortBy != "" {
		query.Set("sortBy", params.SortBy)
	}
	if params.SortDirection != "" {
		query.Set("sortDirection", params.SortDirection)
	}

	var out []ClosedPosition
	err := c.http.GetJSON(ctx, closedPositionsEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// GetTotalValue returns the total current value of a user's positions in USDC.
func (c *Client) GetTotalValue(ctx context.Context, user string, markets []string) (string, error) {
	query := url.Values{}
	query.Set("user", user)
	if len(markets) > 0 {
		query.Set("market", strings.Join(markets, ","))
	}

	var out positionValueResponse
	err := c.http.GetJSON(ctx, valueEndpoint, query, polyhttp.AuthNone, &out)
	return out.Value, err
}

// GetTrades returns trade history based on the provided filters.
func (c *Client) GetTrades(ctx context.Context, params TradeParams) ([]DataTrade, error) {
	query := url.Values{}
	if params.User != "" {
		query.Set("user", params.User)
	}
	if len(params.Markets) > 0 {
		query.Set("market", strings.Join(params.Markets, ","))
	}
	if len(params.EventIDs) > 0 {
		query.Set("eventID", strings.Join(params.EventIDs, ","))
	}
	if params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Offset > 0 {
		query.Set("offset", strconv.Itoa(params.Offset))
	}
	if params.TakerOnly != nil {
		query.Set("takerOnly", strconv.FormatBool(*params.TakerOnly))
	}
	if params.Side != "" {
		query.Set("side", params.Side)
	}

	var out []DataTrade
	err := c.http.GetJSON(ctx, tradesEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// GetActivity returns on-chain activity logs based on the provided filters.
func (c *Client) GetActivity(ctx context.Context, params ActivityParams) ([]Activity, error) {
	query := url.Values{}
	query.Set("user", params.User)
	if len(params.Markets) > 0 {
		query.Set("market", strings.Join(params.Markets, ","))
	}
	if len(params.EventIDs) > 0 {
		query.Set("eventID", strings.Join(params.EventIDs, ","))
	}
	if len(params.ActivityTypes) > 0 {
		query.Set("type", strings.Join(params.ActivityTypes, ","))
	}
	if params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Offset > 0 {
		query.Set("offset", strconv.Itoa(params.Offset))
	}
	if params.Start > 0 {
		query.Set("start", strconv.FormatInt(params.Start, 10))
	}
	if params.End > 0 {
		query.Set("end", strconv.FormatInt(params.End, 10))
	}
	if params.SortBy != "" {
		query.Set("sortBy", params.SortBy)
	}
	if params.SortDirection != "" {
		query.Set("sortDirection", params.SortDirection)
	}
	if params.Side != "" {
		query.Set("side", params.Side)
	}

	var out []Activity
	err := c.http.GetJSON(ctx, activityEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// GetHolders returns top token holders for the specified markets.
func (c *Client) GetHolders(ctx context.Context, params HoldersParams) ([]MetaHolder, error) {
	query := url.Values{}
	if len(params.Markets) > 0 {
		query.Set("market", strings.Join(params.Markets, ","))
	}
	if params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.MinBalance > 0 {
		query.Set("minBalance", strconv.Itoa(params.MinBalance))
	}

	var out []MetaHolder
	err := c.http.GetJSON(ctx, holdersEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// GetTradedCount returns the total count of unique markets a user has traded.
func (c *Client) GetTradedCount(ctx context.Context, user string) (int, error) {
	query := url.Values{}
	query.Set("user", user)

	var out Traded
	err := c.http.GetJSON(ctx, tradedEndpoint, query, polyhttp.AuthNone, &out)
	return out.Traded, err
}

// GetOpenInterest returns open interest for one or more markets.
func (c *Client) GetOpenInterest(ctx context.Context, params OpenInterestParams) ([]OpenInterest, error) {
	query := url.Values{}
	if len(params.Markets) > 0 {
		query.Set("market", strings.Join(params.Markets, ","))
	}

	var out []OpenInterest
	err := c.http.GetJSON(ctx, oiEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// GetLiveVolume returns live trading volume for an event.
func (c *Client) GetLiveVolume(ctx context.Context, eventID int64) (*LiveVolume, error) {
	query := url.Values{}
	query.Set("id", strconv.FormatInt(eventID, 10))

	var out LiveVolume
	err := c.http.GetJSON(ctx, liveVolumeEndpoint, query, polyhttp.AuthNone, &out)
	return &out, err
}

// GetLeaderboard returns rankings from the trader leaderboard.
func (c *Client) GetLeaderboard(ctx context.Context, params LeaderboardParams) ([]TraderLeaderboardEntry, error) {
	query := url.Values{}
	if params.Category != "" {
		query.Set("category", params.Category)
	}
	if params.TimePeriod != "" {
		query.Set("timePeriod", params.TimePeriod)
	}
	if params.SortBy != "" {
		query.Set("orderBy", params.SortBy)
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

	var out []TraderLeaderboardEntry
	err := c.http.GetJSON(ctx, leaderboardEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// GetBuilderLeaderboard returns aggregated performance rankings for builders.
func (c *Client) GetBuilderLeaderboard(ctx context.Context, params BuilderLeaderboardParams) ([]BuilderLeaderboardEntry, error) {
	query := url.Values{}
	if params.TimePeriod != "" {
		query.Set("timePeriod", params.TimePeriod)
	}
	if params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Offset > 0 {
		query.Set("offset", strconv.Itoa(params.Offset))
	}

	var out []BuilderLeaderboardEntry
	err := c.http.GetJSON(ctx, builderLeaderboardEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// GetBuilderVolume returns daily volume data points for builders.
func (c *Client) GetBuilderVolume(ctx context.Context, params BuilderVolumeParams) ([]BuilderVolumeEntry, error) {
	query := url.Values{}
	if params.TimePeriod != "" {
		query.Set("timePeriod", params.TimePeriod)
	}

	var out []BuilderVolumeEntry
	err := c.http.GetJSON(ctx, builderVolumeEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}
