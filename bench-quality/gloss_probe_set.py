#!/usr/bin/env python3
"""Build a term-dense probe set for the glossary-scale arms.

Two strata:
  term  — sentences containing a mined candidate term, greedily chosen so that
          every term gets up to K sentences (rare terms included on purpose);
  ctrl  — sentences containing NO candidate term, to measure collateral damage
          from a large glossary on text it has no business touching.

Usage: gloss_probe_set.py BOOK.tbook CANDIDATES.json --out PROBE.json
"""
import argparse, json, re, sys
from collections import defaultdict

sys.path.insert(0, __file__.rsplit("/", 1)[0])
from glossary_adherence import src_pattern
from glossary_demand import sentences


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("book"); ap.add_argument("candidates")
    ap.add_argument("--lang", default="ru")
    ap.add_argument("--per-term", type=int, default=2)
    ap.add_argument("--ctrl", type=int, default=150)
    ap.add_argument("--min-len", type=int, default=45)
    ap.add_argument("--max-len", type=int, default=240)
    ap.add_argument("--out", required=True)
    a = ap.parse_args()

    cands = json.load(open(a.candidates))
    pats = [(c["term"], src_pattern(c["term"])) for c in cands]
    sents = [(s["src"], (tr or {}).get("text")) for s, tr in sentences(a.book, a.lang)]
    sents = [(i, src, ref) for i, (src, ref) in enumerate(sents)
             if a.min_len <= len(src) <= a.max_len]
    print(f"{len(sents)} candidate sentences in the length window")

    hits = defaultdict(list)          # term -> [(idx, src, ref)]
    per_sent = defaultdict(list)      # idx -> [terms]
    for term, pat in pats:
        for i, src, ref in sents:
            if pat.search(src):
                hits[term].append((i, src, ref))
                per_sent[i].append(term)

    # Rarest terms first, so a rare term is not crowded out by a frequent one.
    chosen, need = {}, {c["term"]: a.per_term for c in cands}
    for term in sorted(hits, key=lambda t: len(hits[t])):
        got = sum(1 for i in chosen if term in per_sent[i])
        for i, src, ref in hits[term]:
            if got >= need[term]:
                break
            if i not in chosen:
                chosen[i] = {"i": i, "src": src, "ref": ref, "terms": per_sent[i], "stratum": "term"}
                got += 1

    ctrl = [(i, src, ref) for i, src, ref in sents if i not in per_sent]
    step = max(1, len(ctrl) // a.ctrl)
    for i, src, ref in ctrl[::step][:a.ctrl]:
        chosen[i] = {"i": i, "src": src, "ref": ref, "terms": [], "stratum": "ctrl"}

    probe = sorted(chosen.values(), key=lambda r: r["i"])
    covered = {t for r in probe for t in r["terms"]}
    json.dump(probe, open(a.out, "w"), ensure_ascii=False, indent=1)
    print(f"probe: {len(probe)} sentences "
          f"({sum(1 for r in probe if r['stratum']=='term')} term / "
          f"{sum(1 for r in probe if r['stratum']=='ctrl')} ctrl), "
          f"{len(covered)}/{len(cands)} candidate terms covered -> {a.out}")
    print(f"  term occurrences in probe: {sum(len(r['terms']) for r in probe)}")


if __name__ == "__main__":
    main()
