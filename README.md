# go-clob-client

Go SDK for the Polymarket CLOB.

This project is active, usable for core read-only and authenticated REST flows, and still incomplete. It is not at feature parity with the official TypeScript, Python, or Rust SDKs yet.

## Status

Current status:

- usable for read-only CLOB queries
- usable for API key bootstrap and authenticated REST calls
- usable for typed limit and market order construction/signing
- still incomplete for broader parity, streaming, RFQ, and non-CLOB APIs

If you need full Polymarket SDK coverage today, use an official SDK. If you want a Go-native client that is actively moving toward parity, this repo is meant for that.

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

- read-only market and orderbook endpoints
- L1 auth headers for API key creation and derivation
- L2 auth headers for authenticated REST calls
- typed limit order creation and signing
- typed market order creation and signing
- order posting and cancellation helpers

Partial or manual only:

- order submission is practical, but the SDK is still early and not yet feature-complete
- some authenticated responses still return `json.RawMessage` instead of stable typed structs
- examples are present, but broader workflow coverage is still growing
- parity testing against official SDK outputs is started, not finished

Not implemented yet:

- websocket and streaming support
- RFQ support
- broader CLOB endpoint parity sweep
- Gamma, data, bridge, and CTF packages

## Trading Notes

This repo now includes typed order construction and signing, but it is still not a complete trading SDK in the “official SDK parity” sense.

What that means in practice:

- core order creation flows exist
- auth bootstrap exists
- posting signed orders exists
- edge-case coverage, wider endpoint support, and streaming support are still in progress

## API Surface

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
- `GetFeeRateBps`

Trading/auth:

- `CreateAPIKey`
- `DeriveAPIKey`
- `CreateOrDeriveAPIKey`
- `CreateOrder`
- `CreateMarketOrder`
- `CreateAndPostOrder`
- `CreateAndPostMarketOrder`
- `BuildPostOrderRequest`
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

## Examples

- `examples/clob/read_only/main.go`
- `examples/clob/auth_bootstrap/main.go`
- `examples/clob/limit_order/main.go`

## Versioning and Parity Goals

The goal of this repo is to track the official SDKs over time while keeping the Go API idiomatic. That means:

- matching official endpoint behavior and auth semantics
- not copying TypeScript/Python class structure directly
- growing coverage in milestones instead of claiming full parity early

The next major milestone is the trading core hardening pass: typed response coverage for authenticated endpoints, stronger signing fixtures, and more end-to-end examples.

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
