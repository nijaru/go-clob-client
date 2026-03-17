package gamma

import (
	"context"
	"fmt"
	"iter"
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
	teamsEndpoint    = "/teams"
	commentsEndpoint = "/comments"
	profileEndpoint  = "/public-profile"
	searchEndpoint   = "/public-search"
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
		httpClient := http.DefaultClient
		httpClient.Timeout = 15 * time.Second
		c.HTTPClient = httpClient
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

// GetMarketBySlug returns a single market by its slug.
func (c *Client) GetMarketBySlug(ctx context.Context, slug string) (*Market, error) {
	query := url.Values{}
	query.Set("slug", slug)

	var out []Market
	err := c.http.GetJSON(ctx, marketsEndpoint, query, polyhttp.AuthNone, &out)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("market not found")
	}
	return &out[0], nil
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
	if params.Resolved != nil {
		query.Set("resolved", strconv.FormatBool(*params.Resolved))
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
	if params.Slug != "" {
		query.Set("slug", params.Slug)
	}
	if params.NegativeRisk != nil {
		query.Set("negative_risk", strconv.FormatBool(*params.NegativeRisk))
	}
	if params.AcceptingOrders != nil {
		query.Set("accepting_orders", strconv.FormatBool(*params.AcceptingOrders))
	}

	var out []Market
	err := c.http.GetJSON(ctx, marketsEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// IterMarkets returns an iterator for markets based on the provided filters.
func (c *Client) IterMarkets(ctx context.Context, params MarketFilterParams) iter.Seq2[Market, error] {
	return func(yield func(Market, error) bool) {
		offset := params.Offset
		limit := params.Limit
		if limit <= 0 {
			limit = 100
		}

		for {
			p := params
			p.Limit = limit
			p.Offset = offset
			markets, err := c.GetMarkets(ctx, p)
			if err != nil {
				yield(Market{}, err)
				return
			}
			if len(markets) == 0 {
				return
			}
			for _, m := range markets {
				if !yield(m, nil) {
					return
				}
			}
			if len(markets) < limit {
				return
			}
			offset += len(markets)
		}
	}
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
	if params.Archived != nil {
		query.Set("archived", strconv.FormatBool(*params.Archived))
	}
	if params.Resolved != nil {
		query.Set("resolved", strconv.FormatBool(*params.Resolved))
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
	if params.NegativeRisk != nil {
		query.Set("negative_risk", strconv.FormatBool(*params.NegativeRisk))
	}

	var out []Event
	err := c.http.GetJSON(ctx, eventsEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// IterEvents returns an iterator for events based on the provided filters.
func (c *Client) IterEvents(ctx context.Context, params EventFilterParams) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		offset := params.Offset
		limit := params.Limit
		if limit <= 0 {
			limit = 100
		}

		for {
			p := params
			p.Limit = limit
			p.Offset = offset
			events, err := c.GetEvents(ctx, p)
			if err != nil {
				yield(Event{}, err)
				return
			}
			if len(events) == 0 {
				return
			}
			for _, e := range events {
				if !yield(e, nil) {
					return
				}
			}
			if len(events) < limit {
				return
			}
			offset += len(events)
		}
	}
}

// Search returns search results matching the query.
func (c *Client) Search(ctx context.Context, query string) ([]Market, error) {
	params := url.Values{}
	params.Set("query", query)

	var out []Market
	err := c.http.GetJSON(ctx, searchEndpoint, params, polyhttp.AuthNone, &out)
	return out, err
}

// GetSeries returns a single series by its ID.
func (c *Client) GetSeries(ctx context.Context, id string) (*Series, error) {
	var out Series
	err := c.http.GetJSON(ctx, seriesEndpoint+"/"+id, nil, polyhttp.AuthNone, &out)
	return &out, err
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

// GetRelatedTags returns tags related to a specific tag.
func (c *Client) GetRelatedTags(ctx context.Context, tagID string) ([]RelatedTag, error) {
	var out []RelatedTag
	err := c.http.GetJSON(ctx, tagsEndpoint+"/"+tagID+"/related-tags", nil, polyhttp.AuthNone, &out)
	return out, err
}

// GetRelatedTagsBySlug returns tags related to a specific tag by slug.
func (c *Client) GetRelatedTagsBySlug(ctx context.Context, slug string) ([]RelatedTag, error) {
	var out []RelatedTag
	err := c.http.GetJSON(ctx, tagsEndpoint+"/slug/"+slug+"/related-tags", nil, polyhttp.AuthNone, &out)
	return out, err
}

// GetTagsRelatedToTag returns tags related to a specific tag.
func (c *Client) GetTagsRelatedToTag(ctx context.Context, tagID string) ([]Tag, error) {
	var out []Tag
	err := c.http.GetJSON(ctx, tagsEndpoint+"/"+tagID+"/related-tags/tags", nil, polyhttp.AuthNone, &out)
	return out, err
}

// GetTagsRelatedToTagBySlug returns tags related to a specific tag by slug.
func (c *Client) GetTagsRelatedToTagBySlug(ctx context.Context, slug string) ([]Tag, error) {
	var out []Tag
	err := c.http.GetJSON(ctx, tagsEndpoint+"/slug/"+slug+"/related-tags/tags", nil, polyhttp.AuthNone, &out)
	return out, err
}

// GetTags returns all tags.
func (c *Client) GetTags(ctx context.Context) ([]Tag, error) {
	var out []Tag
	err := c.http.GetJSON(ctx, tagsEndpoint, nil, polyhttp.AuthNone, &out)
	return out, err
}

// GetEventTags returns all tags associated with an event.
func (c *Client) GetEventTags(ctx context.Context, eventID string) ([]Tag, error) {
	var out []Tag
	err := c.http.GetJSON(ctx, eventsEndpoint+"/"+eventID+"/tags", nil, polyhttp.AuthNone, &out)
	return out, err
}

// GetMarketTags returns all tags associated with a market.
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

// GetTeams returns all sports teams.
func (c *Client) GetTeams(ctx context.Context) ([]Team, error) {
	var out []Team
	err := c.http.GetJSON(ctx, teamsEndpoint, nil, polyhttp.AuthNone, &out)
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

// IterComments returns an iterator for comments based on the provided filters.
func (c *Client) IterComments(ctx context.Context, params CommentFilterParams) iter.Seq2[Comment, error] {
	return func(yield func(Comment, error) bool) {
		offset := params.Offset
		limit := params.Limit
		if limit <= 0 {
			limit = 100
		}

		for {
			p := params
			p.Limit = limit
			p.Offset = offset
			comments, err := c.GetComments(ctx, p)
			if err != nil {
				yield(Comment{}, err)
				return
			}
			if len(comments) == 0 {
				return
			}
			for _, c := range comments {
				if !yield(c, nil) {
					return
				}
			}
			if len(comments) < limit {
				return
			}
			offset += len(comments)
		}
	}
}

// GetComment returns comments by their ID.
func (c *Client) GetComment(ctx context.Context, id string) ([]Comment, error) {
	var out []Comment
	err := c.http.GetJSON(ctx, commentsEndpoint+"/"+id, nil, polyhttp.AuthNone, &out)
	return out, err
}

// GetCommentsByUserAddress returns comments posted by a wallet address.
func (c *Client) GetCommentsByUserAddress(ctx context.Context, address string) ([]Comment, error) {
	var out []Comment
	err := c.http.GetJSON(ctx, commentsEndpoint+"/user_address/"+address, nil, polyhttp.AuthNone, &out)
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
