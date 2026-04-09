// Package bridge provides a client for Polymarket's bridge API.
//
// The package covers supported assets, deposit address generation, quote
// retrieval, transaction status checks, and withdrawals across supported chains.
// It is separate from the CLOB trading API and does not manage orders.
//
// # Creating a client
//
//	c := bridge.New(bridge.Config{})
//
// # Common usage
//
//	ctx := context.Background()
//	assets, err := c.GetSupportedAssets(ctx)
//	if err != nil {
//	    return err
//	}
//	fmt.Println(len(assets.Tokens))
package bridge
