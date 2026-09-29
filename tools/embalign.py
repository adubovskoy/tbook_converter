#!/usr/bin/env python3
"""Local embedding word aligner (SimAlign-style) for --align-mode emb|hybrid.

Serves internal/embalign over JSONL on stdin/stdout: after the model loads it
prints {"ready": true}; then for each request line
{"src": [words...], "tgt": [words...]} it answers {"pairs": [[s, t], ...]}
(word-index pairs into the request lists) or {"error": "..."}. A batch line
{"batch": [{"src": [...], "tgt": [...]}, ...]} answers
{"results": [{"pairs": ...} | {"error": ...}, ...]} — all sources of the batch
share one padded forward pass (likewise targets), which is ~2x faster than
one-by-one on CPU. EOF on stdin exits. Progress/diagnostics go to stderr.

A request item may additionally carry "units" (EXPERIMENTAL, research-only):
a list of known multi-word expression spans as SOURCE WORD index ranges
[start, end) — inclusive start, exclusive end — into that item's "src" list,
e.g. {"src": [...], "tgt": [...], "units": [[4, 6], [11, 12]]}. They feed the
unit-glue step, which is itself off unless EMBALIGN_UNIT_GLUE is set; an item
without "units" (or with the env unset) is handled exactly as before.

Method (after SimAlign, Jalili Sabet et al. 2020): token embeddings from a
multilingual encoder (LaBSE by default, hidden layer 8), cosine similarity
over subwords, then
  argmax   mutual argmax (high precision, lower recall — production default)
  itermax  iterative argmax, 2 rounds (higher recall)
  itermax-guarded
           itermax with a CLOSED-CLASS FREEZE: after the second matching round,
           a source word that is in the SOURCE language's closed-class list
           (tools/stopwords.json, exported from internal/stopwords) keeps
           argmax's pairs and nothing else; every other source word keeps
           itermax's. The union then goes through the unchanged glue and
           grouping steps. Source language: --src-lang (default en).
  itermax-silent
           itermax restricted to SILENT source words. BOTH pipelines are run in
           full — argmax and itermax, each through its own unchanged glue steps
           — and then merged per source word: a word the argmax pipeline lights
           keeps that highlight EXACTLY (never widened, narrowed or moved), and
           only a word the argmax pipeline leaves dark takes itermax's. The
           merge is post-glue on purpose: merging before glue lets the glue
           steps re-decide around the new pairs and move highlights argmax had
           already placed.
  itermax-silent-cc
           itermax-silent plus the closed-class freeze: a silent source word in
           the source language's closed-class list stays silent, so only
           CONTENT words are ever recovered. Source language: --src-lang.
A word pair is aligned if any of its subword pairs is aligned. Runs on CPU.

Setup: tools/embalign-setup.sh (venv with CPU torch + transformers + numpy).
"""

import argparse
import json
import os
import sys

import numpy as np
import torch
from transformers import AutoModel, AutoTokenizer


@torch.inference_mode()
def subword_vectors_batch(model, tokenizer, word_lists, layer):
    """Encode a batch of pre-split word lists in one padded forward pass;
    return per item (unit vectors [n_sub, dim], word_id per subword)."""
    enc = tokenizer(
        word_lists,
        is_split_into_words=True,
        return_tensors="pt",
        truncation=True,
        max_length=512,
        padding=True,
    )
    out = model(**enc, output_hidden_states=True)
    hid = out.hidden_states[layer]  # [batch, seq, dim]
    results = []
    for b in range(len(word_lists)):
        vecs, wids = [], []
        for i, wid in enumerate(enc.word_ids(b)):
            if wid is None:  # CLS/SEP/padding
                continue
            vecs.append(hid[b, i])
            wids.append(wid)
        v = torch.stack(vecs).float()
        v = torch.nn.functional.normalize(v, dim=-1)
        results.append((v.numpy(), wids))
    return results


def subword_vectors(model, tokenizer, words, layer):
    """Single-sentence convenience wrapper over subword_vectors_batch."""
    return subword_vectors_batch(model, tokenizer, [words], layer)[0]


def argmax_pairs(sim):
    """Mutual argmax: (i, j) where j is i's best column and i is j's best row."""
    fwd = sim.argmax(axis=1)
    bwd = sim.argmax(axis=0)
    return {(i, j) for i, j in enumerate(fwd) if bwd[j] == i}


