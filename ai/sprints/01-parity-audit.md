# Sprint 01 — Parity Audit

**Goal:** Confirm every public TS SDK method exists in the Go SDK with correct semantics, and document any intentional divergences.

**Demo:** Written parity table in this file with all rows marked, plus any new tasks created for discovered gaps.

---

## Task 1: Public methods (AuthNone)

**Depends on:** none
**Criteria:** Every method in the table below is confirmed present, named correctly, and takes equivalent params.

| TS SDK                               | Go SDK                         | Status | Notes                              |
| :----------------------------------- | :----------------------------- | :----- | :--------------------------------- |
| `getOk()`                            | `GetOk`                        | -      |                                    |
| `getServerTime()`                    | `GetServerTime`                | -      |                                    |
| `getMarkets()`                       | `GetMarkets`                   | -      |                                    |
| `getMarket(conditionId)`             | `GetMarket`                    | -      |                                    |
| `getSamplingMarkets()`               | `GetSamplingMarkets`           | -      |                                    |
| `getSamplingSimplifiedMarkets()`     | `GetSamplingSimplifiedMarkets` | -      |                                    |
| `getSimplifiedMarkets()`             | `GetSimplifiedMarkets`         | -      |                                    |
| `getOrderBook(tokenID)`              | `GetOrderBook`                 | -      |                                    |
| `getOrderBooks(params)`              | `GetOrderBooks`                | -      |                                    |
| `getPrice(tokenID, side)`            | `GetPrice`                     | -      |                                    |
| `getPrices(params)`                  | `GetPrices`                    | -      | side required, validated           |
| `getAllPrices()`                     | `GetAllPrices`                 | -      |                                    |
| `getMidpoint(tokenID)`               | `GetMidpoint`                  | -      |                                    |
| `getMidpoints(params)`               | `GetMidpoints`                 | -      |                                    |
| `getSpread(tokenID)`                 | `GetSpread`                    | -      |                                    |
| `getSpreads(params)`                 | `GetSpreads`                   | -      |                                    |
| `getLastTradePrice(tokenID)`         | `GetLastTradePrice`            | -      |                                    |
| `getLastTradesPrices(params)`        | `GetLastTradesPrices`          | -      |                                    |
| `getTickSize(tokenID)`               | `GetTickSize`                  | -      |                                    |
| `getNegRisk(tokenID)`                | `GetNegRisk`                   | -      |                                    |
| `getFeeRate(tokenID)`                | `GetFeeRate` + `GetFeeRateBps` | -      | Go adds typed helper               |
| `getPricesHistory(params)`           | `GetPricesHistory`             | -      |                                    |
| `getMarketTradesEvents(conditionID)` | `GetMarketTradesEvents`        | -      |                                    |
| `calculateMarketPrice(...)`          | `CalculateMarketPrice`         | -      | algorithm correctness in sprint 02 |
| `getOrderBookHash(orderbook)`        | `GetOrderBookHash`             | -      |                                    |

---

## Task 2: L1 methods (key bootstrap)

**Depends on:** none
**Criteria:** All key management methods present with equivalent signatures.

| TS SDK                        | Go SDK                 | Status | Notes |
| :---------------------------- | :--------------------- | :----- | :---- |
| `createApiKey(nonce)`         | `CreateAPIKey`         | -      |       |
| `deriveApiKey(nonce)`         | `DeriveAPIKey`         | -      |       |
| `createOrDeriveApiKey(nonce)` | `CreateOrDeriveAPIKey` | -      |       |
| `getApiKeys()`                | `GetAPIKeys`           | -      |       |
| `deleteApiKey()`              | `DeleteAPIKey`         | -      |       |

---

## Task 3: L2 methods (authenticated trading)

**Depends on:** none
**Criteria:** All trading and account methods present and auth level is correct.

| TS SDK                           | Go SDK                   | Status | Notes |
| :------------------------------- | :----------------------- | :----- | :---- |
| `postOrder(order, type)`         | `PostOrder`              | -      |       |
| `postOrders(orders)`             | `PostOrders`             | -      |       |
| `cancelOrder(params)`            | `CancelOrder`            | -      |       |
| `cancelOrders(params)`           | `CancelOrders`           | -      |       |
| `cancelAll()`                    | `CancelAll`              | -      |       |
| `cancelMarketOrders(params)`     | `CancelMarketOrders`     | -      |       |
| `getOpenOrders(params)`          | `GetOpenOrders`          | -      |       |
| `getOrder(orderID)`              | `GetOrder`               | -      |       |
| `getTrades(params)`              | `GetTrades`              | -      |       |
| `getClosedOnlyMode()`            | `GetClosedOnlyMode`      | -      |       |
| `getBalanceAllowance(params)`    | `GetBalanceAllowance`    | -      |       |
| `updateBalanceAllowance(params)` | `UpdateBalanceAllowance` | -      |       |
| `isOrderScoring(params)`         | `IsOrderScoring`         | -      |       |
| `areOrdersScoring(params)`       | `AreOrdersScoring`       | -      |       |
| `getNotifications()`             | `GetNotifications`       | -      |       |
| `dropNotifications(params)`      | `DropNotifications`      | -      |       |
| `deleteNotifications(params)`    | `DeleteNotifications`    | -      |       |

