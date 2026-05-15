#!/usr/bin/env bash
# Run firehose-stellar with the Captive Core fetcher.
# Requires: stellar-core binary on $PATH (override with STELLAR_CORE_BIN env var).

set -e

START_BLOCK="${START_BLOCK:-2391485}"
NETWORK="${NETWORK:-testnet}"

if [[ -n "${STELLAR_CORE_BIN:-}" ]]; then
  if [[ ! -x "${STELLAR_CORE_BIN}" ]]; then
    echo "ERROR: STELLAR_CORE_BIN=${STELLAR_CORE_BIN} is not an executable file" >&2
    exit 1
  fi
else
  STELLAR_CORE_BIN="$(command -v stellar-core || true)"
  if [[ -z "${STELLAR_CORE_BIN}" ]]; then
    echo "ERROR: stellar-core binary not found in PATH." >&2
    echo "Install it (e.g. 'brew install stellar-core' or download from https://github.com/stellar/stellar-core/releases)," >&2
    echo "or set STELLAR_CORE_BIN=/path/to/stellar-core before re-running." >&2
    exit 1
  fi
fi

echo "Using stellar-core binary: ${STELLAR_CORE_BIN}"

firecore start reader-node merger \
  --config-file= \
  --log-format=stackdriver \
  --log-to-file=false \
  --data-dir=data-cc \
  --common-auto-max-procs \
  --common-auto-mem-limit-percent=90 \
  --common-one-block-store-url=data-cc/oneblock \
  --common-first-streamable-block="${START_BLOCK}" \
  --reader-node-data-dir=data-cc/oneblock \
  --reader-node-working-dir=data-cc/work \
  --reader-node-readiness-max-latency=600s \
  --reader-node-debug-firehose-logs=false \
  --reader-node-blocks-chan-capacity=1000 \
  --reader-node-grpc-listen-addr=:9101 \
  --reader-node-manager-api-addr=:8180 \
  --merger-grpc-listen-addr=:10112 \
  --reader-node-path=./devel/firestellar \
  --reader-node-arguments="fetch captive-core ${START_BLOCK} --stellar-core-bin ${STELLAR_CORE_BIN} --stellar-core-network ${NETWORK}"