def itermax_pairs(sim, rounds=2):
    """Iterative mutual argmax: later rounds only match rows/cols that are
    still free on at least one side, restricted to free-free products."""
    pairs = set(argmax_pairs(sim))
    m, n = sim.shape
    for _ in range(rounds - 1):
        used_i = {i for i, _ in pairs}
        used_j = {j for _, j in pairs}
        free_i = np.array([i not in used_i for i in range(m)])
        free_j = np.array([j not in used_j for j in range(n)])
        if not free_i.any() or not free_j.any():
            break
        masked = np.where(np.outer(free_i, free_j), sim, -1.0)
        fresh = {
            (i, j)
            for (i, j) in argmax_pairs(masked)
            if masked[i, j] > 0 and (free_i[i] or free_j[j])
        }
        if not fresh:
            break
        pairs |= fresh
    return pairs


# GUARDED is the opt-in closed-class freeze (D4b report §6). It is a distinct
# METHOD VALUE, not a flag on itermax, so `argmax` and `itermax` stay byte for
# byte what they were: nothing below this line runs unless the caller asks for
# it by name.
GUARDED = "itermax-guarded"

# SILENT / SILENT_CC are D4d's variants (D4c report §8): recall for words that
# light nothing, and nothing else. Like GUARDED they are distinct METHOD
# VALUES, so `argmax` and `itermax` stay byte for byte what they were.
SILENT = "itermax-silent"
SILENT_CC = "itermax-silent-cc"

# Every method value that needs the source language's closed-class list.
# SILENT is in the list only so its stderr counter can split content from
# function words: with freeze_closed off the set changes no pair.
NEEDS_STOP = (GUARDED, SILENT, SILENT_CC)

# Closed-class lists, exported from internal/stopwords by
# `go run ./cmd/stopwordsdump -o tools/stopwords.json` (kept honest by
# TestExportedFileMatches). Loaded lazily: an argmax/itermax run never reads it.
STOPWORDS_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                              "stopwords.json")

_stop_cache = {}


def closed_class_set(lang):
    """Lowercase closed-class words for `lang`, or an empty set when no list
    ships for it (the guard then degrades to plain itermax, loudly)."""
    lang = (lang or "").strip().lower()
    if lang in _stop_cache:
        return _stop_cache[lang]
    try:
        with open(STOPWORDS_FILE, encoding="utf-8") as f:
            words = json.load(f).get(lang) or []
    except OSError as e:
        print(f"closed-class guard: cannot read {STOPWORDS_FILE}: {e}", file=sys.stderr)
        words = []
    if not words:
        print(f"closed-class guard: no list for source language {lang!r} — "
              f"the guard is a no-op and this run is plain itermax", file=sys.stderr)
    _stop_cache[lang] = {w.lower() for w in words}
    return _stop_cache[lang]


# Per-branch word counts for the guard, reported to stderr (the caller's log).
GUARD_STATS = {"closed": 0, "content": 0, "closed_frozen": 0, "content_gained": 0,
               "sentences": 0}


def guard_closed_class(argmax_wp, itermax_wp, words, stop):
    """Per source word, keep argmax's target set for a closed-class word and
    itermax's for every other word; return the union as sorted [s, t] pairs.

    `words` are the source word strings in the SAME local index space as the
    pairs. Well-defined because itermax only ever adds to argmax's pair set, so
    argmax's set for any source word is always available (possibly empty — a
    closed-class word argmax left unaligned stays unaligned here, exactly as in
    an argmax run, and the unchanged glue steps get their usual shot at it)."""
    a_by_s, i_by_s = {}, {}
    for s, t in argmax_wp:
        a_by_s.setdefault(s, set()).add(t)
    for s, t in itermax_wp:
        i_by_s.setdefault(s, set()).add(t)
    out = []
    for s in sorted(set(a_by_s) | set(i_by_s)):
        w = words[s] if 0 <= s < len(words) else ""
        if w.lower() in stop:
            GUARD_STATS["closed"] += 1
            keep = a_by_s.get(s, set())
            if keep != i_by_s.get(s, set()):
                GUARD_STATS["closed_frozen"] += 1
        else:
            GUARD_STATS["content"] += 1
            keep = i_by_s.get(s, set())
            if keep != a_by_s.get(s, set()):
                GUARD_STATS["content_gained"] += 1
        out.extend([s, t] for t in sorted(keep))
    GUARD_STATS["sentences"] += 1
    if GUARD_STATS["sentences"] % 500 == 0:
        log_guard_stats("progress")
    return sorted(out)


