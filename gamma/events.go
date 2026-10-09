package gamma

import (
	"context"
	"iter"
	"net/url"
	"strconv"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

func eventOptionsQuery(options []EventOptions) url.Values {
	if len(options) == 0 {
		return nil
	}
	query := url.Values{}
	setBool(query, "include_chat", options[0].IncludeChat)
	setBool(query, "include_template", options[0].IncludeTemplate)
	setBool(query, "include_best_lines", options[0].IncludeBestLines)
	setString(query, "locale", options[0].Locale)
	return query
}

// GetEvent returns a single event by its ID. Optional Rust-compatible
// include_chat and include_template behavior can be supplied as the third argument.
func (c *Client) GetEvent(
	ctx context.Context,
	id string,
	options ...EventOptions,
) (*Event, error) {
	var out Event
	query := eventOptionsQuery(options)
	err := c.http.GetJSON(
		ctx,
		eventsEndpoint+"/"+url.PathEscape(id),
		query,
		polyhttp.AuthNone,
		&out,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// GetEventBySlug returns a single event by its slug. Optional Rust-compatible
// include_chat and include_template behavior can be supplied as the third argument.
func (c *Client) GetEventBySlug(
	ctx context.Context,
	slug string,
	options ...EventOptions,
) (*Event, error) {
	var out Event
	query := eventOptionsQuery(options)
	err := c.http.GetJSON(
		ctx,
		eventsEndpoint+"/slug/"+url.PathEscape(slug),
		query,
		polyhttp.AuthNone,
		&out,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// GetEvents returns a list of events based on the provided filters.
func (c *Client) GetEvents(ctx context.Context, params EventFilterParams) ([]Event, error) {
	if err := validateOffset(params.Limit, params.Offset, -1); err != nil {
		return nil, err
	}
	var out []Event
	err := c.http.GetJSON(ctx, eventsEndpoint, eventQuery(params), polyhttp.AuthNone, &out)
	return out, err
}

func eventQuery(params EventFilterParams) url.Values {
	query := url.Values{}
	setBool(query, "ascending", params.Ascending)
	setBool(query, "include_best_lines", params.IncludeBestLines)
	setBool(query, "include_children", params.IncludeChildren)
	setBool(query, "ended", params.Ended)
	setBool(query, "live", params.Live)
	setBool(query, "featured_order", params.FeaturedOrder)
	setString(query, "event_date", params.EventDate)
	if params.EventWeek != nil {
		query.Set("event_week", strconv.Itoa(*params.EventWeek))
	}
	setString(query, "start_time_min", params.StartTimeMin)
	setString(query, "start_time_max", params.StartTimeMax)
	setString(query, "parent_event_id", params.ParentEventID)
	setString(query, "partner_slug", params.PartnerSlug)
	setString(query, "title_search", params.TitleSearch)
	setString(query, "locale", params.Locale)
	setString(query, "tag_match", params.TagMatch)
	for _, id := range params.SeriesIDs {
		query.Add("series_id", id)
	}
	for _, id := range params.GameIDs {
		query.Add("game_id", id)
	}
	setBool(query, "active", params.Active)
	setBool(query, "closed", params.Closed)
	setBool(query, "archived", params.Archived)
	setBool(query, "resolved", params.Resolved)
	for _, id := range params.IDs {
		query.Add("id", id)
	}
	for _, order := range params.Orders {
		query.Add("order", order)
	}
	if len(params.Orders) == 0 {
		setString(query, "order", params.Order)
	}
	addGammaFilterValues(query, "tag_id", params.TagID, params.TagIDs)
	for _, id := range params.ExcludeTagIDs {
		query.Add("exclude_tag_id", id)
	}
	addGammaFilterValues(query, "slug", params.Slug, params.Slugs)
	if params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Offset > 0 {
		query.Set("offset", strconv.Itoa(params.Offset))
	}
	setBool(query, "negative_risk", params.NegativeRisk)
	setString(query, "tag_slug", params.TagSlug)
	setBool(query, "related_tags", params.RelatedTags)
	setBool(query, "featured", params.Featured)
	setBool(query, "cyom", params.CYOM)
	setBool(query, "include_chat", params.IncludeChat)
	setBool(query, "include_template", params.IncludeTemplate)
	setString(query, "recurrence", params.Recurrence)
	setString(query, "liquidity_min", params.LiquidityMin)
	setString(query, "liquidity_max", params.LiquidityMax)
	setString(query, "volume_min", params.VolumeMin)
	setString(query, "volume_max", params.VolumeMax)
	setString(query, "start_date_min", params.StartDateMin)
	setString(query, "start_date_max", params.StartDateMax)
	setString(query, "end_date_min", params.EndDateMin)
	setString(query, "end_date_max", params.EndDateMax)

	return query
}

// IterEvents returns an iterator for events based on the provided filters.
func (c *Client) IterEvents(ctx context.Context, p EventFilterParams) iter.Seq2[Event, error] {
	return offsetItems(ctx, p.Limit, p.Offset, 100, -1, func(limit, offset int) ([]Event, error) {
		q := p
		q.Limit = limit
		q.Offset = offset
		return c.GetEvents(ctx, q)
	}, nil, func(item Event) string { return string(item.ID) })
}

// GetEventTags returns all tags associated with an event.
func (c *Client) GetEventTags(ctx context.Context, eventID string) ([]Tag, error) {
	var out []Tag
	err := c.http.GetJSON(
		ctx,
		eventsEndpoint+"/"+url.PathEscape(eventID)+"/tags",
		nil,
		polyhttp.AuthNone,
		&out,
	)
	return out, err
}
