package gamma

import (
	"context"
	"iter"
	"net/url"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

func tagOptionsQuery(options []TagOptions) url.Values {
	if len(options) == 0 {
		return nil
	}
	query := url.Values{}
	setBool(query, "include_template", options[0].IncludeTemplate)
	setString(query, "locale", options[0].Locale)
	return query
}

func relatedTagsOptionsQuery(options []RelatedTagsOptions) url.Values {
	if len(options) == 0 {
		return nil
	}
	query := url.Values{}
	setBool(query, "omit_empty", options[0].OmitEmpty)
	setString(query, "status", options[0].Status)
	return query
}

// GetTag returns a single tag by its ID. Optional Rust-compatible
// include_template behavior can be supplied as the third argument.
func (c *Client) GetTag(
	ctx context.Context,
	id string,
	options ...TagOptions,
) (*Tag, error) {
	var out Tag
	query := tagOptionsQuery(options)
	err := c.http.GetJSON(ctx, tagsEndpoint+"/"+url.PathEscape(id), query, polyhttp.AuthNone, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTagBySlug returns a single tag by its slug. Optional Rust-compatible
// include_template behavior can be supplied as the third argument.
func (c *Client) GetTagBySlug(
	ctx context.Context,
	slug string,
	options ...TagOptions,
) (*Tag, error) {
	var out Tag
	query := tagOptionsQuery(options)
	err := c.http.GetJSON(
		ctx,
		tagsEndpoint+"/slug/"+url.PathEscape(slug),
		query,
		polyhttp.AuthNone,
		&out,
	)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// GetRelatedTags returns tags related to a specific tag. Optional Rust-compatible
// omit_empty and status filters can be supplied as the third argument.
func (c *Client) GetRelatedTags(
	ctx context.Context,
	tagID string,
	options ...RelatedTagsOptions,
) ([]RelatedTag, error) {
	var out []RelatedTag
	err := c.http.GetJSON(
		ctx,
		tagsEndpoint+"/"+url.PathEscape(tagID)+"/related-tags",
		relatedTagsOptionsQuery(options),
		polyhttp.AuthNone,
		&out,
	)
	return out, err
}

// GetRelatedTagsBySlug returns tags related to a specific tag by slug.
func (c *Client) GetRelatedTagsBySlug(
	ctx context.Context,
	slug string,
	options ...RelatedTagsOptions,
) ([]RelatedTag, error) {
	var out []RelatedTag
	err := c.http.GetJSON(
		ctx,
		tagsEndpoint+"/slug/"+url.PathEscape(slug)+"/related-tags",
		relatedTagsOptionsQuery(options),
		polyhttp.AuthNone,
		&out,
	)
	return out, err
}

// GetTagsRelatedToTag returns tags related to a specific tag.
func (c *Client) GetTagsRelatedToTag(
	ctx context.Context,
	tagID string,
	options ...RelatedTagsOptions,
) ([]Tag, error) {
	var out []Tag
	err := c.http.GetJSON(
		ctx,
		tagsEndpoint+"/"+url.PathEscape(tagID)+"/related-tags/tags",
		relatedTagsOptionsQuery(options),
		polyhttp.AuthNone,
		&out,
	)
	return out, err
}

// GetTagsRelatedToTagBySlug returns tags related to a specific tag by slug.
func (c *Client) GetTagsRelatedToTagBySlug(
	ctx context.Context,
	slug string,
	options ...RelatedTagsOptions,
) ([]Tag, error) {
	var out []Tag
	err := c.http.GetJSON(
		ctx,
		tagsEndpoint+"/slug/"+url.PathEscape(slug)+"/related-tags/tags",
		relatedTagsOptionsQuery(options),
		polyhttp.AuthNone,
		&out,
	)
	return out, err
}

func relatedTagResourceQuery(p RelatedTagResourceParams) url.Values {
	query := url.Values{}
	setString(query, "locale", p.Locale)
	setBool(query, "omit_empty", p.OmitEmpty)
	setString(query, "status", p.Status)
	return query
}

// GetRelatedTagResources returns Gamma resources linked from related tags by ID.
func (c *Client) GetRelatedTagResources(
	ctx context.Context,
	tagID string,
	p RelatedTagResourceParams,
) ([]Tag, error) {
	var out []Tag
	err := c.http.GetJSON(
		ctx,
		tagsEndpoint+"/"+url.PathEscape(tagID)+"/related-tags/tags",
		relatedTagResourceQuery(p),
		polyhttp.AuthNone,
		&out,
	)
	return out, err
}

// GetRelatedTagResourcesBySlug returns Gamma resources linked from related tags by slug.
func (c *Client) GetRelatedTagResourcesBySlug(
	ctx context.Context,
	slug string,
	p RelatedTagResourceParams,
) ([]Tag, error) {
	var out []Tag
	err := c.http.GetJSON(
		ctx,
		tagsEndpoint+"/slug/"+url.PathEscape(slug)+"/related-tags/tags",
		relatedTagResourceQuery(p),
		polyhttp.AuthNone,
		&out,
	)
	return out, err
}

// GetTags reads the first page of tags. Use IterTags for a listing.
func (c *Client) GetTags(ctx context.Context) ([]Tag, error) {
	return c.GetTagsPage(ctx, TagFilterParams{})
}

// GetTagsPage returns a single page of tags.
func (c *Client) GetTagsPage(ctx context.Context, p TagFilterParams) ([]Tag, error) {
	if err := validateOffset(p.Limit, p.Offset, -1); err != nil {
		return nil, err
	}
	query := gammaQuery(pageLimit(p.Limit, maxTagPageSize), p.Offset)
	setBool(query, "ascending", p.Ascending)
	setBool(query, "include_template", p.IncludeTemplate)
	setBool(query, "is_carousel", p.IsCarousel)
	setString(query, "locale", p.Locale)
	setString(query, "order", p.Order)

	var out []Tag
	err := c.http.GetJSON(ctx, tagsEndpoint, query, polyhttp.AuthNone, &out)
	return out, err
}

// ListTags returns all tags matching the provided filters.
func (c *Client) ListTags(ctx context.Context, p TagFilterParams) ([]Tag, error) {
	return collect(c.IterTags(ctx, p))
}

// IterTags returns an iterator over tags.
func (c *Client) IterTags(ctx context.Context, p TagFilterParams) iter.Seq2[Tag, error] {
	return offsetItems(
		ctx,
		p.Limit,
		p.Offset,
		maxTagPageSize,
		-1,
		func(limit, offset int) ([]Tag, error) {
			q := p
			q.Limit = limit
			q.Offset = offset
			return c.GetTagsPage(ctx, q)
		},
		nil,
		func(item Tag) string { return string(item.ID) },
	)
}
