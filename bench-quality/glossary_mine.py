#!/usr/bin/env python3
"""Mine glossary candidates from the WHOLE book, frequency-ranked.

Two candidate classes, both deterministic and free:
  names — tokens capitalised mid-sentence and (almost) never seen lowercase;
  coined — frequent lowercase tokens absent from the bilingual lexicon that
           lexcheck already ships (invented terminology: neurachem, bubblefab).

Emits {term, kind, freq, examples} ranked by frequency — the candidate list a
rendering pass turns into a glossary.

Usage: glossary_mine.py BOOK.tbook --lexicon lexicons/en-ru.tsv.gz --out C.json
"""
import argparse, gzip, json, re, sys
from collections import Counter, defaultdict

sys.path.insert(0, __file__.rsplit("/", 1)[0])
from glossary_demand import STOP, WORD, POSS, sentences, cluster, fold, norm_tgt


# Function words and inflections the bilingual lexicon happens not to list.
# The lexicon is a translation dictionary, not a wordlist, so "into"/"going"
# are absent from it; without this the coined-term detector is 60% noise.
FUNC = set("""into onto unto upon within without beyond beside besides during unless
whether neither nor shall aboard toward towards throughout amongst albeit whilst
somewhere someone something anywhere anyone anything everywhere everyone everything
nowhere nothing whoever whatever whenever wherever however therefore otherwise
instead perhaps maybe already almost enough rather quite indeed also though although
because since while until unless once twice thrice ain gonna gotta""".split())


def destem(w):
    """Light English de-inflection so shrugged/going/walked hit the lexicon."""
    out = {w}
    for suf, add in (("ing", ""), ("ed", ""), ("s", ""), ("es", ""), ("ly", ""),
                     ("er", ""), ("est", ""), ("ing", "e"), ("ed", "e")):
        if w.endswith(suf) and len(w) - len(suf) >= 3:
            base = w[: len(w) - len(suf)] + add
            out.add(base)
            if len(base) > 3 and base[-1] == base[-2]:   # grinned -> grin
                out.add(base[:-1])
    return out


def load_lexicon(path):
    known = set()
    op = gzip.open if path.endswith(".gz") else open
    with op(path, "rt", encoding="utf-8") as f:
        for line in f:
            w = line.split("\t", 1)[0].strip().lower()
            if w:
                known.add(w)
    return known


