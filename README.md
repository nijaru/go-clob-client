# go-clob-client

[![Go Reference](https://pkg.go.dev/badge/github.com/nijaru/go-clob-client/clob.svg)](https://pkg.go.dev/github.com/nijaru/go-clob-client/clob)
[![CI](https://github.com/nijaru/go-clob-client/actions/workflows/ci.yml/badge.svg)](https://github.com/nijaru/go-clob-client/actions/workflows/ci.yml)

Go SDK for [Polymarket](https://polymarket.com): trading, market discovery,
portfolios, perpetuals and live feeds. Polymarket US is not supported.

> [!WARNING]
> This is an unofficial, pre-1.0 SDK. APIs may change. Trading and wallet operations
> are tested with fixtures, but have not been validated with production accounts.

## Installation

Requires **Go 1.27+**.

```sh
go get github.com/nijaru/go-clob-client@latest
```

## Quickstart

Read a page of markets without credentials. This example sets the public CLOB
endpoint explicitly. Save it as `main.go` in your Go module and run `go run .`:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/nijaru/go-clob-client/clob"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    client, err := clob.NewClient(clob.Config{
        Host: "https://clob.polymarket.com",
    })
    if err != nil {
        log.Fatal(err)
    }
    page, err := client.GetMarketsPage(ctx, "")
    if err != nil {
        log.Fatal(err)
    }
    for _, market := range page.Data {
        fmt.Println(market.Question)
    }
}
```

Use `GetOrderBook` for a token's bids and asks, or `IterMarkets` to read markets
across pages. See the [CLOB API reference](https://pkg.go.dev/github.com/nijaru/go-clob-client/clob).

## Packages

Each API has its own client. Import only the packages you need.

| Package | Use it for |
| --- | --- |
| [`clob`](https://pkg.go.dev/github.com/nijaru/go-clob-client/clob) | Order books, trading, accounts, rewards, rebates, RFQ and wallet operations |
| [`gamma`](https://pkg.go.dev/github.com/nijaru/go-clob-client/gamma) | Market and event discovery, tags, comments and search |
| [`data`](https://pkg.go.dev/github.com/nijaru/go-clob-client/data) | Portfolios, activity, token references, analytics and leaderboards |
| [`data/legacy`](https://pkg.go.dev/github.com/nijaru/go-clob-client/data/legacy) | Data API v1; separate from the v2 client |
| [`bridge`](https://pkg.go.dev/github.com/nijaru/go-clob-client/bridge) | Supported assets, cross-chain quotes and transfer status |
| [`perps`](perps/README.md) | Perpetuals markets, accounts, trading and collateral |
| [`clob/ws`](https://pkg.go.dev/github.com/nijaru/go-clob-client/clob/ws) | CLOB market and account streams |
| [`clob/ws/rtds`](https://pkg.go.dev/github.com/nijaru/go-clob-client/clob/ws/rtds) | Legacy RTDS feeds |
| [`realtime`](https://pkg.go.dev/github.com/nijaru/go-clob-client/realtime) | Authenticated Polybolt crypto/equity prices and 60-second TWAP streams |
| [`sports`](sports/README.md) | Public live game results |
| [`signing`](https://pkg.go.dev/github.com/nijaru/go-clob-client/signing) | Local and external Ethereum signers |

## Authentication and trading

CLOB clients support three levels of access:

- `NewClient`: public reads, no credentials.
- `NewSignerClient`: order signing, API-key setup and wallet operations.
- `NewAuthenticatedClient`: posting orders and accessing account data; requires
  both a signer and CLOB API credentials.

Use `Config.PrivateKey` for a local key or `Config.Signer` for an external signer,
not both. See [API-key setup](examples/clob/auth_bootstrap/main.go) and
[external signing](examples/clob/external_signer/main.go).

The function below submits a limit order for a direct EOA wallet. Set the four
`POLYMARKET_*` credentials in your environment and pass the outcome token ID.
**Calling it places a real order.** The wallet must already have funds and the
required approvals.

```go
import (
    "context"
    "os"

    "github.com/nijaru/go-clob-client/clob"
    "github.com/quagmt/udecimal"
)

func placeLimitOrder(ctx context.Context, tokenID string) (*clob.PostOrderResponse, error) {
    client, err := clob.NewAuthenticatedClient(clob.Config{
        Host:       "https://clob.polymarket.com",
        ChainID:    clob.PolygonChainID,
        PrivateKey: os.Getenv("POLYMARKET_PRIVATE_KEY"),
        Credentials: &clob.Credentials{
            Key:        os.Getenv("POLYMARKET_API_KEY"),
            Secret:     os.Getenv("POLYMARKET_API_SECRET"),
            Passphrase: os.Getenv("POLYMARKET_API_PASSPHRASE"),
        },
    })
    if err != nil {
        return nil, err
    }
    defer client.Close()

    return client.CreateAndPostOrder(ctx, clob.OrderArgs{
        TokenID: tokenID,
        Price:   udecimal.MustParse("0.45"),
        Size:    udecimal.MustParse("5"),
        Side:    clob.SideBuy,
    }, nil, clob.OrderTypeGTC, false)
}
```

For proxy, Safe or deposit wallets, configure `SignatureType` and `FunderAddress`
for your account; don't use the EOA defaults. Orders take either `TokenID` for a
CTF outcome or `PositionID` for a native V2 position, not both.

For market orders, use `MaxPrice` to cap a BUY price or `MinPrice` to floor a SELL
price. `Amount` is USDC notional for BUY and shares for SELL; `MaxSpend` caps BUY
spending including fees. See the [market-order example](examples/clob/market_order/main.go).

### Trading safety

- Client construction does not submit orders, start heartbeats, deploy wallets or
  grant approvals.
- Set context deadlines. A canceled or failed request may still have reached the
  server; check the order or transaction status before retrying.
- An accepted order is not a confirmed fill. Use `WaitForOrderFillSettlement` when
  you need settlement confirmation.
- Heartbeats are opt-in through `StartHeartbeats(ctx)` and affect all open orders
  under the API key. Only enable them if you can keep sending. `StopHeartbeats`,
  `Close` and `Shutdown` stop the loop; they do not explicitly cancel orders or
  revoke credentials. Cancel and verify orders as part of shutdown.
- EOA transaction batches are **not atomic**: some transactions may succeed before
  a later one fails. Retain any returned handle, even alongside an error, and
  check its submissions before retrying. See the
  [wallet example](examples/clob/wallet_operations/main.go).

Builder and relayer credentials are separate from CLOB credentials. See the
[relayer example](examples/clob/gasless/main.go) and
[wallet discovery example](examples/clob/wallet_discovery/main.go) before deploying
or funding a smart wallet. Perpetuals use [separate credentials and signing](perps/README.md).

## Examples

From a repository checkout, run a local stream demo without credentials or a
remote connection:

```sh
go run ./examples/realtime/stream
```

| Examples | Contents |
| --- | --- |
| [`examples/clob`](examples/clob) | Authentication, orders, wallets, session keys and RFQ |
| [`examples/data`](examples/data) | Positions and token references |
| [`examples/gamma`](examples/gamma) | Search and market/event discovery |
| [`examples/bridge`](examples/bridge) | Supported assets and optional routing-address registration |
| [`examples/perps`](examples/perps) | Market reads and wallet preparation |
| [`examples/ws`](examples/ws) | CLOB WebSockets and RTDS |
| [`examples/realtime/stream`](examples/realtime/stream) | Local Polybolt stream fixture |
| [`examples/sports`](examples/sports) | Local sports stream fixture |

Read each example before running it: some submit orders or transactions. Stream
reconnects do not guarantee replay of missed events; your application must handle
any required backfill.

## Development

```sh
make fmt
make test
make build
make vet
go test -race ./...
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup and
[SECURITY.md](SECURITY.md) for reporting vulnerabilities. The implementation follows
the official [Rust](https://github.com/Polymarket/rs-clob-client-v2),
[TypeScript](https://github.com/Polymarket/ts-sdk) and
[Python](https://github.com/Polymarket/py-sdk) SDKs.

## License

[MIT](LICENSE).
