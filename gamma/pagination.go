package gamma

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"math"
	"net/url"
	"slices"
)

var (
	// ErrPaginationLimit means the service cannot establish a complete listing.
	ErrPaginationLimit = errors.New("gamma pagination limit reached; listing may be incomplete")
	// ErrInvalidCursor means a cursor is malformed or belongs to another query.
	ErrInvalidCursor = errors.New("gamma cursor does not match this query")
	// ErrPaginationStalled means the server repeated a page or cursor.
	ErrPaginationStalled = errors.New("gamma pagination did not advance")
)

// PaginationLimitError identifies an upstream listing boundary.
type PaginationLimitError struct {
	Resource string
	Limit    int
}

func (e *PaginationLimitError) Error() string {
	return fmt.Sprintf("%s: %v (%d)", e.Resource, ErrPaginationLimit, e.Limit)
}
func (e *PaginationLimitError) Unwrap() error { return ErrPaginationLimit }

// Cursor is an opaque, serializable continuation bound to the client host,
// resource and effective query. Pass it unchanged to the corresponding page
// method; it is not the raw server after_cursor token.
type Cursor string

// Page describes a discovery page. HasMore is not a guarantee of completeness.
// LimitReached marks a full page at an upstream boundary; its NextCursor cannot
// be followed. Item iterators surface ErrPaginationLimit after yielding its items.
type Page[T any] struct {
	Items        []T
	HasMore      bool
	LimitReached bool
	NextCursor   Cursor
}

type cursorState struct {
	Version int    `json:"v"`
	Query   string `json:"q"`
	After   string `json:"a,omitempty"`
	Offset  int    `json:"o,omitempty"`
}

func boundQuery(c *Client, path string, query url.Values) string {
	return c.host + "\n" + path + "\n" + query.Encode()
}

func encodeCursor(query, after string, offset int) Cursor {
	data, _ := json.Marshal(cursorState{Version: 1, Query: query, After: after, Offset: offset})
	return Cursor(base64.RawURLEncoding.EncodeToString(data))
}

func decodeCursor(cursor Cursor, query string) (cursorState, error) {
	if cursor == "" {
		return cursorState{}, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(string(cursor))
	if err != nil {
		return cursorState{}, ErrInvalidCursor
	}
	var state cursorState
	if err := json.Unmarshal(data, &state); err != nil || state.Version != 1 ||
		state.Query != query ||
		state.Offset < 0 {
		return cursorState{}, ErrInvalidCursor
	}
	return state, nil
}

func validateOffset(limit, offset, maxOffset int) error {
	if limit < 0 || offset < 0 {
		return fmt.Errorf("gamma pagination: limit and offset must be nonnegative")
	}
	if maxOffset >= 0 && offset > maxOffset {
		return &PaginationLimitError{Resource: "comments", Limit: maxOffset}
	}
	return nil
}

// walkPages protects cancellation, early stopping and cursor cycles uniformly.
func walkPages[T any](
	ctx context.Context,
	initial Cursor,
	fetch func(Cursor) (*Page[T], error),
) iter.Seq2[Page[T], error] {
	return func(yield func(Page[T], error) bool) {
		cursor := initial
		seen := map[Cursor]bool{}
		for {
			if err := ctx.Err(); err != nil {
				yield(Page[T]{}, err)
				return
			}
			if seen[cursor] {
				yield(Page[T]{}, ErrPaginationStalled)
				return
			}
			seen[cursor] = true
			page, err := fetch(cursor)
			if err != nil {
				yield(Page[T]{}, err)
				return
			}
			if !yield(*page, nil) || !page.HasMore || page.LimitReached {
				return
			}
			if page.NextCursor == "" {
				yield(Page[T]{}, ErrPaginationStalled)
				return
			}
			cursor = page.NextCursor
		}
	}
}

func pageItems[T any](
	pages iter.Seq2[Page[T], error],
	resource string,
	limit int,
) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var zero T
		for page, err := range pages {
			if err != nil {
				yield(zero, err)
				return
			}
			for _, item := range page.Items {
				if !yield(item, nil) {
					return
				}
			}
			if page.LimitReached {
				yield(zero, &PaginationLimitError{Resource: resource, Limit: limit})
				return
			}
		}
	}
}

// Offset pagination stays available for the Rust surface and explicit offsets.
// fill counts the records to which the server actually applied limit (comment
// replies do not consume it). Detect a repeated page instead of looping forever.
func offsetItems[T any](
	ctx context.Context,
	limit, offset, maxPageSize, maxOffset int,
	fetch func(int, int) ([]T, error),
	fill func([]T) int,
	id func(T) string,
) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var zero T
		var previous []string
		if err := validateOffset(limit, offset, maxOffset); err != nil {
			yield(zero, err)
			return
		}
		limit = iteratorLimit(limit, 20, maxPageSize)
		first := true
		for {
			if err := ctx.Err(); err != nil {
				yield(zero, err)
				return
			}
			if err := validateOffset(limit, offset, maxOffset); err != nil {
				yield(zero, err)
				return
			}
			items, err := fetch(limit, offset)
			if err != nil {
				yield(zero, err)
				return
			}
			if len(items) == 0 {
				return
			}
			ids := make([]string, len(items))
			for i, item := range items {
				ids[i] = id(item)
			}
			if !first && slices.Equal(ids, previous) {
				yield(zero, ErrPaginationStalled)
				return
			}
			previous = ids
			first = false
			for _, item := range items {
				if !yield(item, nil) {
					return
				}
			}
			count := len(items)
			if fill != nil {
				count = fill(items)
			}
			if count < limit {
				return
			}
			if offset > math.MaxInt-limit {
				yield(zero, ErrPaginationStalled)
				return
			}
			offset += limit
		}
	}
}

func collect[T any](items iter.Seq2[T, error]) ([]T, error) {
	var all []T
	for item, err := range items {
		if err != nil {
			return all, err
		}
		all = append(all, item)
	}
	return all, nil
}
