#!/usr/bin/env bash
# run_bench.sh — local + CI bench runner for OpenForge 4-category bench suite
# (DESIGN §13.5).
#
# Outputs go test -bench stdout to bench-result.txt in the repo root so
# scripts/bench_compare.py can diff against bench/baseline.txt.
#
# Env vars (all optional; unset = skip the bench that needs them):
#   BENCH_PG_DSN     postgres DSN for pipeline create throughput bench
#   BENCH_EMBED_DSN  reserved for the future embedding adapter
#   GOCACHE          go build cache dir (Windows-friendly default below)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

export GOCACHE="${GOCACHE:-$REPO_ROOT/.cache/go-build}"
mkdir -p "$GOCACHE"

echo "[run_bench] go version: $(go version)"
echo "[run_bench] GOCACHE=$GOCACHE"

# -benchtime=2s stabilizes ns/op on local + CI runners. 100x guarantees
# >= 100 iterations so the in-bench p50/p95/p99 percentiles are
# statistically meaningful (the bench files clamp the index, but a
# 100-iter sample is what the comparison script expects).
# -count=1 disables the per-package test-result cache so we always re-run.
go test -bench=. -benchmem -benchtime='2s' -count=1 -run='^$' ./test/bench/ \
  | tee "$REPO_ROOT/bench-result.txt"

echo "[run_bench] results written to $REPO_ROOT/bench-result.txt"
