#!/usr/bin/env python3
"""Judge the D4b alignment sets produced by itermax_prep.py.

Two modes:

  --mode pairs  blind A/B over two alignment renderings of the SAME sentence and
                the SAME translation. Each batch file is judged by a FRESH
                `claude -p` process, so the two presentation orders of a pair are
                independent judgements (that is what makes the de-swap in
                itermax_analyze.py meaningful).
                Writes verdict-<k>-o<n>.json {id: "X"|"Y"|"tie"} and
                notes-<k>-o<n>.json {id: {"w":…, "r": "<one-line reason>"}}.

  --mode lit    for each source word that was silent under argmax and is lit
                under itermax, is the newly highlighted target fragment a correct
                rendering of that word?
                Writes litverdict-<k>.json {id: {"v": "correct|partial|wrong",
                "r": "<one-line reason>"}}.

Rate discipline copied from judge_pairs.py: --jobs 5, 30 s * attempt backoff,
batches of 10, existing verdict files kept so a re-run only fills gaps.

Usage:
  itermax_judge.py DIR --mode pairs [--model sonnet] [--jobs 5]
"""
import argparse
import concurrent.futures as cf
import glob
import json
import os
import re
import subprocess
import sys
import time

# internal/translate/judge.go, the j3 ALIGNMENT criterion, verbatim
# (with {SRC} = English and {TGT} = Russian substituted, as the judge does).
J3_ALIGNMENT = """ALIGNMENT — is each pair mapped BY MEANING? (Russian word mapped to an English word that does not
   translate it — e.g. positional drift, where words are paired by position instead of meaning —
   → not ok. Unmapped words, function words attached to a neighboring content word, and loose
   but related renderings → ok.)"""

PAIRS_RUBRIC = """You grade WORD ALIGNMENTS for a language-learning reader where readers tap an English
word to see which Russian word(s) translate it.

Every item gives one English sentence (src), ONE Russian translation of it (tr) — the SAME
translation for both sides — and two competing alignments of that sentence, X and Y. Each
alignment is a list of ["<Russian fragment>", "<English word(s)>"] pairs in sentence order
("" = an inserted Russian word claimed by no English word).

The translation is identical in X and Y. Do NOT grade the translation. Grade ONLY which
alignment maps words more correctly, by this criterion:

2) %s

Decide, for each item, which alignment maps words BY MEANING more correctly: "X", "Y", or
"tie" when neither is clearly better (they differ only in which function word rides along
with a content word, or the differences cancel out).

Give a one-line reason in ENGLISH quoting the decisive pair(s) verbatim.

Return ONLY one JSON object, no prose and no markdown fences:
{"<id>": {"w": "X" or "Y" or "tie", "r": "<one line, quoting the decisive pair(s)>"}, ...}
Every id in the input must appear exactly once in the output.

Items to judge:
""" % J3_ALIGNMENT

LIT_RUBRIC = """You check single word-alignment mappings for a language-learning reader where readers tap an
English word to see which Russian word(s) translate it.

Every item gives an English sentence (src), its Russian translation (tr), one English word
from that sentence (word), and the Russian fragment (frag) that tapping that word now
highlights. Judge ONLY that one mapping; do not grade the translation.

Criterion:

2) %s

For each item answer:
  "correct" — frag is the Russian rendering of word in this sentence (its stem, its inflected
              form, or the fragment that carries that word's meaning here; a function word
              riding along with the right content word is still correct);
  "partial" — frag overlaps the right rendering but is off: it is the neighbouring word of the
              right one, a wider or narrower span that includes the right word, or a related
              but not equivalent word;
  "wrong"   — frag does not translate word at all (positional drift, an unrelated word).

Give a one-line reason in ENGLISH quoting frag and the Russian word you would have expected.

Return ONLY one JSON object, no prose and no markdown fences:
{"<id>": {"v": "correct" or "partial" or "wrong", "r": "<one line>"}, ...}
Every id in the input must appear exactly once in the output.

Items to judge:
""" % J3_ALIGNMENT


