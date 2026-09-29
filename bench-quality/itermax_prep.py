#!/usr/bin/env python3
"""Build the blind alignment-judging set for D4b (argmax vs itermax).

Two arms of one book differ only in `align`; the translation text is identical.
This script:
  1. walks both .tbook files in the same deterministic order (manifest chapter
     order, paragraphs, then tables, then notes by id) and matches sentences by
     position, asserting identical `src` and identical translation text;
  2. marks a sentence CHANGED when its alignment chunk list differs;
  3. samples --n changed sentences by an EVEN STRIDE over the changed list in
     document order (stride = k*len/n), so the sample spans the whole list and is
     reproducible and spread over the whole book rather than random;
  4. renders each arm's alignment exactly as internal/translate/judge.go's
     alignPairs does — ["<target fragment>", "<source word(s)>"], chunks in
     file order, "" for a chunk that claims no source word — and writes blind
     A/B batches in both presentation orders;
  5. separately lists every source CONTENT word that no chunk referenced under
     the base arm and at least one chunk references under the itermax arm (the
     "newly lit" words), with the target fragment the tap now highlights.

Usage:
  itermax_prep.py BASE.tbook ITERMAX.tbook --book ac --out DIR [--n 150]
"""
import argparse
import json
import os
import random
import zipfile

# internal/learner/words.go contentStop, verbatim.
CONTENT_STOP = set("""
the a an of to in on at by for with from as into upon and or but if that which who whom whose
this these those it its he she they them his her their we you i me my our your be is are was were been am
have has had do does did not no so than then there here very will would shall should can could may might
must up out down over under about after before between through during without within
""".split())


def is_content_word(w):
    """internal/learner/words.go IsContentWord: >= 4 runes, not in contentStop."""
    lw = w.lower()
    return len(lw) >= 4 and lw not in CONTENT_STOP


# internal/stopwords/stopwords.go lists["en"], verbatim — the list cmd/score
# uses for its source content-word metrics. No length rule.
EN_STOP = set("""
a an the
and or but nor so yet both either neither
of in on at to for from by with without as into onto over under
between through after before above below against about along
i you he she it we they me him her us them my your his its our their
mine yours hers ours theirs this that these those who whom whose which what
myself yourself himself herself itself ourselves themselves
is are was were be been being am
do does did done have has had having
will would shall should can could may might must
not no nor never none
if then than because while when where why how whether
there here too very just also only even still already yet again
""".split())


def is_score_content(w):
    """cmd/score's source content-word rule: not in the en stopword list."""
    return w.lower() not in EN_STOP


def sentences(path):
    """Every sentence of the book in cmd/tbookdiff `collect` order."""
    out = []
    with zipfile.ZipFile(path) as z:
        man = json.loads(z.read("manifest.json"))
        chapters = [c["file"] if isinstance(c, dict) else c for c in man.get("chapters", [])]
        if not chapters:
            chapters = sorted(n for n in z.namelist()
                              if n.startswith("chapters/") and n.endswith(".json"))
        for name in chapters:
            if not name.startswith("chapters/"):
                name = "chapters/" + name
            d = json.loads(z.read(name))
            for para in d.get("paragraphs", []):
                out.extend(para)
            for t in d.get("tables", []):
                for row in t.get("rows", []):
                    for cell in row:
                        out.extend(cell.get("sentences", []))
        notes = man.get("notes") or {}
        if isinstance(notes, dict):
            for nid in sorted(notes):
                for para in notes[nid].get("paragraphs", []):
                    out.extend(para)
    return out


def align_of(s, lang):
    return (s.get("tr", {}).get(lang) or {}).get("align") or []


def text_of(s, lang):
    return ((s.get("tr", {}).get(lang) or {}).get("text") or "")


def chunk_key(align):
    return tuple((c["t"][0], c["t"][1], tuple(c.get("w") or [])) for c in align)


def word_text(s, i):
    src = s["src"]
    ws = s.get("words") or []
    if i < 0 or i >= len(ws):
        return ""
    a, b = ws[i]
    if a < 0 or b > len(src) or a >= b:
        return ""
    return src[a:b]


def render_pairs(s, lang):
    """internal/translate/judge.go alignPairs, verbatim behaviour."""
    tr = text_of(s, lang)
    pairs = []
    for c in align_of(s, lang):
        a, b = c["t"][0], c["t"][1]
        a = max(a, 0)
        b = min(b, len(tr))
        if a >= b:
            continue
        ws = [w for w in (word_text(s, i) for i in (c.get("w") or [])) if w]
        pairs.append([tr[a:b], " ".join(ws)])
    return pairs


