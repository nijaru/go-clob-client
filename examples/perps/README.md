# Read-only Perps example

From the repository root:

```sh
go run ./examples/perps
go run ./examples/perps -stream
```

The example prints the first instrument, its ticker/book and recent hourly
candles. `-stream` then reads public BBO updates. It stops on Ctrl-C or after
30 seconds. It never submits orders, creates credentials or sends transactions.

Optional `PERPS_PROXY` and `PERPS_SECRET` environment variables enable an account
balance read; no private key is needed. Set them securely in your environment,
not in source or shell history. The example does not print them.

`PERPS_HOST` and `PERPS_WS_HOST` override the REST and WebSocket hosts for local
fixtures or another configured environment.

See [`../../perps/README.md`](../../perps/README.md) for credential lifecycle,
trading/TP-SL, managed execution, builder consent, collateral boundaries and the
pinned upstream capability inventory. Those operations are intentionally not
executed by this example.
