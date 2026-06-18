# Change log

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this
project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html). See [MAINTAINERS.md](./MAINTAINERS.md)
for instructions to keep up to date.

## Unreleased

* Support Stellar Protocol 27 (Zipper): bump `go-stellar-sdk` to v0.6.0 (CAP-0071 Soroban auth XDR) and require `stellar-core >= 27.0.0-3288.7696c069d`. An older captive-core halts at the P27 upgrade ledger (mainnet 2026-07-08, testnet 2026-06-18).
* Add captive-core fetcher backend (`firestellar fetch captive-core`) that spawns a `stellar-core` subprocess and streams ledgers via the captive-core peer + history archive path. Captive-core is now the supported backend going forward; the RPC poller is kept for compatibility but no longer actively developed.
* Add cursor persistence shared between both backends: `--state-dir` writes `cursor.json` after each emitted block so restarts resume at `last_fired_block + 1`. Default `--state-dir` is now `/data/work` for both backends (was `/data/poller` / `/data/captive-core`).
* Add `--ignore-cursor` flag to start fresh from `<first-streamable-block>` when running under a supervisor that tracks downstream state (e.g. `firecore reader-node`).
* Add `--stellar-core-network` plus `--stellar-core-network-passphrase` / `--stellar-core-history-archive-urls` for custom-network captive-core deployments.
* Add `test/` battlefield integration suite: in-process captive-core + poller fetchers driven against `stellar/quickstart`, with deterministic snapshot comparison and cross-backend diffing.
* Enforce a minimum `stellar-core` version for captive-core, asserted post-install in the Docker build.
* CI now reads the Go toolchain version from `go.mod` (`go-version-file`) instead of pinning it inside the workflow.
* Fix poller hash / previous-hash encoding bug.
* Fix `json.Number` handling in XDR normalizers (preserves large-int precision in snapshot/diff round-trips).
* Fix battlefield snapshot templating corrupting any field that contains a bound value as a substring (e.g. a low ledger sequence number landing inside a deterministic account address); bound values now match whole field values only.
* Walk the current store when searching for cursor data.

## v1.0.6 

* bump SDK to match rpc V26

## v1.0.5 ~failed build~

## v1.0.4

* ignore extraneous fields on GetLatestLedgerResult response since we only use it for block number (sequence)
* Update firehose-core module to latest version
* Update `BLOCK_TO_FETCH` in test
* Add "keep" flag to fetcher log for improved debugging
* Expand fetch statistics with total times and inter-call delays
* Add block fetch performance tracking and statistics logging
* Update module dependencies and Go version (go-stellar-sdk: 20251210134752-6c46f8811c13)
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
