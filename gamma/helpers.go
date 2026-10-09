package gamma

import (
	"net/url"
	"strconv"
)

const (
	maxSeriesPageSize         = 50
	maxTagPageSize            = 100
	maxCommentsByUserPageSize = 100
	maxCommentsPageSize       = 100
	maxTeamsPageSize          = 100
	maxClarificationsPageSize = 100
)

func gammaQuery(limit, offset int) url.Values {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	return q
}

func setBool(q url.Values, key string, val *bool) {
	if val != nil {
		q.Set(key, strconv.FormatBool(*val))
	}
}

func setInt(q url.Values, key string, val int) {
	if val > 0 {
		q.Set(key, strconv.Itoa(val))
	}
}

func setString[T ~string](q url.Values, key string, val T) {
	if val != "" {
		q.Set(key, string(val))
	}
}

func iteratorLimit(limit, defaultLimit, max int) int {
	if limit <= 0 {
		return min(defaultLimit, max)
	}
	return min(limit, max)
}

func pageLimit(limit, max int) int {
	if limit <= 0 {
		return 0
	}
	return min(limit, max)
}

func addGammaFilterValues(query url.Values, key, single string, values []string) {
	if len(values) > 0 {
		for _, value := range values {
			query.Add(key, value)
		}
		return
	}
	setString(query, key, single)
}
