# go-clob-client

Go SDK for the Polymarket CLOB. **Reference SDKs (tiered oracle model):**
- **Rust `rs-clob-client-v2`** — parity anchor for core mechanics (EIP-712 signing, wire format, CTF ABI, fee math) **and** the entire non-perps surface (clob/data/gamma/bridge/ctf/rtds/rfq). It is now a *unified* SDK, not CLOB-only.
- **TS `ts-sdk`** — oracle for **perps** (added Jun 2026, absent from Rust) and the newest surface.
- **py `py-sdk`** — surface-breadth oracle for endpoints Rust lags.

> History: the original "Rust is the sole reference" framing dates from the v1 era, when the only
> official clients were three language ports of the v1 CLOB API and Rust was the closest match to an
> idiomatic Go client. Post-v2, Rust is unified and py/ts are mature peers — so we target the
> **union**, weighted by tier (see Development Workflow below).

## Project Structure

| Directory | Purpose |
| --------- | ------- |
| `clob/` | Public CLOB SDK package |
| `data/` | Public read-only Data API package |
| `gamma/` | Public Gamma markets/events/tags package |
| `bridge/` | Public deposit-address discovery package |
| `perps/` | Public perpetuals market-data and account package |
| `internal/polyauth/` | Shared Polymarket auth and signing logic |
| `internal/polyhttp/` | Shared HTTP transport and response handling |
| `examples/` | Runnable examples grouped by API family |

### Working state (local-only)

**Purpose:** Keep execution state out of public git history. The session is the default
working memory; these paths are excluded via `.git/info/exclude`.

- `.tasks/` - deferred/live-validation backlog in the original `tk` v0 JSON layout
  (one file per task). Current `tk` (0.2.x, format-3 reader) cannot parse it, so use
  `tk ready` only after a deliberate store migration; until then inspect the task
  files directly. Do not bulk-convert or close tasks as incidental cleanup.
- Prior `ai/` session files were removed from tracking and are not recreated here.
  Durable private context lives in knowledge at
  `~/github/nijaru/knowledge/projects/go-clob-client/`; older history is preserved
  under `~/github/nijaru/agent-context/projects/github.com/nijaru/go-clob-client/`.

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

## Go Idioms

Use the `go-reference` skill when work depends on current release behavior or version gates. Key modern idioms:

- `slices` / `maps` packages — not manual loops or `sort.Slice`
- `iter.Seq` / `iter.Seq2` — range-over-function iterators (Go 1.23+)
- `sync.WaitGroup.Go` — replaces `Add(1); go func() { defer Done() }()`
- `errors.AsType[T](err)` — type-safe error unwrapping (Go 1.26)
- `t.Context()` in tests — not `context.TODO()`

## Commands

```bash
# Format
make fmt

# Static analysis
make vet

# Test
make test

# Build
make build

# Tidy module metadata
go mod tidy
```

## Verification Steps

Commands that should pass before shipping:

- Build: `make build`
- Tests: `make test`
- Vet: `make vet`
- Format: `make fmt`

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
| Read-only client | `client, err := clob.NewClient(clob.Config{})` |
| Signing client | `client, err := clob.NewSignerClient(clob.Config{ChainID: clob.PolygonChainID, PrivateKey: key})` |
| Authenticated client | `client, err := clob.NewAuthenticatedClient(clob.Config{ChainID: clob.PolygonChainID, PrivateKey: key, Credentials: creds})` |
| API key bootstrap | `creds, err := client.CreateOrDeriveAPIKey(ctx, 0)` |

## Development Workflow

1. Compare behavior against the reference SDKs using the tiered oracle model: Rust v2 for core mechanics + non-perps surface, TS for perps, py for surface breadth.
2. Record consequential design decisions in `~/github/nijaru/knowledge/projects/go-clob-client/decisions/` before broadening the surface.
3. Implement one coherent API slice at a time with tests and an example.
4. Keep `README.md` up to date as public capabilities, examples, status, or limitations change.
5. Run `make fmt`, `make test`, and `make build`.
6. Note deferred follow-ups in `.tasks/`; keep the session record current within the session.

## Current Focus

Active work lives in the session. Deferred follow-ups are tracked in local `.tasks/`;
orientation and history pointers are in
`~/github/nijaru/knowledge/projects/go-clob-client/README.md`.
