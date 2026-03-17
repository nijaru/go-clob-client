## 2026-03-12

### Context

The repo is intended to reach feature parity with the official SDKs and stay aligned over time, not remain a tiny single-package CLOB client.

### Decision

Organize the repo by API family from the start: `clob/` as the first public package, shared code under `internal/`, and future families added as siblings.

### Rationale

- Prevents a root-package CLOB client from becoming the accidental architecture for unrelated APIs.
- Keeps the public API idiomatic to Go while preserving semantic parity with the reference SDKs.
- Makes future websocket, Gamma, and data packages additive instead of invasive.

### Tradeoffs

- Slightly more structure up front.
- Examples and docs must use a subpackage import path.

## 2026-03-12

### Context

The SDK needed to state its incompleteness clearly while still becoming practically useful for trading workflows.

### Decision

Add the first trading-core slice now: typed limit/market order creation and signing, while keeping the README explicit that parity is still in progress.

### Rationale

- Makes the SDK materially useful beyond read-only and raw payload posting.
- Keeps user expectations accurate by separating “usable” from “complete”.
- Gives future parity work a real typed trading foundation instead of more raw JSON wrappers.

## 2026-03-12

### Context

The public Go package was starting to accumulate mixed type files, sparse godoc coverage, a panic path in salt generation, and inconsistent pagination semantics across rewards versus orders/trades.

### Decision

Keep the current repo/package layout, but do a public API polish pass inside `clob/`: split public types by domain, validate remote builder auth eagerly even though it breaks the pre-1.0 API, remove the salt-generation panic, and normalize rewards endpoints to page helpers plus flattened convenience methods.

### Rationale

- Preserves the right long-term architecture while cleaning up public Go ergonomics.
- Uses the pre-1.0 window to make the API more idiomatic before users depend on rough edges.
- Keeps rewards semantics consistent with the rest of the SDK.

## 2026-03-12

### Context

Several market helper endpoints in the Go client were still reusing a generic `PriceResponse`/slice model even though the official SDKs expose different wire semantics for midpoint, all-prices, spreads, last-trade, and geoblock checks.

### Decision

Model the aggregate market helpers with dedicated typed responses that match the actual wire shape, add explicit `GetAllPrices` and `CheckGeoblock` helpers, and keep geoblock on its own configurable host.

### Rationale

- Keeps the Go SDK semantically aligned with the official SDKs without copying their package structure.
- Makes the public API easier to understand because each endpoint now returns a shape that matches what the server actually sends.
- Avoids hard-coding the Polymarket site geoblock endpoint into the main CLOB host configuration.

## 2026-03-12

### Context

The repo needed a conventional root-level way to format and verify Go code, and the remaining high-value parity gaps included the official SDK’s chain-address helper methods.

### Decision

Add a minimal root `Makefile` for `fmt`, `test`, `build`, and `check`, and expose chain-aware collateral, conditional-token, and exchange address helpers on `clob.Client`.

### Rationale

- Gives contributors one obvious repo-level workflow without introducing a larger build system.
- Keeps formatting policy explicit and easy to mirror later in CI.
- Fills a small but useful parity gap with the official SDKs while keeping the Go API simple.

## 2026-03-12

### Context

The repo had a local Makefile workflow but no public CI enforcing the same formatter, test, and build checks on pull requests and pushes to `main`.

### Decision

Add a minimal GitHub Actions CI workflow that installs pinned `golines` and `gofumpt` versions, runs `make fmt`, verifies no formatting diff remains, then runs `make test` and `make build`.

### Rationale

- Keeps CI behavior aligned with the contributor-facing root commands instead of inventing a separate workflow path.
- Pins formatter tool versions so formatting does not drift unexpectedly across CI runs.
- Gives the public repo a conventional merge gate before the websocket milestone begins.

## 2026-03-13 (cleanup)

### Context

Post-review cleanup: raw market methods, builder header injection strategy, `CreateOrDeriveAPIKey` error handling, and `SetCredentials` concurrency.

### Decision