def lit_map(s, lang):
    """source word index -> list of target fragments its taps light."""
    tr = text_of(s, lang)
    out = {}
    for c in align_of(s, lang):
        a, b = max(c["t"][0], 0), min(c["t"][1], len(tr))
        if a >= b:
            continue
        for i in (c.get("w") or []):
            out.setdefault(i, []).append(tr[a:b])
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("base")
    ap.add_argument("itermax")
    ap.add_argument("--book", required=True)
    ap.add_argument("--lang", default="ru")
    ap.add_argument("--n", type=int, default=150)
    ap.add_argument("--seed", type=int, default=19)
    ap.add_argument("--batch", type=int, default=10)
    ap.add_argument("--out", required=True)
    ap.add_argument("--b-label", default="itermax",
                    help="name of the second arm in key.json (D4c reuses this script "
                         "for base vs the guarded arm; the sampling and rendering are "
                         "identical, only the label changes)")
    args = ap.parse_args()

    A, B = sentences(args.base), sentences(args.itermax)
    if len(A) != len(B):
        raise SystemExit(f"sentence count differs: {len(A)} vs {len(B)}")

    changed, lit_rows = [], []
    n_trans_differ = 0
    for idx, (sa, sb) in enumerate(zip(A, B)):
        if sa.get("src") != sb.get("src"):
            raise SystemExit(f"src mismatch at {idx}")
        ta, tb = text_of(sa, args.lang), text_of(sb, args.lang)
        if not ta or not tb:
            continue
        if ta != tb:
            n_trans_differ += 1
            continue
        if chunk_key(align_of(sa, args.lang)) == chunk_key(align_of(sb, args.lang)):
            continue
        changed.append({
            "i": idx,
            "src": sa["src"],
            "tr": ta,
            "base": render_pairs(sa, args.lang),
            "itermax": render_pairs(sb, args.lang),
        })
        # newly lit content words (cmd/score content list; the ledger list is
        # recorded per row so both populations can be reported)
        la, lb = lit_map(sa, args.lang), lit_map(sb, args.lang)
        for wi in range(len(sa.get("words") or [])):
            w = word_text(sa, wi)
            if not w or not is_score_content(w):
                continue
            if wi in la or wi not in lb:
                continue
            lit_rows.append({
                "i": idx, "wi": wi, "src": sa["src"], "tr": ta,
                "word": w, "frag": " … ".join(lb[wi]),
                "ledger": is_content_word(w),
            })

    os.makedirs(args.out, exist_ok=True)
    json.dump(changed, open(f"{args.out}/changed.json", "w"),
              ensure_ascii=False, indent=1)
    json.dump(lit_rows, open(f"{args.out}/lit.json", "w"),
              ensure_ascii=False, indent=1)

    # Even stride over the changed list in document order, spanning the whole
    # list: index k -> floor(k * len / n), k = 0 … n-1. (A plain
    # floor(len/n) stride would stop 13 % short of the end on AC.)
    n = min(args.n, len(changed))
    idx = [(k * len(changed)) // n for k in range(n)]
    stride = len(changed) / n if n else 1
    picked = [changed[i] for i in idx]

    rng = random.Random(args.seed)
    key, items = {}, []
    for j, rec in enumerate(picked, 1):
        sid = str(j)
        swap = rng.random() < 0.5          # True => X is the itermax arm
        x, y = ((rec["itermax"], rec["base"]) if swap
                else (rec["base"], rec["itermax"]))
        key[sid] = {"src": rec["src"], "i": rec["i"], "swap": swap}
        items.append({"id": sid, "src": rec["src"], "tr": rec["tr"], "X": x, "Y": y})
    json.dump({"a_label": "base", "b_label": args.b_label, "stride": stride,
               "changed": len(changed), "pairs": key},
              open(f"{args.out}/key.json", "w"), ensure_ascii=False, indent=1)

    nb = 0
    for k in range(0, len(items), args.batch):
        nb += 1
        chunk = items[k:k + args.batch]
        json.dump(chunk, open(f"{args.out}/batch-{nb:02d}-o1.json", "w"),
                  ensure_ascii=False, indent=1)
        json.dump([{"id": it["id"], "src": it["src"], "tr": it["tr"],
                    "X": it["Y"], "Y": it["X"]} for it in chunk],
                  open(f"{args.out}/batch-{nb:02d}-o2.json", "w"),
                  ensure_ascii=False, indent=1)

    nl = 0
    for k in range(0, len(lit_rows), args.batch):
        nl += 1
        json.dump([{"id": f"{r['i']}:{r['wi']}", "src": r["src"], "tr": r["tr"],
                    "word": r["word"], "frag": r["frag"]}
                   for r in lit_rows[k:k + args.batch]],
                  open(f"{args.out}/lit-{nl:02d}.json", "w"),
                  ensure_ascii=False, indent=1)

    print(f"{args.book}: sentences {len(A)}, translations differing {n_trans_differ}, "
          f"alignment changed {len(changed)}")
    print(f"  sampled {n} by stride {stride} -> {nb} batches x 2 orders")
    print(f"  newly lit content words {len(lit_rows)} -> {nl} batches")


if __name__ == "__main__":
    main()
