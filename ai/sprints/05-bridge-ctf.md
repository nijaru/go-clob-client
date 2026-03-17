---
description: Implement Bridge and CTF APIs for full parity
---

## Goals

- [ ] Implement Bridge API client in `bridge/` package
- [ ] Implement CTF API client (split/merge/redeem) in `clob/` or `ctf/` package
- [ ] Add examples for both
- [ ] Ensure full `json/v2` compliance with `omitzero` tags

## Bridge API Endpoints

- `GET /supported-assets`: List all supported chains and tokens for deposits
- `POST /deposit`: Generate unique deposit addresses
- `GET /deposit-status`: Check status of a deposit
- `GET /quote`: Get a quote for a bridge/swap
- `POST /withdrawal`: Create a withdrawal request

Base URL: `https://bridge.polymarket.com`

## CTF API Operations (On-chain)

- `Split`: USDC.e -> Yes + No
- `Merge`: Yes + No -> USDC.e
- `Redeem`: Winning Token -> USDC.e

These will likely be implemented as methods on a `CTFClient` or added to `clob.Client` if they use the same auth/transport.
Polymarket's Relayer API handles these gaslessly.

## Tasks

- [ ] Create `bridge` package
- [ ] Create `ctf` package (or add to `clob`)
- [ ] Implement types for both
- [ ] Implement client methods
- [ ] Add unit tests with mocks/fixtures
- [ ] Add runnable examples
