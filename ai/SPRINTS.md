## Sprint Index

| Sprint                                              | Goal                                                              | Status  |
| :-------------------------------------------------- | :---------------------------------------------------------------- | :------ |
| [01-parity-audit](sprints/01-parity-audit.md)       | Method-by-method TS SDK comparison, identify any gaps             | done |
| [02-logic-review](sprints/02-logic-review.md)       | Verify correctness of all critical algorithms and auth paths      | done |
| [03-type-wire-audit](sprints/03-type-wire-audit.md) | Confirm all request/response types match the live API wire format | done |
| Go-Expert Review                                    | Modernize to Go 1.26 idioms, clear gopls diagnostics              | done |
| 04-modernization                                    | Full migration to Go 1.26 `json/v2` and `omitzero` tags           | done |
| 05-bridge-ctf                                       | Implement Bridge and CTF APIs for full parity                     | done |
| 06-2026-standards                                   | Tiered architecture, Heartbeats, 15-batch limits                  | done |

## Notes

- Sprint 06 was a critical architectural refactor driven by March 2026 standards verified against the Rust SDK.
- "Demo" for each sprint = `make build && make test` passing with new or updated tests covering the area.
- Final Reference: TypeScript SDK v5.8.0 and Rust SDK (early 2026 Release).
