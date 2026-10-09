package gamma

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

// Search returns structured search results matching the search parameters.
func (c *Client) Search(ctx context.Context, p SearchParams) (*SearchResults, error) {
	if err := validateOffset(p.LimitPerType, p.Page, -1); err != nil {
		return nil, err
	}
	if p.Page > 100 {
		return nil, &PaginationLimitError{Resource: "search", Limit: 100}
	}
	if p.Query == "" {
		return nil, fmt.Errorf("gamma search: query is required")
	}
	if p.Sort != "" && !p.Sort.IsValid() {
		return nil, fmt.Errorf("gamma search: invalid sort %q", p.Sort)
	}

	query := url.Values{}
	query.Set("q", p.Query)
	setBool(query, "ascending", p.Ascending)
	setBool(query, "cache", p.Cache)
	setString(query, "events_status", p.EventsStatus)
	setInt(query, "limit_per_type", p.LimitPerType)
	setInt(query, "page", p.Page)
	for _, tag := range p.EventsTag {
		query.Add("events_tag", tag)
	}
	for _, id := range p.ExcludeTagIDs {
		query.Add("exclude_tag_id", strconv.Itoa(id))
	}
	if p.KeepClosedMarkets != nil {
		query.Set("keep_closed_markets", strconv.Itoa(*p.KeepClosedMarkets))
	}
	setBool(query, "optimized", p.Optimized)
	for _, preset := range p.Presets {
		query.Add("presets", preset)
	}
	setString(query, "recurrence", p.Recurrence)
	setBool(query, "search_profiles", p.SearchProfiles)
	setBool(query, "search_tags", p.SearchTags)
	setString(query, "sort", p.Sort)

	var out SearchResults
	err := c.http.GetJSON(ctx, searchEndpoint, query, polyhttp.AuthNone, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
