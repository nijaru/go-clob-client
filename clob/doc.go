// Package clob provides a Go client for Polymarket's CLOB HTTP and WebSocket APIs.
//
// # Client tiers
//
// The package uses a three-tier client hierarchy that enforces authentication
// requirements at compile time:
//
//   - [Client] — read-only, no credentials required.
//   - [SignerClient] — extends Client with L1 Ethereum-signed methods (order
//     creation, API key management). Requires a private key.
//   - [AuthenticatedClient] — extends SignerClient with L2 API-key methods
//     (posting orders, managing positions, heartbeats). Requires a private key
//     and API credentials.
//
// # Creating a client
//
//	// Read-only
//	c, err := clob.NewClient(clob.Config{})
//
//	// Signing only
//	c, err := clob.NewSignerClient(clob.Config{
//	    PrivateKey: os.Getenv("POLYMARKET_PRIVATE_KEY"),
//	})
//
//	// Fully authenticated (also starts background heartbeat loop)
//	c, err := clob.NewAuthenticatedClient(clob.Config{
//	    PrivateKey: os.Getenv("POLYMARKET_PRIVATE_KEY"),
//	    Credentials: &clob.Credentials{
//	        Key:        os.Getenv("POLYMARKET_API_KEY"),
//	        Secret:     os.Getenv("POLYMARKET_API_SECRET"),
//	        Passphrase: os.Getenv("POLYMARKET_API_PASSPHRASE"),
//	    },
//	})
//	defer c.Close()
//
// # Pagination iterators
//
// List methods returning large result sets expose both a slice variant
// (GetMarkets) and a Go 1.23+ range-over-function iterator (IterMarkets) for
// memory-efficient streaming:
//
//	for market, err := range client.IterMarkets(ctx) {
//	    if err != nil { return err }
//	    fmt.Println(market.ConditionID)
//	}
//
// # Error handling
//
// Non-2xx responses are returned as [*APIError], which exposes the HTTP status
// code and response body. Use the package-level sentinel errors with
// [errors.Is] for common cases:
//
//	if errors.Is(err, clob.ErrNotFound) { ... }
//	if errors.Is(err, clob.ErrRateLimit) { ... }
//
// All clients are safe for concurrent use by multiple goroutines.
package clob
