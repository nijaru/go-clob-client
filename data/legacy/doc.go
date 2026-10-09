// Package legacy implements the supported v1 Polymarket Data API contract.
//
// New integrations should use github.com/nijaru/go-clob-client/data for the
// current v2 API. This package retains the v1 response models and offset-based
// endpoints exposed by the Rust SDK; it never acts as an automatic fallback
// after a v2 request fails.
//
// Create a client with New, or stream list endpoints with their Iter methods.
// No signing or authenticated trading operations are performed by this package.
package legacy
