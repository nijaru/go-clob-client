## Current State

| Metric | Value                    | Updated    |
| ------ | ------------------------ | ---------- |
| Build  | `go build ./...` passing | 2026-03-16 |
| Tests  | `go test ./...` passing  | 2026-03-16 |

## What's Done

- **Tiered Client Architecture (2026 Refactor)**: Refactored SDK into `Client`, `SignerClient`, and `AuthenticatedClient` to implement **Type-Level Auth Guards**. Trading endpoints are now mathematically impossible to call without correct credentials.
- **Heartbeats API (2026 Standard)**: Fully automated background heartbeat loop in `AuthenticatedClient` with **Heartbeat ID rotation**.
- **Batch Order Parity (2026 standard)**: Increased `PostOrders` batch limit to **15** and added **Post-Only** validation for GTC/GTD orders.
- **Bridge API 2026**: Updated to Jan 2026 specifications for `/withdraw` and `/status` endpoints.
- **Modern Go 1.26 Refactor**: 
    - Migrated to `encoding/json/v2` and `encoding/json/jsontext`.
    - Implemented **Iterators** (`iter.Seq2`) for all paginated list methods (Orders, Trades, Markets).
    - Integrated `errors.AsType` for cleaner error handling.
- Repo organized with family packages: `clob/`, `gamma/`, `data/`, `bridge/`.
- WebSocket (Phase 3): `clob/ws` market and user channels with WS auth derivation.
- RFQ (Phase 4): full RFQ API surface parity with TypeScript SDK.
- Orderbook hash generation (SHA-1) matching TypeScript `generateOrderBookSummaryHash`.
- Bug fix: Fixed Market Buy rounding logic to maintain strict parity with official reference SDKs.

## Completed Sprints

- Sprint 01: Parity Audit
- Sprint 02: Logic Review (EIP-712, Market Price, Fees)
- Sprint 03: Type & Wire Audit (RFQ, WS formats)
- Sprint 04 (2026 Standard): Tiered Architecture, Heartbeats, Go 1.26 Modernization.

## Bug Fixes (2026-03-16)

- Fixed Market Buy rounding: maintained flipped precision parity with official SDKs to ensure backend acceptance.
- Improved HTTP error parsing: `newAPIError` now extracts `"message"` from structured JSON error objects.
- Fixed websocket goroutine leaks and ensured idempotent `Close()`.

## Active Work
- None. v0.0.1-ready for 2026 Polymarket standards.

## Reference Priority

1. **Rust** (`polymarket-rust-sdk`) — primary for 2026 performance standards
2. **TypeScript** (`@polymarket/clob-client`) — primary for wire-format/hash parity
3. **Python** (`py-clob-client`) — secondary reference

## Verified Against Official SDKs

- **Type Guards**: Matches Rust SDK's structural safety patterns.
- **Orderbook hash (SHA-1)**: matches TypeScript `generateOrderBookSummaryHash`.
- **CalculateMarketPrice**: matches official algorithms.
- **Order signing**: EIP-712 domain/struct hash verified.
- **Heartbeat rotation**: Matches 2026 Rust SDK implementation.
