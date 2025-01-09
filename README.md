# Firehose for Stellar

[![reference](https://img.shields.io/badge/godoc-reference-5272B4.svg?style=flat-square)](https://pkg.go.dev/github.com/streamingfast/firehose-stellar)

Quick start with Firehose for Stellar can be found in the official Firehose docs. Here some quick links to it:

- [Firehose Overview](https://firehose.streamingfast.io/introduction/firehose-overview)
- [Concepts & Architectures](https://firehose.streamingfast.io/concepts-and-architeceture)
  - [Components](https://firehose.streamingfast.io/concepts-and-architeceture/components)
  - [Data Flow](https://firehose.streamingfast.io/concepts-and-architeceture/data-flow)
  - [Data Storage](https://firehose.streamingfast.io/concepts-and-architeceture/data-storage)
  - [Design Principles](https://firehose.streamingfast.io/concepts-and-architeceture/design-principles)

## Running the Firehose poller

The below command with start streaming Firehose Stellar blocks, check `proto/sf/stellar/type/v1/block.proto` for more information.

```bash
firestellar fetch {FIRST_STREAMABLE_BLOCK} --endpoints {STELLAR_RPC_ENDPOINT} --state-dir {STATE_DIR}
```

## Contributing

For more information, please read the [CONTRIBUTING.md](CONTRIBUTING.md) file.
