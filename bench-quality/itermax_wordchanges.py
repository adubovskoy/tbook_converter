#!/usr/bin/env python3
"""Exact per-source-word change list for every alignment-changed sentence.

For each changed sentence (keyed by its document index, the same `i` that
itermax_prep.py writes into changed.json and key.json) it records every source
word whose tap lights a different target fragment under itermax than under
base, with its class (content by cmd/score's rule, else closed) and how the
highlight moved (lit / dark / widened / narrowed / shifted / moved).

itermax_analyze.py cross-tabs the blind verdicts against this file.

Usage:
  itermax_wordchanges.py BASE.tbook ITERMAX.tbook --out DIR [--lang ru]
"""
import argparse
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from itermax_prep import (align_of, chunk_key, is_score_content, lit_map,  # noqa: E402
                          sentences, text_of, word_text)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("base")
    ap.add_argument("itermax")
    ap.add_argument("--lang", default="ru")
    ap.add_argument("--out", required=True)
    args = ap.parse_args()

    A = sentences(args.base)
    B = sentences(args.itermax)
    out = {}
    for idx, (sa, sb) in enumerate(zip(A, B)):
        if not text_of(sa, args.lang):
            continue
        if chunk_key(align_of(sa, args.lang)) == chunk_key(align_of(sb, args.lang)):
            continue
        la, lb = lit_map(sa, args.lang), lit_map(sb, args.lang)
        rows = []
        for wi in range(len(sa.get("words") or [])):
            w = word_text(sa, wi)
            if not w:
                continue
            fa, fb = " ".join(la.get(wi, [])), " ".join(lb.get(wi, []))
            if fa == fb:
                continue
            if not fa:
                how = "lit"
            elif not fb:
                how = "dark"
            else:
                x, y = set(la.get(wi, [])), set(lb.get(wi, []))
                how = ("widened" if x < y else "narrowed" if y < x
                       else "shifted" if x & y else "moved")
            rows.append({"wi": wi, "w": w,
                         "kind": "content" if is_score_content(w) else "closed",
                         "how": how, "base": fa, "itermax": fb})
        out[str(idx)] = rows
    os.makedirs(args.out, exist_ok=True)
    json.dump(out, open(f"{args.out}/wordchanges.json", "w"),
              ensure_ascii=False, indent=1)
    print(f"{len(out)} changed sentences, "
          f"{sum(len(v) for v in out.values())} word changes -> {args.out}/wordchanges.json")


if __name__ == "__main__":
    main()
