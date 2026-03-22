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
- Add tests for new behaviour, especially for auth flows and order construction.
- When adding new API endpoints, mirror the naming conventions in the existing `clob/` package.
- Keep public API changes minimal and backwards-compatible where possible.
- Reference the [Rust SDK](https://github.com/Polymarket/rs-clob-client) for API parity questions.

## Commit messages

Write concise messages explaining *why* the change is made, not just what it does.

## Opening a pull request

- Ensure all CI checks pass before requesting review.
- Link any related issues in the PR description.
