// Package gamma provides a read-only client for Polymarket's Gamma discovery API.
// It covers markets, events, series, tags and related tags, sports and teams,
// comments, public profiles, search, market clarifications, and service status.
// Use clob for orderbooks and trading.
//
// # Discovery and pagination
//
// GetMarkets and GetEvents read offset pages. GetMarketsPage and GetEventsPage
// read keyset pages; their Cursor values continue the same effective query.
// IterMarketsKeyset and IterEventsKeyset yield individual records, while
// IterMarketPages and IterEventPages retain page metadata. Event keyset discovery
// defaults to open events. An explicit Closed pointer overrides that default.
// Cursors bind the host, resource, filters and sort, but keyset page size can
// change. They are SDK continuations, not raw service tokens or cursors from
// another language SDK. Market sorts normalize each comma-separated volume or
// liquidity token to volumeNum or liquidityNum; event sorts remain unchanged.
// The public market listing forces active=true and archived=false server-side;
// sending those filters does not enable discovery of inactive/archived markets.
//
// Comments use keyset pagination for Event/Series reads without holder filtering
// or position data, ordered by id or createdAt. Without an explicit order, reads
// are newest first. Other comment reads and user-address listings cannot start
// past offset 200. Limits on parent-thread pages count roots; replies ride along.
// Public search serves at most 100 pages. Page APIs expose HasMore and
// LimitReached at those boundaries; item iterators return ErrPaginationLimit
// after yielding accessible records. List methods preserve partial results on
// error. A successful bounded page does not imply a complete listing.
//
// # Wire values
//
// Decimal preserves string-or-number decimals without float64 rounding; Rat
// converts a present value to an exact rational. FlexibleID preserves numeric
// or string identifiers. Sports tags are decoded from comma-separated strings.
// Market outcomes, prices, CLOB token IDs and V2 position IDs accept direct or
// JSON-encoded arrays and retain their wire order, including legacy multi-outcome
// markets. Unknown protocol and combo status values pass through unchanged.
// GetPublicProfile returns nil for a JSON null response.
//
// # Scope and limitations
//
// Models expose Gamma's flat wire fields, not the grouped binary-market models
// used by TS/Python. Most optional scalar fields use Go zero values, so absent,
// null and explicit zero/false are not distinguished. Decimal's empty value and
// pointer fields retain absence where documented. Timestamp fields using
// time.Time require RFC3339 JSON strings. Unknown fields are ignored.
//
// The endpoint surface follows merged Rust 561830b, TS 087f9443 and Python
// ed8d04ca. Fixture and selected live-read checks do not establish exhaustive
// field/validation parity. Filters are mostly forwarded for server validation;
// APIError retains upstream response details, with no automatic retry policy.
// Discovery actions served by Data or RFQ (holders,
// price history, open interest, resolutions, live volume and combo discovery)
// are outside this Gamma package. No write or authenticated operations exist.
//
// # Creating a client
//
//	c := gamma.New(gamma.Config{})
//
// # Keyset discovery
//
//	ctx := context.Background()
//	for market, err := range c.IterMarketsKeyset(ctx, gamma.MarketFilterParams{}, "") {
//		if err != nil {
//			return err
//		}
//		fmt.Println(market.Question)
//		break // Early stopping does not fetch another page.
//	}
package gamma
