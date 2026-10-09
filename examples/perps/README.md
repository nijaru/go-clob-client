# Perps examples

From the repository root:

```sh
go run ./examples/perps
go run ./examples/perps -stream
```

The example prints the first instrument, its ticker/book and recent hourly
candles. `-stream` then creates a public WebSocket pool and reads BBO updates
through a context-owned handle. Other handles can share that pool and be canceled
independently. It stops on Ctrl-C or after 30 seconds. It never submits orders, creates credentials or sends transactions.

Optional `PERPS_PROXY` and `PERPS_SECRET` environment variables enable an account
balance read; no private key is needed. Set them securely in your environment,
not in source or shell history. The example does not print them.

`PERPS_HOST` and `PERPS_WS_HOST` override the REST and WebSocket hosts for local
fixtures or another configured environment.

See [`../../perps/README.md`](../../perps/README.md) for credential lifecycle,
trading/TP-SL, managed execution, builder consent, collateral boundaries and the
pinned upstream capability inventory. Those operations are intentionally not
executed by the read-only example.

## Collateral preparation and execution

`walletdeposit` defaults to unsigned preparation with no network requests.
This **public, unfunded test key and dummy contracts** demonstrate the ABI only;
never fund this key or use these addresses for real collateral:

```sh
PRIVATE_KEY=0000000000000000000000000000000000000000000000000000000000000001 \
COLLATERAL_TOKEN=0x0000000000000000000000000000000000000002 \
PERPS_DEPOSIT_CONTRACT=0x0000000000000000000000000000000000000004 \
go run ./examples/perps/walletdeposit -wallet safe -amount 100000000
```

Preparation does not need `RPC_URL`, CLOB credentials or builder credentials.
`-wallet deposit` selects offline beacon derivation unless `WALLET_ADDRESS` is
explicit. `-action discover-deposit` performs read-only legacy-first discovery
through `RELAYER_HOST` (default: production), prints the address, and does not
construct a managed execution client or deploy anything.

Mutations require explicit `-action approve`, `deposit`, `approve-deposit`, or
`deploy`. Deployment selects Safe or beacon creation according to `-wallet`.
Use your own signer, verified contracts and `RPC_URL`; managed execution also
does not require CLOB credentials. Smart wallets use separate
`BUILDER_API_KEY`, `BUILDER_API_SECRET`, and
`BUILDER_API_PASSPHRASE`. The example passes `clob.Config.Signer`, not exported
key material; replace `signing.NewLocalSigner` with your device/service signer.
EOA sends use its `signing.TransactionSigner` capability with SDK broadcasting,
or its optional `signing.TransactionSender` for wallet-owned sending.

On errors, the example prints every retained hash, including uncertain sends.
Reconcile those submissions before retrying. Receipt success does not prove
perps ledger credit. No mutation is required to run the offline example.
