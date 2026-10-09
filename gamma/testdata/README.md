# Gamma wire fixtures

These fixtures come from pinned upstream tests, not Go response structs.

- `rust-sports.json`: [`sports_should_succeed` in `tests/gamma.rs`](https://github.com/Polymarket/rs-clob-client-v2/blob/561830b9ee502c6e67cce314f0e53a80b8885b09/tests/gamma.rs). The payload is unchanged; indentation was normalized.
- `py-market.json` and `py-event.json`: [`test_market_normalizes_groups_from_flat_payload` and `test_event_normalizes_groups_from_flat_payload`](https://github.com/Polymarket/py-sdk/blob/ed8d04cade617d2f4c6f90ebe1842249eaf5c59b/tests/unit/test_gamma_models.py). Each fixture combines the test's minimal payload with its keyword overrides, preserving all values.

Keyset envelope, cursor, ordering, filter, and boundary expectations also follow the stable [TS discovery actions](https://github.com/Polymarket/ts-sdk/tree/087f9443635316e4e6be5dc4e22bd93f08cea443/packages/client/src/actions) and [Gamma bindings](https://github.com/Polymarket/ts-sdk/tree/087f9443635316e4e6be5dc4e22bd93f08cea443/packages/bindings/src/gamma). Their source references are noted beside the tests.
