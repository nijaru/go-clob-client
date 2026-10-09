# Perps

Experimental Go SDK for Polymarket perpetuals. Perps uses separate REST,
WebSocket, credential and signing protocols from CLOB event markets.

```go
client := perps.New(perps.Config{})
instruments, err := client.GetInstruments(ctx, perps.InstrumentsParams{})
```

See [`../examples/perps`](../examples/perps) for a runnable read-only example.
Decimal amounts remain strings. Times are Unix milliseconds unless a method
says otherwise; withdrawal signatures use Unix seconds internally.

## Credentials and trading

`NewAuthenticated` accepts a proxy and secret for account reads. Supply the
matching delegated private key for signed commands. It validates local key
identity, not server ownership or expiration.

For owner operations, use `NewOwner` with an `OwnerSigner` (local key, hardware
wallet or external signer). `CreateCredentials` generates a delegated key;
`Resume` checks the server's credential listing, owner and expiration.
`RevokeCredentials` explicitly revokes a proxy. Credential creation returns
private key material even on an uncertain submission failure: retain it securely
and reconcile or revoke the proxy rather than blindly creating another.

```go
account, err := owner.Resume(ctx, storedCredentials)
if err != nil { return err }
session, err := account.OpenSession(ctx, perps.SessionConfig{})
if err != nil { return err }
defer session.Close()
// PostOrders returns per-item acknowledgements. PlaceOrder also waits for an
// orders update; its error can be an *perps.OrderPlacementError containing
// client identity and accepted acknowledgements for reconciliation.
```

A session follows its context and reconnects/resubscribes automatically. Consume
both `Events()` and `Errors()`. Typed `AsOrder`, `AsFills`, `AsPortfolio` and other
`As*` decoders coexist with raw event JSON. Book events are deltas. Reconnect and
sequence-gap events require caller-owned backfill/snapshot recovery. Notification
and builder-receipt sequences are sparse, not contiguous; a server notification
resync must be backfilled from the last successfully processed notification,
**not** from the highest dropped sequence in the resync frame.

A full event queue closes the session with `ErrPerpsSlowConsumer`; it never
silently drops an account update. Commands in flight during disconnect are not
resubmitted. Queries, waiters and retries obey their supplied context; callers
should set deadlines.

Builder attribution is selected through `SessionConfig.BuilderAddress`. Opening
a session does not grant consent. Attribution uses the lower of the platform
cap and saved owner-approved maximum; missing/zero approval disables it. Explicit
owner approval/revocation updates this session's terms; other sessions must call
`RefreshBuilder`. Existing orders retain their saved attribution.


## Wallet-managed collateral

`NewCollateralWallet` binds an `OwnerConfig` to the existing CLOB transaction
engine through `CollateralWalletConfig.Transactions` (`clob.Config`). It derives
Safe/proxy wallets, or a beacon deposit wallet when no funder is supplied, and
validates the owner, chain and withdrawal wallet. Construction is offline and
never deploys, approves or sends. Token and deposit-contract addresses and an
RPC URL must be explicit. The adapter currently requires a local transaction
key and CLOB credentials; gasless wallets also require separate `BuilderAuth`.
Perps credentials are not CLOB or relayer credentials.

- `PrepareCollateralApproval` prepares an exact allowance; zero revokes.
- `ApproveCollateral` sends only approval. `Deposit` sends only a deposit and
  credits the owner signer, not the smart-wallet address.
- `ApproveAndDeposit` explicitly authorizes both effects: one atomic relayer
  batch for smart wallets, or an approval confirmed before deposit for EOAs.
  An EOA failure can leave an allowance without a deposit. Retain the returned
  `CollateralTransaction` even on error: its submissions and confirmed prefix
  support reconciliation. A failed send may have reached the network without
  returning a hash. Do not blindly retry it.
- `DeployDepositWallet` explicitly submits current beacon-wallet creation.
  `Readiness` reports on-chain code and relayer registry status separately.
  Registry readiness never substitutes for code or a transaction receipt.
- `CollateralTransaction.Wait(ctx)` checks relayer outcomes and successful RPC
  receipts, validates receipt identity and chain, and returns collected receipts
  even on failure. It sends nothing, is cancellable, and supports concurrent
  waits. Success means mined execution, **not** perps ledger credit or reorg
  finality. After a partial EOA failure it waits only for recorded submissions,
  not the missing or uncertain deposit.

RPC chain identity is checked before submission. Gas estimation, EIP-1559
signing and relayer serialization remain owned by CLOB. Its gasless engine may
retry explicit transient submission rejections; this adapter adds no send retry.
There is no automatic allowance skip, unlimited approval or zero-first token
reset. Tokens requiring a reset need explicit revocation and confirmation first;
a successful receipt does not check an ERC-20 boolean return value.

See [`walletdeposit`](../examples/perps/walletdeposit/main.go). It defaults to
unsigned preparation; mutation requires an explicit `-action`. For existing
legacy deposit wallets, supply the funder explicitly after using CLOB's
`DeriveCurrentDepositWallet(ctx)` or a known validated derivation. The constructor
selects beacon derivation offline rather than discovering deployed legacy code.
The lower-level `OwnerClient.Deposit` and `TransactionSender` remain available
for externally managed wallets and hardware/external transaction signers.

## Public market subscriptions

Create an explicitly context-owned pool, then subscribe with a separate context
for each independently owned handle:

