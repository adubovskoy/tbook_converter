#!/usr/bin/env python3
"""Compare glossary BUILDER variants: sample size, entry cap, head-term rule.

The production builder (internal/translate/glossary.go: glossarySystemPrompt +
glossarySampleMax) sends one request with an evenly-spread sample of the book and
asks for at most 40 entries. This measures what raising the sample and the cap
recovers on its own, against the frequency-mined term list as ground truth —
i.e. whether a local miner is still needed after the cheap prompt change.

Usage: gloss_builder_arms.py BOOK.tbook --mined GLOSS.json --out-dir DIR
"""
import argparse, json, os, re, sys
from collections import Counter

sys.path.insert(0, __file__.rsplit("/", 1)[0])
from glossary_demand import fold, sentences
from gloss_scale_probe import API, env

BASE = open(os.path.join(os.path.dirname(os.path.abspath(__file__)),
                         ".artifacts/glossary-scale/prompt-go-glossary.txt"),
            encoding="utf-8").read()

HEAD_RULE = """
Give the HEAD TERM, never a phrase built around it: "Suntouch", not "Suntouch House";
"stack", not "cortical stack storage". One entry per term, in its base form.
Every recurring character, place and organisation name belongs here — a name has no
single obvious rendering."""


def variant(cap, head):
    s = BASE.replace("At most 40 entries.", f"At most {cap} entries.")
    if head:
        s = s.replace("\n\nReply with ONLY", HEAD_RULE + "\n\nReply with ONLY")
    return s


def sample(book, lang, n):
    src = [s["src"] for s, _ in sentences(book, lang)]
    step = max(1, len(src) // n)
    return src[::step][:n]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("book"); ap.add_argument("--lang", default="ru")
    ap.add_argument("--mined", required=True); ap.add_argument("--out-dir", required=True)
    ap.add_argument("--title", default="Altered Carbon"); ap.add_argument("--author", default="Richard Morgan")
    a = ap.parse_args()

    mined = json.load(open(a.mined))
    names = {fold(e["src"]): e for e in mined if e.get("kind") == "name"}
    must = {k: e for k, e in names.items() if e["freq"] >= 20}      # names a reader meets constantly
    api = API(env())

    arms = [("s200-c40 (production)", 200, 40, False),
            ("s600-c150", 600, 150, False),
            ("s600-c150+head", 600, 150, True),
            ("s1200-c250+head", 1200, 250, True)]
    print(f"ground truth: {len(names)} mined names, {len(must)} of them with freq >= 20\n")
    print(f"{'builder arm':<24} {'entries':>8} {'single':>7} {'phrases':>8} "
          f"{'names>=20 found':>16} {'all mined names':>16}")
    print("-" * 84)
    for label, ns, cap, head in arms:
        sys_p = variant(cap, head)
        user = json.dumps({"title": a.title, "author": a.author,
                           "sentences": sample(a.book, a.lang, ns)}, ensure_ascii=False)
        res = api.chat(sys_p, user)
        g = [e for e in (res.get("glossary") or []) if e.get("src") and e.get("tgt")]
        words = set()
        for e in g:
            for w in re.findall(r"[A-Za-z][A-Za-z'’\-]*", e["src"]):
                words.add(fold(w))
        single = sum(1 for e in g if len(re.findall(r"[^\W_]+", e["src"])) == 1)
        hit_must = sum(1 for k in must if k in words)
        hit_all = sum(1 for k in names if k in words)
        print(f"{label:<24} {len(g):>8} {single:>7} {len(g)-single:>8} "
              f"{f'{hit_must}/{len(must)}':>16} {f'{hit_all}/{len(names)}':>16}")
        missing = sorted((must[k] for k in must if k not in words), key=lambda e: -e["freq"])
        if missing:
            print("      missed frequent names: "
                  + ", ".join(f"{e['src']}×{e['freq']}" for e in missing[:12]))
        json.dump(g, open(os.path.join(a.out_dir, f"builder-{label.split()[0]}.json"), "w"),
                  ensure_ascii=False, indent=1)
    st = [s for s in api.stats if s.get("prompt")]
    print(f"\nbuilder cost: ${sum((s['cost'] or 0) for s in st):.4f} over {len(st)} requests; "
          f"prompt {[s['prompt'] for s in st]}")


if __name__ == "__main__":
    main()
