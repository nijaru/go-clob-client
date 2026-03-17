# Sprint 03 — Type & Wire Format Audit

**Goal:** Confirm every request and response struct matches the live API wire format, catching missing fields, wrong JSON keys, and wrong Go types.

**Demo:** Any struct mismatch produces a new test that fails before the fix and passes after.

---

## Task 1: `BookParams` batch endpoint field coverage

**File:** `clob/market_types.go`
**Depends on:** none
**Criteria:**

- `BookParams{TokenID, Side}` — confirm `Side` is omitted from JSON when empty (omitempty)
- Confirm `POST /books` does NOT require Side (orderbook endpoint) while `POST /prices` DOES
- Confirm `POST /midpoints`, `POST /spreads`, `POST /last-trades-prices` do not use Side
- Test: serialize `BookParams{TokenID: "x"}` and assert no `"side"` key in JSON output

---

## Task 2: `Market` and `SimplifiedMarket` field coverage

**File:** `clob/market_types.go`
**Depends on:** none
**Criteria:**

- Compare all fields in `Market` and `SimplifiedMarket` against the TS SDK `Market` type
- Identify any fields present in the TS type that are missing from the Go struct (they'd be silently dropped on decode)
- Pay particular attention to: `closed`, `archived`, `accepting_order_timestamp`, `minimum_order_size`, `minimum_tick_size`, `condition_id`, `question_id`, `tokens`, `rewards`, `end_date_iso`, `game_start_time`
- Note any fields that exist in Go but not in TS (Go additions are fine, document them)

---

## Task 3: `OpenOrder` and `OpenOrderParams` field coverage

**File:** `clob/order_types.go`
**Depends on:** none
**Criteria:**

- `OpenOrderParams`: confirm all filter fields (`OrderID`, `Market`, `AssetID`) match API query param names
- `OpenOrder` response struct: confirm all fields present (`order_id`, `asset_id`, `status`, `outcome`, `original_size`, `remaining_size`, `matched_amount`, `price`, `side`, `created_at`, `expiration`, `order_type`, `associate_trades`, `maker_address`)
- Test: decode a fixture JSON response and assert key fields populated

---

## Task 4: `Trade` and `TradeParams` field coverage

**File:** `clob/order_types.go`
**Depends on:** none
**Criteria:**

- `TradeParams`: `ID`, `MakerAddress`, `Market`, `AssetID` — verify query param names match API
- `Trade` response: confirm all fields (`id`, `taker_order_id`, `market`, `asset_id`, `side`, `size`, `fee_rate_bps`, `price`, `status`, `match_time`, `last_update`, `outcome`, `bucket_index`, `owner`, `maker_orders`, `type`)
- Test: decode a fixture JSON trade response

---

## Task 5: `PricesResponse` and `MidpointsResponse` decode

**File:** `clob/market_types.go`
**Depends on:** none
**Criteria:**

- `PricesResponse` is `map[string]map[Side]string` — wire format is `{"token_id": {"BUY": "0.41", "SELL": "0.59"}}`
- `MidpointsResponse` is `map[string]string` — wire format is `{"token_id": "0.50"}`
- `SpreadsResponse` is `map[string]string` — wire format is `{"token_id": "0.18"}`
- Test: decode known JSON fixtures into each type and assert correct values

---

## Task 6: `BalanceAllowanceParams` and `BalanceAllowanceResponse`

**File:** `clob/auth_types.go` or `clob/market_types.go`
**Depends on:** none
**Criteria:**

- Request params include `asset_type` and `token_id` — confirm query param names
- Response includes `balance` and `allowance` — confirm field names
- `AssetType` enum values match API strings (`COLLATERAL`, `CONDITIONAL`)

---

## Task 7: `Notification` and `DeleteNotificationsParams`

**File:** `clob/auth_types.go`
**Depends on:** none
**Criteria:**

- `Notification` struct fields match TS SDK `Notification` type
- `DeleteNotificationsParams` body fields match `POST /notifications` delete shape
- `DropNotifications` (alias) delegates correctly to `DeleteNotifications`

---

## Task 8: `RewardRate`, `CurrentReward`, `RewardsPercentages` coverage

**File:** `clob/reward_types.go`
**Depends on:** none
**Criteria:**

- All reward response types have correct JSON keys
- Paginated reward responses include `data` and `next_cursor` wrapped by `Page[T]`
- `GetRawRewardsForMarket` vs `GetRewardsForMarket` — confirm both return the same underlying struct

---

## Task 9: `BuilderAPIKey` and builder types

**File:** `clob/builder_types.go`
**Depends on:** none
**Criteria:**

- `BuilderAPIKey` fields match TS SDK builder key response
- `PostHeartbeat` payload and response match API

---

## Task 10: `RFQRequest` and `RFQQuote` wire format

**File:** `clob/rfq_types.go`
**Depends on:** none
**Criteria:**

- `RFQRequest`: confirm `requestId` vs `id` in response — the HTTP API returns `requestId` not `id` for POST /rfq/request
- `RFQQuote`: confirm `quoteId` vs `id`
- `AcceptRFQQuoteResponse`: API returns `tradeIds []string` not `order SignedOrder` — **HIGH RISK**: verify current struct is correct
- `ApproveRFQOrderRequest`: confirm body fields match API

**Technical notes:**

- From the API docs: `POST /rfq/accept` response is `{"tradeIds": [...]}` — if our `AcceptRFQQuoteResponse` has `Order SignedOrder` this may be wrong. Needs verification against the live API or TS SDK source.

---

## Task 11: `GeoblockResponse` field coverage

**File:** `clob/market_types.go`
**Depends on:** none
**Criteria:**

- `GeoblockResponse` matches the `polymarket.com/api/geoblock` response shape
- `CheckGeoblock` uses the geoblock HTTP client (separate host), not the CLOB host

---

## Task 12: `WS` event types coverage

**File:** `clob/ws/types.go`
**Depends on:** none
**Criteria:**

- `BookEvent` fields: `asset_id`, `market`, `timestamp`, `hash`, `bids`, `asks`
- `PriceChangeEvent` fields: `asset_id`, `price`, `side`
- `TickSizeChangeEvent` fields: `asset_id`, `tick_size`
- `LastTradePriceEvent` fields: `asset_id`, `price`, `side`
- `OrderEvent` fields: all order fields as streamed by the user channel
- `TradeEvent` fields: all trade fields as streamed by the user channel
- Test: decode fixture JSON for each event type
