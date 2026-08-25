#!/usr/bin/env python3
"""Measure how often the enforced glossary rendering actually appears.

For every glossary entry, find the sentences whose SOURCE contains the term and
check whether the TARGET sentence contains the enforced rendering (matched with
the pipeline's own inflection-tolerant prefix rule, so Бэнкрофт/Бэнкрофта count
as the same rendering). Reports adherence per entry and in aggregate.

Usage: glossary_adherence.py BOOK.tbook GLOSSARY.json [--lang ru] [--json OUT]
"""
import argparse, json, re, sys, zipfile
from collections import Counter

sys.path.insert(0, __file__.rsplit("/", 1)[0])
from glossary_demand import fold, prefix_plausible, sentences

TOKEN = re.compile(r"[^\W\d_]+", re.UNICODE)
# Function words that carry no identity in a multi-word rendering.
SKIP_TGT = set("в на из под над за для по с со и или но а не то что как это the of a an".split())


def content_words(s):
    return [w for w in TOKEN.findall(s) if len(w) >= 4 and w.lower() not in SKIP_TGT]


def src_pattern(term):
    esc = r"[\s\-]+".join(re.escape(w) for w in re.findall(r"[^\W_]+", term))
    return re.compile(r"(?<![^\W_])" + esc + r"(?![^\W_])", re.IGNORECASE | re.UNICODE)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("book"); ap.add_argument("glossary")
    ap.add_argument("--lang", default="ru")
    ap.add_argument("--json"); ap.add_argument("--quiet", action="store_true")
    a = ap.parse_args()

    gloss = json.load(open(a.glossary))
    if isinstance(gloss, dict):            # sidecar file shape {scope…, terms:[…]}
        gloss = gloss["terms"]
    sents = [(s["src"], tr["text"]) for s, tr in sentences(a.book, a.lang) if tr]
    pats = [(e, src_pattern(e["src"]), content_words(e["tgt"])) for e in gloss]

    rows = []
    for e, pat, want in pats:
        hits = [(src, tgt) for src, tgt in sents if pat.search(src)]
        if not want or not hits:
            rows.append({"src": e["src"], "tgt": e["tgt"], "occurrences": len(hits),
                         "full": 0, "partial": 0, "miss": 0, "checkable": bool(want)})
            continue
        full = partial = 0
        misses = []
        for src, tgt in hits:
            toks = [fold(t) for t in TOKEN.findall(tgt)]
            got = sum(any(prefix_plausible(fold(w), t) for t in toks) for w in want)
            if got == len(want):
                full += 1
            elif got:
                partial += 1
            else:
                misses.append(tgt)
        rows.append({"src": e["src"], "tgt": e["tgt"], "occurrences": len(hits),
                     "full": full, "partial": partial, "miss": len(misses),
                     "checkable": True, "miss_examples": misses[:3]})

    ck = [r for r in rows if r["checkable"] and r["occurrences"]]
    occ = sum(r["occurrences"] for r in ck)
    full = sum(r["full"] for r in ck); part = sum(r["partial"] for r in ck)
    miss = sum(r["miss"] for r in ck)
    print(f"{a.glossary.split('/')[-1]}: {len(gloss)} entries, {len(ck)} checkable and present "
          f"({sum(1 for r in rows if r['checkable'] and not r['occurrences'])} never occur in the book)")
    print(f"occurrences {occ}: full {full} ({100*full/occ:.1f}%), "
          f"partial {part} ({100*part/occ:.1f}%), miss {miss} ({100*miss/occ:.1f}%)")
    single = [r for r in ck if len(TOKEN.findall(r["src"])) == 1]
    if single:
        so = sum(r["occurrences"] for r in single); sf = sum(r["full"] for r in single)
        print(f"  single-word terms ({len(single)}): {100*sf/so:.1f}% full on {so} occurrences")
    multi = [r for r in ck if len(TOKEN.findall(r["src"])) > 1]
    if multi:
        mo = sum(r["occurrences"] for r in multi); mf = sum(r["full"] for r in multi)
        print(f"  multi-word terms ({len(multi)}): {100*mf/mo:.1f}% full on {mo} occurrences")

    if not a.quiet:
        print("\nworst adherence (>=3 occurrences):")
        for r in sorted((r for r in ck if r["occurrences"] >= 3),
                        key=lambda r: (r["full"] / r["occurrences"], -r["occurrences"]))[:15]:
            print(f"  {r['src']:<34} → {r['tgt']:<34} {r['full']:>3}/{r['occurrences']:<3} full"
                  f"  {r['miss']} miss")
    if a.json:
        json.dump(rows, open(a.json, "w"), ensure_ascii=False, indent=1)


if __name__ == "__main__":
    main()
