# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-03-16

### Added
- Feature parity with Polymarket TypeScript SDK v5.8.0.
- `clob` package: Full REST and WebSocket support, including RFQ and CTF (split/merge/redeem).
- `gamma` package: Market discovery and metadata.
- `data` package: User analytics and leaderboards.
- `bridge` package: Cross-chain deposits and withdrawals.
- `internal/polyauth`: Centralized EIP-712 signing and builder auth logic.
- `internal/polyhttp`: Modernized HTTP client with `json/v2` integration.
- High-performance decimal arithmetic using `udecimal`.
- Modernized Go 1.26 stack with `omitzero` and `jsontext`.

### Fixed
- Deterministic fixture coverage for market orders and proxy behaviors.
- Race conditions in credential management.
- Cache synchronization for trading metadata.

### Security
- Builder API key support for gasless and sponsored trades.
- Remote builder signing integration for secure EOA separation.
