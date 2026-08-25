#!/usr/bin/env python3
"""Merge the two gender sources into a glossary, precision first.

Sources: the render pass's tag (LLM) and the deterministic pronoun miner. The
combining rule is a veto by DIRECTION, not by split alone — an early version
rejected any name whose pronoun evidence was split, which threw away real
characters (Pascale, 53 observations at 0.57, is female and the LLM said so).

  accept the LLM tag        unless the miner's majority points the other way
                            (>= min-evidence observations)
  accept a miner-only tag   when the entity is a person, the evidence is large
                            and confident, and the LLM simply did not tag it

Thresholds were swept on two books: lowering the miner-only evidence bar from 40 to
15 adds tags (Girardieau, +1 hand-checkable character) with zero new errors.

Usage: gender_merge.py GLOSS_WITH_TAGS.json MINED_GENDER.json --out G.json [--truth k=v,…]
"""
import argparse, json


def merge(gloss, mined, min_evidence=15, miner_only_evidence=15, miner_only_conf=0.8):
    tags, rejected = {}, []
    for e in gloss:
        gl = e.get("gender")
        if not gl:
            continue
        if e.get("entity") != "person":
            rejected.append((e["src"], gl, f"entity={e.get('entity')}, not a named person"))
            continue
        m = mined.get(e["src"])
        if m and m["evidence"] >= min_evidence and m["gender"] != gl:
            rejected.append((e["src"], gl, f"miner majority {m['gender']} on "
                                          f"{m['evidence']} observations (conf {m['confidence']})"))
            continue
        tags[e["src"]] = gl
    for e in gloss:
        if e["src"] in tags or e.get("entity") != "person":
            continue
        m = mined.get(e["src"])
        if m and m["evidence"] >= miner_only_evidence and m["confidence"] >= miner_only_conf:
            tags[e["src"]] = m["gender"]
    return tags, rejected


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("gloss"); ap.add_argument("mined")
    ap.add_argument("--out", required=True); ap.add_argument("--truth", default="")
    a = ap.parse_args()
    gloss = json.load(open(a.gloss)); mined = json.load(open(a.mined))
    tags, rejected = merge(gloss, mined)
    out = [{"src": e["src"], "tgt": e["tgt"],
            **({"gender": tags[e["src"]]} if e["src"] in tags else {})} for e in gloss]
    json.dump(out, open(a.out, "w"), ensure_ascii=False, indent=1)
    json.dump([{"src": d["src"], "tgt": d["tgt"]} for d in out],
              open(a.out.replace(".json", "-ng.json"), "w"), ensure_ascii=False, indent=1)
    persons = sum(1 for e in gloss if e.get("entity") == "person")
    print(f"{len(out)} entries, {persons} persons, {len(tags)} carry gender, "
          f"{len(rejected)} rejected")
    for n, gl, why in rejected:
        print(f"  reject {n}[{gl}]: {why}")
    if a.truth:
        truth = dict(kv.split("=") for kv in a.truth.split(","))
        wrong = [(n, t, tags[n]) for n, t in truth.items() if n in tags and tags[n] != t]
        print(f"hand truth: {sum(1 for n in truth if n in tags)}/{len(truth)} covered, "
              f"{len(wrong)} wrong {wrong if wrong else ''}")
        print("  not covered:", ", ".join(n for n in truth if n not in tags) or "—")


if __name__ == "__main__":
    main()