1. Delete all raw/compatibility market helpers (`GetOK`, `GetMarkets`, `GetMarket`, etc. returning `*CursorPage` or `json.RawMessage`). Only typed methods remain.
2. Replace `shouldInjectBuilderHeaders` path-switch with `AuthL2Builder` auth level in `polyhttp`. Builder-aware endpoints declare `AuthL2Builder` at the call site instead of having the auth callback infer from path strings.
3. Fix `CreateOrDeriveAPIKey` to only fall back to derive on `*APIError`. Network errors propagate.
4. Protect `SetCredentials` and all `creds` reads with `sync.RWMutex`.

### Rationale

- Raw methods were not backwards compat — they were never the intended final API and no external users exist. Pre-1.0 is the right time to delete them.
- Hardcoded path list in `shouldInjectBuilderHeaders` was a maintenance hazard: every new builder-aware endpoint required a manual update in a different file with no compile-time enforcement.
- `CreateOrDeriveAPIKey` swallowing network errors caused silent fallback on transient failures, not just "key already exists" responses.
- `SetCredentials` is called during bootstrap and could be called again to rotate credentials on a live client; a mutex makes this safe at negligible cost.

---

## 2026-03-13

### Context

The official SDKs expose orderbook hash helpers, but the current Python and Rust implementations diverge: Python hashes a compact JSON payload with SHA1, while Rust hashes serialized orderbooks with SHA256.

### Decision

Add `GetOrderBookHash` in Go using the current Python helper behavior for now, because it is explicitly exposed and fixture-tested in the official Python client.

### Rationale

- Fills a real helper gap without blocking on cross-SDK inconsistency outside this repo.
- Keeps the Go implementation aligned with the official helper that currently documents itself as server-compatible.
- Preserves the divergence as an explicit later parity-audit item instead of silently choosing an arbitrary behavior.

## 2026-03-13: Orderbook Array Traversal
**Context:** The `CalculateMarketPrice` function traverses orderbook bids and asks to determine the execution price for an order of a given size. We needed to ensure the traversal order accurately reflects the API's sorting behavior.
**Decision:** Keep the backward loop (`for i := len(levels) - 1; i >= 0; i--`) and explicitly document the API behavior in the code.
**Rationale:** Verified against the live `clob.polymarket.com` API that Bids are returned sorted ascending and Asks are sorted descending. In both cases, the "top of the book" (best price) is at the end of the array, so iterating backwards always starts at the most competitive price correctly.

## 2026-03-13: Websocket Package Boundary

### Context

Phase 3 needed a decision on whether websocket support should live in a public sibling `ws/` package or under `clob/ws/`. The repo README originally reserved sibling packages for distinct Polymarket API families, while the local roadmap left websocket placement open. We also wanted to balance Go ergonomics with parity against the official SDKs, using TypeScript as the primary reference and Rust as the secondary reference.

### Decision

Expose websocket streaming as `clob/ws/`. If websocket connection management becomes reusable, keep that reuse private under `internal/polyws/` until another public package actually needs it.

### Rationale

- Polymarket documents the market and user websocket channels as part of the CLOB market-data surface, not as a standalone product family.
- The official TypeScript CLOB SDK currently has no websocket package at all, so there is no primary-reference pressure toward a top-level public `ws/` import path.
- The official Rust SDK exposes websocket usage as `polymarket_client_sdk::clob::ws::Client` while separately keeping lower-level websocket infrastructure in a generic `ws` module. That split maps well to Go as a public `clob/ws/` package backed by private internals.
- A top-level public `ws/` package would either leak CLOB-specific message types into what sounds like a cross-family transport package, or force premature public abstractions before there is a second consumer.

### Tradeoffs

- This narrows the first public websocket surface to CLOB-specific streams instead of betting early on cross-family reuse.
- If future non-CLOB websocket APIs appear, we may later add a separate public package for them, but that can be done from proven needs instead of speculation.

## 2026-03-13: Websocket First Slice Protocol Pin

### Context

Before implementing `clob/ws/`, we needed to reconcile the earlier Claude-session notes with the current upstream sources. The prior notes said the websocket heartbeat used `PING` every 50 seconds, but the current Polymarket docs and the local Rust SDK no longer match that exact detail. We also needed to decide which message families to ship first instead of trying to cover the entire websocket surface in one pass.

