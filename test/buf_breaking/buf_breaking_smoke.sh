#!/usr/bin/env bash
# buf breaking change smoke test (DESIGN §13.3)
# Verifies: buf build + buf breaking against baseline image.bin produces 0 breaking changes.
# This is a TDD smoke test — not a real Go test (buf is not a Go program),
# but a runnable script with exit code assertion suitable for CI gate.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

BASELINE="${BASELINE:-proto/image.bin}"
WORKDIR="${WORKDIR:-$(mktemp -d)}"
CURRENT="${CURRENT:-${WORKDIR}/buf-current.bin}"

trap 'rm -rf "$WORKDIR"' EXIT

echo "[buf-breaking] step 1/3: buf build -> $CURRENT"
buf build proto --as-file-descriptor-set -o "$CURRENT"

echo "[buf-breaking] step 2/3: buf breaking --against $BASELINE"
buf breaking proto --against "$BASELINE"

echo "[buf-breaking] step 3/3: success (no breaking changes detected)"
