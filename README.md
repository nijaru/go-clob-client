# go-clob-client

[![Go Reference](https://pkg.go.dev/badge/github.com/nijaru/go-clob-client/clob.svg)](https://pkg.go.dev/github.com/nijaru/go-clob-client/clob)
[![CI](https://github.com/nijaru/go-clob-client/actions/workflows/ci.yml/badge.svg)](https://github.com/nijaru/go-clob-client/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nijaru/go-clob-client)](https://goreportcard.com/report/github.com/nijaru/go-clob-client)

> [!WARNING]
> Unofficial, community-maintained SDK. Not extensively tested in production trading environments. Use at your own risk.

> [!NOTE]
> **Parity status (2026-07-26):** Core mechanics and the non-perps CLOB/Data/Gamma/Bridge/CTF/
> RTDS/RFQ surfaces are tracked against the current official Rust, TypeScript, and Python SDKs,
> with current combo pagination, Gamma discovery endpoints, and full CLOB WebSocket event fields.
> **Perps** remains a separate package: public market-data REST, authenticated account reads,
> delegated account sessions (including heartbeat/reconnect), and signed entry-order placement/cancel/leverage commands are wire-compatible
> with the current TS SDK. Smart-wallet collateral return is available through the authenticated
> CLOB client; owner-signed credential lifecycle, other collateral mutations, TP/SL orchestration,
> and public BBO streams remain separate follow-ups. Parity here means contract-level capability with
> idiomatic Go APIs, not a drop-in copy
> of TypeScript or Python method names. See `ai/ROADMAP.md`.

Go SDK for the [Polymarket](https://polymarket.com) CLOB and adjacent APIs. Tracks stable capability
parity with the official [Rust V2](https://github.com/Polymarket/rs-clob-client-v2),
[TypeScript](https://github.com/Polymarket/ts-sdk), and [Python](https://github.com/Polymarket/py-sdk)
SDKs while keeping an idiomatic Go API.

## Install

Requires **Go 1.26+**.

```bash
go get github.com/nijaru/go-clob-client@latest
```

The module exposes several focused packages:

- **`clob`** — trading, orderbooks, prices, account management, websockets, heartbeats, wallet operations
- **`data`** — read-only analytics: positions, trades, activity, combo portfolios, holders, leaderboards
- **`gamma`** — markets, events, tags, sports, comments, profiles, and clarifications
- **`bridge`** — deposit-address discovery
- **`perps`** — public perpetuals market data and authenticated account/session access

## Quickstart

### Read-Only

```go
import "github.com/nijaru/go-clob-client/clob"

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

`NewAuthenticatedClient` starts a background **heartbeat loop** to maintain order liveness. Always call `Close` or `Shutdown` when done.

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

resp, err := client.CreateAndPostOrder(ctx, clob.OrderArgs{
	TokenID: os.Getenv("POLYMARKET_TOKEN_ID"),
	Price:   udecimal.MustParse("0.45"),
	Size:    udecimal.MustParse("5"),
	Side:    clob.SideBuy,
}, nil, clob.OrderTypeGTC, false)
```

For explicit control over immediate fills, pass the accepted response to
`client.WaitForOrderFillSettlement(ctx, *resp, clob.OrderSettlementOptions{})`.
It waits for confirmed or failed fill outcomes and returns typed timeout or all-fills-failed errors.

### Market Orders

Market orders use `Amount` as USDC notional for BUY and share count for SELL.

```go
// Sell 25 shares at market
resp, err := client.CreateAndPostMarketOrder(ctx, clob.MarketOrderArgs{
	TokenID: os.Getenv("POLYMARKET_TOKEN_ID"),
	Amount:  udecimal.MustParse("25"),
	Side:    clob.SideSell,
}, nil, clob.OrderTypeFOK)

// Buy $10 worth of shares at market
resp, err := client.CreateAndPostMarketOrder(ctx, clob.MarketOrderArgs{
	TokenID: os.Getenv("POLYMARKET_TOKEN_ID"),
	Amount:  udecimal.MustParse("10"),
	Side:    clob.SideBuy,
}, nil, clob.OrderTypeFOK)

// Buy $10 worth of shares, but spend at most $10.50 total (including fees)
maxSpend := udecimal.MustParse("10.50")
resp, err := client.CreateAndPostMarketOrder(ctx, clob.MarketOrderArgs{
	TokenID:  os.Getenv("POLYMARKET_TOKEN_ID"),
	Amount:   udecimal.MustParse("10"),
	Side:     clob.SideBuy,
	MaxSpend: &maxSpend,
}, nil, clob.OrderTypeFOK)
```

### Iterators

List endpoints expose both a slice and a Go 1.26 range-over-function iterator:

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

client := data.New(data.Config{})

positions, err := client.GetPositions(ctx, data.PositionParams{
	User: "0x1234...",
})
```

## Client Tiers

The SDK enforces authentication through a three-tier hierarchy:

| Tier                  | Constructor              | Auth Required           | Capabilities                                  |
| --------------------- | ------------------------ | ----------------------- | --------------------------------------------- |
| `Client`              | `NewClient`              | None                    | Public market data, orderbooks, prices        |
| `SignerClient`        | `NewSignerClient`        | Private key             | Order building & signing, API key management  |
| `AuthenticatedClient` | `NewAuthenticatedClient` | Private key + API creds | Order posting, account management, heartbeats, gasless relayer |

Upgrade incrementally:

```go
base, _ := clob.NewClient(clob.Config{})
signer, _ := base.AsSigner(privateKey, clob.SignatureTypeEOA, "")
authed, _ := signer.AsAuthenticated(creds, nil)
```

## Wallet Types

| Value | Constant                      | Wallet type                              |
| ----- | ----------------------------- | ---------------------------------------- |
| `0`   | `SignatureTypeEOA`            | MetaMask, hardware wallets (direct key)  |
| `1`   | `SignatureTypePolyProxy`      | Email / Magic wallet (delegated signing) |
| `2`   | `SignatureTypePolyGnosisSafe` | Browser proxy wallet                     |
| `3`   | `SignatureTypePoly1271`       | Smart-contract / deposit wallet          |

Set `FunderAddress` explicitly for proxy/Magic wallets where the signing key differs from the on-chain funder. For EOA wallets it defaults to the signing key address.

### Gasless relayer

Proxy, Safe, and deposit (Poly1271) wallets can only act through Polymarket's relayer, which submits on-chain calls as meta-transactions so the caller pays no gas. `AuthenticatedClient` exposes the full lifecycle for these wallet types; EOA wallets broadcast directly via go-ethereum (see CTF operations).

```go
// Submit a batch of calls through the relayer and wait for confirmation.
handle, err := client.PrepareGaslessTransaction(ctx, calls, "merge")
if err != nil { return err }
outcome, err := handle.Wait(ctx)
// outcome.TransactionHash is the on-chain tx hash once mined.
```

`PrepareGaslessTransaction` handles nonce fetch, per-scheme signing, payload assembly, and submit with retry on transient failures (rate limit, wallet contention, stale nonce). `DeployDepositWallet` submits the unsigned wallet deployment; `IsWalletDeployed` checks on-chain deployment. The relayer host defaults to `https://relayer-v2.polymarket.com` (override via `Config.RelayerHost`).

For the common CTF operations, gasless convenience methods build the calldata and route it through the relayer — the non-EOA equivalents of the on-chain `SignerClient` methods, sharing identical calldata and contract targets:

```go
// Merge a complementary YES/NO pair back into collateral, gaslessly.
handle, err := client.MergePositionsGasless(ctx, clob.MergeBinary(usdc, conditionID, amount), "merge")
// Also: SplitPositionGasless, RedeemPositionsGasless, RedeemNegRiskGasless.
outcome, err := handle.Wait(ctx)
```

Token wallet operations use the same explicit split: `SignerClient.ApproveERC20`,
`SignerClient.ApproveERC1155ForAll`, and `SignerClient.TransferERC20` broadcast directly from an
EOA; the corresponding `AuthenticatedClient` `*Gasless` methods route calls through proxy, Safe,
or deposit wallets. `clob.MaxUint256()` returns a fresh unlimited-approval amount.

For one-call trading setup, `PrepareTradingApprovals` reads the current on-chain state and skips
allowances already present. Use `SignerClient.SetupTradingApprovals` for sequential EOA
transactions or `AuthenticatedClient.SetupTradingApprovalsGasless` for one relayed batch. The
resolver follows the current Polygon contract set; callers should treat a nil gasless handle as
“already approved.”

### Collateral return

Proxy, Safe, and deposit wallets can plan and execute the official collateral-return workflow. The
plan is an inspectable server response; review its `NetPUSDOut`, operations, and position summary
before submitting the exact router call it carries. A truncated plan covers one chunk, so wait for
that transaction and request a fresh plan for the remainder.

```go
plan, err := client.PlanCollateralReturn(ctx)
if err != nil {
	return err
}

handle, err := client.ExecuteCollateralReturnPlan(ctx, *plan)
if err != nil {
	return err
}
_, err = handle.Wait(ctx)
```

This workflow does not support EOA wallets or submit approvals implicitly. The service host defaults
to `https://combos-rfq-collateral-return.polymarket.com` and can be overridden with
`Config.CollateralReturnHost`.

Perps account reads use an existing delegated credential, and `OpenSession` performs the official
authenticated WebSocket handshake, account-channel subscription, application heartbeat, and
automatic reconnect/resubscription:

```go
import "github.com/nijaru/go-clob-client/perps"

client, err := perps.NewAuthenticated(perps.AuthenticatedConfig{
	Credentials: perps.PerpsCredentials{
		Proxy:  os.Getenv("POLYMARKET_PERPS_PROXY"),
		Secret: os.Getenv("POLYMARKET_PERPS_SECRET"),
	},
})
if err != nil {
	log.Fatal(err)
}
portfolio, err := client.GetPortfolio(ctx)
if err != nil {
	log.Fatal(err)
}
fmt.Printf("withdrawable: %s\n", portfolio.Withdrawable)
session, err := client.OpenSession(ctx, perps.SessionConfig{})
if err != nil {
	log.Fatal(err)
}
defer session.Close()
for event := range session.Events() {
	log.Printf("perps %s update: %s", event.Channel, event.Data)
}
```

Signed perps entry-order placement, low-level batch/cancel/cancel-all, and leverage commands are
available when the delegated private key is supplied in `PerpsCredentials`. TP/SL orchestration and
owner-signed delegated-credential creation/revocation remain separate follow-up surfaces.

## Error Handling

API errors are returned as `*clob.APIError` with HTTP status and body:

```go
if errors.Is(err, clob.ErrNotFound)    { /* 404 */ }
if errors.Is(err, clob.ErrRateLimit)   { /* 429 */ }
if errors.Is(err, clob.ErrGeoBlocked)  { /* 451 */ }
if errors.Is(err, clob.ErrUnauthorized){ /* 401/403 */ }
```

## Examples

| Example        | Path                           | What it shows                                |
| -------------- | ------------------------------ | -------------------------------------------- |
| Read-only      | `examples/clob/read_only`      | Orderbook, prices, market data               |
| Auth bootstrap | `examples/clob/auth_bootstrap` | Creating and deriving API keys               |
| Limit order    | `examples/clob/limit_order`    | Placing a GTC limit order                    |
| Market order   | `examples/clob/market_order`   | Placing a FOK market order                   |
| CTF operations | `examples/clob/ctf_operations` | Splitting, merging, and redeeming shares     |
| Gasless        | `examples/clob/gasless`        | Submitting CTF and arbitrary calls through the relayer |
| WebSocket      | `examples/ws`                  | Real-time orderbook and user event streaming |
| Data API       | `examples/data`                | Positions and read-only data endpoints       |
| Bridge         | `examples/bridge`              | Deposit addresses (EVM, Solana, Bitcoin)     |
| Gamma          | `examples/gamma`               | Search, events, and discovery metadata       |
| Perps         | `examples/perps`               | Instruments, tickers, books, candles, trades, funding |

```bash
export POLYMARKET_PRIVATE_KEY=0x...
go run ./examples/clob/read_only
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the tiered official-SDK oracle model and contribution
workflow.

## Security

See [SECURITY.md](SECURITY.md).
