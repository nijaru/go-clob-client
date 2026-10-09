// Package perps provides an experimental Go client for the Polymarket Perps API.
//
// Perps uses its own REST and WebSocket hosts, separate from CLOB event markets.
// New provides public market reads, position snapshots, registration and builder
// status, plus six public streaming topics. NewAuthenticated provides delegated
// account reads, history iterators, notifications, TWAP/chase execution and
// OpenSession for trading and typed account updates. NewOwner provides explicit
// owner-signed credential creation/resume/revocation, builder consent, collateral
// transfers, withdrawal and a caller-owned deposit transaction boundary.
//
// Monetary values use exact decimal strings; timestamps are Unix milliseconds
// unless explicitly documented otherwise. Sessions follow their context and
// must be closed when no longer needed. Reconnects and sequence gaps emit resync
// signals; book events are deltas, not reconstructed snapshots. Slow consumers
// close the session rather than silently losing updates.
//
// Submissions are not automatically retried except explicit order_in_flight
// cancellation item rejections. Reconcile uncertain outcomes before resubmitting.
// OrderPlacementError and OrderWithTPSLResult retain reconciliation identities.
// See README.md in this package for the upstream capability inventory and gaps.
package perps
