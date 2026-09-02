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

> **Captive-core is the supported backend going forward.** The RPC poller is kept for compatibility but is no longer actively developed — new deployments should use captive-core.

### Captive-core backend (recommended)

Spawns a `stellar-core` subprocess and streams ledgers from it.

```bash
firestellar fetch captive-core {FIRST_STREAMABLE_BLOCK} \
  --stellar-core-bin /usr/bin/stellar-core \
  --stellar-core-network mainnet \
  --state-dir {STATE_DIR}
```

### RPC backend (legacy)

Streams ledgers from a Stellar RPC endpoint. Maintenance-only — prefer captive-core for new work.

```bash
firestellar fetch rpc {FIRST_STREAMABLE_BLOCK} --endpoints {STELLAR_RPC_ENDPOINT} --state-dir {STATE_DIR}
```

### Resume behavior (`--state-dir` / `--ignore-cursor`)

Both backends persist the last fired block to `{STATE_DIR}/cursor.json` after each successful emission. On restart, the fetcher resumes at `last_fired_block + 1` instead of replaying from `{FIRST_STREAMABLE_BLOCK}`.

- `--state-dir` — directory holding `cursor.json`. Default: `/data/work` (both backends). Pass an empty string to disable persistence.
- `--ignore-cursor` — ignore any persisted `cursor.json` and start fresh from `{FIRST_STREAMABLE_BLOCK}`. Use this when running under a supervisor (e.g. `firecore reader-node`) that already tracks downstream state and passes the correct start block on restart.

The cursor schema is shared between the two backends, so a single state directory can be reused if you switch backends.

## Releasing

A release is a tag push. `.github/workflows/release.yml` builds the binaries, pushes the
images, creates the GitHub release and opens the Homebrew formula PR. Nothing is released
from a developer machine — do not run `sfreleaser release`.

1. Retitle the `## Unreleased` section of [CHANGELOG.md](CHANGELOG.md) to `## vX.Y.Z` and
   merge that. The tag run reads its release notes from that section and fails before
   building anything if it is missing.
2. Tag `main` and push it:

   ```bash
   git tag vX.Y.Z && git push origin vX.Y.Z
   ```

3. Merge the formula PR the run opens against `streamingfast/homebrew-tap`. It is not
   auto-merged, and until it lands `brew install` serves the previous version.

`workflow_dispatch` runs the full build without publishing anything, which is how the
cross-compile matrix gets validated before a tag exists.

## Contributing

For more information, please read the [CONTRIBUTING.md](CONTRIBUTING.md) file.
