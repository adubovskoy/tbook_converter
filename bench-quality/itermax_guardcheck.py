#!/usr/bin/env python3
"""Structural check of the closed-class guard (D4b report §6, step 1).

Given the three arms of one book — base (`argmax`), `itermax` and the guarded
arm — it verifies, for every source word of every sentence, that

  * a CLOSED-CLASS source word (internal/stopwords `en`, cmd/score's rule)
    lights exactly what it lights under base, and
  * a CONTENT source word lights exactly what it lights under itermax,

and reports how every disagreement moved (lit / dark / widened / narrowed /
shifted / moved — "moved" is a disjoint jump). It also recounts the newly lit
content words of the guarded arm against base, which is the metric the guard is
supposed to preserve from itermax.

Usage:
  itermax_guardcheck.py BASE.tbook ITERMAX.tbook GUARD.tbook --book ac --out DIR
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
    """How a tap's highlight moved from fragment set a to fragment set b."""
    if not a and not b:
        return "same"
    if not a:
        return "lit"
    if not b:
        return "dark"
    if a == b:
        return "same"
    return ("widened" if a < b else "narrowed" if b < a
            else "shifted" if a & b else "moved")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("base")
    ap.add_argument("itermax")
    ap.add_argument("guard")
    ap.add_argument("--book", required=True)
    ap.add_argument("--lang", default="ru")
    ap.add_argument("--out", required=True)
    args = ap.parse_args()

    A = sentences(args.base)
    B = sentences(args.itermax)
    C = sentences(args.guard)
    if not (len(A) == len(B) == len(C)):
        raise SystemExit(f"sentence counts differ: {len(A)} / {len(B)} / {len(C)}")

    res = {
        "book": args.book,
        "words": {"closed": 0, "content": 0},
        # guarded vs the arm it is supposed to equal for that class
        "agree": {"closed": 0, "content": 0},
        "disagree_how": {"closed": {}, "content": {}},
        "closed_vs_itermax_differ": 0,   # how much work the guard actually did
        "content_vs_base_differ": 0,
        "sentences": {"n": 0, "changed_vs_base": 0, "changed_vs_itermax": 0},
        "newly_lit_vs_base": {"content": 0, "closed": 0, "ledger": 0},
        "lit_to_silent_vs_base": {"content": 0, "closed": 0},
    }
    offenders = []
    for idx, (sa, sb, sc) in enumerate(zip(A, B, C)):
        if sa.get("src") != sc.get("src"):
            raise SystemExit(f"src mismatch at {idx}")
        if not text_of(sa, args.lang):
            continue
        if text_of(sa, args.lang) != text_of(sc, args.lang):
            raise SystemExit(f"translation text differs at {idx}")
        res["sentences"]["n"] += 1
        ka, kb, kc = (chunk_key(align_of(s, args.lang)) for s in (sa, sb, sc))
        if kc != ka:
            res["sentences"]["changed_vs_base"] += 1
        if kc != kb:
            res["sentences"]["changed_vs_itermax"] += 1
        la = {k: set(v) for k, v in lit_map(sa, args.lang).items()}
        lb = {k: set(v) for k, v in lit_map(sb, args.lang).items()}
        lc = {k: set(v) for k, v in lit_map(sc, args.lang).items()}
        for wi in range(len(sa.get("words") or [])):
            w = word_text(sa, wi)
            if not w:
                continue
            kind = "content" if is_score_content(w) else "closed"
            want = (lb if kind == "content" else la).get(wi, set())
            other = (la if kind == "content" else lb).get(wi, set())
            got = lc.get(wi, set())
            res["words"][kind] += 1
            if want != other:
                res["closed_vs_itermax_differ" if kind == "closed"
                    else "content_vs_base_differ"] += 1
            if got == want:
                res["agree"][kind] += 1
            else:
                h = how(want, got)
                res["disagree_how"][kind][h] = res["disagree_how"][kind].get(h, 0) + 1
                offenders.append({
                    "i": idx, "wi": wi, "w": w, "kind": kind, "how": h,
                    "src": sa["src"], "tr": text_of(sa, args.lang),
                    "base": sorted(la.get(wi, set())),
                    "itermax": sorted(lb.get(wi, set())),
                    "guard": sorted(got),
                })
            # recovery bookkeeping against base
            if kind == "content" and wi not in la and wi in lc:
                res["newly_lit_vs_base"]["content"] += 1
                if is_content_word(w):
                    res["newly_lit_vs_base"]["ledger"] += 1
            if kind == "closed" and wi not in la and wi in lc:
                res["newly_lit_vs_base"]["closed"] += 1
            if wi in la and wi not in lc:
                res["lit_to_silent_vs_base"][kind] += 1

    os.makedirs(args.out, exist_ok=True)
    json.dump(res, open(f"{args.out}/guardcheck-{args.book}.json", "w"),
              ensure_ascii=False, indent=1)
    json.dump(offenders, open(f"{args.out}/guardcheck-{args.book}-offenders.json", "w"),
              ensure_ascii=False, indent=1)

    w, a = res["words"], res["agree"]
    print(f"{args.book}: {res['sentences']['n']} translated sentences; "
          f"changed vs base {res['sentences']['changed_vs_base']}, "
          f"vs itermax {res['sentences']['changed_vs_itermax']}")
    for kind, ref in (("closed", "base"), ("content", "itermax")):
        d = w[kind] - a[kind]
        print(f"  {kind:7s} source words {w[kind]:6d}: {a[kind]:6d} identical to {ref} "
              f"({a[kind]/w[kind]:.4%}), {d} differ "
              + (", ".join(f"{k}={v}" for k, v in sorted(res['disagree_how'][kind].items()))
                 if d else ""))
    print(f"  guard work: {res['closed_vs_itermax_differ']} closed-class words where "
          f"base differs from itermax (frozen), {res['content_vs_base_differ']} content words "
          f"where itermax differs from base (kept)")
    print(f"  newly lit vs base: content {res['newly_lit_vs_base']['content']} "
          f"(ledger {res['newly_lit_vs_base']['ledger']}), "
          f"closed {res['newly_lit_vs_base']['closed']}; "
          f"lit->silent content {res['lit_to_silent_vs_base']['content']}, "
          f"closed {res['lit_to_silent_vs_base']['closed']}")


if __name__ == "__main__":
    main()
