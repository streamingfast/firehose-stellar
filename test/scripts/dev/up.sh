#!/usr/bin/env bash
# Bring up the test chain (stellar/quickstart container). Idempotent.
#
# Useful for the manual flow:
#   test/scripts/dev/up.sh
#   BATTLEFIELD_MANAGE_STACK=0 go test ./test/scenarios/... -v
#   test/scripts/dev/down.sh
#
# Most of the time you don't need this — `go test` auto-manages quickstart.

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.yml"
PROJECT="${COMPOSE_PROJECT_NAME:-battlefield-stellar}"

echo ">> starting quickstart"
COMPOSE_PROJECT_NAME="$PROJECT" docker compose -f "$COMPOSE_FILE" up -d --wait
echo ">> quickstart ready (horizon http://localhost:8000)"
