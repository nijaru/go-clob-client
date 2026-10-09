# Live sports streams

`sports` receives public game-result updates from
`wss://sports-api.polymarket.com/ws`. It is separate from Gamma sports REST,
CLOB WebSockets, legacy RTDS, and authenticated Polybolt price streaming.

```go
stream, err := sports.Dial(ctx, sports.Config{})
if err != nil {
    return err
}
defer stream.Close()
for event := range stream.Events() {
    fmt.Println(event.GameID, event.LeagueAbbreviation, event.Status, event.Score)
}
return stream.Err()
```

The context owns the entire stream lifetime, not just the upgrade. `Close` is
concurrent-safe and joins reads, writes, socket cleanup, replacement upgrades,
and reconnect delays. `Done` signals completed cleanup; `Err` reports termination.
Explicit close returns nil; context cancellation/deadline and a full event queue
return errors. Buffered events remain readable after termination.

## Wire contract

- Public connection; no authentication, initial subscription message, filters,
  unsubscribe message, acknowledgement, or resume token. Connecting receives all
  games. Closing the connection unsubscribes.
- The server sends literal lowercase text `ping`; the client responds with
  literal text `pong`. No proactive application ping is sent. RFC 6455 control
  frames are handled by `coder/websocket`, independently of this heartbeat.
- Only an application `ping` resets the 30-second stale deadline. Game traffic
  does not. Disconnects (including normal remote closes) and stale connections
  reconnect using full-jitter exponential backoff: 250 ms base, 30-second ceiling,
  reset after a successful upgrade. Failed initial upgrades return immediately.
- Messages are flat JSON objects, not topic/type envelopes. Required fields:
  `gameId`, `leagueAbbreviation`, `status`, `live`, `ended`, `score`.
  Optional nullable fields: `sportradarGameId`, `slug`, `homeTeam`, `awayTeam`,
  `period`, `elapsed`, `finishedTimestamp`, `finished_timestamp`, `turn`.
- `Event` embeds typed `GameResult` fields and preserves the complete payload in
  `Raw`, including unknown metadata. Both finished-timestamp spellings retain
  their original JSON independently; numeric/string units are not inferred.
  Timestamp strings remain opaque. Status and score strings are not normalized.
- Malformed/unrelated messages are skipped and counted in `Stats`. Reconnect
  count and the last transport/heartbeat error are also available there. Upgrade
  errors retain HTTP status; wrapped WebSocket errors retain close code/reason.

One `Dial` has one queue and one lifecycle owner. Multiple readers compete for
that queue; they do not fan out. Use separate streams or application-owned fanout
when needed. No delivery guarantee, replay, gap detection, deduplication, or
cross-package unified subscription manager is provided by this package.

The default queue holds 1024 events and the message limit is 1 MiB, both
configurable. Overflow terminates with `ErrSlowConsumer` instead of dropping
updates or delaying pongs. Transport timeouts and reconnect bounds are configurable;
zero values use defaults. Headers are copied and apply only to the upgrade.

## Merged reference contracts and fixtures

Implementation references, pinned to merged default-branch commits:

- [TS sports manager, 087f9443](https://github.com/Polymarket/ts-sdk/blob/087f9443/packages/client/src/websockets/sports.ts),
  [wire schema](https://github.com/Polymarket/ts-sdk/blob/087f9443/packages/bindings/src/subscriptions/sports.ts),
  [heartbeat](https://github.com/Polymarket/ts-sdk/blob/087f9443/packages/client/src/websockets/heartbeat.ts),
  [host](https://github.com/Polymarket/ts-sdk/blob/087f9443/packages/client/src/environments.ts),
  [reconnect lifecycle](https://github.com/Polymarket/ts-sdk/blob/087f9443/packages/client/src/websockets/lifecycle.ts).
- [Python manager, ed8d04ca](https://github.com/Polymarket/py-sdk/blob/ed8d04ca/src/polymarket/_internal/streams/sports/manager.py),
  [event model](https://github.com/Polymarket/py-sdk/blob/ed8d04ca/src/polymarket/models/sports_events.py),
  [heartbeat](https://github.com/Polymarket/py-sdk/blob/ed8d04ca/src/polymarket/_internal/streams/sports/heartbeat.py),
  [host](https://github.com/Polymarket/py-sdk/blob/ed8d04ca/src/polymarket/environments.py),
  [backoff](https://github.com/Polymarket/py-sdk/blob/ed8d04ca/src/polymarket/_internal/ws/backoff.py).

`testdata/ts-minimal.json` copies the game object in TS
[`sports.test.ts`](https://github.com/Polymarket/ts-sdk/blob/087f9443/packages/client/src/websockets/sports.test.ts).
`testdata/python-game.json` copies `_WIRE` in Python
[`test_streams_sports_events.py`](https://github.com/Polymarket/py-sdk/blob/ed8d04ca/tests/unit/test_streams_sports_events.py).
Timestamp cases use the literals from that same Python test file. Tests decode
these independent upstream fixtures and exercise real in-process WebSockets;
they are not captures from a live server.

## Local example

From the repository root, run `go run ./examples/sports`. The default starts an
in-process fixture, exchanges a heartbeat, prints one result, and joins cleanup.
It requires no credentials and makes no remote connection. To explicitly read
one result from the public live feed:

```sh
go run ./examples/sports -url wss://sports-api.polymarket.com/ws -timeout 1m
```
