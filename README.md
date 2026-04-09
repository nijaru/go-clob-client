# go-clob-client

[![Go Reference](https://pkg.go.dev/badge/github.com/nijaru/go-clob-client/clob.svg)](https://pkg.go.dev/github.com/nijaru/go-clob-client/clob)
[![CI](https://github.com/nijaru/go-clob-client/actions/workflows/ci.yml/badge.svg)](https://github.com/nijaru/go-clob-client/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nijaru/go-clob-client)](https://goreportcard.com/report/github.com/nijaru/go-clob-client)

> [!WARNING]
> Unofficial, community-maintained SDK. Not extensively tested in production trading environments. Use at your own risk.

Go SDK for the [Polymarket](https://polymarket.com) CLOB and Data APIs. Targets the latest stable Go release and tracks feature parity with the [official Rust SDK](https://github.com/Polymarket/rs-clob-client).

## Features

- **CLOB**: full REST + WebSocket support for markets, orders, and user events.
- **Data API**: read-only positions, trades, activity, holders, live volume, and leaderboards.
- **Heartbeats**: automated background heartbeats with ID rotation to keep orders live.
- **Batch orders**: post and cancel up to 15 orders per request.
- **RFQ**: submit and query request-for-quote flows.
- **Gamma**: market discovery, search, tags, and event metadata.
- **Bridge**: cross-chain deposit addresses (EVM, Solana, Bitcoin).
- **CTF**: on-chain split, merge, and redeem operations for conditional tokens.
- **Builder auth**: dual L2/builder header flows for institutional integrations.

## Install

Requires **Go 1.26.1+**.

```bash
go get github.com/nijaru/go-clob-client@latest
```

Import the package you need: `clob` for trading and CLOB APIs, `data` for read-only Data API access.

This repo exposes multiple public packages within one module. The most common entrypoints are `github.com/nijaru/go-clob-client/clob` and `github.com/nijaru/go-clob-client/data`.

## Choose Your Path

Use `clob` if you need:

- orderbooks, prices, and market data
- signed order creation and submission
- account management, heartbeats, or websockets

Use `data` if you need:

- read-only analytics and reporting
- positions, trades, activity, holders, and leaderboards
- no signing or trading flows

If you are integrating trading, start with `clob.NewClient` for read-only checks, then move to `NewSignerClient` or `NewAuthenticatedClient` once wallet and API credentials are configured.

## Quickstart

### CLOB Read-Only

```go
import "github.com/nijaru/go-clob-client/clob"

ctx := context.Background()

client, err := clob.NewClient(clob.Config{})
if err != nil {
	log.Fatal(err)
}

book, err := client.GetOrderBook(ctx, "<token-id>")
if err != nil {
	log.Fatal(err)
}

fmt.Printf("Best bid: %s\n", book.Bids[len(book.Bids)-1].Price)
```

### Authenticated Trading

`NewAuthenticatedClient` automatically starts a background **heartbeat loop** to maintain order liveness and priority. Always call `Close` or `Shutdown` when done.

```go
client, err := clob.NewAuthenticatedClient(clob.Config{
	PrivateKey: os.Getenv("POLYMARKET_PRIVATE_KEY"),
	Credentials: &clob.Credentials{
		Key:        os.Getenv("POLYMARKET_API_KEY"),
		Secret:     os.Getenv("POLYMARKET_API_SECRET"),
		Passphrase: os.Getenv("POLYMARKET_API_PASSPHRASE"),
	},
})
if err != nil {
	log.Fatal(err)
}

defer client.Close()

ctx := context.Background()
resp, err := client.CreateAndPostOrder(ctx, clob.OrderArgs{
	TokenID: os.Getenv("POLYMARKET_TOKEN_ID"),
	Price:   udecimal.MustParse("0.45"),
	Size:    udecimal.MustParse("5"),
	Side:    clob.SideBuy,
}, nil, clob.OrderTypeGTC, false, false)
```

### Market Orders

Market orders require `AmountKind` to specify how the `Amount` field is interpreted. Sell orders always use `AmountShares`; buy orders accept either `AmountShares` or `AmountUSDC` (default).

```go
// Sell 25 shares at market price (FOK)
resp, err := client.CreateAndPostMarketOrder(ctx, clob.MarketOrderArgs{
    TokenID:    os.Getenv("POLYMARKET_TOKEN_ID"),
    Amount:     udecimal.MustParse("25"),
    AmountKind: clob.AmountShares,  // required for sell; also valid for buy
    Side:       clob.SideSell,
}, nil, clob.OrderTypeFOK, false)

// Buy $10 of shares at market price (FOK)
resp, err := client.CreateAndPostMarketOrder(ctx, clob.MarketOrderArgs{
    TokenID:    os.Getenv("POLYMARKET_TOKEN_ID"),
    Amount:     udecimal.MustParse("10"),
    AmountKind: clob.AmountUSDC,    // spend $10 USDC
    Side:       clob.SideBuy,
}, nil, clob.OrderTypeFOK, false)
```

`CalculateMarketPrice` accepts the same `AmountKind` parameter and will return an error if `AmountUSDC` is used for a sell.

### Pagination Iterators

All list endpoints expose both a slice variant and a Go 1.26 range-over-function iterator for memory-efficient streaming:

```go
for order, err := range client.IterOpenOrders(ctx, clob.OpenOrderParams{}) {
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Order ID: %s\n", order.ID)
}
```

### Data API

```go
import "github.com/nijaru/go-clob-client/data"

ctx := context.Background()
client := data.New(data.Config{})

positions, err := client.GetPositions(ctx, data.PositionParams{
	User: "0x1234...",
})
if err != nil {
	log.Fatal(err)
}

for _, pos := range positions {
	fmt.Printf("%s: %s shares\n", pos.Title, pos.Size)
}
```

## Before Live Trading

Read the wallet and allowance notes below before sending real orders. In particular:

- choose the correct `SignatureType` for your wallet path
- set `FunderAddress` when using proxy or delegated wallets
- confirm token allowances if you are trading from an EOA wallet
- always close authenticated clients cleanly so heartbeat state shuts down cleanly

## Client Tiers

The SDK enforces authentication requirements through a three-tier hierarchy — you cannot call trading methods on an unauthenticated client.

| Tier                  | Constructor              | Auth Required           | Capabilities                                  |
| --------------------- | ------------------------ | ----------------------- | --------------------------------------------- |
| `Client`              | `NewClient`              | None                    | Public market data, orderbooks, prices        |
| `SignerClient`        | `NewSignerClient`        | Private key             | Order building & signing, API key management  |
| `AuthenticatedClient` | `NewAuthenticatedClient` | Private key + API creds | Order posting, account management, heartbeats |

You can also upgrade incrementally:

```go
base, _ := clob.NewClient(clob.Config{})
signer, _ := base.AsSigner(privateKey, clob.SignatureTypeEOA, "")
authed := signer.AsAuthenticated(creds, nil)
```

## Wallet Types

### Signature Types

The `SignatureType` field tells Polymarket how to verify your signatures:

| Value | Constant                      | Wallet type                                                                        |
| ----- | ----------------------------- | ---------------------------------------------------------------------------------- |
| `0`   | `SignatureTypeEOA`            | MetaMask, hardware wallets — any wallet where you control the private key directly |
| `1`   | `SignatureTypePolyProxy`      | Email / Magic wallet (delegated signing)                                           |
| `2`   | `SignatureTypePolyGnosisSafe` | Browser proxy wallet (proxy contract)                                              |

```go
client, err := clob.NewSignerClient(clob.Config{
	PrivateKey:    os.Getenv("POLYMARKET_PRIVATE_KEY"),
	SignatureType: clob.SignatureTypePolyProxy,
	FunderAddress: "<your-polymarket-wallet-address>",
})
```

### Funder Address

The **funder address** is the address that actually holds your funds on Polymarket. For EOA wallets it equals your signing key's address, so you can omit it. For proxy/Magic wallets the signing key differs from the on-chain funder — set `FunderAddress` explicitly.

### Token Allowances

> **MetaMask and EOA users only.** Proxy/Magic wallet users have allowances pre-approved.

Before Polymarket can execute trades you must grant the exchange contracts permission to move your tokens. You need to approve two token types, each for three exchange contracts:

**USDC** (`0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174`):

- `0x4bFb41d5B3570DeFd03C39a9A4D8dE6Bd8B8982E` — main exchange
- `0xC5d563A36AE78145C45a50134d48A1215220f80a` — neg-risk exchange
- `0xd91E80cF2E7be2e162c6513ceD06f1dD0dA35296` — neg-risk adapter

**Conditional Tokens** (`0x4D97DCd97eC945f40cF65F87097ACe5EA0476045`): same three contracts as above.

**You only need to do this once per wallet.** See `examples/clob/ctf_operations` for a Go implementation.

## Examples

Runnable examples are in `examples/`:

| Example        | Path                           | What it shows                                |
| -------------- | ------------------------------ | -------------------------------------------- |
| Read-only      | `examples/clob/read_only`      | Orderbook, prices, market data               |
| Auth bootstrap | `examples/clob/auth_bootstrap` | Creating and deriving API keys               |
| Bridge         | `examples/bridge`              | Supported assets and deposit-address flows   |
| Data API       | `examples/data`                | Positions and read-only data endpoints       |
| Limit order    | `examples/clob/limit_order`    | Placing a GTC limit order                    |
| Market order   | `examples/clob/market_order`   | Placing a FOK market order                   |
| Gamma          | `examples/gamma`               | Search, events, and discovery metadata       |
| WebSocket      | `examples/ws`                  | Real-time orderbook and user event streaming |
| CTF operations | `examples/clob/ctf_operations` | Splitting, merging, and redeeming shares     |

Run any example with:

```bash
export POLYMARKET_PRIVATE_KEY=0x...
go run ./examples/clob/read_only
```

Copy `.env.example` to `.env` for a full set of required variables.

## API Notes

### RFQ

The RFQ flow uses three methods:

| Method                      | Returns                             | Notes                                                |
| --------------------------- | ----------------------------------- | ---------------------------------------------------- |
| `GetRFQQuotes(ctx, params)` | `(*RFQQuotesResponse, error)`       | Lists quotes for one or more request IDs             |
| `AcceptRFQQuote(ctx, req)`  | `error`                             | Requester accepts a quote; server returns plain OK   |
| `ApproveRFQOrder(ctx, req)` | `(*ApproveRFQOrderResponse, error)` | Quoter approves the matched order; returns trade IDs |

### Market Price

`GetMarketTradePriceHistory` returns `MarketPrice` values where `.P` is `udecimal.Decimal` (not `float64`), preserving the full precision of the API response.

## Error Handling

API errors are returned as `*clob.APIError` and expose the HTTP status code and body. Use the package-level sentinel errors with `errors.Is` for common cases:

```go
if errors.Is(err, clob.ErrNotFound) {
	// 404
}

if errors.Is(err, clob.ErrRateLimit) {
	// 429
}

if errors.Is(err, clob.ErrGeoBlocked) {
	// 451
}

if errors.Is(err, clob.ErrUnauthorized) {
	// 401/403
}
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Primary reference for API parity is the [Rust SDK](https://github.com/Polymarket/rs-clob-client).

## Security

See [SECURITY.md](SECURITY.md).

## About Polymarket

[Polymarket](https://polymarket.com) is the world's largest prediction market, where you trade on the probability of real-world events. Markets reflect accurate, unbiased, real-time probabilities derived from open trading. See [docs.polymarket.com](https://docs.polymarket.com) for the full API reference.