### Decision

Pin the first Go websocket slice to the current public market channel contract:

- base websocket host: `wss://ws-subscriptions-clob.polymarket.com`
- market channel: `/ws/market`
- user channel: `/ws/user`
- text heartbeat messages: send `PING`, expect `PONG`
- market subscribe payloads use `type`, `assets_ids`, optional `initial_dump`, and optional `custom_feature_enabled`
- first implemented market event set: `book`, `price_change`, `tick_size_change`, `last_trade_price`

Treat the current docs as the source of truth for heartbeat cadence for now; they currently say to send `PING` every 10 seconds.

### Rationale

- The official TypeScript CLOB SDK still has no websocket client, so the current docs and Rust SDK are the best available references.
- The docs and Rust SDK agree on the channel layout, payload shape, and text `PING`/`PONG` heartbeat model.
- Shipping the four core market events first captures the highest-value public streaming surface without blocking on authenticated user events or custom-feature market events.

### Tradeoffs

- Rust currently defaults to a tighter heartbeat interval than the docs describe, so the first Go implementation may intentionally prefer the documented cadence over Rust's default transport tuning.
- Custom-feature market events (`best_bid_ask`, `new_market`, `market_resolved`) and authenticated user events are deferred to follow-up tasks instead of being bundled into the first market-stream implementation.
## 2026-03-15: Modernized WebSocket Stack

### Context

The original plan for `clob/ws/` implementation contemplated `gorilla/websocket` or a manual transport. However, `gorilla` is unmaintained, and modern high-frequency SDKs benefit from context-native APIs and lower allocation overhead.

### Decision

Use `github.com/coder/websocket` (the maintained fork of `nhooyr/websocket`) for all CLOB WebSocket implementations.

### Rationale

- Native `context.Context` support eliminates complex custom lifecycle management and goroutine leaks.
- Zero-allocation reads and faster masking are ideal for the high-volume market feeds expected from the Polymarket CLOB.
- The API is more idiomatic to modern Go (2025/2026) than `gorilla`'s older event model.

### Tradeoffs

- Minimal learning curve for those only familiar with `gorilla`, but heavily offset by safer/cleaner code.

---

## 2026-03-15: High-Performance Financial Decimals

### Context

The Go SDK initially used `github.com/shopspring/decimal`. While popular, it is known for panic-on-error paths and high heap allocations during frequent arithmetic (like orderbook price derivation).

### Decision

Add `github.com/quagmt/udecimal` as a core dependency for high-performance financial arithmetic, and plan a migration of `clob/trading.go` to this new standard.

### Rationale

- `udecimal` is explicitly designed for transactional financial systems (SOTA 2024-2026).
- Zero heap allocations for arithmetic minimize GC pressure in high-update market price calculations.
- Panic-free design (returns errors instead of crashing) improves SDK reliability for automated trading bots.

### Tradeoffs

- Requires a migration of existing `trading.go` logic, which will be handled as a dedicated task (`goclobclient-bj2q`).

---

## 2026-03-16: Shared RFQ Wire Struct for MarshalJSON

### Context

`AcceptRFQQuoteRequest` and `ApproveRFQOrderRequest` both marshal an RFQ order to a camelCase wire format with Numeric salt/expiration. The two `MarshalJSON` methods duplicated the same `wireRFQOrder` struct definition and field-mapping logic.

### Decision

Extract a shared `wireRFQOrder` struct and `marshalRFQOrder()` helper in `clob/rfq_types.go`. Both request types delegate their `MarshalJSON` to this helper, passing a config flag for the wire field name (`trade` vs `order`).

### Rationale

- Eliminates 60+ lines of duplicated wire-format mapping.
- Single point of change when the RFQ wire format evolves.
- Keeps the public API unchanged — only internal implementation is refactored.

### Tradeoffs

- None — purely internal refactor with no public surface change.
---

## 2026-03-16: Modernize to Go 1.26 Idioms

### Context

