# Sprint 02 — Logic & Correctness Review

**Goal:** Read and verify every critical algorithm and auth path for bugs, edge cases, and divergence from the reference SDK.

**Demo:** All findings documented here with pass/fail verdict; any bugs produce a failing test before and a fix after.

---

## Task 1: EIP-712 order signing

**File:** `clob/trading.go` (`signOrder`, `buildSignedLimitOrder`, `buildSignedMarketOrder`)
**Depends on:** none
**Criteria:**

- Domain hash (`protocolName`, `protocolVersion`, `chainID`, `verifyingContract`) matches the deployed exchange contract for both Polygon (137) and Amoy (80002)
- Order type hash field order matches the contract ABI exactly
- `makerAmount`/`takerAmount` are computed correctly from `price * size` for both BUY and SELL
- Neg-risk path uses the correct exchange address
- Salt is a valid uint256 (no overflow)
- Test: sign a known order against a known expected signature hash

**Technical notes:**

- Reference: TS SDK `src/order-builder/exchange.order.builder.ts`
- Contract addresses: `clob/contracts.go`
- Neg-risk contract address is separate — verify `GetExchangeAddress(negRisk bool)` returns the right one

---

## Task 2: `CalculateMarketPrice` traversal

**File:** `clob/trading.go:220`
**Depends on:** none
**Criteria:**

- BUY fills from the asks side (lowest ask first)
- SELL fills from the bids side (highest bid first)
- Fills accumulate until `amount` is satisfied or book is exhausted
- Returned price is the weighted average (or worst fill price — clarify which matches TS SDK)
- FOK vs FAK handling: FOK must fill completely or return error; FAK partial fill is OK
- Test: synthetic orderbook with known prices and sizes, assert correct market price

**Technical notes:**

- TS SDK reference: `src/http-helpers/index.ts` → `calculateMarketPrice`
- Current implementation: `clob/trading.go:220` — read carefully for off-by-one errors in price accumulation

---

## Task 3: `resolveFeeRateBps` validation logic

**File:** `clob/trading.go:631`
**Depends on:** none
**Criteria:**

- When market fee rate is 0 (no fee), any user-supplied fee is accepted (or rejected?)
- When user supplies 0, market rate is used
- When user supplies non-zero and market rate is non-zero and they differ, error is returned
- When market fee API call fails, error propagates correctly
- Test: unit test all four combinations above

**Technical notes:**

- Current condition: `if marketFeeRateBps > 0 && userFeeRateBps != 0 && userFeeRateBps != marketFeeRateBps`
- Compare against TS SDK handling in `src/order-builder/builder.ts`

---

## Task 4: L1 auth header generation

**File:** `internal/polyauth/`
**Depends on:** none
**Criteria:**

- `POLY_ADDRESS` is the checksummed Ethereum address
- `POLY_SIGNATURE` is the keccak256 hash of `{timestamp}{nonce}` signed with the private key (EIP-191 personal sign)
- `POLY_TIMESTAMP` is a Unix timestamp in seconds as a string
- `POLY_NONCE` is the nonce as a string
- `POLY_CHAIN_ID` is the chain ID as a string
- Test: known private key → known address and signature

**Technical notes:**

- Reference: TS SDK `src/headers/index.ts`
- Verify the exact message being signed matches `keccak256(timestamp + nonce)` or the EIP-191 variant

---

## Task 5: L2 auth header generation

**File:** `internal/polyauth/`
**Depends on:** none
**Criteria:**

- HMAC-SHA256 is computed over `{timestamp}{method}{path}{body}`
- Signature is base64-encoded (standard or URL-safe? verify)
- `POLY_API_KEY`, `POLY_SIGNATURE`, `POLY_TIMESTAMP`, `POLY_PASSPHRASE` headers all present
- Test: known key/secret/passphrase, known timestamp and payload → known expected HMAC

**Technical notes:**

- Reference: TS SDK `src/headers/index.ts` → `createL2Headers`
- The body included in the HMAC is the raw JSON bytes — verify this matches for GET requests (empty body) vs POST

---

## Task 6: Order amount calculations (makerAmount/takerAmount)

**File:** `clob/trading.go` (`buildSignedLimitOrder`, `buildSignedMarketOrder`)
**Depends on:** task 1
**Criteria:**

- Limit BUY: `makerAmount = price * size * 1e6` (USDC collateral scaled), `takerAmount = size * 1e6` (outcome tokens scaled)
- Limit SELL: `makerAmount = size * 1e6` (outcome tokens), `takerAmount = price * size * 1e6` (USDC)
- Market FOK/FAK: amounts computed from `CalculateMarketPrice` result
- Rounding: all amounts use the tick-size-specific rounding config from `roundingConfig`
- Test: known price/size/side/tick-size → known expected makerAmount and takerAmount

**Technical notes:**

- `tokenScaleFactor = 1e6` confirmed
- `roundingConfig` maps tick sizes to decimal precision; verify scale values match TS SDK

---

## Task 7: Pagination cursor logic

**File:** `clob/pagination.go`
**Depends on:** none
**Criteria:**

- `initialCursor` and `endCursor` constants match API sentinel values
- `nextPageCursor` correctly detects the terminal cursor and returns `done = true`
- All paginated `GetXxx` convenience methods terminate correctly on the last page
- Test: mock server that returns 2 pages then end cursor; assert all records collected

---

## Task 8: `CancelOrder` vs `CancelOrders` response type

**File:** `clob/orders.go`
**Depends on:** none
**Criteria:**

- `CancelOrder` returns `*CancelOrdersResponse` — verify the single-cancel API actually returns the same shape as the bulk cancel
- If single-cancel returns a different shape, split the response type
- Check `CancelMarketOrders` response type too

---

## Task 9: `UpdateBalanceAllowance` auth level

**File:** `clob/account.go`
**Depends on:** none
**Criteria:**

- Verify `UpdateBalanceAllowance` uses `AuthL2` not `AuthL1` or `AuthNone`
- Verify all account-mutating methods use the right auth level vs the TS SDK
- Cross-check every method's `AuthLevel` arg against the TS SDK docs

---

## Task 10: `SignedOrder` salt format on unmarshal

**File:** `clob/order_types.go`
**Depends on:** none
**Criteria:**

- Salt in API responses can be a JSON number (integer) or a JSON string
- Current `SignedOrder` salt field handles both forms without error
- Test: decode a response with numeric salt; decode a response with string salt; assert both parse correctly
