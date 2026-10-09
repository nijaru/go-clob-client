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
| Public streams | Trades, BBO, book, tickers, statistics, candles; instrument/all-topic selection, heartbeat and reconnect |
| Owner credentials | Create, validate/resume, revoke; external typed-data signer boundary |
| Account snapshots | Balances, portfolio, seven-day stats, leverage/margin config, auto-cancel status; owner-authenticated position snapshots |
| Orders and history | Open orders and filtered order lookup/history; fills with sort/trade-ID cursor; funding payments, deposits, withdrawals, internal transfers, equity and PnL iterators |
| Notifications | All eight known variants including ADL; pages/iterator, unread count, mark IDs or inclusive timestamp/ID boundary read, server resync |
| Entry orders | GTC, GTD, IOC, FOK; price-less IOC/FOK, post-only, reduce-only, client IDs, command deadlines, per-item batch acknowledgements, placement update waiters |
| Conditional exits | Atomic order-scoped TP/SL, position-scoped full/partial exits, fixed market/limit order triggers and trailing market stop loss |
| Cancellation/risk | Numeric/client-ID cancellation, instrument/all cancel-all, bounded retries only for explicit `order_in_flight` items; auto-cancel arm/disarm; single/batch leverage and isolated-margin adjustment |
| Managed execution | TWAP create/read/pause/resume/cancel; chase create/read/cancel, bounds, run/child identities and progress records |
| Builders | Status, durable owner consent/revocation, approvals, batch attribution, exact fill fees, sparse receipts, cursor earnings and fixed-window/cutoff summary |
| Collateral | Deposit call preparation and caller-provided transaction sender; owner withdrawal and exact-decimal internal transfer with reconciliation label |

## Remaining gaps and limitations

- **Wallet-managed deposits are partial.** Unlike upstream secure-client
  workflows, Go does not derive Safe/deposit wallets, relay gasless deposits,
  approve token spending, estimate gas or track receipts here. Configure token,
  deposit contract and withdrawal wallet explicitly; `TransactionSender` owns
  broadcasting and wallet lifecycle. Deposit ABI and withdrawal signing are
  implemented, but that is not end-to-end wallet parity.
- **Public subscription handles are partial.** Each Go `MarketStream` owns a
  fixed subscription set and its own socket. Upstream managers multiplex
  independently cancellable subscription handles and dynamically subscribe/
  unsubscribe on one connection. Open separate Go streams when independent
  ownership is needed; no shared-socket manager is provided.
- Timestamp-only histories cannot guarantee access to every record in a full
  equal-timestamp boundary. Go returns `ErrPaginationNonProgress` rather than
  advancing past potentially unseen records as upstream fallback paginators do.
  This is an explicit safety difference, not proof of exhaustive history access.
- No built-in registration-result cache, reconstructed book or automatic resync
  backfill. Several response structs preserve fields without reproducing all
  upstream schema validation. Fixture verification does not establish live
  service or wallet compatibility.

No additional public trading/account operation group was found missing in the
listed pinned surfaces; the partial wallet and subscription workflows above
prevent a complete-parity claim. No live orders, wallet transactions or remote
mutations are needed for the package tests.

Submissions are attempted once except the documented cancellation-item retry.
On transport failure after submission, reconcile before retrying: use order/client
IDs, returned proxy material, TWAP/chase collections, or internal-transfer history
and your label. `PlaceOrderWithTPSL` returns acknowledgements and client identity
alongside errors so accepted legs can be inspected.
