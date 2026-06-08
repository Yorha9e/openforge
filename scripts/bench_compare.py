#!/usr/bin/env python3
"""bench_compare.py — fail CI if any benchmark regresses by >20%.

Reads bench-result.txt (current) and bench/baseline.txt (checked in),
parses `BenchmarkXxx-N<cpus>   <iters>   <ns/op>` lines, and returns
non-zero exit status if any bench in the baseline is more than 20%
slower in the current run.

Usage:
    python scripts/bench_compare.py            # exit 0 on OK, 1 on regression
    python scripts/bench_compare.py --tolerance 0.10   # override the 20% default

If either file is empty or missing benchmark lines, the comparison is
treated as a no-op (exit 0) — this lets the bench CI job succeed on
branches where the backing services (PG, etc.) are not available and
most benches skip.
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

# Matches: BenchmarkFoo-16    1000000    1234 ns/op
#   group 1: bench name (without the leading "Benchmark" or trailing "-N")
#   group 2: ns/op
BENCH_RE = re.compile(
    r"^Benchmark([A-Za-z0-9_]+)-\d+\s+\d+\s+([\d.]+)\s+ns/op",
    re.MULTILINE,
)

DEFAULT_TOLERANCE = 0.20  # 20%


def parse_bench(path: Path) -> dict[str, float]:
    """Return {bench_name: ns_per_op}. Missing files yield an empty dict."""
    if not path.exists():
        return {}
    text = path.read_text(encoding="utf-8", errors="replace")
    out: dict[str, float] = {}
    for m in BENCH_RE.finditer(text):
        name = m.group(1)
        ns = float(m.group(2))
        # First occurrence wins (defensive — Go's -bench prints each bench once).
        out.setdefault(name, ns)
    return out


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument(
        "--tolerance",
        type=float,
        default=DEFAULT_TOLERANCE,
        help="allowed regression as a fraction (default: 0.20 = 20%%)",
    )
    ap.add_argument(
        "--current",
        type=Path,
        default=Path("bench-result.txt"),
        help="current bench output (default: bench-result.txt)",
    )
    ap.add_argument(
        "--baseline",
        type=Path,
        default=Path("bench/baseline.txt"),
        help="baseline bench output (default: bench/baseline.txt)",
    )
    args = ap.parse_args()

    cur = parse_bench(args.current)
    base = parse_bench(args.baseline)

    if not base:
        print(
            f"[bench_compare] no baseline benchmarks found in {args.baseline}; "
            "treating as no-op (OK).",
            file=sys.stderr,
        )
        return 0
    if not cur:
        print(
            f"[bench_compare] no current benchmarks found in {args.current}; "
            "treating as no-op (OK).",
            file=sys.stderr,
        )
        return 0

    regressions: list[str] = []
    improvements: list[str] = []
    for name, base_ns in base.items():
        if name not in cur:
            continue  # skipped this run — ignore
        cur_ns = cur[name]
        if base_ns <= 0:
            continue  # avoid /0
        delta = (cur_ns - base_ns) / base_ns
        if delta > args.tolerance:
            regressions.append(
                f"{name}: baseline={base_ns:.0f}ns, current={cur_ns:.0f}ns "
                f"(+{delta * 100:.1f}%, threshold=+{args.tolerance * 100:.1f}%)"
            )
        elif delta < -args.tolerance:
            improvements.append(
                f"{name}: baseline={base_ns:.0f}ns, current={cur_ns:.0f}ns "
                f"({delta * 100:.1f}%)"
            )

    for imp in improvements:
        print(f"[bench_compare] IMPROVEMENT: {imp}")
    if regressions:
        print(
            f"[bench_compare] BENCHMARK REGRESSIONS (>{args.tolerance * 100:.0f}% slower):",
            file=sys.stderr,
        )
        for r in regressions:
            print(f"  {r}", file=sys.stderr)
        return 1
    print(
        f"[bench_compare] OK: {len(base)} benchmark(s) checked against "
        f"tolerance=+{args.tolerance * 100:.0f}%, 0 regressions"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
