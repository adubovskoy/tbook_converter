#!/usr/bin/env python3
"""Lexcheck flag delta between two arms, with each affected sentence's
word-level alignment changes attached (D4b report §5.5, for the guarded arm).

Usage: itermax_lexdelta.py BASE.tbook ARM.tbook --wordchanges DIR/wordchanges.json
"""
import argparse
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from itermax_prep import sentences, text_of  # noqa: E402


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("base")
    ap.add_argument("arm")
    ap.add_argument("--wordchanges", default="")
    ap.add_argument("--lang", default="ru")
    args = ap.parse_args()

    fa = json.load(open(args.base + ".lexflagged.json", encoding="utf-8"))
    fb = json.load(open(args.arm + ".lexflagged.json", encoding="utf-8"))
    A = sentences(args.base)
    idx_of = {}
    for i, s in enumerate(A):
        idx_of.setdefault(s.get("src", ""), i)
    wc = json.load(open(args.wordchanges, encoding="utf-8")) if args.wordchanges else {}

    new = [s for s in fb if s not in set(fa)]
    gone = [s for s in fa if s not in set(fb)]
    print(f"{os.path.basename(args.base)} {len(fa)} flags -> "
          f"{os.path.basename(args.arm)} {len(fb)} flags: {len(new)} new, {len(gone)} gone")
    for tag, rows in (("NEW", new), ("GONE", gone)):
        for s in rows:
            i = idx_of.get(s)
            print(f"\n[{tag}] {s}")
            if i is None:
                print("  (sentence not located)")
                continue
            print(f"  tr: {text_of(A[i], args.lang)}")
            for r in wc.get(str(i), []):
                print(f"  {r['kind']:7s} {r['w']!r}: {r['base']!r} -> {r['itermax']!r} ({r['how']})")
            if not wc.get(str(i)):
                print("  (no source word changed its highlight — target-side chunk move only)")


if __name__ == "__main__":
    main()