---

## Task 4: Trading helpers

**Depends on:** none
**Criteria:** Helper signatures match TS SDK semantics; option structs cover equivalent fields.

| TS SDK                             | Go SDK                  | Status | Notes |
| :--------------------------------- | :---------------------- | :----- | :---- |
| `createOrder(args, options)`       | `CreateOrder`           | -      |       |
| `createMarketOrder(args, options)` | `CreateMarketOrder`     | -      |       |
| `createAndPostOrder(...)`          | `CreateAndPostOrder`    | -      |       |
| `buildPostOrderRequest(...)`       | `BuildPostOrderRequest` | -      |       |

---

## Task 5: RFQ surface

**Depends on:** none
**Criteria:** All RFQ methods present; confirm `createRfqRequest` param shape (TS uses `tokenID/price/side/size`; Go exposes wire `assetIn/assetOut`).

| TS SDK (`client.rfq.*`)         | Go SDK                  | Status | Notes                                                                                                                |
| :------------------------------ | :---------------------- | :----- | :------------------------------------------------------------------------------------------------------------------- |
| `createRfqRequest(params)`      | `CreateRFQRequest`      | -      | **VERIFY:** TS abstracts to tokenID/price/side/size; Go uses wire format. Decide if Go should add high-level helper. |
| `cancelRfqRequest(id)`          | `CancelRFQRequest`      | -      |                                                                                                                      |
| `getRfqRequests(params)`        | `GetRFQRequests`        | -      | `markets` field added; `requestIds` now repeatable                                                                   |
| `createRfqQuote(params)`        | `CreateRFQQuote`        | -      |                                                                                                                      |
| `cancelRfqQuote(id)`            | `CancelRFQQuote`        | -      |                                                                                                                      |
| `getRfqRequesterQuotes(params)` | `GetRFQRequesterQuotes` | -      |                                                                                                                      |
| `getRfqQuoterQuotes(params)`    | `GetRFQQuoterQuotes`    | -      |                                                                                                                      |
| `getRfqBestQuote(requestID)`    | `GetRFQBestQuote`       | -      |                                                                                                                      |
| `acceptRfqQuote(quoteID)`       | `AcceptRFQQuote`        | -      |                                                                                                                      |
| `approveRfqOrder(params)`       | `ApproveRFQOrder`       | -      |                                                                                                                      |
| `rfqConfig()`                   | `GetRFQConfig`          | -      |                                                                                                                      |

---

## Task 6: Builder methods

**Depends on:** none
**Criteria:** All builder methods present with correct auth level.

| TS SDK                     | Go SDK                | Status | Notes |
| :------------------------- | :-------------------- | :----- | :---- |
| `createBuilderApiKey()`    | `CreateBuilderAPIKey` | -      |       |
| `getBuilderApiKeys()`      | `GetBuilderAPIKeys`   | -      |       |
| `revokeBuilderApiKey()`    | `RevokeBuilderAPIKey` | -      |       |
| `getBuilderTrades(params)` | `GetBuilderTrades`    | -      |       |
| `postHeartbeat(params)`    | `PostHeartbeat`       | -      |       |

---

## Task 7: TS SDK constructor options without Go equivalents

**Depends on:** tasks 1-6
**Criteria:** Decision recorded in `ai/DECISIONS.md` for each item below.

| TS SDK option   | Go SDK equivalent                              | Decision needed                                        |
| :-------------- | :--------------------------------------------- | :----------------------------------------------------- |
| `tickSizeTtlMs` | none — manual `ClearTickSizeCache` only        | Add TTL to caches, or document as intentional omission |
| `retryOnError`  | none                                           | Add retry config to `Config`, or out-of-scope for v1   |
| `throwOnError`  | n/a (Go uses error returns)                    | Confirm typed `APIError` struct is sufficient          |
| `getSigner`     | `BuilderAuth` interface covers builder signing | Confirm no generic remote signer use case exists       |

---

## Task 8: WebSocket parity

**Depends on:** none
**Criteria:** WS channel types and subscription message shapes match TS SDK.

- [ ] Confirm `SubscribeMarket` asset ID list matches TS `subscribe({type: "market", ...})`
- [ ] Confirm `SubscribeUser` auth payload matches TS user channel subscription
- [ ] Confirm `BookEvent`, `PriceChangeEvent`, `TickSizeChangeEvent`, `LastTradePriceEvent` field names and types
- [ ] Confirm `OrderEvent`, `TradeEvent` field names and types
- [ ] Confirm heartbeat interval matches TS SDK (10s)
- [ ] Check if TS SDK reconnects on disconnect — does `clob/ws` need reconnect logic?
