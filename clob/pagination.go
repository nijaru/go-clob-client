package clob

// Page is a paginated Polymarket response envelope.
type Page[T any] struct {
	Limit      int    `json:"limit"`
	Count      int    `json:"count"`
	NextCursor string `json:"next_cursor"`
	Data       []T    `json:"data"`
}

func normalizedCursor(nextCursor string) string {
	if nextCursor == "" {
		return initialCursor
	}
	return nextCursor
}

func nextPageCursor(currentCursor, nextCursor string) (string, bool) {
	switch nextCursor {
	case "":
		return "", true
	case currentCursor:
		return "", true
	case endCursor:
		return endCursor, false
	default:
		return nextCursor, false
	}
}