SILENT_STATS = {"kept": 0, "lit_content": 0, "lit_closed": 0, "frozen_closed": 0,
                "sentences": 0}


def silent_only(argmax_wp, itermax_wp, words, stop, freeze_closed):
    """Keep the argmax pipeline's target set for every source word it lights,
    and the itermax pipeline's only for source words it leaves dark.

    Both inputs are FINAL pair lists — each already through glue_idioms and
    glue_target — so nothing a word already lights can change: no widening, no
    narrowing, no move. The only difference against an argmax run is that some
    previously silent source word now lights something. With `freeze_closed`, a
    silent word in the source language's closed-class list stays silent too, so
    only CONTENT words are ever recovered."""
    a_by_s, i_by_s = {}, {}
    for s, t in argmax_wp:
        a_by_s.setdefault(s, set()).add(t)
    for s, t in itermax_wp:
        i_by_s.setdefault(s, set()).add(t)
    out = []
    for s in sorted(set(a_by_s) | set(i_by_s)):
        if s in a_by_s:
            SILENT_STATS["kept"] += 1
            keep = a_by_s[s]
        else:
            w = words[s] if 0 <= s < len(words) else ""
            closed = w.lower() in stop
            if closed and freeze_closed:
                SILENT_STATS["frozen_closed"] += 1
                continue
            SILENT_STATS["lit_closed" if closed else "lit_content"] += 1
            keep = i_by_s[s]
        out.extend([s, t] for t in sorted(keep))
    SILENT_STATS["sentences"] += 1
    if SILENT_STATS["sentences"] % 500 == 0:
        log_silent_stats("progress")
    return sorted(out)


def log_silent_stats(tag):
    g = SILENT_STATS
    print(f"silent-only recall ({tag}): {g['sentences']} sentences, "
          f"{g['kept']} source words kept exactly as argmax aligned them; of the words "
          f"argmax left unaligned, a round-two pair was taken for "
          f"{g['lit_content']} CONTENT and {g['lit_closed']} closed-class words, and "
          f"{g['frozen_closed']} closed-class words were left dark by the freeze "
          f"(counts are post-glue: both pipelines run in full before the merge)",
          file=sys.stderr)


def log_guard_stats(tag):
    g = GUARD_STATS
    print(f"closed-class guard ({tag}): {g['sentences']} sentences, "
          f"{g['closed']} closed-class source words frozen to argmax "
          f"({g['closed_frozen']} of them differed from itermax), "
          f"{g['content']} content source words taken from itermax "
          f"({g['content_gained']} of them differed from argmax)",
          file=sys.stderr)


def to_word_pairs(sub_pairs, src_wids, tgt_wids):
    """Subword pairs -> sorted unique (srcWord, tgtWord) pairs."""
    return sorted({(src_wids[i], tgt_wids[j]) for i, j in sub_pairs})


def align_pair(model, tokenizer, src_words, tgt_words, layer, method, units=None,
               stop=None):
    """Word alignment for one pre-split sentence pair -> sorted [s, t] pairs.
    Empty/blank words are excluded from the model input but original indices
    are preserved in the output pairs."""
    return align_batch(model, tokenizer, [(src_words, tgt_words)], layer, method,
                       [units], stop)[0]


# GLUE_MIN gates the idiom-glue step (0 disables): an uncovered source word is
# attached to a target word when (a) a nearby source word already maps
# there (±3 covers split phrasal particles), (b) that target is the uncovered
# word's own best match, and (c) their
# word-level similarity clears this floor. Catches phrasal verbs / idioms that
# translate as one word ("gave up" -> "бросил" claims both words), which mutual
# argmax alone cannot express when the target word has too few subwords.
GLUE_MIN = float(os.environ.get("EMBALIGN_GLUE_MIN", "0.3"))

# TGT_GLUE_MIN gates the mirror step for uncovered TARGET words (0 disables;
# defaults to GLUE_MIN). Needed when the target is more analytic than the
# source: one source word renders as a multi-word group ("взвинченный" ->
# "estaba nervioso"), mutual argmax awards the group's argmax winner only, and
# the remaining group members would ship unaligned.
TGT_GLUE_MIN = float(os.environ.get("EMBALIGN_TGT_GLUE_MIN", "") or GLUE_MIN)


