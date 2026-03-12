# go-clob-client

Go SDK for the Polymarket CLOB, using the official TypeScript, Python, and Rust clients as the initial reference set.

## Layout

This repo is organized for broader long-term Polymarket coverage:

- `clob/` is the first public package
- `internal/polyauth/` holds shared Polymarket signing and auth-header logic
- `internal/polyhttp/` holds shared HTTP transport and response handling
- future families like `gamma/`, `data/`, `ws/`, `bridge/`, and `ctf/` can be added alongside `clob/`

## Status

This repo now has a first usable SDK slice:

- Read-only market and orderbook endpoints
- Polymarket L1 auth headers for API key creation/derivation
- Polymarket L2 auth headers for authenticated REST calls
- Raw signed-order submission and cancellation helpers
- Tests and a runnable read-only example

Order construction/signing, RFQ, websocket streaming, and the broader Polymarket API surface are planned next.

## Install

```bash
go get github.com/nijaru/go-clob-client/clob
```

## Quickstart

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/nijaru/go-clob-client/clob"
)

func main() {
    client, err := clob.New(clob.Config{})
    if err != nil {
        log.Fatal(err)
    }

    serverTime, err := client.GetServerTime(context.Background())
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(serverTime)
}
```

## Authenticated setup

```go
client, err := clob.New(clob.Config{
    ChainID:    clob.PolygonChainID,
    PrivateKey: os.Getenv("POLYMARKET_PRIVATE_KEY"),
})
if err != nil {
    log.Fatal(err)
}

creds, err := client.CreateOrDeriveAPIKey(context.Background(), 0)
if err != nil {
    log.Fatal(err)
}
client.SetCredentials(*creds)
```

## Current API surface

Read-only:

- `GetOK`
- `GetServerTime`
- `GetSamplingSimplifiedMarkets`
- `GetSamplingMarkets`
- `GetSimplifiedMarkets`
- `GetMarkets`
- `GetMarket`
- `GetOrderBook`
- `GetOrderBooks`
- `GetMidpoint`
- `GetMidpoints`
- `GetPrice`
- `GetPrices`
- `GetSpread`
- `GetSpreads`
- `GetLastTradePrice`
- `GetLastTradesPrices`
- `GetTickSize`
- `GetNegRisk`
- `GetFeeRate`

Authenticated:

- `CreateAPIKey`
- `DeriveAPIKey`
- `CreateOrDeriveAPIKey`
- `GetAPIKeys`
- `DeleteAPIKey`
- `GetClosedOnly`
- `GetOpenOrders`
- `GetOrder`
- `GetTrades`
- `PostOrder`
- `PostOrders`
- `CancelOrder`
- `CancelOrders`
- `CancelAll`

## Development

```bash
go test ./...
go build ./...
```
