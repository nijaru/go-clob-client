package data

func boundedLimit(limit, max int) int {
	if limit <= 0 {
		return 0
	}
	return min(limit, max)
}

func iteratorLimit(limit, defaultLimit, max int) int {
	if limit <= 0 {
		return min(defaultLimit, max)
	}
	return min(limit, max)
}
