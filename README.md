# go-clob-client

[![Go Reference](https://pkg.go.dev/badge/github.com/nijaru/go-clob-client/clob.svg)](https://pkg.go.dev/github.com/nijaru/go-clob-client/clob)
[![CI](https://github.com/nijaru/go-clob-client/actions/workflows/ci.yml/badge.svg)](https://github.com/nijaru/go-clob-client/actions/workflows/ci.yml)

Go SDK for [Polymarket](https://polymarket.com) trading, discovery, portfolios,
perpetuals and live feeds. It targets the stable capability union of the official
[Rust](https://github.com/Polymarket/rs-clob-client-v2),
[TypeScript](https://github.com/Polymarket/ts-sdk) and
[Python](https://github.com/Polymarket/py-sdk) SDKs through protocol-oriented Go
packages, not a shared client façade.

> [!WARNING]
> Unofficial SDK. Trading, relaying and wallet operations have fixture-backed
> verification, not production-account validation. Do not interpret passing tests
> or endpoint coverage as proof of live trading compatibility.

## Install

Requires **Go 1.27+**.

```bash
go get github.com/nijaru/go-clob-client@latest
```

| Package | Purpose |
| --- | --- |
| [`clob`](https://pkg.go.dev/github.com/nijaru/go-clob-client/clob) | Event-market trading, account management, RFQ, wallets and signing |
| [`data`](https://pkg.go.dev/github.com/nijaru/go-clob-client/data) | Data API v2 portfolios, activity, analytics, leaderboards and indexed approvals |
| [`data/legacy`](https://pkg.go.dev/github.com/nijaru/go-clob-client/data/legacy) | Explicitly isolated, supported v1 Data contracts |
| [`gamma`](https://pkg.go.dev/github.com/nijaru/go-clob-client/gamma) | Markets, events, tags, comments, profiles, sports metadata and search |
| [`bridge`](https://pkg.go.dev/github.com/nijaru/go-clob-client/bridge) | Cross-chain assets, quotes, transfer status and routing-address registration |
| [`perps`](perps/README.md) | Perpetuals markets, credentials, accounts, trading, execution and collateral |
| [`realtime`](https://pkg.go.dev/github.com/nijaru/go-clob-client/realtime) | Authenticated Polybolt crypto/equity spot and 60-second TWAP streams |
| [`sports`](sports/README.md) | Public live game-result streams |
| [`signing`](https://pkg.go.dev/github.com/nijaru/go-clob-client/signing) | Context-aware local/external Ethereum signing capabilities |

Constructors do not start heartbeats, deploy wallets, grant approvals or submit
transactions. Network operations take contexts; streams and transaction handles
make lifetime and confirmation ownership explicit. These are pre-1.0 APIs: this
migration changes public interfaces without compatibility shims or a release.

## Public market data

```go
import "github.com/nijaru/go-clob-client/clob"

client, err := clob.NewClient(clob.Config{})
if err != nil {
    return err
}
book, err := client.GetOrderBook(ctx, tokenID)
if err != nil {
    return err
}
if len(book.Bids) > 0 {
    fmt.Println(book.Bids[len(book.Bids)-1].Price)
}
```

A read-only example needs no key:

```bash
go run ./examples/clob/read_only
```

## Event-market trading

CLOB has public, signing and authenticated views. Wallet, relayer and collateral
operations use the signing view without CLOB credentials. API-key bootstrap
requires L1 Ethereum signing; posting and account operations use L2 credentials. Builder and
relayer credentials are separate from CLOB credentials. `Config.Signer` accepts
an external/hardware EOA signer; `PrivateKey` is a mutually exclusive local-key
convenience. Signatures are verified against the pinned EOA, including low-S and
recovery-byte checks; deposit-wallet wrapping signs nested EIP-712 data rather
than asking a hardware wallet to authorize an opaque hash. EOAs can expose either
verified transaction signing with SDK-owned broadcasting, or wallet-owned
`TransactionSender` for nonce, gas and sending. An opaque returned hash does not
prove signed intent; its sender owns correct execution. Known and unknown-hash
send failures remain inspectable uncertainty, never automatic retries.

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
    return err
}
// This call submits an order. Construction above does not.
resp, err := client.CreateAndPostOrder(ctx, clob.OrderArgs{
    TokenID: tokenID,
    Price:   udecimal.MustParse("0.45"),
    Size:    udecimal.MustParse("5"),
    Side:    clob.SideBuy,
}, nil, clob.OrderTypeGTC, false)
```

If order liveness needs automatic heartbeats, explicitly call
`StartHeartbeats(ctx)`. `StopHeartbeats(ctx)` joins the loop and permits a restart.
`Close`/`Shutdown` permanently close **only the heartbeat lifecycle**: they do not
revoke credentials or disable foreground requests. `Deauthenticate(ctx)` joins
that lifecycle and returns a public view; existing Go references are not consumed.
Discard authenticated references when their ownership ends.

For confirmed fills, use `WaitForOrderFillSettlement` with the accepted response.
Cancellation of a wait cannot undo an order or transaction already submitted.
After an uncertain submission, reconcile its identities before retrying.

### Price-protected market orders

`Amount` means USDC notional for BUY and shares for SELL. `MaxPrice` protects BUY;
`MinPrice` protects SELL. Bounds must be positive and tick-aligned. An explicit
`Price` may tighten, never weaken, the bound. No book-derived protection bound is
invented. Protected BUY shares round down while preserving six-decimal USDC
notional; unsafe final wire ratios are rejected.

```go
maxPrice := udecimal.MustParse("0.50")
maxSpend := udecimal.MustParse("10.50")
resp, err := client.CreateAndPostMarketOrder(ctx, clob.MarketOrderArgs{
    TokenID:  tokenID,
    Amount:   udecimal.MustParse("10"),
    Side:     clob.SideBuy,
    MaxPrice: &maxPrice,
    MaxSpend: &maxSpend, // includes applicable platform and builder fees
}, nil, clob.OrderTypeFOK)
```

Orders select exactly one of `TokenID` (CTF outcome) and `PositionID` (native V2
position). Native positions use Exchange V3 and their own metadata/book/allowance
routing. Token orders follow `/version`: V1 uses legacy nonce/taker/fee fields and
its own signing domain; V2/V3 use timestamp, metadata and builder fields. Version
mismatch and eligible stale metadata rebuild once without overriding caller ticks.
The wire asset field is `tokenId` for both identifier kinds. Legacy fee-cache
setters do not override V2/V3 market fee curves.

### RFQ

Legacy RFQ REST operations and combo requester/quoter flows are distinct. Combo
request/accept uses `BuilderAuth` and the builder gateway; no quotes and maker
declines are ordinary results. `WaitForComboFill` treats FILLED/CONFIRMED as settled
only with a valid transaction hash.

`OpenComboRFQSession` owns a quoter WebSocket session with typed events, signed
`Quote`, acknowledged `CancelQuote`, and last-look `RespondToConfirmation`.
Session-key signers cannot use combo RFQ. Reconnect is explicit; ambiguous command
cancellation closes the socket rather than miscorrelating a late acknowledgement.
See the [local quoter example](examples/clob/combo_rfq_quoter/main.go).

## Wallet operations

| Signature type | Wallet model |
| --- | --- |
| `SignatureTypeEOA` | Direct EOA |
| `SignatureTypePolyProxy` | Polymarket proxy |
| `SignatureTypePolyGnosisSafe` | Polymarket Safe |
| `SignatureTypePoly1271` | Owner/session-signed deposit wallet |

Wallet identity is not interchangeable with signer identity. Funder derivation and
owner/session checks depend on the chosen scheme and supported chain. Constructors
use explicit/offline identity; remote discovery and readiness are separate calls.
`DiscoverDepositWallet` selects a deployed legacy wallet first, otherwise its beacon
derivation, without changing client identity. `IsWalletDeployedAt` is an explicit
public probe. Safe and beacon creation are explicit, owner-bound operations;
standalone proxy and legacy/session-target creation have no evidenced public flow.

`WalletOperations` resolves market protocol and prepares/routes split, merge and
redeem operations for legacy CTF, native V2 positions and combos. Token approvals,
transfers, trading approval snapshots, indexed approvals and collateral-return
plans remain explicit. A collateral-return plan may cover only one chunk: confirm
it before obtaining the next plan.

Smart-wallet calls relay one atomic batch. EOA batches are sequential and
**not atomic**; failure can leave approvals or a confirmed prefix behind. A nonnil
handle may accompany an error: retain its `Submissions` and reconcile before
retrying. Call `Wait(ctx)` or `WaitReceipts(ctx)` explicitly; relayer registry
visibility, mined receipts, CLOB indexing and perps ledger credit are different
states. Deployment is not approval, funding or order readiness.

Configure relayer authentication separately:

```go
relayCtx, err := clob.WithRelayerAuth(ctx, clob.RelayerAuthConfig{
    APIKey: &clob.RelayerAPIKey{
        Key:     os.Getenv("RELAYER_API_KEY"),
        Address: relayerKeyOwner,
    },
})
if err != nil { return err }
handle, err := client.PrepareGaslessTransaction(relayCtx, calls, "merge")
if err != nil { return err }
outcome, err := handle.Wait(relayCtx)
```

Alternatively select builder authentication. Explicit relayer selection replaces,
never mixes with, the client's builder scheme. Use the selected context for both
submission and waits. Retry is limited to evidenced relayer rejections, not an
arbitrary transport failure after a possible broadcast.

Deposit-wallet session keys have a 4,315-hour authorization lifetime. Listing,
authorization, readiness and revocation are separate operations. Authorization
waits for confirmation and matching registry scopes; revocation may return on
registry removal before chain confirmation. Authorization requires builder auth;
revocation supports builder or Relayer API-key auth.

## Data and discovery

```go
import "github.com/nijaru/go-clob-client/data"

client, err := data.NewClient(data.Config{})
if err != nil { return err }
page, err := client.GetPositions(ctx, data.PositionsParams{
    User: wallet,
    Page: data.PageParams{Limit: 100},
})
if err != nil { return err }
for _, position := range page.Items {
    fmt.Println(position.AssetID, position.CurrentSize, position.CurrentValue)
}
```

Data v2 exposes server `HasMore`/`NextCursor`; row count is not completion. Iterators
follow opaque cursors, detect cycles and stop without prefetch after cancellation
or an early break. `data/legacy` is a separate import, never an automatic fallback.
Financial wire decimals retain their digits; accounting ZIPs stream to a
caller-owned writer. Indexed JSON reads retry HTTP 429 twice, with cancellable
waits and a five-second server-delay ceiling; accounting downloads do not retry.
Known combo IDs normalize to structural condition IDs; unknown wallet activity
keeps its raw payload.

Gamma provides offset and query-bound keyset discovery. Comment offsets count
roots, not replies. Offset-only comments cap at 200 and search at 100 pages;
iterators report incomplete enumeration instead of silently claiming exhaustion.
Nullable state retains unknown versus explicit false/zero, financial decimals
remain exact, and `OutcomeDetails` pairs legacy and native assets by wire index.
Timestamps normalize Gamma's ISO/PostgreSQL forms to UTC. See
[Gamma package documentation](https://pkg.go.dev/github.com/nijaru/go-clob-client/gamma)
for filter/keyset eligibility and wire models.

Bridge uses `NewClient`, full uint64 chain IDs, canonical uint256 base-unit amounts
and lossless USD estimates. `CreateDepositAddress`/`CreateWithdrawalAddress`
register routing addresses: neither signs or transfers funds. The bridge example
performs only public reads unless `-deposit-wallet` explicitly enables registration.

## Perpetuals and live feeds

Perps has separate credentials, REST/WebSocket protocols and owner/session signing.
It supports credential creation/resumption/revocation, account/history reads,
notifications/ADL, GTD, TP/SL and trailing exits, cancellation/risk, TWAP/chase,
builders, internal transfers and explicit collateral workflows. Owner and delegated
signers use the shared verified signing boundary. The managed collateral adapter
uses the signing client without CLOB credentials; smart wallets authenticate
separately with relayer API keys or builder credentials. Externally managed calls
can also use the independent owner/sender boundary. An optional
`signing.TransactionWaiter` reconciles fee replacements while verifying signed
call intent and mined receipt identity; original submission hashes remain intact.
See
[the perps guide](perps/README.md) for signing, reconciliation and wallet limits.

`NewMarketStream(ctx)` pools public filters with independently cancellable handles.
Account sessions own their handshake, heartbeat and reconnect. Overflow is an
explicit error, not silent update loss. Book reconstruction and resync backfill
remain caller-owned. Timestamp-only histories report nonprogress when exhaustive
retrieval cannot be proven.

Legacy CLOB WebSocket/RTDS feeds remain available. RTDS shares broad wire topics,
filters local interests independently, and exposes generic `Unsubscribe`. `realtime` separately implements
Polybolt's authenticated crypto/equity spot and fixed 60-second TWAP feeds with
pooled filters, acknowledgements, snapshots and provider provenance. Requested
provider and actual source are distinct. `sports` is a public all-games feed, not
a server-filtered subscription protocol. Reconnect does not imply replay or gap
recovery. Stream examples default to local fixtures.

## Examples and checks

| Example | Purpose |
| --- | --- |
| [`examples/clob`](examples/clob) | Public reads, auth, protected orders, CTF, wallets, session keys and RFQ |
| [`examples/data`](examples/data) | Data v2 portfolios and analytics |
| [`examples/gamma`](examples/gamma) | Search and keyset discovery |
| [`examples/bridge`](examples/bridge) | Cross-chain assets; optional explicit address registration |
| [`examples/perps`](examples/perps) | Market reads and local wallet preparation |
| [`examples/realtime/stream`](examples/realtime/stream) | Local Polybolt fixture |
| [`examples/sports`](examples/sports) | Local sports fixture |
| [`examples/ws`](examples/ws) | Legacy CLOB WebSocket/RTDS |

```bash
make fmt
make test
make build
make vet
go test -race ./...
```

Coverage is grounded in merged Rust `561830b`, TypeScript `087f9443` and Python
`ed8d04ca`. Implementation and fixture coverage do not establish exhaustive live
schema or account compatibility. Polymarket US is a separate exchange and is not
included. No release or production-readiness claim accompanies this migration.

See [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md).
