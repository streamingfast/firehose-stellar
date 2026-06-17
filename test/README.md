# test/ — battlefield integration tests for firehose-stellar

Drives end-to-end scenarios against firestellar's two fetcher backends and asserts the structural transaction view against committed snapshots.

> Captive-core is the supported backend going forward; the RPC poller is kept here for cross-backend regression checks only and is no longer actively developed.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ go test ./test/scenarios/...                                │
│                                                             │
│  ┌─────────────────────────────┐    ┌────────────────────────┐
│  │ InProcessCaptiveCoreFetcher │    │ InProcessRPCFetcher    │
│  │  → captivecore.Backend (lib)│    │  → rpc.Fetcher (lib)   │
│  └─────────────┬───────────────┘    └─────────────┬──────────┘
│                │                                   │
└────────────────┼───────────────────────────────────┼────────┘
                 │ peer + history                    │ HTTP
                 │ :11625, :1570                     │ :8000/soroban/rpc
                 ▼                                   ▼
        ┌────────────────────────────────────────────────────┐
        │ stellar/quickstart container (the chain)           │
        │   - stellar-core validator                         │
        │   - horizon + friendbot + soroban-rpc on :8000     │
        └────────────────────────────────────────────────────┘
                                ▲
                                │ submit tx
                                │
        ┌────────────────────────────────────────────────────┐
        │ scenarios_test.go (RunScenario)                    │
        └────────────────────────────────────────────────────┘
