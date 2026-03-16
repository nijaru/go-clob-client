package gamma

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

const (
	DefaultHost = "https://gamma-api.polymarket.com"

	marketsEndpoint  = "/markets"
	eventsEndpoint   = "/events"
	seriesEndpoint   = "/series"
	tagsEndpoint     = "/tags"
	sportsEndpoint   = "/sports"
	commentsEndpoint = "/comments"
	profileEndpoint  = "/public-profile"
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

// GetMarket returns a single market by its ID.
func (c *Client) GetMarket(ctx context.Context, id string) (*Market, error) {
	var out Market
	err := c.http.GetJSON(ctx, marketsEndpoint+"/"+id, nil, polyhttp.AuthNone, &out)
	return &out, err
}

// GetMarkets returns a list of markets based on the provided filters.
func (c *Client) GetMarkets(ctx context.Context, params MarketFilterParams) ([]Market, error) {
	query := url.Values{}
	if params.Active != nil {
		query.Set("active", strconv.FormatBool(*params.Active))
	}
	if params.Closed != nil {
		query.Set("closed", strconv.FormatBool(*params.Closed))
	}
	if params.Archived != nil {
		query.Set("archived", strconv.FormatBool(*params.Archived))
	}
	if params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Offset > 0 {
		query.Set("offset", strconv.Itoa(params.Offset))
	}
	if params.Order != "" {
		query.Set("order", params.Order)
	}
	if params.Ascending != nil {
		query.Set("ascending", strconv.FormatBool(*params.Ascending))
	}
	if params.TagID != "" {
		query.Set("tag_id", params.TagID)
	}
	if params.EventID != "" {
		query.Set("event_id", params.EventID)
	}

	var out []Market
	err := c.http.GetJSON(ctx, marketsEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// GetEvent returns a single event by its ID.
func (c *Client) GetEvent(ctx context.Context, id string) (*Event, error) {
	var out Event
	err := c.http.GetJSON(ctx, eventsEndpoint+"/"+id, nil, polyhttp.AuthNone, &out)
	return &out, err
}

// GetEventBySlug returns a single event by its slug.
func (c *Client) GetEventBySlug(ctx context.Context, slug string) (*Event, error) {
	var out Event
	err := c.http.GetJSON(ctx, eventsEndpoint+"/slug/"+slug, nil, polyhttp.AuthNone, &out)
	return &out, err
}

// GetEvents returns a list of events based on the provided filters.
func (c *Client) GetEvents(ctx context.Context, params EventFilterParams) ([]Event, error) {
	query := url.Values{}
	if params.Active != nil {
		query.Set("active", strconv.FormatBool(*params.Active))
	}
	if params.Closed != nil {
		query.Set("closed", strconv.FormatBool(*params.Closed))
	}
	if params.TagID != "" {
		query.Set("tag_id", params.TagID)
	}
	if params.Slug != "" {
		query.Set("slug", params.Slug)
	}
	if params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Offset > 0 {
		query.Set("offset", strconv.Itoa(params.Offset))
	}

	var out []Event
	err := c.http.GetJSON(ctx, eventsEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// GetSearch returns markets matching the query.
func (c *Client) GetSearch(ctx context.Context, query string) ([]Market, error) {
	params := url.Values{}
	params.Set("query", query)

	var out []Market
	err := c.http.GetJSON(ctx, "/search", params, polyhttp.AuthNone, &out)
	return out, err
}

// GetSeries returns a single series by its ID.
func (c *Client) GetSeries(ctx context.Context, id string) (*Series, error) {
	var out Series
	err := c.http.GetJSON(ctx, seriesEndpoint+"/"+id, nil, polyhttp.AuthNone, &out)
	return &out, err
}

// GetAllSeries returns all series.
func (c *Client) GetAllSeries(ctx context.Context) ([]Series, error) {
	var out []Series
	err := c.http.GetJSON(ctx, seriesEndpoint, nil, polyhttp.AuthNone, &out)
	return out, err
}

// GetTag returns a single tag by its ID.
func (c *Client) GetTag(ctx context.Context, id string) (*Tag, error) {
	var out Tag
	err := c.http.GetJSON(ctx, tagsEndpoint+"/"+id, nil, polyhttp.AuthNone, &out)
	return &out, err
}

// GetTagBySlug returns a single tag by its slug.
func (c *Client) GetTagBySlug(ctx context.Context, slug string) (*Tag, error) {
	var out Tag
	err := c.http.GetJSON(ctx, tagsEndpoint+"/slug/"+slug, nil, polyhttp.AuthNone, &out)
	return &out, err
}

// GetTags returns all tags.
func (c *Client) GetTags(ctx context.Context) ([]Tag, error) {
	var out []Tag
	err := c.http.GetJSON(ctx, tagsEndpoint, nil, polyhttp.AuthNone, &out)
	return out, err
}

// GetMarketTags returns tags for a specific market.
func (c *Client) GetMarketTags(ctx context.Context, marketID string) ([]Tag, error) {
	var out []Tag
	err := c.http.GetJSON(ctx, marketsEndpoint+"/"+marketID+"/tags", nil, polyhttp.AuthNone, &out)
	return out, err
}

// GetSports returns all sports metadata.
func (c *Client) GetSports(ctx context.Context) ([]Sport, error) {
	var out []Sport
	err := c.http.GetJSON(ctx, sportsEndpoint, nil, polyhttp.AuthNone, &out)
	return out, err
}

// GetMarketTypes returns all valid sports market types.
func (c *Client) GetMarketTypes(ctx context.Context) ([]MarketType, error) {
	var out []MarketType
	err := c.http.GetJSON(ctx, sportsEndpoint+"/market-types", nil, polyhttp.AuthNone, &out)
	return out, err
}

// GetComments returns a list of comments based on the provided filters.
func (c *Client) GetComments(ctx context.Context, params CommentFilterParams) ([]Comment, error) {
	query := url.Values{}
	if params.ConditionID != "" {
		query.Set("condition_id", params.ConditionID)
	}
	if params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Offset > 0 {
		query.Set("offset", strconv.Itoa(params.Offset))
	}

	var out []Comment
	err := c.http.GetJSON(ctx, commentsEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// GetPublicProfile returns the public profile for a wallet address.
func (c *Client) GetPublicProfile(ctx context.Context, address string) (*PublicProfile, error) {
	query := url.Values{}
	query.Set("address", address)

	var out PublicProfile
	err := c.http.GetJSON(ctx, profileEndpoint, query, polyhttp.AuthNone, &out)
	return &out, err
}
