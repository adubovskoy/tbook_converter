#!/usr/bin/env python3
"""Measure how many enforceable terms a book actually has (glossary demand).

Mines recurring proper nouns / capitalised terms from the SOURCE side of a
.tbook, then uses the stored word alignment to read back which TARGET rendering
each occurrence actually got. An unglossed recurring name that comes out as
several different renderings is direct evidence that the glossary is too small.

Usage: glossary_demand.py BOOK.tbook [--lang ru] [--min-freq 3] [--json OUT]
"""
import argparse, json, re, sys, unicodedata, zipfile
from collections import Counter, defaultdict

STOP = set("""the a an and or but if then so as at by for in of on to with from into
that this these those he she it they we you i his her its their our your my me him them
there here when while what which who whom whose how why not no nor own very too also
was were is are be been being have has had do does did done said say says would could
should will shall may might must can cannot am about after again against all almost
along already although always among another any anyone anything around because become
been before behind below beneath beside besides between beyond both came come coming
did didn don down during each either else enough even ever every everything except far
few finally first found four from front full further gave get give given go going gone
got half hard has have having he her here hers herself him himself his how however i
if in inside instead into is it its itself just keep kept know known last later least
left less let like little long look looked made make many may maybe me mean might mine
more most much must my myself near need never new next nine no none nor not nothing now
of off often on once one only onto or other others our ours out outside over own past
perhaps put quite rather really right round said same saw say see seemed seen set several
shall she should side since six so some someone something soon still such take taken tell
than that the their them then there these they thing think this those though three through
thus till time to together too took toward towards two under until up upon us used using
very want was way we well went were what when where whether which while who whole whom
whose why will with within without would yes yet you your yours""".split())
# Sentence-initial capitals are ambiguous; a term must appear capitalised at
# least once in a non-initial position to count as a proper noun.
WORD = re.compile(r"[A-Za-z][A-Za-z'’\-]*")
POSS = re.compile(r"[’']s$")

# prefixPlausible / fold are ports of internal/lexcheck/lexcheck.go so that
# "the same rendering in another case" is judged by the same rule the pipeline
# already uses (Бэнкрофт/Бэнкрофта = one rendering; Айрин/Ирэн = two).
PREFIX_MIN_LEN, PREFIX_MIN_COMMON, PREFIX_SLACK = 4, 3, 3


def fold(s):
    return "".join(c for c in s.lower() if c.isalpha())


def prefix_plausible(a, b):
    if a == b:
        return a != ""
    la, lb = len(a), len(b)
    if la < PREFIX_MIN_LEN or lb < PREFIX_MIN_LEN:
        return False
    m = min(la, lb)
    common = 0
    while common < m and a[common] == b[common]:
        common += 1
    slack = PREFIX_SLACK if m >= 6 else m - PREFIX_MIN_COMMON
    return common >= PREFIX_MIN_COMMON and common >= m - slack


def cluster(counter):
    """Collapse inflectional variants; return [(label, count, [forms])] desc."""
    items = counter.most_common()
    clusters = []
    for form, n in items:
        f = fold(form)
        for cl in clusters:
            if prefix_plausible(fold(cl["label"]), f):
                cl["n"] += n
                cl["forms"].append(form)
                break
        else:
            clusters.append({"label": form, "n": n, "forms": [form]})
    clusters.sort(key=lambda c: -c["n"])
    return clusters


def sentences(path, lang):
    """Yield (sentence, translation|None) pairs from a .tbook, or from a
    dumpsents JSON dump of a book that has not been translated yet (source-side
    analysis and probe building need no translation)."""
    if path.endswith(".json"):
        d = json.load(open(path, encoding="utf-8"))
        for s in d["sentences"]:
            yield s, None
        return
    z = zipfile.ZipFile(path)
    names = sorted((n for n in z.namelist() if n.startswith("chapters/")),
                   key=lambda n: int(re.search(r"(\d+)", n).group(1)))
    for n in names:
        ch = json.loads(z.read(n))
        for para in ch["paragraphs"]:
            for s in para:
                tr = (s.get("tr") or {}).get(lang)
                yield s, tr


