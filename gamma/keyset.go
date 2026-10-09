package gamma

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"maps"
	"math"
	"net/url"
	"strconv"

	"github.com/nijaru/go-clob-client/internal/polyhttp"
)

func keysetPage[T any](
	ctx context.Context,
	c *Client,
	path, key string,
	query url.Values,
	cursor Cursor,
) (*Page[T], error) {
	bindingQuery := maps.Clone(query)
	bindingQuery.Del("limit") // Page size does not alter a keyset continuation query.
	binding := boundQuery(c, path, bindingQuery)
	state, err := decodeCursor(cursor, binding)
	if err != nil {
		return nil, err
	}
	if state.Offset != 0 || (cursor != "" && state.After == "") {
		return nil, ErrInvalidCursor
	}
	setString(query, "after_cursor", state.After)
	var wire map[string]json.RawMessage
	if err := c.http.GetJSON(ctx, path, query, polyhttp.AuthNone, &wire); err != nil {
		return nil, err
	}
	raw, ok := wire[key]
	if !ok || string(raw) == "null" {
		return nil, fmt.Errorf("gamma %s keyset: missing %s array", key, key)
	}
	var items []T
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("gamma %s keyset: %w", key, err)
	}
	page := &Page[T]{Items: items}
	if raw, ok := wire["next_cursor"]; ok && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		var after string
		if err := json.Unmarshal(raw, &after); err != nil || after == "" {
			return nil, fmt.Errorf("gamma %s keyset: invalid next_cursor", key)
		}
		if after == state.After {
			return nil, ErrPaginationStalled
		}
		page.HasMore = true
		page.NextCursor = encodeCursor(binding, after, 0)
	}
	return page, nil
}

// GetMarketsPage reads /markets/keyset. Offset is not valid on a keyset request.
// Unlike the TS binary model, Go retains legacy multi-outcome markets.
func (c *Client) GetMarketsPage(
	ctx context.Context,
	p MarketFilterParams,
	cursor Cursor,
) (*Page[Market], error) {
	if err := validateOffset(p.Limit, p.Offset, -1); err != nil {
		return nil, err
	}
	if p.Offset != 0 {
		return nil, fmt.Errorf("gamma markets keyset: offset is not supported")
	}
	query := marketQuery(p)
	query.Set("limit", strconv.Itoa(iteratorLimit(p.Limit, 20, math.MaxInt)))
	return keysetPage[Market](ctx, c, marketsEndpoint+"/keyset", "markets", query, cursor)
}

// IterMarketPages traverses stable market keyset discovery without offset drift.
func (c *Client) IterMarketPages(
	ctx context.Context,
	p MarketFilterParams,
	cursor Cursor,
) iter.Seq2[Page[Market], error] {
	return walkPages(
		ctx,
		cursor,
		func(next Cursor) (*Page[Market], error) { return c.GetMarketsPage(ctx, p, next) },
	)
}

// IterMarketsKeyset yields markets from keyset discovery. IterMarkets remains
// available for the Rust offset-listing surface.
func (c *Client) IterMarketsKeyset(
	ctx context.Context,
	p MarketFilterParams,
	cursor Cursor,
) iter.Seq2[Market, error] {
	return pageItems(c.IterMarketPages(ctx, p, cursor), "markets", 0)
}

// GetEventsPage reads /events/keyset. The stable discovery default is open events;
// an explicit Closed pointer overrides it. Offset is not a keyset parameter.
func (c *Client) GetEventsPage(
	ctx context.Context,
	p EventFilterParams,
	cursor Cursor,
) (*Page[Event], error) {
	if err := validateOffset(p.Limit, p.Offset, -1); err != nil {
		return nil, err
	}
	if p.Offset != 0 {
		return nil, fmt.Errorf("gamma events keyset: offset is not supported")
	}
	query := eventQuery(p)
	if p.Closed == nil {
		query.Set("closed", "false")
	}
	query.Set("limit", strconv.Itoa(iteratorLimit(p.Limit, 20, math.MaxInt)))
	return keysetPage[Event](ctx, c, eventsEndpoint+"/keyset", "events", query, cursor)
}

// IterEventPages traverses events using query-bound service cursors.
func (c *Client) IterEventPages(
	ctx context.Context,
	p EventFilterParams,
	cursor Cursor,
) iter.Seq2[Page[Event], error] {
	return walkPages(
		ctx,
		cursor,
		func(next Cursor) (*Page[Event], error) { return c.GetEventsPage(ctx, p, next) },
	)
}

// IterEventsKeyset yields events from keyset discovery.
func (c *Client) IterEventsKeyset(
	ctx context.Context,
	p EventFilterParams,
	cursor Cursor,
) iter.Seq2[Event, error] {
	return pageItems(c.IterEventPages(ctx, p, cursor), "events", 0)
}
