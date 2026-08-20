#!/usr/bin/env python3
"""Run the blind pairwise judge over batches produced by prepare_pairs.py.

Each batch file is judged by a FRESH `claude -p` process (no shared context
between batches or between the two presentation orders), so the two orders of a
pair are independent judgements — which is what makes the de-swap in
analyze_pairs.py meaningful.

Usage:
  judge_pairs.py pairs-dir [--model sonnet] [--jobs 6] [--target-lang Russian]

Writes next to every batch-<k>-o<n>.json:
  verdict-<k>-o<n>.json   {id: "X"|"Y"|"tie"}          <- read by analyze_pairs.py
  notes-<k>-o<n>.json     {id: {"w":…, "d":"<class>"}} <- defect class of the loser

Existing verdict files are kept (re-running only fills the gaps).
"""
import argparse, concurrent.futures as cf, glob, json, os, re, subprocess, sys, time

RUBRIC = """You are grading two {lang} translations of one English sentence taken from a novel.
For every item decide which translation is better, or answer "tie".

Rubric, in strict priority order:
1. fidelity - nothing added, dropped, or distorted; named entities kept right;
2. {lang} grammar - agreement, case government, real existing words;
3. idiom and register - idioms rendered as idioms, register matches the source;
4. naturalness - reads like original prose, not translationese.

Answer "tie" when the two differ only in dialogue punctuation style (em dash vs
guillemets), in a neutral synonym choice, or in anything else a competent editor
would call preference rather than a defect. Judge only the text you are shown;
you have no wider context, so do not punish a choice that some other passage
might justify.

For the losing side, name its single worst defect class, one of:
fidelity, grammar-agreement, case-government, nonexistent-word, idiom-calque,
collocation, terminology, register, naturalness, punctuation, other.

Return ONLY one JSON object, no prose and no markdown fences:
{{"<id>": {{"w": "X" or "Y" or "tie", "d": "<defect class of the loser, or empty for a tie>"}}, ...}}
Every id in the input must appear exactly once in the output.

Items to judge:
"""


def judge(batch_path, model, lang, attempts=3):
    items = json.load(open(batch_path))
    prompt = RUBRIC.format(lang=lang) + json.dumps(items, ensure_ascii=False, indent=1)
    env = {k: v for k, v in os.environ.items() if k != "ANTHROPIC_API_KEY"}
    last = ""
    for att in range(attempts):
        if att:
            time.sleep(30 * att)  # a rate-limited window needs waiting out, not an instant retry
        try:
            out = subprocess.run(
                ["claude", "-p", "--model", model, "--tools", ""],
                input=prompt, capture_output=True, text=True,
                timeout=900, env=env).stdout
        except subprocess.TimeoutExpired:
            continue
        last = out
        m = re.search(r"\{.*\}", out, re.S)
        if not m:
            continue
        try:
            got = json.loads(m.group(0))
        except json.JSONDecodeError:
            continue
        notes, verdict = {}, {}
        for it in items:
            g = got.get(it["id"])
            if isinstance(g, str):
                g = {"w": g, "d": ""}
            if not isinstance(g, dict) or g.get("w") not in ("X", "Y", "tie"):
                notes, verdict = {}, {}
                break
            notes[it["id"]] = {"w": g["w"], "d": g.get("d", "")}
            verdict[it["id"]] = g["w"]
        if verdict:
            return verdict, notes
    return None, last[:300]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("dir")
    ap.add_argument("--model", default="sonnet")
    ap.add_argument("--jobs", type=int, default=6)
    ap.add_argument("--target-lang", default="Russian")
    args = ap.parse_args()

    todo = []
    for b in sorted(glob.glob(f"{args.dir}/batch-*.json")):
        v = b.replace("batch-", "verdict-")
        if not os.path.exists(v):
            todo.append((b, v))
    if not todo:
        print("nothing to judge (all verdict files present)")
        return
    print(f"judging {len(todo)} batches with {args.jobs} parallel {args.model} judges")
    fails = 0
    with cf.ThreadPoolExecutor(max_workers=args.jobs) as ex:
        futs = {ex.submit(judge, b, args.model, args.target_lang): (b, v) for b, v in todo}
        for f in cf.as_completed(futs):
            b, v = futs[f]
            verdict, notes = f.result()
            name = os.path.basename(b)
            if verdict is None:
                fails += 1
                print(f"FAIL {name}: {notes!r}", file=sys.stderr)
                continue
            json.dump(verdict, open(v, "w"), ensure_ascii=False, indent=1)
            json.dump(notes, open(v.replace("verdict-", "notes-"), "w"),
                      ensure_ascii=False, indent=1)
            print(f"ok   {name}  {len(verdict)} items")
    print(f"done; {fails} batches failed" if fails else "done")


if __name__ == "__main__":
    main()
