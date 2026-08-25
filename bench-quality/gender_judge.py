#!/usr/bin/env python3
"""Judge whether a translation marks the gender of a named character, and which.

For targets where gender does not sit in a verb ending (Spanish, French, German)
a regex cannot score agreement, so a cheap model answers one mechanical question
per sentence: is any word agreeing with this name gender-marked, and how.

Usage: gender_judge.py --dir ART --probe probe-deid.json --lang Spanish arm:LABEL …
"""
import argparse, json, os, sys
from concurrent.futures import ThreadPoolExecutor

sys.path.insert(0, __file__.rsplit("/", 1)[0])
from gloss_scale_probe import API, env

SYS = """You check grammatical gender in {LANG} sentences.

You receive a JSON array of items {id, name, text}: text is a {LANG} sentence, name is
a character mentioned in it.

For EACH item decide whether any word in text is GENDER-MARKED for that character —
an adjective, participle, article, or pronoun that agrees with them. Ignore words
agreeing with anything else, and ignore the name itself.

Reply with ONLY a JSON object mapping each "id" to one of:
  "m"    — the marked word(s) are masculine
  "f"    — the marked word(s) are feminine
  "none" — nothing in the sentence marks that character's gender
No commentary."""


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dir", required=True); ap.add_argument("--probe", required=True)
    ap.add_argument("--lang", default="Spanish"); ap.add_argument("--batch", type=int, default=16)
    ap.add_argument("arms", nargs="+")
    a = ap.parse_args()
    probe = {p["i"]: p for p in json.load(open(os.path.join(a.dir, a.probe)))}
    api = API(env())
    sys_p = SYS.replace("{LANG}", a.lang)

    print(f"{'arm':<22} {'marked':>8} {'correct':>8} {'wrong':>7} {'unmarked':>9} {'accuracy':>9}")
    print("-" * 66)
    saved = {}
    for spec in a.arms:
        name, label = (spec.split(":", 1) + [spec])[:2]
        rows = [r for r in json.load(open(os.path.join(a.dir, f"out-{name}.json"))) if r.get("tr")]
        items = [{"id": str(r["i"]), "name": probe[r["i"]]["name"], "text": r["tr"]} for r in rows]
        chunks = [items[i:i + a.batch] for i in range(0, len(items), a.batch)]
        got = {}
        with ThreadPoolExecutor(8) as ex:
            for res in ex.map(lambda ch: api.chat(sys_p, json.dumps(ch, ensure_ascii=False)), chunks):
                got.update(res)
        ok = wrong = none = 0
        for r in rows:
            v = got.get(str(r["i"]))
            w = probe[r["i"]]["gender"]
            if v == w:
                ok += 1
            elif v in ("m", "f"):
                wrong += 1
            else:
                none += 1
        dec = ok + wrong
        print(f"{label:<22} {dec:>8} {ok:>8} {wrong:>7} {none:>9} "
              f"{100*ok/dec if dec else 0:>8.1f}%")
        saved[label] = {"marked": dec, "ok": ok, "wrong": wrong, "unmarked": none,
                        "verdicts": got}
    tag = a.probe.replace(".json", "")
    json.dump(saved, open(os.path.join(a.dir, f"gender-judge-{a.lang.lower()}-{tag}.json"), "w"),
              ensure_ascii=False, indent=1)
    st = [s for s in api.stats if s.get("prompt")]
    print(f"judge cost ${sum((s['cost'] or 0) for s in st):.4f}")


if __name__ == "__main__":
    main()