def run_claude(prompt, model, attempts=3):
    env = {k: v for k, v in os.environ.items() if k != "ANTHROPIC_API_KEY"}
    last = ""
    for att in range(attempts):
        if att:
            time.sleep(30 * att)  # a rate-limited window needs waiting out
        try:
            out = subprocess.run(
                ["claude", "-p", "--model", model, "--tools", "", "--strict-mcp-config", "--mcp-config", "{\"mcpServers\":{}}"],
                input=prompt, capture_output=True, text=True,
                timeout=900, env=env).stdout
        except subprocess.TimeoutExpired:
            continue
        last = out
        m = re.search(r"\{.*\}", out, re.S)
        if not m:
            continue
        try:
            return json.loads(m.group(0)), None
        except json.JSONDecodeError:
            continue
    return None, last[:300]


def judge_pairs_batch(path, model):
    items = json.load(open(path, encoding="utf-8"))
    got, err = run_claude(PAIRS_RUBRIC + json.dumps(items, ensure_ascii=False, indent=1), model)
    if got is None:
        return None, err
    verdict, notes = {}, {}
    for it in items:
        g = got.get(it["id"])
        if isinstance(g, str):
            g = {"w": g, "r": ""}
        if not isinstance(g, dict) or g.get("w") not in ("X", "Y", "tie"):
            return None, f"bad verdict for id {it['id']}: {g!r}"
        verdict[it["id"]] = g["w"]
        notes[it["id"]] = {"w": g["w"], "r": g.get("r", "")}
    return verdict, notes


def judge_lit_batch(path, model):
    items = json.load(open(path, encoding="utf-8"))
    got, err = run_claude(LIT_RUBRIC + json.dumps(items, ensure_ascii=False, indent=1), model)
    if got is None:
        return None, err
    out = {}
    for it in items:
        g = got.get(it["id"])
        if isinstance(g, str):
            g = {"v": g, "r": ""}
        if not isinstance(g, dict) or g.get("v") not in ("correct", "partial", "wrong"):
            return None, f"bad verdict for id {it['id']}: {g!r}"
        out[it["id"]] = {"v": g["v"], "r": g.get("r", "")}
    return out, None


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("dir")
    ap.add_argument("--mode", choices=("pairs", "lit"), required=True)
    ap.add_argument("--model", default="sonnet")
    ap.add_argument("--jobs", type=int, default=5)
    args = ap.parse_args()

    todo = []
    if args.mode == "pairs":
        for b in sorted(glob.glob(f"{args.dir}/batch-*.json")):
            v = b.replace("batch-", "verdict-")
            if not os.path.exists(v):
                todo.append((b, v))
    else:
        for b in sorted(glob.glob(f"{args.dir}/lit-*.json")):
            v = b.replace("/lit-", "/litverdict-")
            if not os.path.exists(v):
                todo.append((b, v))
    if not todo:
        print("nothing to judge (all verdict files present)")
        return

    print(f"judging {len(todo)} batches with {args.jobs} parallel {args.model} judges "
          f"({args.mode})")
    fails = 0
    fn = judge_pairs_batch if args.mode == "pairs" else judge_lit_batch
    with cf.ThreadPoolExecutor(max_workers=args.jobs) as ex:
        futs = {ex.submit(fn, b, args.model): (b, v) for b, v in todo}
        for f in cf.as_completed(futs):
            b, v = futs[f]
            res, extra = f.result()
            name = os.path.basename(b)
            if res is None:
                fails += 1
                print(f"FAIL {name}: {extra!r}", file=sys.stderr)
                continue
            json.dump(res, open(v, "w"), ensure_ascii=False, indent=1)
            if args.mode == "pairs":
                json.dump(extra, open(v.replace("verdict-", "notes-"), "w"),
                          ensure_ascii=False, indent=1)
            print(f"ok   {name}  {len(res)} items")
    print(f"done; {fails} batches failed" if fails else "done")


if __name__ == "__main__":
    main()
