# go-clob-client

> [!WARNING]
> **Experimental SDK**: This implementation of the March 2026 Polymarket Technical Standards (including Type-Level Auth Guards and automated Heartbeats) is feature-complete but has not been extensively tested in high-volume live trading environments. Use at your own risk. Errors in financial software can lead to irreversible loss of funds.

Go SDK for the Polymarket CLOB.

- **Full 2026 Parity**: Implements Heartbeats API, 15-order batch limits, and the new Bridge withdrawal endpoints.
- **Type-Level Auth Guards**: Strict client tiers (`Client`, `SignerClient`, `AuthenticatedClient`) prevent accidental unauthenticated calls to trading endpoints.
- **Modern Go 1.26**: Built for Go 1.26 with `json/v2` (external package), iterators for pagination, and type-safe error handling.
- **Comprehensive Coverage**: CLOB (REST + WS), RFQ, Gamma (Metadata), Data (User Stats), Bridge (Cross-chain), and CTF (On-chain operations).

## Install

```bash
go get github.com/nijaru/go-clob-client/clob
```

Import path:

```go
import "github.com/nijaru/go-clob-client/clob"
```

## Architecture: Type-Level Auth Guards

The SDK uses a three-tiered client structure to ensure safety. You cannot call trading methods on a base client; you must "upgrade" it by providing the necessary credentials.

1. **`clob.Client`**: Public methods only (`GetOrderBook`, `GetMarkets`).
2. **`clob.SignerClient`**: Extends base with L1 methods (`CreateOrder`, `CreateAPIKey`). Requires `PrivateKey`.
3. **`clob.AuthenticatedClient`**: Extends signer with L2/L3 methods (`PostOrder`, `PostHeartbeat`, `SplitTokens`). Requires `API Credentials`.

## Quickstart

### Read-Only Access

```go
clientRaw, _ := clob.New(clob.Config{})
client := clientRaw.(*clob.Client)

book, _ := client.GetOrderBook(ctx, "<token-id>")
fmt.Printf("Best bid: %s\n", book.Bids[len(book.Bids)-1].Price)
```

### Full Authenticated Trading

Providing both a `PrivateKey` and `Credentials` to `New()` returns an `*AuthenticatedClient` which automatically starts the background **Heartbeat loop** to maintain order liveness and priority.

```go
clientRaw, _ := clob.New(clob.Config{
    PrivateKey:  os.Getenv("PK"),
    Credentials: &clob.Credentials{...},
})
client := clientRaw.(*clob.AuthenticatedClient)

// Automatic heartbeats are now running in the background.
// Batch up to 15 orders (2026 standard)
resp, _ := client.PostOrders(ctx, orders)
```

### Modern Pagination (Iterators)

All list methods support Go 1.23+ iterators for clean, memory-efficient processing:

```go
for order, err := range client.IterOpenOrders(ctx, clob.OpenOrderParams{}) {
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Order ID: %s\n", order.ID)
}
```

## Features

- **CLOB**: Full REST + WebSocket (Market & User channels) support.
- **2026 Standards**: 
    - Automated **Heartbeats** with ID rotation.
    - **Batch Order Limits** increased to 15.
    - **Post-Only** validation for GTC/GTD orders.
    - **Maker Rebates** (`rebate_estimated`) in responses.
- **Bridge**: Updated Jan 2026 API for `/withdraw` and `/status`.
- **CTF**: On-chain `Split`, `Merge`, and `Redeem` operations.
- **Gamma/Data**: Full market discovery and user analytics.

## Examples

Explore the `examples/` directory for runnable implementations:

- **Read-Only**: `examples/clob/read_only` — Public orderbook and market data.
- **Auth Bootstrap**: `examples/clob/auth_bootstrap` — Creating and deriving API keys.
- **Trading**: `examples/clob/limit_order` — Placing limit orders with tiered clients.
- **WebSockets**: `examples/ws` — Real-time orderbook and user event streaming.
- **On-chain Ops**: `examples/clob/ctf_operations` — Splitting, merging, and redeeming shares.

Requires Go 1.26.1+.
