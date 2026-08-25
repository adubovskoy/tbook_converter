#!/usr/bin/env python3
"""Analyse the glossary-scale arms.

For each arm: adherence to its own glossary, adherence on a FIXED shared term
set (the dilution measurement — same 244 real entries, different amounts of
padding around them), and collateral drift on the control stratum (sentences
containing no glossary term at all, compared against the no-glossary arm).

Usage: gloss_scale_analyze.py --dir ART  arm=NAME:GLOSSARY[:LABEL] …
"""
import argparse, difflib, json, os, re, sys
from collections import Counter

sys.path.insert(0, __file__.rsplit("/", 1)[0])
from glossary_adherence import content_words, src_pattern
from glossary_demand import fold, prefix_plausible

TOKEN = re.compile(r"[^\W\d_]+", re.UNICODE)


def hit(tgt_text, want_words):
    toks = [fold(t) for t in TOKEN.findall(tgt_text)]
    got = sum(any(prefix_plausible(fold(w), t) for t in toks) for w in want_words)
    return "full" if got == len(want_words) else ("partial" if got else "miss")


def adherence(rows, gloss, only=None):
    """rows: [{src, tr}]; gloss: [{src,tgt}]. Returns per-term and totals."""
    per, tot = {}, Counter()
    for e in gloss:
        if only is not None and e["src"] not in only:
            continue
        want = content_words(e["tgt"])
        if not want:
            continue
        pat = src_pattern(e["src"])
        c = Counter()
        for r in rows:
            if r.get("tr") and pat.search(r["src"]):
                c[hit(r["tr"], want)] += 1
        if sum(c.values()):
            per[e["src"]] = c
            tot += c
    return per, tot


def norm(s):
    return re.sub(r"\s+", " ", (s or "").strip())


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dir", required=True)
    ap.add_argument("--probe", default="probe.json")
    ap.add_argument("--base", default="a0", help="no-glossary arm for collateral comparison")
    ap.add_argument("--shared", required=True, help="glossary file defining the shared term set")
    ap.add_argument("arms", nargs="+", help="NAME:GLOSSARY_FILE_or_'-'")
    a = ap.parse_args()

    D = a.dir
    probe = {r["i"]: r for r in json.load(open(os.path.join(D, a.probe)))}
    shared = json.load(open(os.path.join(D, a.shared)))
    shared_terms = {e["src"] for e in shared}
    arms = []
    for spec in a.arms:
        name, gl = spec.split(":", 1)
        rows = json.load(open(os.path.join(D, f"out-{name}.json")))
        gloss = json.load(open(os.path.join(D, gl))) if gl != "-" else []
        arms.append((name, gloss, rows))

    # Sentences translated in every arm — the comparable set.
    common = set.intersection(*[{r["i"] for r in rows if r.get("tr")} for _, _, rows in arms])
    print(f"probe {len(probe)} sentences; {len(common)} translated in all "
          f"{len(arms)} arms and used below")
    base_rows = {r["i"]: r for _, _, rows in arms for r in rows
                 if r["i"] in common and _ == a.base} if False else None
    base = next(rows for n, _, rows in arms if n == a.base)
    base_by_i = {r["i"]: r for r in base}

    hdr = (f"{'arm':>7} {'N':>6} {'own gloss adherence':>26} "
           f"{'shared-244 adherence':>26} {'ctrl identical':>15} {'ctrl diff':>10}")
    print("\n" + hdr); print("-" * len(hdr))
    out = {}
    for name, gloss, rows in arms:
        rows = [r for r in rows if r["i"] in common]
        own_per, own = adherence(rows, gloss)
        sh_per, sh = adherence(rows, shared, only=shared_terms)

        def pct(c):
            n = sum(c.values())
            return f"{100*c['full']/n:5.1f}% full /{100*c['miss']/n:5.1f}% miss ({n})" if n else "—"

        ctrl = [r for r in rows if probe[r["i"]]["stratum"] == "ctrl"]
        same = sum(1 for r in ctrl if norm(r["tr"]) == norm(base_by_i[r["i"]].get("tr")))
        sim = [difflib.SequenceMatcher(None, norm(r["tr"]), norm(base_by_i[r["i"]].get("tr"))).ratio()
               for r in ctrl]
        print(f"{name:>7} {len(gloss):>6} {pct(own):>26} {pct(sh):>26} "
              f"{f'{same}/{len(ctrl)}':>15} {1-sum(sim)/len(sim):>9.3f}")
        out[name] = {"n_entries": len(gloss), "own": dict(own), "shared": dict(sh),
                     "ctrl_identical": same, "ctrl_n": len(ctrl),
                     "ctrl_mean_diff": 1 - sum(sim) / len(sim),
                     "per_term_shared": {k: dict(v) for k, v in sh_per.items()}}

    # Pairwise control-stratum distance: if every pair sits at the same value,
    # the difference is sampling noise (temperature 0.3), not the glossary.
    print("\ncontrol-stratum mean difference, every arm pair (noise floor):")
    labels = [n for n, _, _ in arms]
    by = {n: {r["i"]: r for r in rows if r["i"] in common} for n, _, rows in arms}
    ctrl_ids = [i for i in common if probe[i]["stratum"] == "ctrl"]
    print("        " + " ".join(f"{n:>7}" for n in labels))
    for x in labels:
        cells = []
        for y in labels:
            if x == y:
                cells.append("      —")
                continue
            d = [1 - difflib.SequenceMatcher(None, norm(by[x][i].get("tr")),
                                             norm(by[y][i].get("tr"))).ratio() for i in ctrl_ids]
            cells.append(f"{sum(d)/len(d):>7.3f}")
        print(f"{x:>7} " + " ".join(cells))

    # Terms that gain or lose the most between arms.
    names = [n for n, _, _ in arms]
    if len(names) >= 2:
        first, last = out[names[1]], out[names[-1]]
        print(f"\nper-term shifts, {names[1]} (N={first['n_entries']}) → "
              f"{names[-1]} (N={last['n_entries']}):")
        deltas = []
        for t, c in last["per_term_shared"].items():
            b = first["per_term_shared"].get(t)
            if not b:
                continue
            nb, nl = sum(b.values()), sum(c.values())
            if nb and nl:
                deltas.append((c.get("full", 0) / nl - b.get("full", 0) / nb, t, b, c))
        deltas.sort()
        for d, t, b, c in deltas[:6] + deltas[-4:]:
            if abs(d) > 1e-9:
                print(f"  {t:<22} {b.get('full',0)}/{sum(b.values())} → "
                      f"{c.get('full',0)}/{sum(c.values())}  ({d:+.0%})")
    json.dump(out, open(os.path.join(D, "scale-summary.json"), "w"), ensure_ascii=False, indent=1)


if __name__ == "__main__":
    main()
