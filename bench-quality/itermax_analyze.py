#!/usr/bin/env python3
"""Analyze the D4b alignment verdicts.

  itermax_analyze.py DIR --mode pairs   de-swaps both presentation orders, counts
                                        a pair decisive only when the two orders
                                        agree, runs a two-sided sign test, and
                                        cross-tabs the decisive verdicts against
                                        what actually changed in the sentence
                                        (closed-class source words re-mapped vs
                                        content words newly lit).
  itermax_analyze.py DIR --mode lit     correct/partial/wrong distribution.

Prints a summary and writes summary-<mode>.json next to the verdicts.
"""
import argparse
import glob
import json
import math
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from itermax_prep import EN_STOP, is_score_content  # noqa: E402


def binom_two_sided(k, n):
    if n == 0:
        return 1.0
    p = sum(math.comb(n, i) for i in range(0, min(k, n - k) + 1)) / 2 ** n * 2
    return min(1.0, p)


def classify(rows):
    """Counts per (word class, how the highlight moved) for one sentence, from
    itermax_wordchanges.py's exact per-word-index change list."""
    cls = {}
    for r in rows:
        cls[f"{r['kind']}_{r['how']}"] = cls.get(f"{r['kind']}_{r['how']}", 0) + 1
    return cls


def do_pairs(d):
    key = json.load(open(f"{d}/key.json", encoding="utf-8"))
    la, lb, pairs = key["a_label"], key["b_label"], key["pairs"]
    wchg = json.load(open(f"{d}/wordchanges.json", encoding="utf-8"))

    v1, v2, notes = {}, {}, {}
    for f in glob.glob(f"{d}/verdict-*-o1.json"):
        v1.update(json.load(open(f, encoding="utf-8")))
    for f in glob.glob(f"{d}/verdict-*-o2.json"):
        v2.update(json.load(open(f, encoding="utf-8")))
    for f in glob.glob(f"{d}/notes-*.json"):
        order = 1 if "-o1" in f else 2
        for k, v in json.load(open(f, encoding="utf-8")).items():
            notes.setdefault(k, {})[order] = v

    def dewrap(sid, v, order):
        if v not in ("X", "Y"):
            return "tie"
        swap = pairs[sid]["swap"]
        picked_first = (v == "X") if order == 1 else (v == "Y")
        return lb if (picked_first == swap) else la

    wins = {la: 0, lb: 0}
    ties = inconsistent = missing = 0
    rows = []
    for sid in pairs:
        a1, a2 = v1.get(sid), v2.get(sid)
        if a1 is None or a2 is None:
            missing += 1
            continue
        r1, r2 = dewrap(sid, a1, 1), dewrap(sid, a2, 2)
        if r1 == "tie" or r2 == "tie":
            outcome = "tie"
            ties += 1
        elif r1 != r2:
            outcome = "tie"
            inconsistent += 1
        else:
            outcome = r1
            wins[r1] += 1
        wr = wchg.get(str(pairs[sid]["i"]), [])
        rows.append({
            "id": sid, "i": pairs[sid]["i"], "outcome": outcome,
            "src": pairs[sid]["src"],
            "reasons": [notes.get(sid, {}).get(1, {}).get("r", ""),
                        notes.get(sid, {}).get(2, {}).get("r", "")],
            "cls": classify(wr),
            "words": wr,
        })

    dec = wins[la] + wins[lb]
    p = binom_two_sided(min(wins[la], wins[lb]), dec) if dec else 1.0

    # what the losing side lost on
    agg, shape = {}, {}
    for r in rows:
        agg.setdefault(r["outcome"], {})
        for k, v in r["cls"].items():
            agg[r["outcome"]][k] = agg[r["outcome"]].get(k, 0) + v
        kinds = {w["kind"] for w in r["words"]}
        tag = ("closed-only" if kinds == {"closed"}
               else "content-only" if kinds == {"content"}
               else "mixed" if kinds else "none")
        shape.setdefault(tag, {}).setdefault(r["outcome"], 0)
        shape[tag][r["outcome"]] += 1

    out = {"pairs": len(pairs), "judged": len(pairs) - missing, "missing": missing,
           "ties": ties, "order_inconsistent": inconsistent, "decisive": dec,
           "wins": wins, "p": p, "class_totals": agg,
           "by_sentence_shape": shape, "rows": rows}
    json.dump(out, open(f"{d}/summary-pairs.json", "w"), ensure_ascii=False, indent=1)

    print(f"pairs: {len(pairs)}  judged: {len(pairs)-missing}  missing: {missing}")
    print(f"ties: {ties}  order-inconsistent (counted as ties): {inconsistent}")
    print(f"decisive: {dec}")
    for l in (la, lb):
        print(f"  {l}: {wins[l]}  ({wins[l]/dec:.1%} of decisive)" if dec else f"  {l}: 0")
    print(f"sign test two-sided p = {p:.4g}")
    print("word-change totals by outcome (source words inside the sampled sentences):")
    for o in sorted(agg):
        print(f"  {o}: " + ", ".join(f"{k}={v}" for k, v in sorted(agg[o].items())))
    print("outcome by what the sentence's changes touch:")
    for t in sorted(shape):
        print(f"  {t}: " + ", ".join(f"{k}={v}" for k, v in sorted(shape[t].items())))
    return out


def do_lit(d):
    lit = {r["i"]: r for r in []}
    rows = json.load(open(f"{d}/lit.json", encoding="utf-8"))
    by_id = {f"{r['i']}:{r['wi']}": r for r in rows}
    got = {}
    for f in sorted(glob.glob(f"{d}/litverdict-*.json")):
        got.update(json.load(open(f, encoding="utf-8")))
    dist = {"correct": 0, "partial": 0, "wrong": 0}
    out_rows = []
    for k, r in by_id.items():
        g = got.get(k)
        if not g:
            continue
        dist[g["v"]] += 1
        out_rows.append({**r, **g})
    n = sum(dist.values())
    print(f"newly lit words: {len(by_id)}  judged: {n}  missing: {len(by_id)-n}")
    for v in ("correct", "partial", "wrong"):
        print(f"  {v}: {dist[v]}  ({dist[v]/n:.1%})" if n else f"  {v}: 0")
    json.dump({"n": n, "total": len(by_id), "dist": dist, "rows": out_rows},
              open(f"{d}/summary-lit.json", "w"), ensure_ascii=False, indent=1)
    return dist


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("dir")
    ap.add_argument("--mode", choices=("pairs", "lit"), required=True)
    args = ap.parse_args()
    (do_pairs if args.mode == "pairs" else do_lit)(args.dir)


if __name__ == "__main__":
    main()
