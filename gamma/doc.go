// Package gamma provides a read-only client for Polymarket's Gamma API.
//
// Gamma covers market and event discovery rather than trading. Use it for
// market lookup, slug-based resolution, search, tags, comments, and related
// metadata. Use the clob package for orderbooks and trading operations.
//
// # Creating a client
//
//	c := gamma.New(gamma.Config{})
//
// # Common usage
//
//	ctx := context.Background()
//	market, err := c.GetMarketBySlug(ctx, "example-market-slug")
//	if err != nil {
//	    return err
//	}
//	fmt.Println(market.Question)
package gamma
