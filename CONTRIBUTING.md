# Contributing to go-clob-client

Contributions are welcome. Please follow these guidelines:

## Getting started

1. Fork the repository and create a branch from `main`.
2. Install Go 1.26.1+ and the formatting tools:
   ```bash
   go install golang.org/x/tools/cmd/goimports@latest
   go install github.com/segmentio/golines@v0.13.0
   go install mvdan.cc/gofumpt@v0.9.2
   ```
3. Make your changes, then run:
   ```bash
   make fmt    # format code
   make vet    # static analysis
   make test   # run tests
   make build  # verify build
   ```

## Guidelines

- Match the style of the surrounding code.
- Add tests for new behavior, especially for auth flows, order construction, and request/response shaping.
- When adding new API endpoints, compare the current Rust, TypeScript, and Python SDKs using the
  tiered oracle model: Rust anchors core mechanics and stable non-perps contracts; TS/Python fill
  newer or broader surfaces. Express the result idiomatically in Go.
- Prefer package-level separation (`clob/`, `data/`, `gamma/`, `bridge/`) over growing one package too wide.
- Avoid gratuitous compatibility shims; preserve wire compatibility when a migration requires it,
  and prefer clean breaks for pre-1.0 Go interfaces.
- Keep perps as a separate package and keep wallet-model-specific on-chain operations explicit.
- Reference the [Rust V2 SDK](https://github.com/Polymarket/rs-clob-client-v2), [TypeScript SDK](https://github.com/Polymarket/ts-sdk), and [Python SDK](https://github.com/Polymarket/py-sdk) for parity questions.

## Commit messages

Write concise messages explaining *why* the change is made, not just what it does.

## Opening a pull request

- Ensure all CI checks pass before requesting review.
- Link any related issues in the PR description.
