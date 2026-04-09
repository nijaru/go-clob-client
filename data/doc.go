// Package data provides a read-only client for Polymarket's Data API.
//
// The package covers positions, trades, activity, holders, value, open
// interest, live volume, and leaderboard endpoints. It does not include any
// signing, order posting, or authenticated trading flows; use the clob package
// for those.
//
// # Creating a client
//
//	c := data.New(data.Config{})
//
// # Common usage
//
//	ctx := context.Background()
//	positions, err := c.GetPositions(ctx, data.PositionParams{User: "0x1234..."})
//	if err != nil {
//	    return err
//	}
//	for _, pos := range positions {
//	    fmt.Println(pos.Title, pos.Size)
//	}
//
// # Iterators
//
// Large list endpoints expose both slice-returning methods (for example,
// [Client.GetPositions]) and range-over-function iterators (for example,
// [Client.IterPositions]) for memory-efficient streaming.
package data
