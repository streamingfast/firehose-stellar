#!/usr/bin/env bash
# Tear quickstart down and bring it back up for a clean chain.

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

"$SCRIPT_DIR/down.sh" >/dev/null 2>&1 || true
"$SCRIPT_DIR/up.sh"
