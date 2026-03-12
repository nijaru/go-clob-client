# go-clob-client

Go SDK for the Polymarket CLOB, modeled after the official TypeScript, Python, and Rust clients.

## Project Structure

| Directory | Purpose |
| --------- | ------- |
| `clob/` | Public CLOB SDK package |
| `internal/polyauth/` | Shared Polymarket auth and signing logic |
| `internal/polyhttp/` | Shared HTTP transport and response handling |
| `examples/` | Runnable examples grouped by API family |
| `ai/` | Local-only AI session context excluded via `.git/info/exclude` |
| `.tasks/` | Local-only task tracker state excluded via `.git/info/exclude` |

### AI Context Organization

**Purpose:** Keep project state between sessions without polluting public git history.

**Session files** (local only):

- `ai/STATUS.md` - current state, blockers, active work
- `ai/DESIGN.md` - package layout and SDK architecture
- `ai/DECISIONS.md` - append-only design decisions
- `ai/ROADMAP.md` - phased plan for broader endpoint coverage

**Reference files** (local only):

- `ai/research/` - notes from API and SDK comparisons
- `ai/design/` - deeper component notes
- `ai/tmp/` - scratch artifacts

**Task tracking:** `tk` CLI with `.tasks/` kept local-only. Use `tk ready` to find pending work.

## Technology Stack

| Component | Technology |
| --------- | ---------- |
| Language | Go |
| Module path | `github.com/nijaru/go-clob-client` |
| First public package | `github.com/nijaru/go-clob-client/clob` |
| HTTP | `net/http` |
| Ethereum signing | `github.com/ethereum/go-ethereum` |
| Testing | `go test` |
| Formatting | `golines --base-formatter gofumpt` |

## Commands

```bash
# Format
golines --base-formatter gofumpt -w .

# Test
go test ./...

# Build example
go build ./...

# Tidy module metadata
go mod tidy
```

## Verification Steps

Commands that should pass before shipping:

- Build: `go build ./...`
- Tests: `go test ./...`
- Format: `golines --base-formatter gofumpt -w .`

## Code Standards

| Aspect | Standard |
| ------ | -------- |
| Package design | Small cohesive files, functional core around signing/serialization |
| Errors | Return typed errors, avoid swallowing HTTP/API details |
| Auth | Mirror Polymarket L1/L2 header semantics from the reference SDKs |
| JSON | Prefer structs for stable wire format; use `json.RawMessage` for unstable API payloads |
| Public API | Favor explicit request/response types over loose maps where schema is stable |

## Examples

| Pattern | Example |
| ------- | ------- |
| Read-only client | `client, err := clob.New(clob.Config{})` |
| Authenticated client | `client, err := clob.New(clob.Config{ChainID: clob.PolygonChainID, PrivateKey: key})` |
| API key bootstrap | `creds, err := client.CreateOrDeriveAPIKey(ctx, 0)` |

## Development Workflow

1. Compare behavior against the TypeScript, Python, and Rust clients.
2. Record design decisions in local `ai/` files before broadening the surface.
3. Implement one coherent API slice at a time with tests and an example.
4. Keep `README.md` up to date as public capabilities, examples, status, or limitations change.
5. Run `go test ./...` and `go build ./...`.
6. Update local `ai/STATUS.md` and task logs with what changed.

## Current Focus

See local `ai/STATUS.md` for active work and `ai/DESIGN.md` for the package plan.