def norm_tgt(s):
    s = unicodedata.normalize("NFC", s).strip().lower()
    return re.sub(r"^[^\w]+|[^\w]+$", "", s)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("book")
    ap.add_argument("--lang", default="ru")
    ap.add_argument("--min-freq", type=int, default=3)
    ap.add_argument("--json")
    a = ap.parse_args()

    sents = list(sentences(a.book, a.lang))
    print(f"{a.book}: {len(sents)} sentences, {sum(1 for _, t in sents if t)} translated into {a.lang}")

    # Pass 1: which capitalised tokens ever appear mid-sentence (proper nouns).
    proper = Counter()
    total_caps = Counter()
    lower_seen = Counter()   # same token seen lowercase -> a common word, not a name
    for s, _ in sents:
        toks = [(m.group(0), m.start()) for m in WORD.finditer(s["src"])]
        for i, (w, off) in enumerate(toks):
            w = POSS.sub("", w)  # Bancroft's -> Bancroft
            if w and w[0].islower():
                lower_seen[w.lower()] += 1
                continue
            if not w or not w[0].isupper() or w.lower() in STOP or len(w) < 3:
                continue
            if w == "I" or w.startswith("I’") or w.startswith("I'"):
                continue  # I’m / I’d — capitalised only because of "I"
            total_caps[w] += 1
            if i > 0:  # not sentence-initial
                proper[w] += 1

    # A term qualifies if it is capitalised mid-sentence at least twice.
    terms = {w for w, c in proper.items() if c >= 2}
    # A proper noun is never (or almost never) written lowercase in the book;
    # "World"/"God"/"Real" are capitalised common words and drift for reasons a
    # glossary cannot fix, so they are reported separately.
    names = {w for w in terms if lower_seen[w.lower()] <= max(1, total_caps[w] // 20)}
    freq = Counter({w: total_caps[w] for w in terms})
    print(f"of {len(terms)} candidates, {len(names)} are never written lowercase "
          f"(proper nouns); {len(terms) - len(names)} are capitalised common words")

    # Pass 2: read back the target rendering of every occurrence via alignment.
    variants = defaultdict(Counter)     # term -> Counter(target rendering)
    for s, tr in sents:
        if not tr or not tr.get("align"):
            continue
        src, words, text = s["src"], s.get("words") or [], tr["text"]
        idx_of = {}
        for i, (b, e) in enumerate(words):
            tok = src[b:e]
            m = WORD.search(tok)
            if not m:
                continue
            base = POSS.sub("", m.group(0))
            if base in terms:
                idx_of.setdefault(i, base)
        if not idx_of:
            continue
        for ch in tr["align"]:
            ws = ch.get("w") or []
            hit = {idx_of[i] for i in ws if i in idx_of}
            if len(hit) == 1 and len(ws) <= 3:
                t = norm_tgt(text[ch["t"][0]:ch["t"][1]])
                if t:
                    variants[hit.pop()][t] += 1

    ranked = sorted(freq.items(), key=lambda kv: -kv[1])
    mass = sum(freq.values())
    print(f"\ncandidate terms (capitalised mid-sentence >=2x): {len(ranked)}, "
          f"total occurrences {mass}")
    for k in (20, 40, 60, 100, 150, 200, 300, 400, len(ranked)):
        if k > len(ranked):
            continue
        top = sum(c for _, c in ranked[:k])
        print(f"  top {k:>4}: {100*top/mass:5.1f}% of term occurrences  "
              f"(min freq in cut: {ranked[k-1][1]})")

    for mf in (2, 3, 5, 10, 20, 50):
        print(f"  terms with freq >= {mf:>3}: {sum(1 for _, c in ranked if c >= mf)}")

    # Inconsistency: recurring terms rendered more than one way.
    print(f"\n--- rendering consistency (terms with freq >= {a.min_freq}, alignment-derived) ---")
    rows = []
    for w, c in ranked:
        if c < a.min_freq or w not in variants:
            continue
        v = variants[w]
        tot = sum(v.values())
        if tot < 2:
            continue
        cl = cluster(v)
        rows.append({"term": w, "name": w in names, "freq": c, "aligned": tot, "distinct": len(cl),
                     "dominant_share": cl[0]["n"] / tot,
                     "variants": [(c2["label"], c2["n"]) for c2 in cl[:6]],
                     "surface": v.most_common(8)})
    rows.sort(key=lambda r: (-(r["distinct"] > 1), -r["aligned"] * (1 - r["dominant_share"])))
    multi = [r for r in rows if r["distinct"] > 1]
    print(f"{len(rows)} terms have alignment evidence; {len(multi)} get MORE THAN ONE rendering")
    if rows:
        inconsistent_mass = sum(r["aligned"] * (1 - r["dominant_share"]) for r in rows)
        print(f"off-dominant occurrences: {inconsistent_mass:.0f} of "
              f"{sum(r['aligned'] for r in rows)} aligned ({100*inconsistent_mass/sum(r['aligned'] for r in rows):.1f}%)")
    for r in multi[:25]:
        vs = ", ".join(f"{t}×{n}" for t, n in r["variants"])
        print(f"  {r['term']:<18} freq {r['freq']:>4}  {r['distinct']} variants  {vs}")

    if a.json:
        json.dump({"terms": [{"term": w, "freq": c} for w, c in ranked], "consistency": rows},
                  open(a.json, "w"), ensure_ascii=False, indent=1)
        print(f"\nwrote {a.json}")


if __name__ == "__main__":
    main()
