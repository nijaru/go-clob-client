// Package perps provides a Go client for the Polymarket Perps API.
//
// The Perps surface is a distinct product from the CLOB event markets and lives
// behind its own host (https://api.perpetuals.polymarket.com). It is modeled as
// a separate top-level package rather than folded into clob/ to keep the two
// protocols and their auth/session models apart.
//
// Oracle: the official TypeScript SDK (ts-sdk) is the reference for perps; the
// Rust SDK has no perps module yet. Market-data endpoints are public GET
// requests under /v1/info/*. Authenticated account reads and the delegated
// WebSocket session are available through NewAuthenticated; signed order
// commands remain a separate follow-up.
//
// Decimal values are represented as their wire strings (e.g. "123.450000") to
// avoid precision loss; callers should parse with a decimal library as needed.
package perps
