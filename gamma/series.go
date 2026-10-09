package gamma

import (
	"context"
	"iter"
	"net/url"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

func seriesOptionsQuery(options []SeriesOptions) url.Values {
	if len(options) == 0 {
		return nil
	}
	query := url.Values{}
	setBool(query, "include_chat", options[0].IncludeChat)
	setString(query, "locale", options[0].Locale)
	return query
}

// GetSeries returns a single series by its ID. Optional Rust-compatible
// include_chat behavior can be supplied as the third argument.
func (c *Client) GetSeries(
	ctx context.Context,
	id string,
	options ...SeriesOptions,
) (*Series, error) {
	var out Series
	query := seriesOptionsQuery(options)
	err := c.http.GetJSON(
		ctx,
		seriesEndpoint+"/"+url.PathEscape(id),
		query,
		polyhttp.AuthNone,
		&out,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// GetSeriesPage returns a single page of series.
func (c *Client) GetSeriesPage(
	ctx context.Context,
	p SeriesFilterParams,
) ([]Series, error) {
	if err := validateOffset(p.Limit, p.Offset, -1); err != nil {
		return nil, err
	}
	query := gammaQuery(pageLimit(p.Limit, maxSeriesPageSize), p.Offset)
	setBool(query, "ascending", p.Ascending)
	setBool(query, "closed", p.Closed)
	setBool(query, "exclude_events", p.ExcludeEvents)
	setString(query, "locale", p.Locale)
	setString(query, "order", p.Order)
	setString(query, "recurrence", p.Recurrence)
	addGammaFilterValues(query, "slug", p.Slug, p.Slugs)
	for _, id := range p.CategoriesIDs {
		query.Add("categories_ids", id)
	}
	for _, label := range p.CategoriesLabels {
		query.Add("categories_labels", label)
	}
	setBool(query, "include_chat", p.IncludeChat)

	var out []Series
	err := c.http.GetJSON(ctx, seriesEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// ListSeries returns all series matching the provided filters.
func (c *Client) ListSeries(ctx context.Context, p SeriesFilterParams) ([]Series, error) {
	return collect(c.IterSeries(ctx, p))
}

// IterSeries returns an iterator over series.
func (c *Client) IterSeries(ctx context.Context, p SeriesFilterParams) iter.Seq2[Series, error] {
	return offsetItems(
		ctx,
		p.Limit,
		p.Offset,
		maxSeriesPageSize,
		-1,
		func(limit, offset int) ([]Series, error) {
			q := p
			q.Limit = limit
			q.Offset = offset
			return c.GetSeriesPage(ctx, q)
		},
		nil,
		func(item Series) string { return string(item.ID) },
	)
}