```

Both fetchers run **in-process** — firestellar's `rpc` and `captivecore` packages are called as libraries. The only container is stellar/quickstart, which IS the chain.

Why:
- **No drift.** Tests build against the same module — flag/API changes can't desync.
- **Cross-backend diff always on.** Both fetchers run per scenario, runner asserts they agree.
- **Fast loop.** `go test` cold-start is ~30s (quickstart boot only).

## Run the test suite

```bash
go test ./test/scenarios/... -v
go test ./test/scenarios/... -run Payment
SNAPSHOTS_UPDATE=. go test ./test/scenarios/... -v             # regen all snapshots
SNAPSHOTS_UPDATE=payment/native go test ./test/scenarios/...   # regen one
```

`TestMain` brings up quickstart (resetting it to a clean chain) before the suite and tears it down after. Set `BATTLEFIELD_MANAGE_STACK=0` to opt out (manual lifecycle, see below).

### Logs

Chatty fetcher output (stellar-core subprocess + rpc.Fetcher zap logs) is redirected to file so the test terminal stays readable. Tail from another shell:

```bash
tail -f test/.data/fetchers.log     # stellar-core + zap fetcher logs
tail -f test/.data/compose.log      # docker compose output (quickstart up/down)
```

### Cross-backend signal

Default fetcher set is `{captive-core, poller}` — both run per scenario, runner diffs their views. Captive-core is the primary backend; the poller stays in the suite as a regression check. A divergence between fetchers fails the test before snapshot comparison runs, so the error message tells you which fetcher emitted bad output.

If `stellar-core` is not on `$PATH`, the captive-core fetcher silently disables itself and only the poller runs (cross-diff becomes a no-op; snapshot is the only assertion). Install stellar-core to run the supported path.

## Prerequisites

- `docker` + `docker compose` plugin (for quickstart)
- `go` 1.26+
- `stellar-core` binary on `$PATH` (for captive-core in-process fetcher)
  - Required minimum: **`27.0.0-3288.7696c069d`** (Protocol 27 "Zipper"; an older core halts at the P27 upgrade ledger)
  - macOS: `brew upgrade stellar/sdf/stellar-core` (or `brew install` for first-time)
  - Linux: `apt install stellar-core` from SDF apt repo (https://apt.stellar.org); run `apt update && apt install --only-upgrade stellar-core` on existing hosts to pick up the Protocol 27 build
  - Override location via `STELLAR_CORE_BIN=/path/to/stellar-core`

The stellar-core version must be protocol-compatible with the quickstart image — same major version is the safest match. The compose stack pulls `stellar/quickstart:testing` with `pull_policy: always` (override via `QUICKSTART_PULL_POLICY=missing`) so the bundled stellar-core stays current with SDF's patched release.

## Snapshot regen + determinism

```bash
SNAPSHOTS_UPDATE=. go test ./test/scenarios/...
```

Only inherently non-deterministic fields are templated as `$name` placeholders: tx hash, ledger sequence, `createdAt` timestamp, signatures, signature hints, sequence numbers. Everything else — including account addresses, SAC contract IDs, op codes, amounts — is compared as a literal so any drift fails byte-equality directly.

### Why account addresses are deterministic

Keypairs derive from `sha256(t.Name() + "/" + accountName)` (see `runner.Config.AccountSeedScope`). The same `(test name, role)` pair always produces the same `G…` address across runs. Two consequences:

1. Snapshots can commit literal `G…` strings instead of templated placeholders.
2. **Renaming a `Test…` function invalidates that scenario's snapshot.** Workflow:
   ```bash
   SNAPSHOTS_UPDATE=^<scenario/id>$ go test ./test/scenarios/...
   ```

Same applies to any test-code change that alters submitted operations — snapshot drift is the intended signal.

## Manual stack control

For fast iteration across many test cycles, opt out of TestMain's lifecycle:

```bash
test/scripts/dev/up.sh                                       # boot quickstart
BATTLEFIELD_MANAGE_STACK=0 go test ./test/scenarios/... -v   # use it
test/scripts/dev/down.sh                                     # tear down
```

`test/scripts/dev/reset.sh` restarts quickstart for a clean chain without rebooting docker daemon state.

`BATTLEFIELD_MANAGE_STACK=0` tells TestMain to trust the pre-existing stack. With it unset (default), `go test` owns the lifecycle.

## Configuration

| Var | Default | Meaning |
|---|---|---|
| `BATTLEFIELD_MANAGE_STACK` | `1` | TestMain brings quickstart up/down |
| `AUTO_RESET` | `1` | reset chain before tests; `0` to reuse current state |
| `KEEP_RUNNING` | `0` | leave quickstart up after tests finish |
| `SKIP_TESTS` | `0` | bring quickstart up, skip the test body |
| `DEBUG` | `0` | stream raw `docker compose` output to stderr |
| `SNAPSHOTS_UPDATE` | unset | regex: regenerate matching snapshots |
| `STELLAR_CORE_BIN` | from `$PATH` | override captive-core's stellar-core binary location |
| `BATTLEFIELD_SNAPSHOTS` | `snapshots` | snapshot root |
| `BATTLEFIELD_WAIT_TIMEOUT` | `90s` | poll deadline per ledger |
| `BATTLEFIELD_DATA_DIR` | `test/.data` | dev-stack state dir |

The dev image is amd64-only (SDF only ships amd64 stellar-core packages); on Apple Silicon, quickstart runs under Rosetta. The host-side stellar-core (for the in-process captive-core fetcher) runs natively on whatever arch it was built for.

## Layout

```
test/
├── lib/
│   ├── devstack/           docker-compose lifecycle (quickstart up/down)
│   ├── firehose/           Fetcher iface + in-process impls (RPC, CaptiveCore)
│   ├── runner/             submit → fetch → diff fetchers → snapshot
│   ├── snapshot/           load + $var resolve + deepcompare + SNAPSHOTS_UPDATE
│   ├── stellar/            keypair + horizon + txnbuild helpers
│   └── xdr/                envelope/result/events → JSON-friendly maps
├── scenarios/              *_test.go scenarios (TestMain owns lifecycle)
├── snapshots/              <group>/<id>.expected.json
└── scripts/dev/            docker-compose.yml + up/down/reset.sh + configs/
```

Library code under `firehose-stellar/captivecore/` and `firehose-stellar/rpc/` (one module level up) is the in-process surface battlefield calls.

## Scenarios

| File | What it covers |
|---|---|
| `scenarios_test.go` | native payment, double-send, create_account, manage_data, asset issuance + trustline, multi-op, account_merge, failed-trustline |
| `edge_test.go` | fee bump, multi-sig, manage_sell_offer, bump_sequence |
| `soroban_test.go` | invoke, contract events, cross-contract, diagnostic events. **Placeholder** — gated behind `BATTLEFIELD_SOROBAN=1`. |

### TODO

- Soroban contract suite (deploy + invoke + events + panic)
- V0 envelope (`EnvelopeTypeEnvelopeTypeTxV0`)
- Sponsorship sandwich (`BeginSponsoringFutureReserves` / op / `End…`)
- Clawback + asset trustline flags

## Troubleshooting

- **`stellar-core: command not found`** — install via brew/apt, or set `STELLAR_CORE_BIN=/path/to/stellar-core`. Captive-core is the supported backend, so install it for full coverage; the legacy poller still runs without it.
- **`bind: Address already in use`** — captive-core listens on a peer port; the follower config uses `PEER_PORT=11626` to avoid colliding with quickstart's `:11625`. If you've changed quickstart's host port mapping, update the follower config.
- **`stack didn't produce blocks`** — `docker compose -f test/scripts/dev/docker-compose.yml logs quickstart`.
- **`fetcher disagreement for X`** — congrats, you found a real divergence between captive-core and the poller. The error includes the JSON path that differs.
- **`docker compose` plugin missing** — install Docker Desktop or the compose-plugin package.
