package data

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

const DefaultHost = "https://data-api.polymarket.com"

type Config struct {
	Host       string
	HTTPClient *http.Client
	UserAgent  string
}

// Client reads the current Data API. It is safe for concurrent use provided
// the configured HTTP transport is. It performs no signing or trading writes.
// Indexed JSON reads retry HTTP 429 twice, honoring server retry delays (one second
// when absent). Delays over five seconds propagate without retrying; the caller's
// context bounds requests and waits. Accounting downloads are not retried.
type Client struct{ http *polyhttp.Client }

func NewClient(config Config) (*Client, error) {
	if config.Host == "" {
		config.Host = DefaultHost
	}
	parsed, err := url.Parse(config.Host)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" ||
		parsed.User != nil {
		return nil, &InputError{
			Field:   "host",
			Message: "must be an HTTP(S) URL without credentials, query, or fragment",
		}
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}
	if config.UserAgent == "" {
		config.UserAgent = "go-clob-client/data"
	}
	return &Client{
		http: &polyhttp.Client{
			BaseURL:    strings.TrimRight(config.Host, "/"),
			HTTPClient: config.HTTPClient,
			UserAgent:  config.UserAgent,
		},
	}, nil
}

type APIError = polyhttp.APIError

// InputError identifies a locally rejected request; no HTTP call was made.
type InputError struct{ Field, Message string }

func (e *InputError) Error() string { return "data: " + e.Field + ": " + e.Message }

var (
	ErrInvalidResponse = errors.New("data: invalid service response")
	ErrCursorCycle     = errors.New("data: pagination cursor repeated")
)

// Page preserves the service's exact continuation state. A final page has
// HasMore=false and an empty NextCursor. Reuse a cursor only with its original
// filters. Cursors are opaque: clients never interpret or manufacture them.
type Page[T any] struct {
	Items      []T
	HasMore    bool
	NextCursor string
}

// PageParams controls one request. Limit=0 uses the server default. Cursor=""
// starts a new traversal; all iterators resume from the supplied cursor.
type PageParams struct {
	Limit  int
	Cursor string
}

func getPage[T any](
	ctx context.Context,
	c *Client,
	path string,
	q url.Values,
	p PageParams,
	maximum int,
) (Page[T], error) {
	var empty Page[T]
	if p.Limit < 0 || p.Limit > maximum {
		return empty, &InputError{
			Field:   "page.limit",
			Message: fmt.Sprintf("must be between 0 and %d", maximum),
		}
	}
	if p.Limit != 0 {
		q.Set("limit", fmt.Sprint(p.Limit))
	}
	if p.Cursor != "" {
		q.Set("cursor", p.Cursor)
	}
	var envelope struct {
		Data       json.RawMessage `json:"data"`
		Pagination *struct {
			HasMore    *bool           `json:"has_more"`
			NextCursor json.RawMessage `json:"next_cursor"`
		} `json:"pagination"`
	}
	if err := c.getReadJSON(ctx, path, q, &envelope); err != nil {
		return empty, err
	}
	if len(envelope.Data) == 0 || envelope.Data[0] != '[' || envelope.Pagination == nil ||
		envelope.Pagination.HasMore == nil ||
		len(envelope.Pagination.NextCursor) == 0 {
		return empty, fmt.Errorf("%w: list requires data and pagination", ErrInvalidResponse)
	}
	page := Page[T]{HasMore: *envelope.Pagination.HasMore}
	if !bytes.Equal(envelope.Pagination.NextCursor, []byte("null")) {
		if err := json.Unmarshal(envelope.Pagination.NextCursor, &page.NextCursor); err != nil ||
			page.NextCursor == "" {
			return empty, fmt.Errorf("%w: invalid next_cursor", ErrInvalidResponse)
		}
	}
	if page.HasMore != (page.NextCursor != "") {
		return empty, fmt.Errorf("%w: has_more and next_cursor disagree", ErrInvalidResponse)
	}
	if err := decodeWire(envelope.Data, &page.Items); err != nil {
		return empty, fmt.Errorf("%w: %s: %w", ErrInvalidResponse, path, err)
	}
	return page, nil
}

func getValue[T any](
	ctx context.Context,
	c *Client,
	path string,
	q url.Values,
	nullable bool,
) (*T, error) {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := c.getReadJSON(ctx, path, q, &envelope); err != nil {
		return nil, err
	}
	if len(envelope.Data) == 0 {
		return nil, fmt.Errorf("%w: missing data", ErrInvalidResponse)
	}
	if bytes.Equal(envelope.Data, []byte("null")) {
		if nullable {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: unexpected null data", ErrInvalidResponse)
	}
	var value T
	if err := decodeWire(envelope.Data, &value); err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrInvalidResponse, path, err)
	}
	return &value, nil
}

func getList[T any](ctx context.Context, c *Client, path string, q url.Values) ([]T, error) {
	value, err := getValue[[]T](ctx, c, path, q, false)
	if err != nil {
		return nil, err
	}
	return *value, nil
}

// walk stops on cancellation or consumer break, without a goroutine or
// prefetch. Empty pages can have a continuation; row count is not a cursor.
func walk[T any](
	ctx context.Context,
	start PageParams,
	fetch func(PageParams) (Page[T], error),
) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		p := start
		seen := map[string]struct{}{}
		var zero T
		if p.Cursor != "" {
			seen[p.Cursor] = struct{}{}
		}
		for {
			if err := ctx.Err(); err != nil {
				yield(zero, err)
				return
			}
			page, err := fetch(p)
			if err != nil {
				yield(zero, err)
				return
			}
			if page.HasMore {
				if _, exists := seen[page.NextCursor]; exists {
					yield(zero, ErrCursorCycle)
					return
				}
				seen[page.NextCursor] = struct{}{}
			}
			for _, item := range page.Items {
				if !yield(item, nil) {
					return
				}
			}
			if !page.HasMore {
				return
			}
			p.Cursor = page.NextCursor
		}
	}
}
