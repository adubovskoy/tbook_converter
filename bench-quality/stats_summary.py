#!/usr/bin/env python3
"""Summarize a --stats NDJSON log per phase: requests, errors, tokens, cost, latency.

Usage: stats_summary.py run-stats.jsonl [more-stats.jsonl ...]

Several files are pooled (a resumed run writes a second log). Cost is present
only for OpenRouter (gonka reports tokens without a price).
"""
import json, sys, collections


def pct(v, p):
    if not v:
        return 0
    v = sorted(v)
    return v[min(len(v) - 1, int(len(v) * p))]


def main():
    rows = []
    for path in sys.argv[1:]:
        with open(path) as f:
            rows += [json.loads(l) for l in f if l.strip()]
    by = collections.defaultdict(list)
    for r in rows:
        by[r.get("phase", "?")].append(r)
    print(f"{'phase':10} {'req':>5} {'ok':>5} {'err':>5} {'in tok':>9} {'out tok':>8} "
          f"{'cost $':>9} {'p50 s':>7} {'p90 s':>7} {'max s':>7}  errors")
    for phase, rs in sorted(by.items()):
        ok = [r for r in rs if r.get("status") == 200]
        err = [r for r in rs if r.get("status") != 200 or r.get("err")]
        lat = [r.get("latency_ms", 0) / 1000 for r in ok]
        codes = collections.Counter(
            f"{r.get('status')}" for r in rs if r.get("status") != 200)
        trunc = sum(1 for r in rs if r.get("finish_reason") not in (None, "", "stop"))
        print(f"{phase:10} {len(rs):5} {len(ok):5} {len(err):5} "
              f"{sum(r.get('prompt_tokens') or 0 for r in rs):9} "
              f"{sum(r.get('completion_tokens') or 0 for r in rs):8} "
              f"{sum(r.get('cost') or 0 for r in rs):9.4f} "
              f"{pct(lat, .5):7.1f} {pct(lat, .9):7.1f} {max(lat, default=0):7.1f}  "
              f"{dict(codes)}" + (f" trunc={trunc}" if trunc else ""))


if __name__ == "__main__":
    main()
