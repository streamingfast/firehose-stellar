# Firehose for Stellar

[![reference](https://img.shields.io/badge/godoc-reference-5272B4.svg?style=flat-square)](https://pkg.go.dev/github.com/streamingfast/firehose-stellar)

Quick start with Firehose for Stellar can be found in the official Firehose docs. Here some quick links to it:

- [Firehose Overview](https://firehose.streamingfast.io/introduction/firehose-overview)
- [Concepts & Architectures](https://firehose.streamingfast.io/concepts-and-architeceture)
  - [Components](https://firehose.streamingfast.io/concepts-and-architeceture/components)
  - [Data Flow](https://firehose.streamingfast.io/concepts-and-architeceture/data-flow)
  - [Data Storage](https://firehose.streamingfast.io/concepts-and-architeceture/data-storage)
  - [Design Principles](https://firehose.streamingfast.io/concepts-and-architeceture/design-principles)

## Running the Firehose fetcher

Two fetcher backends are available. Both emit the same `pbbstream.Block` shape; check `proto/sf/stellar/type/v1/block.proto` for the payload schema.

### RPC backend

Streams ledgers from a Stellar RPC endpoint.

```bash
firestellar fetch rpc {FIRST_STREAMABLE_BLOCK} --endpoints {STELLAR_RPC_ENDPOINT} --state-dir {STATE_DIR}
```

### Captive-core backend

Spawns a `stellar-core` subprocess and streams ledgers from it.

```bash
firestellar fetch captive-core {FIRST_STREAMABLE_BLOCK} \
  --stellar-core-bin /usr/bin/stellar-core \
  --stellar-core-network mainnet \
  --state-dir {STATE_DIR}
```

### Resume behavior (`--state-dir` / `--ignore-cursor`)

Both backends persist the last fired block to `{STATE_DIR}/cursor.json` after each successful emission. On restart, the fetcher resumes at `last_fired_block + 1` instead of replaying from `{FIRST_STREAMABLE_BLOCK}`.

- `--state-dir` — directory holding `cursor.json`. Defaults: `/data/poller` (rpc), `/data/captive-core` (captive-core). Pass an empty string to disable persistence.
- `--ignore-cursor` — ignore any persisted `cursor.json` and start fresh from `{FIRST_STREAMABLE_BLOCK}`. Use this when running under a supervisor (e.g. `firecore reader-node`) that already tracks downstream state and passes the correct start block on restart.

The cursor schema is shared between the two backends, so a single state directory can be reused if you switch backends.

## Contributing

For more information, please read the [CONTRIBUTING.md](CONTRIBUTING.md) file.