# UNIT_GLUE gates the EXPERIMENTAL expression-aware glue step (research-only,
# OFF unless EMBALIGN_UNIT_GLUE is set to a truthy value). Its input is the
# request's optional "units" — spans of source words that form one known
# multi-word expression (idiom / phrasal verb / slang / fixed expression),
# supplied by the caller from a lexicon, NOT inferred here.
UNIT_GLUE = (os.environ.get("EMBALIGN_UNIT_GLUE", "") or "0").strip().lower() \
    not in ("", "0", "false", "no", "off")


def glue_units(word_pairs, units):
    """EXPERIMENTAL: make every word of a known expression tap the whole group
    the expression rendered as.

    Contract: for each unit span [start, end) of SOURCE word indices, collect
    the target words that ANY word of the unit is already aligned to; if that
    set is non-empty, align EVERY word of the unit to EVERY target word in it.
    Deterministic, no threshold, additive only (never removes a pair), and it
    never invents an anchor: a unit whose words are all unaligned is left
    alone. Words outside units are untouched.

    Guard: skip the unit when the collected target set is larger than
    max(3, 2 * unit_length). A unit that already claims a big scattered target
    set is evidence of a mis-alignment, and smearing every unit word over all
    of it would turn one bad pair into a whole wrong clause under the tap.
    The cap is generous enough for the real cases (a 2-word phrasal verb
    rendering as up to 3 target words, a 5-word idiom as up to 10).

    Indices are the ORIGINAL request indices (this runs after the local->
    request index mapping), which is the space "units" is expressed in.
    """
    if not UNIT_GLUE or not units:
        return word_pairs
    pairs = {(int(s), int(t)) for s, t in word_pairs}
    for u in units:
        if len(u) != 2:
            continue
        a, b = int(u[0]), int(u[1])
        if b <= a:
            continue
        tset = sorted({t for (s, t) in pairs if a <= s < b})
        if not tset:
            continue  # no anchor anywhere in the unit: invent nothing
        if len(tset) > max(3, 2 * (b - a)):
            continue  # guard: refuse to smear a suspiciously wide target set
        for s in range(a, b):
            for t in tset:
                pairs.add((s, t))
    return sorted([s, t] for s, t in pairs)


def glue_idioms(word_pairs, sim, s_wids, t_wids):
    """Add (uncovered-src-word, tgt-word) pairs per the GLUE_MIN contract.
    Only ever ADDS pairs; the mutual-argmax set is preserved. Local indices."""
    if GLUE_MIN <= 0 or not word_pairs:
        return word_pairs
    row_w = np.array(s_wids)
    col_w = np.array(t_wids)
    n_src = row_w.max() + 1
    tgt_of = {}
    for s, t in word_pairs:
        tgt_of.setdefault(s, set()).add(t)
    added = []
    for wi in range(n_src):
        if wi in tgt_of:
            continue
        neigh = set()
        for d in (-3, -2, -1, 1, 2, 3):
            neigh |= tgt_of.get(wi + d, set())
        if not neigh:
            continue
        sub = sim[row_w == wi]  # this word's subword rows
        if sub.size == 0:
            continue
        col_best = sub.max(axis=0)  # best sim per target subword
        # word-level sim per target word
        tj_best, v_best = -1, -1.0
        for tj in np.unique(col_w):
            v = col_best[col_w == tj].max()
            if v > v_best:
                tj_best, v_best = int(tj), float(v)
        if tj_best in neigh and v_best >= GLUE_MIN:
            added.append([wi, tj_best])
    return sorted(word_pairs + added)


def glue_target(word_pairs, sim, s_wids, t_wids):
    """Mirror of glue_idioms for uncovered TARGET words, per the TGT_GLUE_MIN
    contract: an uncovered target word is attached to a source word when (a) a
    nearby target word (±3) already maps to it, (b) that source word is the
    uncovered word's own best match, and (c) their word-level similarity clears
    the floor. Only ever ADDS pairs. Local indices."""
    if TGT_GLUE_MIN <= 0 or not word_pairs:
        return word_pairs
    row_w = np.array(s_wids)
    col_w = np.array(t_wids)
    n_tgt = col_w.max() + 1
    src_of = {}
    for s, t in word_pairs:
        src_of.setdefault(t, set()).add(s)
    added = []
    for tj in range(n_tgt):
        if tj in src_of:
            continue
        neigh = set()
        for d in (-3, -2, -1, 1, 2, 3):
            neigh |= src_of.get(tj + d, set())
        if not neigh:
            continue
        sub = sim[:, col_w == tj]  # this word's subword columns
        if sub.size == 0:
            continue
        row_best = sub.max(axis=1)  # best sim per source subword
        # word-level sim per source word
        si_best, v_best = -1, -1.0
        for si in np.unique(row_w):
            v = row_best[row_w == si].max()
            if v > v_best:
                si_best, v_best = int(si), float(v)
        if si_best in neigh and v_best >= TGT_GLUE_MIN:
            added.append([si_best, tj])
    return sorted(word_pairs + added)


