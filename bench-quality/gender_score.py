#!/usr/bin/env python3
"""Score gender agreement in the gender-probe arms.

Each probe sentence has exactly one past-tense source verb and a known-gender
name as its subject, and the source never marks the gender itself. In the
Russian output the first past-tense form is therefore the one under test.

Usage: gender_score.py --dir ART --probe probe-gender.json arm=NAME:LABEL …
"""
import argparse, json, os, re, sys
from collections import Counter

sys.path.insert(0, __file__.rsplit("/", 1)[0])
from gender_probe import past_gender

TOKEN = re.compile(r"[^\W\d_]+", re.UNICODE)


def verdict(tr, want):
    if not tr:
        return "nooutput", None
    for t in TOKEN.findall(tr):
        g = past_gender(t)
        if g:
            if g == want:
                return "ok", t
            if g in ("pl", "n"):
                return "other", t          # plural/neuter: not the tested contrast
            return "wrong", t
    return "noverb", None


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dir", required=True); ap.add_argument("--probe", default="probe-gender.json")
    ap.add_argument("arms", nargs="+")
    ap.add_argument("--show", type=int, default=6)
    a = ap.parse_args()
    probe = json.load(open(os.path.join(a.dir, a.probe)))
    want = {p["i"]: p for p in probe}

    res = {}
    for spec in a.arms:
        name, label = (spec.split(":", 1) + [spec])[:2]
        rows = json.load(open(os.path.join(a.dir, f"out-{name}.json")))
        c, per = Counter(), {}
        for r in rows:
            p = want[r["i"]]
            v, form = verdict(r.get("tr"), p["gender"])
            c[v] += 1
            per[r["i"]] = (v, form, r.get("tr"))
        res[label] = (c, per)

    # the shipped translation (reference field) as a fifth column
    c, per = Counter(), {}
    for p in probe:
        v, form = verdict(p.get("ref"), p["gender"])
        c[v] += 1
        per[p["i"]] = (v, form, p.get("ref"))
    res["shipped (kimi+repair)"] = (c, per)

    hdr = f"{'arm':<26} {'correct':>9} {'wrong':>7} {'plural/neut':>12} {'no verb':>8} {'accuracy':>9}"
    print(hdr); print("-" * len(hdr))
    for label, (c, _) in res.items():
        dec = c["ok"] + c["wrong"]
        print(f"{label:<26} {c['ok']:>9} {c['wrong']:>7} {c['other']:>12} {c['noverb']:>8} "
              f"{100*c['ok']/dec if dec else 0:>8.1f}%")

    labels = list(res)
    base = labels[0]
    print(f"\npaired comparison against «{base}» (same sentences, both decisive):")
    for label in labels[1:]:
        b, x = res[base][1], res[label][1]
        fixed = [i for i in b if b[i][0] == "wrong" and x[i][0] == "ok"]
        broke = [i for i in b if b[i][0] == "ok" and x[i][0] == "wrong"]
        print(f"  {label:<26} fixed {len(fixed):>3}, broke {len(broke):>3}")
        for i in fixed[:a.show]:
            print(f"     + {want[i]['name']} ({want[i]['gender']}): «{b[i][1]}» → «{x[i][1]}»")
            print(f"       {want[i]['src'][:120]}")
        for i in broke[:a.show]:
            print(f"     − {want[i]['name']} ({want[i]['gender']}): «{b[i][1]}» → «{x[i][1]}»")
    json.dump({k: dict(v[0]) for k, v in res.items()},
              open(os.path.join(a.dir, "gender-summary.json"), "w"), indent=1)


if __name__ == "__main__":
    main()
