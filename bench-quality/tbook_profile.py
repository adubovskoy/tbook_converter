#!/usr/bin/env python3
"""Profile the translation text of one or more .tbook files (no API calls).

Reports, per file: sentence/word counts, empty translations, mean translation
length, type-token ratio, Latin-script leakage, dialogue punctuation style, and
— with --name — how a source proper name is rendered across the book, which is
the one book-wide consistency defect a glossary does not cover.

Usage:
  tbook_profile.py a.tbook b.tbook … [-t ru] [--name Sylveste --name Nagorny]
"""
import argparse, collections, json, re, sys, zipfile

LAT = re.compile(r"[A-Za-z]{2,}")
CYR_WORD = re.compile(r"[А-Яа-яЁё]+")
WORD = re.compile(r"\w+", re.UNICODE)


def sentences(path, tgt):
    with zipfile.ZipFile(path) as z:
        for n in sorted(z.namelist()):
            if n.startswith("chapters/") and n.endswith(".json"):
                for para in json.loads(z.read(n))["paragraphs"]:
                    for s in para:
                        tr = (s.get("tr", {}).get(tgt, {}) or {}).get("text") or ""
                        yield s.get("src", ""), tr.strip()


def profile(path, tgt, names):
    rows = list(sentences(path, tgt))
    trs = [t for _, t in rows]
    empty = sum(1 for t in trs if not t)
    toks = [w.lower() for t in trs for w in WORD.findall(t)]
    lat = collections.Counter(w for t in trs for w in LAT.findall(t))
    quoted = [(s, t) for s, t in rows if s.lstrip().startswith(("“", '"', "‘"))]
    style = collections.Counter()
    for _, t in quoted:
        if t.startswith("«"):
            style["guillemets"] += 1
        elif t.startswith(("—", "-", "–")):
            style["em-dash"] += 1
        elif t.startswith(("“", '"')):
            style["quotes"] += 1
        else:
            style["none"] += 1
    out = {
        "file": path.split("/")[-1],
        "sentences": len(rows),
        "empty": empty,
        "mean_tr_chars": round(sum(len(t) for t in trs) / max(1, len(trs) - empty), 1),
        "ttr": round(len(set(toks)) / max(1, len(toks)), 3),
        "latin_tokens": sum(lat.values()),
        "latin_top": lat.most_common(6),
        "quote_openers": len(quoted),
        "quote_style": dict(style),
        "names": {},
    }
    for name in names:
        pat = re.compile(r"\b" + re.escape(name), re.I)
        variants = collections.Counter()
        for s, t in rows:
            if pat.search(s):
                for w in CYR_WORD.findall(t):
                    if len(w) >= 4:
                        variants[w] += 1
        # keep only forms that look like a rendering of the name: same first letter
        # class is too weak, so report the most frequent capitalised forms
        caps = collections.Counter({w: c for w, c in variants.items() if w[0].isupper()})
        out["names"][name] = caps.most_common(8)
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("books", nargs="+")
    ap.add_argument("-t", "--target", default="ru")
    ap.add_argument("--name", action="append", default=[])
    ap.add_argument("--json", action="store_true")
    args = ap.parse_args()
    res = [profile(b, args.target, args.name) for b in args.books]
    if args.json:
        json.dump(res, sys.stdout, ensure_ascii=False, indent=1)
        return
    for r in res:
        print(f"== {r['file']}")
        print(f"   sentences {r['sentences']}  empty {r['empty']}  "
              f"mean chars {r['mean_tr_chars']}  TTR {r['ttr']}")
        print(f"   latin tokens {r['latin_tokens']} {r['latin_top']}")
        print(f"   quote openers {r['quote_openers']} -> {r['quote_style']}")
        for n, v in r["names"].items():
            print(f"   {n}: {v}")


if __name__ == "__main__":
    main()
