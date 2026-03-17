## Phases

| Phase | Scope                                              | Status |
| ----- | -------------------------------------------------- | ------ |
| 1     | Core CLOB HTTP client and auth                     | done   |
| 2     | Order building, signing, and typed trading surface | done   |
| 3     | Websocket and streaming support                    | done   |
| 4     | RFQ and remaining CLOB surface                     | done   |
| 5     | Non-CLOB Polymarket APIs                           | done   |
| 6     | 2026 Standard & Tiered Architecture                | done   |
| 7     | Stabilization and v1.0 Release                     | ready  |

## Reference Priority

1. **Rust** (`polymarket-rust-sdk`) — primary reference for 2026 performance and safety standards
2. **TypeScript** (`@polymarket/clob-client`) — primary reference for wire-format and hash parity
3. **Python** (`py-clob-client`) — secondary reference

## Completed

- **Tiered Client Architecture**: structural safety via `Client`, `SignerClient`, and `AuthenticatedClient`.
- **2026 High-Performance Standards**: automated heartbeats, increased batch limits, rebate support.
- **Go 1.26 Modernization**: full migration to `jsonv2`, Go 1.23+ iterators, and `errors.AsType`.
- **Full Parity**: CLOB (REST + WS), RFQ, Gamma, Data, Bridge, and CTF.
- Orderbook hash (SHA-1) and CalculateMarketPrice verified.
- GitHub Actions CI ensuring modern Go standards.

## Out Of Scope Until Explicitly Planned

- Release automation or version publishing.
- A public top-level `ws/` package (staying under `clob/ws/`).
- Higher-level trade execution bots (SDK provides primitives only).
