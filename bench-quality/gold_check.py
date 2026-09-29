#!/usr/bin/env python3
"""Adversarial check of a GOLD-lite draft by a second model family.

Every (src, ref) pair of a gold JSONL is shown to a FRESH `claude -p` process in
batches of 10 (same rate discipline as judge_pairs.py: --jobs 5, 30 s backoff),
which returns a verdict ok | minor | wrong plus a one-line English reason
quoting the offending words. Existing check-*.json are kept, so re-running only
fills the gaps.

  gold_check.py gold/en-ru.jsonl --lang Russian --work .artifacts/.../check-en-ru
  gold_check.py gold/en-ru.jsonl --lang Russian --work ... --merge
"""
import argparse, concurrent.futures as cf, glob, json, os, re, subprocess, sys, time

RUBRIC = """You are checking {lang} translations of English sentences from Arthur Conan Doyle's
"The Adventures of Sherlock Holmes". Each item has the English source and one {lang}
translation. You are the adversarial reader: your job is to find what is wrong with the
translation, not to praise it. You see one sentence at a time and no wider context, so do
not punish a choice some other passage might justify.

Give every item exactly one verdict:
  "ok"    - accurate and grammatical; a competent editor would publish it as is.
            Stylistic preference, a neutral synonym, or dialogue punctuation style is NOT a defect.
  "minor" - understandable but flawed: an awkward collocation, a register slip, a small
            nuance lost or added, a debatable term choice.
  "wrong" - a defect that misleads the reader: wrong meaning, content dropped or invented,
            a named entity distorted, a word that does not exist in {lang}, broken agreement
            or case government, or any text left in the wrong language.

Be strict about fidelity and {lang} grammar, lenient about style.

"reason": ONE line, written in ENGLISH, quoting the offending {lang} (or English) words
verbatim inside guillemets, e.g. «...». Empty string for an "ok" verdict.

Return ONLY one JSON object, no prose and no markdown fences:
{{"<id>": {{"v": "ok" or "minor" or "wrong", "r": "<one-line English reason>"}}, ...}}
Every id in the input must appear exactly once in the output.

Items to check:
"""


def check(batch_path, out_path, model, lang, attempts=3):
    items = json.load(open(batch_path))
    prompt = RUBRIC.format(lang=lang) + json.dumps(items, ensure_ascii=False, indent=1)
    env = {k: v for k, v in os.environ.items() if k != "ANTHROPIC_API_KEY"}
    last = ""
    for att in range(attempts):
        if att:
            time.sleep(30 * att)
        try:
            out = subprocess.run(["claude", "-p", "--model", model, "--tools", "", "--strict-mcp-config", "--mcp-config", "{\"mcpServers\":{}}"],
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
        res = {}
        for it in items:
            g = got.get(it["id"])
            if not isinstance(g, dict) or g.get("v") not in ("ok", "minor", "wrong"):
                res = {}
                break
            res[it["id"]] = {"v": g["v"], "r": (g.get("r") or "").strip()}
        if res:
            json.dump(res, open(out_path, "w"), ensure_ascii=False, indent=1)
            return len(res), None
    return 0, last[:300]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("jsonl")
    ap.add_argument("--lang", required=True)
    ap.add_argument("--work", required=True)
    ap.add_argument("--model", default="sonnet")
    ap.add_argument("--jobs", type=int, default=5)
    ap.add_argument("--batch", type=int, default=10)
    ap.add_argument("--merge", action="store_true",
                    help="only fold existing check-*.json back into the JSONL")
    args = ap.parse_args()

    rows = [json.loads(l) for l in open(args.jsonl, encoding="utf-8") if l.strip()]
    os.makedirs(args.work, exist_ok=True)

    if not args.merge:
        todo = []
        for k in range(0, len(rows), args.batch):
            nb = k // args.batch + 1
            b, v = f"{args.work}/batch-{nb:02d}.json", f"{args.work}/check-{nb:02d}.json"
            if not os.path.exists(b):
                json.dump([{"id": r["id"], "src": r["src"], "translation": r["ref"]}
                           for r in rows[k:k + args.batch]],
                          open(b, "w"), ensure_ascii=False, indent=1)
            if not os.path.exists(v):
                todo.append((b, v))
        if todo:
            print(f"checking {len(todo)} batches with {args.jobs} parallel {args.model} checkers")
            fails = 0
            with cf.ThreadPoolExecutor(max_workers=args.jobs) as ex:
                futs = {ex.submit(check, b, v, args.model, args.lang): b for b, v in todo}
                for f in cf.as_completed(futs):
                    n, err = f.result()
                    name = os.path.basename(futs[f])
                    if err is not None:
                        fails += 1
                        print(f"FAIL {name}: {err!r}", file=sys.stderr)
                    else:
                        print(f"ok   {name}  {n} items")
            print(f"done; {fails} batches failed" if fails else "done")
        else:
            print("nothing to check (all check files present)")

    got = {}
    for f in glob.glob(f"{args.work}/check-*.json"):
        got.update(json.load(open(f)))
    miss = 0
    with open(args.jsonl, "w", encoding="utf-8") as f:
        for r in rows:
            g = got.get(r["id"])
            if g:
                r["check"], r["check_reason"] = g["v"], g["r"]
                r["status"] = "silver" if g["v"] == "ok" else "disputed"
            else:
                miss += 1
                r.pop("check", None); r.pop("check_reason", None)
                r["status"] = "draft"
            f.write(json.dumps(r, ensure_ascii=False) + "\n")
    import collections
    c = collections.Counter(r.get("check", "unchecked") for r in rows)
    print(f"{args.jsonl}: {len(rows)} rows, {miss} unchecked; {dict(c)}")


if __name__ == "__main__":
    main()