def glued(sub_pairs, sim, s_wids, t_wids):
    """One matcher's subword pairs -> that arm's FINAL local word pairs: the
    same word conversion and the same two glue steps a normal run applies."""
    local = [[s, t] for s, t in to_word_pairs(sub_pairs, s_wids, t_wids)]
    local = glue_idioms(local, sim, s_wids, t_wids)
    return glue_target(local, sim, s_wids, t_wids)


def match_pairs(sv, s_wids, tv, t_wids, s_map, t_map, method, units=None,
                src_words=None, stop=None):
    """Similarity + argmax matching for one encoded pair -> [s, t] word pairs.
    `units` (optional) drives the experimental unit-glue step, which runs last
    and in request-index space (see glue_units). `src_words`/`stop` are read
    only by the GUARDED method (source word strings in local index space, and
    the source language's closed-class set)."""
    sim = sv @ tv.T
    if method in (SILENT, SILENT_CC):
        # Both pipelines in full, then merge — see silent_only.
        local = silent_only(
            glued(argmax_pairs(sim), sim, s_wids, t_wids),
            glued(itermax_pairs(sim), sim, s_wids, t_wids),
            src_words or [], stop or set(), method == SILENT_CC)
    else:
        if method == GUARDED:
            local = guard_closed_class(
                to_word_pairs(argmax_pairs(sim), s_wids, t_wids),
                to_word_pairs(itermax_pairs(sim), s_wids, t_wids),
                src_words or [], stop or set())
        else:
            sub = itermax_pairs(sim) if method == "itermax" else argmax_pairs(sim)
            local = [[s, t] for s, t in to_word_pairs(sub, s_wids, t_wids)]
        local = glue_idioms(local, sim, s_wids, t_wids)
        local = glue_target(local, sim, s_wids, t_wids)
    pairs = [[s_map[s], t_map[t]] for s, t in local]
    return glue_units(pairs, units)


def encode_sorted(model, tokenizer, word_lists, layer, sub_batch=32):
    """Encode word lists in length-sorted sub-batches (results in input order).
    Sorting groups similar lengths so padding is minimal — measured ~1.9x over
    unsorted batches on real book sentences; per-item vectors are unchanged
    (padding subwords are masked and excluded via word_ids)."""
    order = sorted(range(len(word_lists)), key=lambda i: len(word_lists[i]))
    results = [None] * len(word_lists)
    for k in range(0, len(order), sub_batch):
        idx = order[k : k + sub_batch]
        vecs = subword_vectors_batch(model, tokenizer, [word_lists[j] for j in idx], layer)
        for j, res in zip(idx, vecs):
            results[j] = res
    return results