def drift_candidates(book, lang, known, min_freq, min_off=0.15):
    """Ordinary words used in a book-specific sense.

    Frequency mining cannot see them (`stack`, `sleeve` are everyday English and
    sit in the lexicon), but a reference translation can: read back what each
    occurrence was rendered as and keep the words whose rendering does not
    settle on one form. Renderings are read through the stored alignment, and
    inflections are collapsed with the pipeline's own prefix rule.
    """
    freq, variants = Counter(), defaultdict(Counter)
    examples = defaultdict(list)
    for s, tr in sentences(book, lang):
        src, words = s["src"], s.get("words") or []
        interesting = {}
        for i, (b, e) in enumerate(words):
            m = WORD.search(src[b:e])
            if not m:
                continue
            w = m.group(0).lower()
            if len(w) < 4 or w in STOP or w not in known:
                continue
            freq[w] += 1
            interesting[i] = w
            if len(examples[w]) < 2 and 40 < len(src) < 220:
                examples[w].append(src)
        if not tr or not tr.get("align") or not interesting:
            continue
        for ch in tr["align"]:
            ws = ch.get("w") or []
            hit = {interesting[i] for i in ws if i in interesting}
            if len(hit) == 1 and len(ws) <= 2:
                t = norm_tgt(tr["text"][ch["t"][0]:ch["t"][1]])
                if t:
                    variants[hit.pop()][t] += 1

    out = []
    for w, c in freq.items():
        if c < min_freq or w not in variants:
            continue
        v = variants[w]
        tot = sum(v.values())
        if tot < min_freq:
            continue
        cl = cluster(v)
        sig = [x for x in cl if x["n"] >= 2 and x["n"] / tot >= 0.10]
        if len(sig) < 2:
            continue
        off = sum(x["n"] for x in sig[1:]) / tot
        if off < min_off:
            continue
        out.append({"term": w, "kind": "sense", "freq": c, "off_dominant": round(off, 3),
                    "renderings": [(x["label"], x["n"]) for x in sig[:5]],
                    "examples": examples[w][:2]})
    out.sort(key=lambda r: -(r["freq"] * r["off_dominant"]))
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("book")
    ap.add_argument("--lang", default="ru")
    ap.add_argument("--lexicon", default="lexicons/en-ru.tsv.gz")
    ap.add_argument("--min-freq", type=int, default=4)
    ap.add_argument("--out", required=True)
    ap.add_argument("--drift", type=int, default=0,
                    help="also mine N book-specific senses of ordinary words "
                         "from the reference translation's alignment")
    a = ap.parse_args()

    known = load_lexicon(a.lexicon)
    print(f"lexicon: {len(known)} known source words")

    caps, mid, lower, examples = Counter(), Counter(), Counter(), defaultdict(list)
    for s, _ in sentences(a.book, a.lang):
        src = s["src"]
        toks = [m.group(0) for m in WORD.finditer(src)]
        for i, raw in enumerate(toks):
            w = POSS.sub("", raw)
            if len(w) < 3:
                continue
            if w in ("I",) or w.startswith("I’") or w.startswith("I'"):
                continue  # capitalised only because of the pronoun "I"
            if w[0].isupper():
                caps[w] += 1
                if i > 0:
                    mid[w] += 1
                if len(examples[w]) < 2 and 40 < len(src) < 220:
                    examples[w].append(src)
            else:
                lower[w.lower()] += 1
                if len(examples[w.lower()]) < 2 and 40 < len(src) < 220:
                    examples[w.lower()].append(src)

    names = [(w, caps[w]) for w, c in mid.items()
             if c >= 2 and lower[w.lower()] <= max(1, caps[w] // 20)
             and w.lower() not in STOP and caps[w] >= a.min_freq]
    # Coined terms: frequent, lowercase, unknown to the bilingual lexicon.
    def unknown(w):
        if w in FUNC or w in STOP:
            return False
        cands = set()
        for form in (w, w.replace("-", ""), *w.split("-")):
            if len(form) >= 3:
                cands |= destem(form)
        return not (cands & known)

    coined = [(w, c) for w, c in lower.items()
              if c >= a.min_freq and "'" not in w and "’" not in w and unknown(w)]

    drift = drift_candidates(a.book, a.lang, known, max(8, a.min_freq * 2))[:a.drift] if a.drift else []
    if drift:
        print(f"drift candidates (ordinary words with unstable renderings): {len(drift)}")
        for r in drift[:15]:
            print(f"  {r['freq']:>4}  {r['off_dominant']:.2f} off  {r['term']:<14} "
                  + ", ".join(f"«{t}»×{n}" for t, n in r["renderings"][:4]))

    out = list(drift)
    for kind, lst in (("name", names), ("coined", coined)):
        for w, c in lst:
            out.append({"term": w, "kind": kind, "freq": c, "examples": examples.get(w, [])[:2]})
    out.sort(key=lambda r: -r["freq"])
    json.dump(out, open(a.out, "w"), ensure_ascii=False, indent=1)

    print(f"candidates: {len(out)} total — {len(names)} names, {len(coined)} coined "
          f"(freq >= {a.min_freq})")
    for k in (40, 100, 200, 300, 400, len(out)):
        if k <= len(out):
            print(f"  top {k:>4}: min freq {out[k-1]['freq']}, "
                  f"{sum(r['freq'] for r in out[:k])} occurrences")
    print("\ntop 30 candidates:")
    for r in out[:30]:
        print(f"  {r['freq']:>4}  {r['kind']:<6}  {r['term']}")
    print("\ncoined terms in the top 120:")
    print("  " + ", ".join(r["term"] for r in out[:120] if r["kind"] == "coined"))


if __name__ == "__main__":
    main()
