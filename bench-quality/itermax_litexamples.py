#!/usr/bin/env python3
"""Verbatim newly-lit examples by verdict class, from summary-lit.json.

Usage: itermax_litexamples.py DIR [--n 8] [--only correct|partial|wrong]
"""
import argparse
import json


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("dir")
    ap.add_argument("--n", type=int, default=8)
    ap.add_argument("--only", default="")
    args = ap.parse_args()
    s = json.load(open(f"{args.dir}/summary-lit.json", encoding="utf-8"))
    for v in ("correct", "partial", "wrong"):
        if args.only and v != args.only:
            continue
        rows = [r for r in s["rows"] if r["v"] == v]
        print(f"\n################ {v} ({len(rows)}) ################")
        for r in rows[: args.n]:
            print(f"\n{r['word']!r} -> {r['frag']!r}   (ledger={r['ledger']})")
            print(f"  SRC {r['src']}")
            print(f"  TR  {r['tr']}")
            print(f"  judge: {r['r']}")


if __name__ == "__main__":
    main()
