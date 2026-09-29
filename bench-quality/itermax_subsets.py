#!/usr/bin/env python3
"""Subset cross-tabs with sign tests over a judged pairs summary (D4b §5's
table), computed from summary-pairs.json + wordchanges.json.

Usage: itermax_subsets.py DIR [DIR ...]
"""
import argparse
import json
import math
import os


def binom_two_sided(k, n):
    if n == 0:
        return 1.0
    p = sum(math.comb(n, i) for i in range(0, min(k, n - k) + 1)) / 2 ** n * 2
    return min(1.0, p)


SUBSETS = [
    ("all", lambda w: True),
    ("a closed-class word moved to a disjoint fragment",
     lambda w: any(r["kind"] == "closed" and r["how"] == "moved" for r in w)),
    ("no closed-class word moved that way",
     lambda w: not any(r["kind"] == "closed" and r["how"] == "moved" for r in w)),
    ("only closed-class words changed at all",
     lambda w: bool(w) and {r["kind"] for r in w} == {"closed"}),
    ("only content words changed at all",
     lambda w: bool(w) and {r["kind"] for r in w} == {"content"}),
    ("a content word is newly lit",
     lambda w: any(r["kind"] == "content" and r["how"] == "lit" for r in w)),
    ("no content word is newly lit",
     lambda w: not any(r["kind"] == "content" and r["how"] == "lit" for r in w)),
    ("a content word moved to a disjoint fragment",
     lambda w: any(r["kind"] == "content" and r["how"] == "moved" for r in w)),
]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("dirs", nargs="+")
    args = ap.parse_args()
    for d in args.dirs:
        s = json.load(open(f"{d}/summary-pairs.json", encoding="utf-8"))
        la, lb = list(s["wins"].keys())
        print(f"\n=== {os.path.basename(d)}  ({la} vs {lb}) ===")
        print(f"{'subset':52s} {'dec':>4s} {la:>6s} {lb:>6s}   p")
        for name, pred in SUBSETS:
            rows = [r for r in s["rows"] if pred(r["words"])]
            a = sum(1 for r in rows if r["outcome"] == la)
            b = sum(1 for r in rows if r["outcome"] == lb)
            dec = a + b
            p = binom_two_sided(min(a, b), dec)
            print(f"{name:52s} {dec:4d} {a:6d} {b:6d}   {p:.3g}")


if __name__ == "__main__":
    main()
