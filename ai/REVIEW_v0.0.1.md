# Go SDK Review - v0.0.1 Pre-Release Audit

**Date:** 2026-03-16  
**Reviewed against:** TypeScript SDK (Polymarket/clob-client)  
**Go Version:** 1.26 with JSONv2 experiment

---

## Executive Summary

The Go SDK is well-structured and follows Go best practices. Core functionality (order creation, signing, market data) is implemented correctly with good parity to the TypeScript SDK. 

**Status after review:**
- Fixed: Tick size validation against market minimum
- Fixed: Cache clearing consistency
- All tests pass ✓

---

## FIXED ISSUES

### 1. Missing Tick Size Validation Against Market Minimum ✓ FIXED

**Location:** `clob/trading.go:570-596` - `resolveTickSize`

**Issue:** TypeScript validates that user-provided tickSize is >= market minimum. Added this validation to Go SDK.

**Fix:** Added `isTickSizeSmaller` helper and validation in `resolveTickSize`.

### 2. Cache Clearing Consistency ✓ FIXED

**Location:** `clob/client.go:94-135`

**Issues fixed:**
- `ClearTickSizeCache` now clears tickSize, negRisk, and feeRate caches for the token (was only clearing tickSize)
- Added `ClearNegRiskCache` for symmetry with `ClearFeeRateCache`

---

## VERIFIED CORRECT

### Order Building Logic ✓

- **Limit order amounts:** BUY: `takerAmount = size * price`, SELL: `makerAmount = size` - correct
- **Market order amounts:** BUY: `takerAmount = makerAmount / price`, SELL: `takerAmount = makerAmount * price` - correct  
- **Rounding config:** Matches TypeScript exactly (0.1, 0.01, 0.001, 0.0001)
- **Token scale factor:** 10^6 matches TypeScript (`COLLATERAL_TOKEN_DECIMALS = 6`)

### Contract Addresses ✓

All addresses match TypeScript for chain 137 (Polygon) and 80002 (Amoy).

### Signature Types ✓

`SignatureTypeEOA = 0`, `SignatureTypePolyProxy = 1`, `SignatureTypePolyGnosisSafe = 2` - correct

### Auth Headers ✓

L1, L2, and L2Builder auth all implemented correctly with correct header names.

### Pagination ✓

Cursors match (`MA==`, `LTE=`).

### Price Validation ✓

`validatePrice` correctly enforces: `tickSize <= price <= 1 - tickSize`

### Market Price Calculation ✓

`CalculateMarketPrice` correctly handles:
- BUY: accumulate `size * price` across asks (descending)
- SELL: accumulate `size` across bids (ascending)
- FOK: error on insufficient liquidity
- GTC: fallback to worst price

### WebSocket ✓

Ping/pong, reconnection handling, event parsing all look correct.

---

## VERIFICATION COMMANDS

All pass:
```bash
make fmt    # Format check
make build  # Builds successfully  
make test   # All tests pass
```

---

## CONCLUSION

The SDK is **ready for v0.0.1 release** after the fixes. The core trading logic is correct and matches the TypeScript SDK behavior.
