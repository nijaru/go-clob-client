package clob

import (
	"errors"
	"fmt"
	"iter"
)

// Page is a paginated Polymarket response envelope.
type Page[T any] struct {
	Limit      int    `json:"limit"`
	Count      int    `json:"count"`
	NextCursor string `json:"next_cursor"`
	Data       []T    `json:"data"`
}

// ErrPaginationStalled reports a paginated response that cannot advance. The
// service repeated the cursor already requested, so following it would loop
// forever. Stopping silently instead would truncate the listing.
var ErrPaginationStalled = errors.New("clob: pagination did not advance")

func normalizedCursor(nextCursor string) string {
	if nextCursor == "" {
		return initialCursor
	}
	return nextCursor
}

// nextPageCursor returns the cursor for the next page and whether the traversal
// is complete. A repeated cursor is an error rather than an end-of-listing.
func nextPageCursor(currentCursor, nextCursor string) (string, bool, error) {
	switch nextCursor {
	case "":
		return "", true, nil
	case currentCursor:
		return "", true, fmt.Errorf("%w: repeated cursor %q", ErrPaginationStalled, nextCursor)
	case endCursor:
		return endCursor, false, nil
	default:
		return nextCursor, false, nil
	}
}

// paginate walks a cursor-paginated list from start. Empty start begins at the
// first page. Every list iterator shares this traversal so cursor validation
// and end-of-listing handling cannot drift between endpoints.
func paginate[T any](
	start string,
	fetch func(cursor string) (*Page[T], error),
) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		cursor := normalizedCursor(start)
		for cursor != endCursor {
			page, err := fetch(cursor)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			for _, item := range page.Data {
				if !yield(item, nil) {
					return
				}
			}
			next, done, err := nextPageCursor(cursor, page.NextCursor)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			if done {
				return
			}
			cursor = next
		}
	}
}
