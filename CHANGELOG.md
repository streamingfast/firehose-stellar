# Change log

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). See [MAINTAINERS.md](./MAINTAINERS.md)
for instructions to keep up to date.

## v1.0.4

* ignore extraneous fields on GetLatestLedgerResult response since we only use it for block number (sequence)
* Update firehose-core module to latest version
* Update `BLOCK_TO_FETCH` in test
* Add "keep" flag to fetcher log for improved debugging
* Expand fetch statistics with total times and inter-call delays
* Add block fetch performance tracking and statistics logging
* Update module dependencies and Go version
* Add V0 transaction support
* Add `is-mainnet` flag for network passphrase selection
* remove dependency on getTransaction only rely on getLegder.

## v1.0.3

* Bump stellar go lib Horizon to v24.0.0

## v1.0.2

* Fix missing `fetch rpc` command that was removed by mistake.

## v1.0.1

* Updated to final version of Stellar Protocol 23.

## v1.0.0

* Initial released version

* Updated to Stellar Protocol 23.