As part of keeping the SDK current with the latest Go toolchain features, we transitioned to `encoding/json/v2` and `encoding/json/jsontext`.

### Decision

1. Migrated all JSON handling to `encoding/json/v2`.
2. Replaced `omitempty` with `omitzero` in all public and internal structs.
3. Replaced `json.RawMessage` with `jsontext.Value` or `any` where appropriate.
4. Adopted `errors.AsType` for more idiomatic error checking.
5. Enabled `GOEXPERIMENT=jsonv2` in the project `Makefile`.

### Rationale

- `json/v2` provides better performance and safer defaults (e.g., no HTML escaping by default).
- `omitzero` is more precise than `omitempty` as it specifically targets the Go zero value.
- `jsontext.Value` allows for zero-copy manipulation of JSON segments, ideal for a client library that often relays portions of responses.
- `errors.AsType` provides a cleaner, type-safe way to handle specific error types without pointer dance.

### Tradeoffs

- Requires `GOEXPERIMENT=jsonv2` build flag until the package is stabilized in the standard library.
## 2026-03-16: Bridge and CTF APIs Phase 5

### Context

The transition plan called for Phase 5 to implement remaining parity gaps for Bridge and CTF operations. These APIs were missing from the Go SDK but present in the TypeScript, Python, and Rust reference clients.

### Decision

1. Implemented a dedicated `bridge` package at the root (similar to `gamma` and `data`).
2. Added CTF operations (`SplitTokens`, `MergeTokens`, `RedeemTokens`) directly to the `clob.Client`.
3. Added CTF endpoint constants to `clob/endpoints.go`.
4. Refined `gamma` and `data` packages to use `omitzero` tags for full `json/v2` parity.

### Rationale

- Bridge API has a distinct host (`bridge.polymarket.com`) and distinct domain models, justifying a separate public package.
- CTF operations use the same CLOB host and L2 authentication (Builder API keys) as the core trading surface, so adding them to `clob.Client` provides higher ergonomics for traders without requiring a new client setup.
- These additions bring the Go SDK to feature parity with the v5.8.0 TypeScript reference SDK (primary reference).

### Tradeoffs

- CTF endpoints in `clob.Client` slightly inflate the core package, but the operational coupling with L2 auth makes this a net positive for developer experience.

---

## 2026-03-16: Tiered Client Architecture (Type-Level Auth Guards)

### Context

March 2026 standards (verified against official Rust SDK) emphasize "structural safety" for trading SDKs. A monolithic client allows accidental calls to trading methods without credentials, resulting in avoidable runtime errors.

### Decision

Refactor `clob.Client` into three tiered types:
1. `Client`: Public methods only.
2. `SignerClient`: Embeds `Client`, adds L1 methods (order signing, L1 auth).
3. `AuthenticatedClient`: Embeds `SignerClient`, adds L2/L3 methods (post order, account ops).

`clob.New()` now returns `any`, requiring the user to assert the returned value to the tier matching their provided configuration.

### Rationale

- Prevents runtime "missing credentials" errors by making unauthenticated calls to trading methods impossible at compile-time (or type-assertion time).
- Mirrors the "Type-Level Auth Guards" found in the high-performance Rust 2026 SDK.
- Provides a clear "upgrade" path for clients (e.g., `Client.AsSigner()`).

### Tradeoffs

- Breaking change for users of previous SDK versions (monolithic client).
- Requires type assertions when calling `New()`.

---

## 2026-03-16: Modern Iterators for Pagination

### Context

Go 1.23+ introduced `iter.Seq` and `iter.Seq2`. Conventional SDK pagination (slice returns or manual cursor handling) is either memory-heavy or verbose for users.

### Decision

Add `Iter*` methods (e.g., `IterOpenOrders`, `IterTrades`, `IterMarkets`) to all paginated list endpoints using `iter.Seq2[T, error]`.

### Rationale

- Allows users to process extremely large datasets using memory-efficient range-over-function loops.
- Simplifies user code: `for item, err := range client.IterSomething(...)` instead of manual cursor management.
- Modernizes the SDK to current Go standards (2026 target).

### Tradeoffs

- Minor internal complexity to implement the yield blocks.