```go
stream := client.NewMarketStream(ctx) // lazy, public-only; never authenticates
defer stream.Close()
id := 1
handle, err := stream.Subscribe(subscriptionCtx, []perps.MarketSubscription{
    {Topic: perps.MarketBook, InstrumentID: &id},
})
if err != nil { return err }
defer handle.Close()
// Consume handle.Events() and handle.Errors(). Cancel subscriptionCtx to remove
// only this handle; cancel ctx or Close stream to stop the entire pool.
```

`Subscribe` waits for the server acknowledgement, and its context owns the
returned handle's lifetime. Identical filters share a server subscription;
all-ticker/statistics filters replace instrument-specific server filters while
preserving each handle's instrument selection. Filters are copied at subscribe
time. Duplicate/overlapping filters produce one delivery per matching frame.

Handles reconnect/resubscribe automatically and emit reconnect/sequence-gap
resync events. Each handle has a 128-event queue; overflow closes only that handle
with `ErrPerpsSlowConsumer`. It does not block healthy peers or acknowledgements.
`MarketHandle.Close` waits for local removal and queue closure; the pool owns
server unsubscribe cleanup. Rejected, timed-out or uncertain filter operations
reset the socket and resubscribe surviving handles, rather than retain potentially
orphaned subscriptions. A failed initial subscription returns an error; callers
may explicitly subscribe again. The idle socket closes when the last handle ends;
the pool remains reusable until its context ends or `MarketStream.Close` is called.
Pool closure cancels and waits for dial/read/write workers. Book snapshots and resync
backfill remain caller-owned.

## Capability inventory

Compared against merged TypeScript [`087f9443`](https://github.com/Polymarket/ts-sdk/tree/087f9443635316e4e6be5dc4e22bd93f08cea443)
and Python [`ed8d04ca`](https://github.com/Polymarket/py-sdk/tree/ed8d04cade617d2f4c6f90ebe1842249eaf5c59b).
The inventory covers public operations in TS `decorators/perps.ts`,
`websockets/perps/session.ts` and public subscriptions, plus Python
`clients/async_public.py`, `clients/async_secure.py` and `_internal/perps_session.py`.
Wire contracts come from their perps actions, bindings/models and stream parsers.
An operation on `AuthenticatedClient` rather than `Session`, or a batch of one
rather than a single-item wrapper, is not counted as a missing capability.

| Operation group | Go support |
| --- | --- |
| Public market reads | Instruments/category filtering, joined ticker(s)/statistics, book depth, fees; resumable candles, trades and funding-rate history |
| Public account discovery | Registration; anonymous ordered active/historical position snapshots |
| Public streams | Trades, BBO, book, tickers, statistics, candles; shared-socket independently context-owned handles, dynamic filter refcounts, instrument/all-topic selection, heartbeat and reconnect |
| Owner credentials | Create, validate/resume, revoke; external typed-data signer boundary |
| Account snapshots | Balances, portfolio, seven-day stats, leverage/margin config, auto-cancel status; owner-authenticated position snapshots |
| Orders and history | Open orders and filtered order lookup/history; fills with sort/trade-ID cursor; funding payments, deposits, withdrawals, internal transfers, equity and PnL iterators |
| Notifications | All eight known variants including ADL; pages/iterator, unread count, mark IDs or inclusive timestamp/ID boundary read, server resync |
| Entry orders | GTC, GTD, IOC, FOK; price-less IOC/FOK, post-only, reduce-only, client IDs, command deadlines, per-item batch acknowledgements, placement update waiters |
| Conditional exits | Atomic order-scoped TP/SL, position-scoped full/partial exits, fixed market/limit order triggers and trailing market stop loss |
| Cancellation/risk | Numeric/client-ID cancellation, instrument/all cancel-all, bounded retries only for explicit `order_in_flight` items; auto-cancel arm/disarm; single/batch leverage and isolated-margin adjustment |
| Managed execution | TWAP create/read/pause/resume/cancel; chase create/read/cancel, bounds, run/child identities and progress records |
| Builders | Status, durable owner consent/revocation, approvals, batch attribution, exact fill fees, sparse receipts, cursor earnings and fixed-window/cutoff summary |
| Collateral | Explicit derived-wallet approval/deposit, gasless batches, EOA gas estimation, deposit-wallet deployment and RPC receipt tracking; unsigned/external-sender boundary; owner withdrawal and exact-decimal internal transfer with reconciliation label |

## Remaining gaps and limitations

- **Wallet lifecycle still has limits.** The managed adapter requires a local
  transaction key and CLOB credentials because it reuses the CLOB engine;
  external transaction signing remains caller-owned. Safe/proxy wallets must
  already be deployed. Deposit-wallet deployment is explicit and beacon-only.
  Constructors use offline derivation; deployed legacy-wallet discovery is
  separate. Funding, perps ledger-credit waits and reorg finality are caller-owned.
- Timestamp-only histories cannot guarantee access to every record in a full
  equal-timestamp boundary. Go returns `ErrPaginationNonProgress` rather than
  advancing past potentially unseen records. This is an explicit safety
  difference, not proof of exhaustive history access.
- No reconstructed book or automatic resync backfill. Several response structs
  preserve fields without reproducing every upstream schema validator. Fixture
  verification does not establish live service, wallet or relayer compatibility.

No additional public trading/account operation group was found missing in the
listed pinned surfaces. No live orders, wallet transactions or remote
mutations are needed for the package tests.

Submissions are attempted once except the documented cancellation-item retry.
On transport failure after submission, reconcile before retrying: use order/client
IDs, returned proxy material, TWAP/chase collections, or internal-transfer history
and your label. `PlaceOrderWithTPSL` returns acknowledgements and client identity
alongside errors so accepted legs can be inspected.
