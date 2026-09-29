#!/usr/bin/env python3
"""Structural check of the silent-only recall variants (D4d).

Contract under test, stronger than D4c's guard: an arm may only ever light a
source word that base left DARK. Nothing base lit may change — not widened, not
narrowed, not moved, not darkened. For `itermax-silent-cc` the additional
contract is that every newly lit source word is a CONTENT word.

Reports, per book and arm: violations by kind, newly lit content vs closed-class
words (with the tap-ledger subset), and the sentence-level change counts.

Usage:
  itermax_silentcheck.py BASE.tbook ARM.tbook --book ac --arm silent --out DIR
                         [--itermax ITERMAX.tbook]
"""
import argparse
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from itermax_prep import (align_of, chunk_key, is_content_word,  # noqa: E402
                          is_score_content, lit_map, sentences, text_of,
                          word_text)


def how(a, b):
    if a == b:
        return "same"
    if not a:
        return "lit"
    if not b:
        return "dark"
    return ("widened" if a < b else "narrowed" if b < a
            else "shifted" if a & b else "moved")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("base")
    ap.add_argument("arm")
    ap.add_argument("--itermax", default="")
    ap.add_argument("--book", required=True)
    ap.add_argument("--arm-name", required=True)
    ap.add_argument("--lang", default="ru")
    ap.add_argument("--out", required=True)
    args = ap.parse_args()

    A = sentences(args.base)
    B = sentences(args.arm)
    I = sentences(args.itermax) if args.itermax else None
    if len(A) != len(B) or (I and len(I) != len(A)):
        raise SystemExit("sentence counts differ")

    res = {
        "book": args.book, "arm": args.arm_name,
        "words": {"closed": 0, "content": 0},
        "base_lit_unchanged": {"closed": 0, "content": 0},
        "base_lit_violations": {"closed": {}, "content": {}},
        "newly_lit": {"content": 0, "closed": 0, "ledger": 0},
        "still_dark": {"content": 0, "closed": 0},
        "newly_lit_matches_itermax": {"content": 0, "closed": 0},
        "newly_lit_differs_from_itermax": {"content": 0, "closed": 0},
        "sentences": {"n": 0, "changed_vs_base": 0},
    }
    offenders = []
    for idx, (sa, sb) in enumerate(zip(A, B)):
        if sa.get("src") != sb.get("src"):
            raise SystemExit(f"src mismatch at {idx}")
        if not text_of(sa, args.lang):
            continue
        if text_of(sa, args.lang) != text_of(sb, args.lang):
            raise SystemExit(f"translation text differs at {idx}")
        res["sentences"]["n"] += 1
        if chunk_key(align_of(sa, args.lang)) != chunk_key(align_of(sb, args.lang)):
            res["sentences"]["changed_vs_base"] += 1
        la = {k: set(v) for k, v in lit_map(sa, args.lang).items()}
        lb = {k: set(v) for k, v in lit_map(sb, args.lang).items()}
        li = ({k: set(v) for k, v in lit_map(I[idx], args.lang).items()}) if I else {}
        for wi in range(len(sa.get("words") or [])):
            w = word_text(sa, wi)
            if not w:
                continue
            kind = "content" if is_score_content(w) else "closed"
            res["words"][kind] += 1
            a, b = la.get(wi, set()), lb.get(wi, set())
            if a:  # base lit it: nothing may change
                if a == b:
                    res["base_lit_unchanged"][kind] += 1
                else:
                    h = how(a, b)
                    d = res["base_lit_violations"][kind]
                    d[h] = d.get(h, 0) + 1
                    offenders.append({"i": idx, "wi": wi, "w": w, "kind": kind, "how": h,
                                      "src": sa["src"], "base": sorted(a), "arm": sorted(b)})
            elif b:  # newly lit
                res["newly_lit"][kind] += 1
                if kind == "content" and is_content_word(w):
                    res["newly_lit"]["ledger"] += 1
                if I:
                    key = ("newly_lit_matches_itermax" if li.get(wi, set()) == b
                           else "newly_lit_differs_from_itermax")
                    res[key][kind] += 1
            else:
                res["still_dark"][kind] += 1

    os.makedirs(args.out, exist_ok=True)
    tag = f"{args.book}-{args.arm_name}"
    json.dump(res, open(f"{args.out}/silentcheck-{tag}.json", "w"),
              ensure_ascii=False, indent=1)
    json.dump(offenders, open(f"{args.out}/silentcheck-{tag}-offenders.json", "w"),
              ensure_ascii=False, indent=1)

    v = res["base_lit_violations"]
    nv = sum(sum(d.values()) for d in v.values())
    print(f"{tag}: {res['sentences']['n']} translated sentences, "
          f"{res['sentences']['changed_vs_base']} with a changed alignment")
    print(f"  base-lit source words unchanged: closed {res['base_lit_unchanged']['closed']}, "
          f"content {res['base_lit_unchanged']['content']}; "
          f"VIOLATIONS {nv}" + (f" {v}" if nv else ""))
    print(f"  newly lit: content {res['newly_lit']['content']} "
          f"(tap-ledger subset {res['newly_lit']['ledger']}), "
          f"closed-class {res['newly_lit']['closed']}")
    print(f"  still dark: content {res['still_dark']['content']}, "
          f"closed {res['still_dark']['closed']}")
    if I:
        print(f"  newly lit fragments equal to itermax's: "
              f"content {res['newly_lit_matches_itermax']['content']}, "
              f"closed {res['newly_lit_matches_itermax']['closed']}; differ: "
              f"content {res['newly_lit_differs_from_itermax']['content']}, "
              f"closed {res['newly_lit_differs_from_itermax']['closed']}")
    return 0 if nv == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