def align_batch(model, tokenizer, items, layer, method, units_list=None, stop=None):
    """Word alignment for many pre-split (src_words, tgt_words) pairs. Sources
    are encoded in length-sorted padded sub-batches, targets likewise — the
    matching per pair is identical to align_pair (padding subwords are excluded
    via word_ids), so results match the one-by-one path. `units_list`, when
    given, is parallel to `items` (None per item = no expression spans)."""
    maps = []
    src_in, tgt_in = [], []
    for src_words, tgt_words in items:
        s_map = [i for i, w in enumerate(src_words) if w.strip()]
        t_map = [i for i, w in enumerate(tgt_words) if w.strip()]
        maps.append((s_map, t_map))
        if s_map and t_map:
            src_in.append([src_words[i] for i in s_map])
            tgt_in.append([tgt_words[i] for i in t_map])
    src_vecs = encode_sorted(model, tokenizer, src_in, layer) if src_in else []
    tgt_vecs = encode_sorted(model, tokenizer, tgt_in, layer) if tgt_in else []
    out, k = [], 0
    for i, (s_map, t_map) in enumerate(maps):
        if not s_map or not t_map:
            out.append([])
            continue
        (sv, s_wids), (tv, t_wids) = src_vecs[k], tgt_vecs[k]
        local_src = src_in[k]
        k += 1
        units = units_list[i] if units_list else None
        out.append(match_pairs(sv, s_wids, tv, t_wids, s_map, t_map, method, units,
                               local_src, stop))
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--serve", action="store_true", help="JSONL request/response server on stdin/stdout")
    ap.add_argument("--model", default="sentence-transformers/LaBSE")
    ap.add_argument("--layer", type=int, default=8)
    ap.add_argument("--methods", default="argmax",
                    help="argmax | itermax | itermax-guarded | itermax-silent | "
                         "itermax-silent-cc (first entry is used)")
    ap.add_argument("--src-lang", default=os.environ.get("EMBALIGN_SRC_LANG", "") or "en",
                    help="source language code; read only by the closed-class methods "
                         "(itermax-guarded, itermax-silent-cc) (default: en)")
    ap.add_argument("--threads", type=int,
                    default=int(os.environ.get("EMBALIGN_THREADS", "0")) or max(1, (os.cpu_count() or 2) // 2),
                    help="torch intra-op threads (default: physical cores, i.e. logical/2 — measured "
                         "fastest; the torch default and all-logical-cores are 20-500%% slower)")
    args = ap.parse_args()
    if not args.serve:
        ap.error("--serve is the only mode")
    method = [m.strip() for m in args.methods.split(",") if m.strip()][0]
    stop = None
    if method in NEEDS_STOP:
        stop = closed_class_set(args.src_lang)
    if method == GUARDED:
        print(f"closed-class guard ON: itermax, with {len(stop)} {args.src_lang} "
              f"closed-class source words frozen to argmax", file=sys.stderr)
    elif method == SILENT:
        print("silent-only recall ON: itermax pairs kept only for source words "
              "argmax left unaligned", file=sys.stderr)
    elif method == SILENT_CC:
        print(f"silent-only recall ON with closed-class freeze: itermax pairs kept only "
              f"for CONTENT source words argmax left unaligned ({len(stop)} "
              f"{args.src_lang} closed-class words stay dark)", file=sys.stderr)

    print(f"loading {args.model} ...", file=sys.stderr)
    torch.set_num_threads(max(1, args.threads))
    tokenizer = AutoTokenizer.from_pretrained(args.model)
    model = AutoModel.from_pretrained(args.model)
    model.eval()
    # Only hidden_states[args.layer] is read — drop the transformer blocks above
    # it (LaBSE: 12 → 8) for the same output at ~2/3 the compute. hidden_states
    # keeps layer+1 entries when exactly `layer` blocks run.
    enc = getattr(model, "encoder", None)
    if enc is not None and hasattr(enc, "layer") and 0 < args.layer < len(enc.layer):
        del enc.layer[args.layer:]
        print(f"encoder truncated to {args.layer} layers", file=sys.stderr)
    if os.environ.get("EMBALIGN_INT8"):
        # ~2x on CPU; perturbs embeddings slightly — argmax pairs can flip on
        # near-ties. Opt-in until validated per language pair (see speed report).
        import warnings
        with warnings.catch_warnings():
            warnings.simplefilter("ignore")
            model = torch.quantization.quantize_dynamic(model, {torch.nn.Linear}, dtype=torch.qint8)
        print("int8 dynamic quantization enabled", file=sys.stderr)

    if UNIT_GLUE:
        print("EXPERIMENTAL unit glue enabled (EMBALIGN_UNIT_GLUE)", file=sys.stderr)

    print(json.dumps({"ready": True}), flush=True)
    for line in sys.stdin:
        if not line.strip():
            continue
        try:
            req = json.loads(line)
            if "batch" in req:
                results = []
                pairs_list = align_batch(
                    model, tokenizer,
                    [(it["src"], it["tgt"]) for it in req["batch"]],
                    args.layer, method,
                    [it.get("units") for it in req["batch"]],
                    stop,
                )
                results = [{"pairs": p} for p in pairs_list]
                print(json.dumps({"results": results}), flush=True)
            else:
                pairs = align_pair(model, tokenizer, req["src"], req["tgt"], args.layer, method,
                                   req.get("units"), stop)
                print(json.dumps({"pairs": pairs}), flush=True)
        except Exception as e:  # keep serving; the caller decides what to do
            print(json.dumps({"error": f"{type(e).__name__}: {e}"}), flush=True)
    if method == GUARDED:
        log_guard_stats("total")
    elif method in (SILENT, SILENT_CC):
        log_silent_stats("total")


if __name__ == "__main__":
    main()
