package clob

import "encoding/json"

// Page is a paginated Polymarket response envelope.
type Page[T any] struct {
	Limit      int    `json:"limit"`
	Count      int    `json:"count"`
	NextCursor string `json:"next_cursor"`
	Data       []T    `json:"data"`
}

// CursorPage is the raw compatibility page envelope for endpoints that still expose raw JSON.
type CursorPage = Page[json.RawMessage]
