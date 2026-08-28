# Stellar protocol upgrades

Why the `go-stellar-sdk` and `stellar-core` pins in this repo are what they are.

Both are pinned by *protocol*, not by feature: a pre-upgrade captive-core halts at
the upgrade ledger rather than following the network, and a pre-upgrade SDK cannot
decode the new XDR. Neither failure is subtle at runtime, and both are avoidable by
bumping ahead of the vote.

Entries accumulate — the previous protocol's row stays so the reason behind an older
pin is still readable after it has been superseded.

## What reaches the protobuf model

Nothing, so far, for any protocol bump. `proto/sf/stellar/type/v1/block.proto` carries
transactions as opaque XDR (`envelope_xdr`, `result_xdr`, the event byte strings), so
new XDR shapes inside transactions reach consumers untouched and are decoded there with
a matching SDK. `Header` models only `ledger_version`, `previous_ledger_hash`,
`total_coins`, `base_fee` and `base_reserve`; the SCP value is not part of it, and the
only field read off it is `closeTime`, for the block timestamp.

That is why CAP-0083 and CAP-0085 below needed no `.proto` change: one lives in the SCP
value we do not model, the other inside transaction XDR we do not interpret. A ledger
carrying `STELLAR_VALUE_EMPTY_TX_SET` becomes a `Block` with no transactions, which is
what it is.

## Protocol 28

- SDK: `go-stellar-sdk` v0.7.2 (protocol support landed in v0.7.0)
- Core: `stellar-core >= 28.0.1-3508.947aad841` (28.0.0-3494.a9b861321 was the initial P28 release; 28.0.1 is SDF's August 2026 critical security fix)
- Votes: testnet 2026-08-27 1700 UTC, mainnet 2026-09-16 1700 UTC
- [SDF upgrade guide](https://stellar.org/blog/developers/adapter-protocol-28-upgrade-guide)

XDR the pre-P28 SDK cannot decode:

- **CAP-0083** — `STELLAR_VALUE_EMPTY_TX_SET`, a third `StellarValueType`. Consumers of
  raw ledger data must treat ledgers carrying it as empty ledgers.
- **CAP-0085** — `CONTRACT_EXECUTABLE_EXTERNAL_REF`, a third `ContractExecutableType`.
  Custom account contracts that cannot parse it are unable to authorize external-reference
  contract creation.
- **CAP-0086** — sparse map host functions. No backwards incompatibility; existing host
  functions are unchanged.

Validators must additionally run NTP sync starting in P28, to support faster ledger close.

Note that v0.7.2 also carries SDK-side validation changes released after v0.7.0:
`strkey.Decode`/`DecodeAny` enforce SEP-23 payload lengths, `xdr.Asset.LessThan` orders by
XDR encoding, and `xdr.NewPoolId` rejects non-strictly-ordered asset pairs. No caller in
this repo is affected.

## Protocol 27 ("Zipper")

- SDK: `go-stellar-sdk` v0.6.0
- Core: `stellar-core >= 27.0.0-3288.7696c069d`
- Upgraded: testnet 2026-06-18, mainnet 2026-07-08

XDR the pre-P27 SDK cannot decode:

- **CAP-0071** — Soroban auth: `SOROBAN_CREDENTIALS_ADDRESS_V2`,
  `SOROBAN_CREDENTIALS_ADDRESS_WITH_DELEGATES`, and
  `ENVELOPE_TYPE_SOROBAN_AUTHORIZATION_WITH_ADDRESS`.

## Earlier

- **Protocol 26** — SDK bumped to match stellar-rpc v26 (`v1.0.6`).
- **Protocol 23** — the initial released version (`v1.0.0`), finalized in `v1.0.1`.

The `stellar-core` floor before P27 was `26.1.0-3210.427aa3978`, set for SDF's May 2026
critical security advisory rather than for a protocol.

## Bumping

Each upgrade touches the same places — miss one and the failure is silent, because a
stale `STELLAR_CORE_MIN_VERSION` lets the Docker build pass on a core that will halt at
the upgrade ledger, or that still carries a bug SDF has already patched:

- `go.mod` — the SDK release whose changelog names the target protocol
- `Dockerfile` — `ARG STELLAR_CORE_MIN_VERSION`, using the apt version string with the
  codename suffix stripped (`28.0.1-3508.947aad841`, not `…​.noble`). Confirm it exists at
  `https://apt.stellar.org/dists/<codename>/stable/binary-amd64/Packages` for the codename
  of the `firehose-core` base image
- `test/README.md` — the required-minimum line
- `test/scripts/dev/docker-compose.yml` — the quickstart pin comment
- `CHANGELOG.md` and this file

Then check whether the new XDR adds union arms the code switches on: `captivecore` and
`rpc` both switch on `xdr.LedgerCloseMeta` V0/V1/V2 and error on `default`.

A security-only core bump is the same list minus `go.mod` and the XDR check — only the
floor moves. Record it inline on the protocol section it lands under, the way 28.0.1 is
recorded on Protocol 28. The earlier 26.1.0 bump predates that convention and sits under
"Earlier" instead.

`stellar/quickstart:testing` tracks whatever protocol testnet currently runs, so before
the testnet vote it still boots the previous protocol. Use
`QUICKSTART_IMAGE=stellar/quickstart:future` to exercise the new one — see
[test/README.md](../test/README.md).
