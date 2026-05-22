#!/usr/bin/env bash
# Tear down the test chain. Idempotent.

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/docker-compose.yml"
PROJECT="${COMPOSE_PROJECT_NAME:-battlefield-stellar}"

if ! command -v docker >/dev/null 2>&1; then
  echo ">> docker not present; nothing to do"
  exit 0
fi

echo ">> stopping quickstart"
COMPOSE_PROJECT_NAME="$PROJECT" docker compose -f "$COMPOSE_FILE" down --remove-orphans
