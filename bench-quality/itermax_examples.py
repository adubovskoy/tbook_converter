#!/usr/bin/env python3
"""Verbatim win lists from a judged pairs run: for each outcome, the sampled
sentences that outcome won, with the source words whose highlight changed and
both orders' judge reasons.

Usage: itermax_examples.py DIR [--n 8]
"""
import argparse
import json


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("dir")
    ap.add_argument("--n", type=int, default=8)
    args = ap.parse_args()
    s = json.load(open(f"{args.dir}/summary-pairs.json", encoding="utf-8"))
    for side in (s["wins"].keys()):
        rows = [r for r in s["rows"] if r["outcome"] == side]
        print(f"\n################ {side} better ({len(rows)} decisive) ################")
        for r in rows[: args.n]:
            print(f"\n[{r['id']}] {r['src']}")
            for w in r["words"]:
                tag = "CONTENT" if w["kind"] == "content" else "closed "
                print(f"  {tag} {w['w']!r}: base {w['base']!r} -> arm {w['itermax']!r} ({w['how']})")
            for k, reason in enumerate(r["reasons"], 1):
                if reason:
                    print(f"  judge o{k}: {reason}")


if __name__ == "__main__":
    main()
