package data

import (
	"net/url"
	"slices"
)

type marketFilterKind uint8

const (
	marketFilterUnset marketFilterKind = iota
	marketFilterMarkets
	marketFilterEventIDs
)

// MarketFilter selects either specific markets or specific event IDs.
//
// The zero value means "no filter".
type MarketFilter struct {
	kind   marketFilterKind
	values []string
}

// MarketsFilter filters by market condition IDs.
func MarketsFilter(markets ...string) MarketFilter {
	return MarketFilter{
		kind:   marketFilterMarkets,
		values: slices.Clone(markets),
	}
}

// EventIDsFilter filters by event IDs.
func EventIDsFilter(eventIDs ...string) MarketFilter {
	return MarketFilter{
		kind:   marketFilterEventIDs,
		values: slices.Clone(eventIDs),
	}
}

func (f MarketFilter) appendQuery(query url.Values) {
	switch f.kind {
	case marketFilterMarkets:
		setCommaList(query, "market", f.values)
	case marketFilterEventIDs:
		setCommaList(query, "eventId", f.values)
	}
}
