#!/usr/bin/env python3
"""Render a GOLD-lite set as a human review file: disputed items first.

  gold_review.py gold/en-ru.jsonl --lang Russian --out gold/en-ru-review.md [--human]

--human writes the "a reader on the team can promote these to gold" header;
without it the file says the set stays `silver` because nobody reads the language.
"""
import argparse, collections, json

HUMAN = """A human reader of {lang} works through this file. For every item, either

- leave it alone — the reference is right: set `"status": "gold"` for that `id` in
  `{jsonl}`; or
- correct `ref` in `{jsonl}` to the translation you would publish, and set
  `"status": "gold"` with `"ref_model": "human"`.

Until an item is touched by a human its status stays as the machine checker left
it (`silver` for `ok`, `disputed` for `minor`/`wrong`). **Disputed items come
first** — they are where the checker and the reference disagree and where a
human pass buys the most.
"""

NOHUMAN = """**No one on the team reads {lang}.** This file exists for provenance and for a
future reader; nothing in it is promoted to `gold` by this pass. Every item keeps
the machine verdict: `silver` where the adversarial checker said `ok`,
`disputed` where it said `minor` or `wrong`. Treat the whole set as **silver**:
usable as an absolute regression alarm, never as a ground truth.
"""


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("jsonl"); ap.add_argument("--lang", required=True)
    ap.add_argument("--code", required=True, help="target language code, e.g. de")
    ap.add_argument("--out", required=True)
    ap.add_argument("--human", action="store_true")
    ap.add_argument("--checker", default="claude sonnet (via the claude CLI)")
    args = ap.parse_args()

    rows = [json.loads(l) for l in open(args.jsonl, encoding="utf-8") if l.strip()]
    c = collections.Counter(r.get("check", "unchecked") for r in rows)
    order = {"wrong": 0, "minor": 1, "unchecked": 2, "ok": 3}
    rows.sort(key=lambda r: (order.get(r.get("check", "unchecked"), 9), r["id"]))

    model = rows[0]["ref_model"] if rows else "?"
    L = []
    L.append(f"# GOLD-lite review — en→{args.code} ({args.lang})\n")
    L.append(f"Set: `{args.jsonl}` — {len(rows)} sentences, "
             f"*The Adventures of Sherlock Holmes*, chapters 1–4.\n")
    L.append(f"Reference translations: **{model}**, no repair pass. "
             f"Adversarial check: **{args.checker}**, one verdict per sentence.\n")
    L.append(f"Checker verdicts: **wrong {c['wrong']}**, **minor {c['minor']}**, "
             f"**ok {c['ok']}**" + (f", unchecked {c['unchecked']}" if c['unchecked'] else "") + ".\n")
    L.append("## How to use this file\n")
    L.append((HUMAN if args.human else NOHUMAN).format(lang=args.lang, jsonl=args.jsonl))
    L.append("## Items\n")
    cur = None
    for i, r in enumerate(rows, 1):
        v = r.get("check", "unchecked")
        if v != cur:
            cur = v
            title = {"wrong": "Disputed — checker verdict `wrong`",
                     "minor": "Disputed — checker verdict `minor`",
                     "ok": "Checker verdict `ok` (status `silver`)",
                     "unchecked": "Not checked"}[v]
            L.append(f"### {title}\n")
        L.append(f"**{i}. `{r['id']}`** · ch{r['chapter']} ¶{r['para']}.{r['sent']} · "
                 f"`{v}` · status `{r.get('status','draft')}`\n")
        L.append(f"- EN: {r['src']}")
        L.append(f"- {args.code.upper()}: {r['ref']}")
        if r.get("check_reason"):
            L.append(f"- checker: {r['check_reason']}")
        L.append("")
    open(args.out, "w", encoding="utf-8").write("\n".join(L) + "\n")
    print(f"{args.out}: {len(rows)} items ({dict(c)})")


if __name__ == "__main__":
    main()
