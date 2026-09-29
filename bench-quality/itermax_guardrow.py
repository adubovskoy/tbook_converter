#!/usr/bin/env python3
"""Rebuild the align-knob grid's Table A / Table B rows for a set of arms from
the five instruments' JSON dumps, so a new arm can be added to the grid table
with the same numbers computed the same way.

Usage: itermax_guardrow.py ARM [ARM...] --scores DIR --grid DIR --base ac-base
"""
import argparse
import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from itermax_prep import align_of, chunk_key, sentences, text_of  # noqa: E402


def align_changed(base, arm, lang="ru"):
    A, B = sentences(base), sentences(arm)
    common = changed = 0
    for sa, sb in zip(A, B):
        if not text_of(sa, lang) or not text_of(sb, lang):
            continue
        common += 1
        if chunk_key(align_of(sa, lang)) != chunk_key(align_of(sb, lang)):
            changed += 1
    return changed, common


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("arms", nargs="+")
    ap.add_argument("--scores", required=True)
    ap.add_argument("--grid", required=True)
    ap.add_argument("--probe", default="")
    args = ap.parse_args()

    rows = []
    for arm in args.arms:
        corpus = arm.split("-")[0]
        sc = json.load(open(f"{args.scores}/score-{arm}.json"))
        tc, st = sc["targetCoverage"], sc["sourceTap"]
        pr = {"unit": 0, "matched": 0}
        for v in sc.get("probes", []):
            if v.get("verdict") == "src-not-found":
                continue
            pr["matched"] += 1
            if v.get("verdict") == "unit":
                pr["unit"] += 1
        tl = json.load(open(f"{args.scores}/tl-{arm}.json"))["languages"][0]
        tlc = json.load(open(f"{args.scores}/tlc-{arm}.json"))["languages"][0]
        lex = len(json.load(open(f"{args.grid}/{arm}.tbook.lexflagged.json")))
        wall = ""
        wf = f"{args.grid}/{arm}.wall"
        if os.path.exists(wf):
            wall = open(wf).read().strip()
        chg, common = align_changed(f"{args.grid}/{corpus}-base.tbook",
                                    f"{args.grid}/{arm}.tbook")
        probe_word = ""
        pf = f"{args.scores}/probe-{arm}.txt"
        if os.path.exists(pf):
            for line in open(pf):
                if "expression WORD tap coverage" in line:
                    probe_word = line.split("(")[-1].split(")")[0]
        rows.append({
            "arm": arm, "wall": wall,
            "tgtAll": tc["allMean"], "tgtContent": tc["contentMean"],
            "tapAll": st["allMean"], "tapContent": st["contentMean"],
            "tapBelow": st["below"], "tapBelowN": st["contentN"],
            "lex": lex, "alignChanged": chg, "alignCommon": common,
            "probeUnitPct": (round(100 * pr["unit"] / pr["matched"], 1)
                             if pr["matched"] else None), "probeWordPct": probe_word,
            "tl": {k: round(v, 2) for k, v in tl["pct"].items()},
            "contentSilentPct": round(tl["contentSilentPct"], 2),
            "contentSilent": tl["contentSilent"], "contentTaps": tl["contentTaps"],
            "tlc": {k: round(v, 2) for k, v in tlc["pct"].items()},
        })
    print(json.dumps(rows, ensure_ascii=False, indent=1))
    print("\n--- Table A ---")
    print("| arm | wall s | target cov all / content | source tap cov all / content | "
          "sentences tap < 0.80 | lexcheck flags | alignments changed vs base |")
    for r in rows:
        print(f"| `{r['arm']}` | {r['wall']} | {r['tgtAll']:.4f} / {r['tgtContent']:.4f} | "
              f"{r['tapAll']:.4f} / **{r['tapContent']:.4f}** | "
              f"{r['tapBelow']} / {r['tapBelowN']} | {r['lex']} | "
              f"{r['alignChanged']} / {r['alignCommon']} |")
    print("\n--- Table B ---")
    print("| arm | clean | clean-multi | shared | smear | none | content-word silent | "
          "content-only clean / shared / none | probe unit % | probe word cov % |")
    for r in rows:
        t, c = r["tl"], r["tlc"]
        print(f"| `{r['arm']}` | {t['clean']:.2f} | {t['clean-multi']:.2f} | {t['shared']:.2f} | "
              f"{t['smear']:.2f} | {t['none']:.2f} | **{r['contentSilentPct']:.2f}** | "
              f"{c['clean']:.2f} / {c['shared']:.2f} / {c['none']:.2f} | "
              f"{r['probeUnitPct']} | {r['probeWordPct'] or 'n/a'} |")


if __name__ == "__main__":
    main()
