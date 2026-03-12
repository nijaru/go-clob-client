# go-clob-client

> [!WARNING]
> In development. This SDK is usable for core CLOB flows, but it is not yet feature-complete or at parity with the official TypeScript, Python, or Rust SDKs.

Go SDK for the Polymarket CLOB.

## Status

- Read-only health, market, orderbook, price history, and live-activity queries work.
- API key bootstrap, readonly API key management, and authenticated REST calls work.
- Typed limit and market order construction/signing now have deterministic fixture coverage.
- Full parity, builder flows, RFQ, streaming, and non-CLOB packages are still in progress.

If you need complete Polymarket SDK coverage today, use an official SDK. If you want a Go-native client that is actively moving toward parity, this repo is meant for that.

## Install

```bash
go get github.com/nijaru/go-clob-client/clob
```

Import path:

```go
import "github.com/nijaru/go-clob-client/clob"
```

## Quickstart

Read-only example:

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

	book, err := client.GetOrderBook(context.Background(), "<token-id>")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("server time: %d\n", serverTime)
	fmt.Printf("best bid levels: %d\n", len(book.Bids))
}
```

Authenticated setup:

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/nijaru/go-clob-client/clob"
)

func main() {
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
	log.Printf("derived API key %s", creds.Key)
}
```

## Current Support

Available now:

- read-only health, market data, orderbook, price history, and live activity queries
- API key bootstrap plus readonly API key management
- typed limit and market order construction/signing
- order posting, cancel flows, balance/allowance, notifications, scoring, and rewards queries

Still incomplete:

- some older raw market helpers remain alongside newer typed equivalents for compatibility
- parity coverage is still behind the official SDKs
- builder flows, streaming, RFQ, and non-CLOB packages are not implemented yet

## Trading Notes

This repo now includes a usable trading core, but it is still not a complete “official SDK parity” trading SDK.

In practice:

- creating and signing orders works
- bootstrapping auth and posting signed orders works
- market-order and proxy/funder behavior now has deterministic fixture coverage
- broader endpoint parity is still in progress

## Examples

- `examples/clob/read_only/main.go`
- `examples/clob/auth_bootstrap/main.go`
- `examples/clob/limit_order/main.go`
- `examples/clob/market_order/main.go`

## Versioning and Parity Goals

The goal of this repo is to track the official SDKs over time while keeping the Go API idiomatic. That means:

- matching official endpoint behavior and auth semantics
- not copying TypeScript/Python class structure directly
- growing coverage in milestones instead of claiming full parity early

The next major milestone is the remaining parity sweep across the CLOB HTTP surface, especially builder-adjacent flows, broader rewards coverage, and eventually streaming.

## Project Structure

User-facing packages:

- `clob/` for the CLOB SDK

Internal shared packages:

- `internal/polyauth/` for Polymarket signing and auth-header logic
- `internal/polyhttp/` for HTTP transport and JSON decoding

Future Polymarket families such as `gamma/`, `data/`, `ws/`, `bridge/`, and `ctf/` are intended to live beside `clob/`.

## Development

```bash
go test ./...
go build ./...
```
