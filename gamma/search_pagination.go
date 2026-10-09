package gamma

import (
	"context"
	"iter"
)

// SearchPage carries search results and the service's completeness signal.
// NextPage is zero when there is no continuation. At page 100, HasMore may stay
// true and LimitReached is set: the service does not establish completeness.
type SearchPage struct {
	Results      *SearchResults
	HasMore      bool
	LimitReached bool
	NextPage     int
}

// GetSearchPage returns one public-search page with explicit boundary metadata.
func (c *Client) GetSearchPage(ctx context.Context, p SearchParams) (*SearchPage, error) {
	if p.Page == 0 {
		p.Page = 1
	}
	result, err := c.Search(ctx, p)
	if err != nil {
		return nil, err
	}
	page := &SearchPage{Results: result}
	if result.Pagination != nil {
		page.HasMore = result.Pagination.HasMore
	}
	if page.HasMore {
		page.NextPage = p.Page + 1
		page.LimitReached = p.Page == 100
	}
	return page, nil
}

// IterSearchPages stops at the service's 100-page boundary with LimitReached
// on the final page, retaining HasMore. It does not request page 101.
func (c *Client) IterSearchPages(ctx context.Context, p SearchParams) iter.Seq2[SearchPage, error] {
	return func(yield func(SearchPage, error) bool) {
		for {
			if err := ctx.Err(); err != nil {
				yield(SearchPage{}, err)
				return
			}
			page, err := c.GetSearchPage(ctx, p)
			if err != nil {
				yield(SearchPage{}, err)
				return
			}
			if !yield(*page, nil) || !page.HasMore || page.LimitReached {
				return
			}
			p.Page = page.NextPage
		}
	}
}

// IterSearch yields result batches, then ErrPaginationLimit if more results
// may exist beyond the service boundary.
func (c *Client) IterSearch(ctx context.Context, p SearchParams) iter.Seq2[SearchResults, error] {
	return func(yield func(SearchResults, error) bool) {
		for page, err := range c.IterSearchPages(ctx, p) {
			if err != nil {
				yield(SearchResults{}, err)
				return
			}
			if !yield(*page.Results, nil) {
				return
			}
			if page.LimitReached {
				yield(SearchResults{}, &PaginationLimitError{Resource: "search", Limit: 100})
				return
			}
		}
	}
}
