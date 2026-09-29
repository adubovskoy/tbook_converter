#!/usr/bin/env python3
"""Count sentences whose alignment chunk list differs between two .tbook arms
(the grid's `alignments changed vs base` column), and assert the translations
are identical. Also reports whether the two archives' chapter JSON payloads are
byte-identical, which is the strongest form of "this change is a no-op".

Usage: itermax_aligndiff.py A.tbook B.tbook [--lang ru]
"""
import argparse
import json
import os
import sys
import zipfile

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from itermax_prep import align_of, chunk_key, sentences, text_of  # noqa: E402


def chapter_bytes(path):
    out = {}
    with zipfile.ZipFile(path) as z:
        for n in sorted(z.namelist()):
            if n.startswith("chapters/") and n.endswith(".json"):
                out[n] = z.read(n)
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("a")
    ap.add_argument("b")
    ap.add_argument("--lang", default="ru")
    args = ap.parse_args()
    A, B = sentences(args.a), sentences(args.b)
    if len(A) != len(B):
        raise SystemExit(f"sentence counts differ: {len(A)} vs {len(B)}")
    common = changed = textdiff = 0
    for sa, sb in zip(A, B):
        ta, tb = text_of(sa, args.lang), text_of(sb, args.lang)
        if not ta or not tb:
            continue
        common += 1
        if ta != tb:
            textdiff += 1
        if chunk_key(align_of(sa, args.lang)) != chunk_key(align_of(sb, args.lang)):
            changed += 1
    ca, cb = chapter_bytes(args.a), chapter_bytes(args.b)
    same_bytes = ca == cb
    print(f"{os.path.basename(args.a)} vs {os.path.basename(args.b)}: "
          f"{changed} / {common} sentences with a different alignment chunk list; "
          f"{textdiff} translation texts differ; "
          f"chapter payloads byte-identical: {same_bytes}")
    return 0 if changed == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
