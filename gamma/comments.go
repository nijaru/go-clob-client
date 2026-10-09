package gamma

import (
	"context"
	"fmt"
	"iter"
	"net/url"
	"strconv"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

const maxCommentsOffset = 200

func commentQuery(p CommentFilterParams) url.Values {
	query := gammaQuery(pageLimit(p.Limit, maxCommentsPageSize), p.Offset)
	setString(query, "parent_entity_type", p.ParentEntityType)
	setString(query, "parent_entity_id", p.ParentEntityID)
	setBool(query, "get_positions", p.GetPositions)
	setBool(query, "holders_only", p.HoldersOnly)
	setString(query, "order", p.Order)
	setBool(query, "ascending", p.Ascending)
	return query
}

func rootComments(items []Comment) int {
	count := 0
	for _, item := range items {
		if item.ParentCommentID == nil {
			count++
		}
	}
	return count
}

// GetComments reads one offset page, including replies that ride along with
// its top-level comments. Pages starting past offset 200 are not served.
func (c *Client) GetComments(ctx context.Context, p CommentFilterParams) ([]Comment, error) {
	if err := validateOffset(p.Limit, p.Offset, maxCommentsOffset); err != nil {
		return nil, err
	}
	var out []Comment
	err := c.http.GetJSON(ctx, commentsEndpoint, commentQuery(p), polyhttp.AuthNone, &out)
	return out, err
}

func keysetComments(p CommentFilterParams) bool {
	return (p.ParentEntityType == ParentEntityTypeEvent || p.ParentEntityType == ParentEntityTypeSeries) &&
		p.ParentEntityID != "" &&
		(p.GetPositions == nil || !*p.GetPositions) &&
		(p.HoldersOnly == nil || !*p.HoldersOnly) &&
		(p.Order == "" || p.Order == "id" || p.Order == "createdAt")
}

// GetCommentsPage uses /comments/keyset for unfiltered Event/Series reads sorted
// by id or createdAt, and bounded offset pages otherwise. Without Order the
// effective sort is createdAt descending (Ascending is ignored by the service).
// With Order, direction defaults to ascending. Limit counts roots, not replies.
func (c *Client) GetCommentsPage(
	ctx context.Context,
	p CommentFilterParams,
	cursor Cursor,
) (*Page[Comment], error) {
	if err := validateOffset(p.Limit, p.Offset, -1); err != nil {
		return nil, err
	}
	query := commentQuery(p)
	if query.Get("parent_entity_id") == "" || query.Get("parent_entity_type") == "" {
		return nil, fmt.Errorf("gamma comments: parent entity ID and type are required")
	}
	limit := iteratorLimit(p.Limit, 20, maxCommentsPageSize)
	query.Set("limit", strconv.Itoa(limit))
	query.Del("offset")
	if keysetComments(p) {
		if p.Offset != 0 {
			return nil, fmt.Errorf(
				"gamma comments keyset: offset is not supported; use GetComments for offset reads",
			)
		}
		query.Del("get_positions")
		query.Del("holders_only")
		if p.Order == "" {
			query.Set("order", "createdAt")
			query.Set("ascending", "false")
		} else if p.Ascending == nil {
			query.Set("ascending", "true")
		}
		return keysetPage[Comment](ctx, c, commentsEndpoint+"/keyset", "comments", query, cursor)
	}
	return offsetCommentPage(ctx, c, commentsEndpoint, query, cursor, p.Offset, limit, true)
}

func offsetCommentPage(
	ctx context.Context,
	c *Client,
	path string,
	query url.Values,
	cursor Cursor,
	offset, limit int,
	roots bool,
) (*Page[Comment], error) {
	binding := boundQuery(c, path, query)
	if cursor != "" {
		state, err := decodeCursor(cursor, binding)
		if err != nil {
			return nil, err
		}
		if state.After != "" {
			return nil, ErrInvalidCursor
		}
		offset = state.Offset
	}
	if err := validateOffset(limit, offset, maxCommentsOffset); err != nil {
		return nil, err
	}
	query.Set("offset", strconv.Itoa(offset))
	var items []Comment
	if err := c.http.GetJSON(ctx, path, query, polyhttp.AuthNone, &items); err != nil {
		return nil, err
	}
	count := len(items)
	if roots {
		count = rootComments(items)
	}
	hasMore := count >= limit
	page := &Page[Comment]{Items: items, HasMore: hasMore}
	if hasMore {
		page.LimitReached = offset+limit > maxCommentsOffset
		page.NextCursor = encodeCursor(binding, "", offset+limit)
	}
	return page, nil
}

// IterCommentPages yields pages, including LimitReached when the service cannot
// establish completeness. Following a boundary cursor returns ErrPaginationLimit.
func (c *Client) IterCommentPages(
	ctx context.Context,
	p CommentFilterParams,
	cursor Cursor,
) iter.Seq2[Page[Comment], error] {
	return walkPages(
		ctx,
		cursor,
		func(next Cursor) (*Page[Comment], error) { return c.GetCommentsPage(ctx, p, next) },
	)
}

// IterComments yields the complete thread when keyset reads are supported.
// Offset-only reads yield accessible comments then ErrPaginationLimit if needed.
func (c *Client) IterComments(
	ctx context.Context,
	p CommentFilterParams,
) iter.Seq2[Comment, error] {
	// Explicit offset reads retain their offset semantics, even if the same
	// filters could support keyset pagination starting at the first page.
	if p.Offset != 0 {
		return offsetItems(ctx, p.Limit, p.Offset, maxCommentsPageSize, maxCommentsOffset,
			func(limit, offset int) ([]Comment, error) {
				q := p
				q.Limit = limit
				q.Offset = offset
				return c.GetComments(ctx, q)
			}, rootComments, func(item Comment) string { return item.ID })
	}
	return pageItems(c.IterCommentPages(ctx, p, ""), "comments", maxCommentsOffset)
}

// ListComments collects a thread, retaining accessible items on error.
func (c *Client) ListComments(ctx context.Context, p CommentFilterParams) ([]Comment, error) {
	return collect(c.IterComments(ctx, p))
}

// GetComment returns the comment thread identified by id. GetPositions can be
// requested without changing the returned thread shape.
func (c *Client) GetComment(
	ctx context.Context,
	id string,
	options ...CommentOptions,
) ([]Comment, error) {
	query := url.Values{}
	if len(options) > 0 {
		setBool(query, "get_positions", options[0].GetPositions)
	}
	var out []Comment
	err := c.http.GetJSON(
		ctx,
		commentsEndpoint+"/"+url.PathEscape(id),
		query,
		polyhttp.AuthNone,
		&out,
	)
	return out, err
}

// GetCommentsByUserAddress reads the first page, not every comment by this user.
func (c *Client) GetCommentsByUserAddress(ctx context.Context, address string) ([]Comment, error) {
	return c.GetCommentsByUserAddressPage(ctx, address, CommentsByUserAddressParams{})
}

func userCommentsQuery(p CommentsByUserAddressParams) url.Values {
	query := gammaQuery(pageLimit(p.Limit, maxCommentsByUserPageSize), p.Offset)
	setBool(query, "ascending", p.Ascending)
	setString(query, "order", p.Order)
	return query
}

// GetCommentsByUserAddressPage reads one bounded offset page of user comments.
func (c *Client) GetCommentsByUserAddressPage(
	ctx context.Context,
	address string,
	p CommentsByUserAddressParams,
) ([]Comment, error) {
	if err := validateOffset(p.Limit, p.Offset, maxCommentsOffset); err != nil {
		return nil, err
	}
	var out []Comment
	err := c.http.GetJSON(
		ctx,
		commentsEndpoint+"/user_address/"+url.PathEscape(address),
		userCommentsQuery(p),
		polyhttp.AuthNone,
		&out,
	)
	return out, err
}

// GetUserCommentsPage adds explicit completeness metadata to user comment reads.
// There is no stable keyset or range-filter escape from this listing's cap.
func (c *Client) GetUserCommentsPage(
	ctx context.Context,
	address string,
	p CommentsByUserAddressParams,
	cursor Cursor,
) (*Page[Comment], error) {
	if err := validateOffset(p.Limit, p.Offset, -1); err != nil {
		return nil, err
	}
	limit := iteratorLimit(p.Limit, 20, maxCommentsByUserPageSize)
	query := userCommentsQuery(p)
	query.Del("offset")
	query.Set("limit", strconv.Itoa(limit))
	return offsetCommentPage(
		ctx,
		c,
		commentsEndpoint+"/user_address/"+url.PathEscape(address),
		query,
		cursor,
		p.Offset,
		limit,
		false,
	)
}

// IterUserCommentPages exposes the last accessible full page as LimitReached.
func (c *Client) IterUserCommentPages(
	ctx context.Context,
	address string,
	p CommentsByUserAddressParams,
	cursor Cursor,
) iter.Seq2[Page[Comment], error] {
	return walkPages(
		ctx,
		cursor,
		func(next Cursor) (*Page[Comment], error) { return c.GetUserCommentsPage(ctx, address, p, next) },
	)
}

// IterCommentsByUserAddress returns accessible comments, then ErrPaginationLimit
// if their listing is still full at the service boundary.
func (c *Client) IterCommentsByUserAddress(
	ctx context.Context,
	address string,
	p CommentsByUserAddressParams,
) iter.Seq2[Comment, error] {
	return pageItems(
		c.IterUserCommentPages(ctx, address, p, ""),
		"user comments",
		maxCommentsOffset,
	)
}

// ListCommentsByUserAddress preserves partial items with any pagination error.
func (c *Client) ListCommentsByUserAddress(
	ctx context.Context,
	address string,
	p CommentsByUserAddressParams,
) ([]Comment, error) {
	return collect(c.IterCommentsByUserAddress(ctx, address, p))
}
