package clob

// Page is a paginated Polymarket response envelope.
type Page[T any] struct {
	Limit      int    `json:"limit"`
	Count      int    `json:"count"`
	NextCursor string `json:"next_cursor"`
	Data       []T    `json:"data"`
}
