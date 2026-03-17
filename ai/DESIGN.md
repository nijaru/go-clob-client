## System Overview

Public Go SDK for Polymarket with API-family packages. `clob/` is the core package; additional families sit beside it.

## Tiered Client Architecture (Type-Level Auth Guards)

The SDK implements strict structural safety patterns to prevent unauthenticated access to trading endpoints.

| Type                  | Auth Requirements | Capability                                                                 |
| --------------------- | ----------------- | -------------------------------------------------------------------------- |
| `clob.Client`         | None              | Public data (orderbooks, markets, prices, server time)                     |
| `clob.SignerClient`   | Private Key (L1)  | L1 operations (Order signing, API key bootstrap)                           |
| `clob.AuthenticatedClient` | API Creds (L2/L3) | L2/L3 operations (Post order, cancel, account data, heartbeats, CTF, RFQ) |

### State Transitions
- `clob.New()`: Returns `any` (assert to specific tier based on input config).
- `Client.AsSigner()`: Upgrades a public client to an L1 client.
- `SignerClient.AsAuthenticated()`: Upgrades an L1 client to a fully authenticated trading client.

## 2026 High-Performance Features

- **Automated Heartbeats**: `AuthenticatedClient` runs a background goroutine that polls `/v1/heartbeats` every 5s (default). It automatically rotates `heartbeat_id` to maintain order priority and prevent auto-cancellation on disconnect.
- **Modern Iterators**: All paginated list methods expose `Iter*` methods using Go 1.23+ `iter.Seq2`. This allows memory-efficient range-over-function processing.
- **jsonv2 Migration**: Built for Go 1.26 `encoding/json/v2`, utilizing `omitzero` tags and the faster experimental encoder.

## Layered Design

| Layer                      | Purpose                                                                                                     |
| -------------------------- | ----------------------------------------------------------------------------------------------------------- |
| `clob/`                    | Core trading client and CTF operations                                                                      |
| `clob/ws/`                 | Public CLOB websocket client and typed stream messages                                                      |
| `gamma/`                   | Public Gamma API client for market discovery and metadata                                                   |
| `data/`                    | Public Data API client for user positions, trades, and leaderboards                                         |
| `bridge/`                  | Jan 2026 Bridge API implementation (/withdraw, /status)                                                     |
| `internal/polyauth/`       | Shared Polymarket signing logic (EIP-712, HMAC)                                                             |
| `internal/polyhttp/`       | Modern Go 1.26 HTTP transport with `jsonv2` and structured error parsing                                    |

## Components

| Component         | Purpose                                        | Status |
| ----------------- | ---------------------------------------------- | ------ |
| `clob.Client`     | Tiered client system                           | active |
| `polyauth.Signer` | Ethereum signing and HMAC generation           | active |
| `polyhttp.Client` | Type-safe JSON request execution               | active |
| `clob/ws`         | Webhook streaming with auth derivation         | active |

## Data Flow

1. User creates a client via `clob.New()`.
2. To trade, user must have a `*clob.AuthenticatedClient`.
3. `internal/polyhttp` executes requests, calling `addAuthHeaders` on the tiered client.
4. `addAuthHeaders` dynamically selects between L1 (EIP-712) or L2 (HMAC) signing based on the active client struct.
5. Paginated methods utilize `iter.Seq2` for efficient streaming of results.
6. The background Heartbeat loop starts automatically on `AuthenticatedClient` initialization and stops on `Close()`.
