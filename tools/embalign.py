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

Method (after SimAlign, Jalili Sabet et al. 2020): token embeddings from a
multilingual encoder (LaBSE by default, hidden layer 8), cosine similarity
over subwords, then
  argmax   mutual argmax (high precision, lower recall — production default)
  itermax  iterative argmax, 2 rounds (higher recall)
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


def to_word_pairs(sub_pairs, src_wids, tgt_wids):
    """Subword pairs -> sorted unique (srcWord, tgtWord) pairs."""
    return sorted({(src_wids[i], tgt_wids[j]) for i, j in sub_pairs})


def align_pair(model, tokenizer, src_words, tgt_words, layer, method):
    """Word alignment for one pre-split sentence pair -> sorted [s, t] pairs.
    Empty/blank words are excluded from the model input but original indices
    are preserved in the output pairs."""
    return align_batch(model, tokenizer, [(src_words, tgt_words)], layer, method)[0]


# GLUE_MIN gates the idiom-glue step (0 disables): an uncovered source word is
# attached to a target word when (a) a nearby source word already maps
# there (±3 covers split phrasal particles), (b) that target is the uncovered
# word's own best match, and (c) their
# word-level similarity clears this floor. Catches phrasal verbs / idioms that
# translate as one word ("gave up" -> "бросил" claims both words), which mutual
# argmax alone cannot express when the target word has too few subwords.
GLUE_MIN = float(os.environ.get("EMBALIGN_GLUE_MIN", "0.3"))


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


def match_pairs(sv, s_wids, tv, t_wids, s_map, t_map, method):
    """Similarity + argmax matching for one encoded pair -> [s, t] word pairs."""
    sim = sv @ tv.T
    sub = itermax_pairs(sim) if method == "itermax" else argmax_pairs(sim)
    local = [[s, t] for s, t in to_word_pairs(sub, s_wids, t_wids)]
    local = glue_idioms(local, sim, s_wids, t_wids)
    return [[s_map[s], t_map[t]] for s, t in local]


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


def align_batch(model, tokenizer, items, layer, method):
    """Word alignment for many pre-split (src_words, tgt_words) pairs. Sources
    are encoded in length-sorted padded sub-batches, targets likewise — the
    matching per pair is identical to align_pair (padding subwords are excluded
    via word_ids), so results match the one-by-one path."""
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
    for s_map, t_map in maps:
        if not s_map or not t_map:
            out.append([])
            continue
        (sv, s_wids), (tv, t_wids) = src_vecs[k], tgt_vecs[k]
        k += 1
        out.append(match_pairs(sv, s_wids, tv, t_wids, s_map, t_map, method))
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--serve", action="store_true", help="JSONL request/response server on stdin/stdout")
    ap.add_argument("--model", default="sentence-transformers/LaBSE")
    ap.add_argument("--layer", type=int, default=8)
    ap.add_argument("--methods", default="argmax", help="argmax | itermax (first entry is used)")
    ap.add_argument("--threads", type=int,
                    default=int(os.environ.get("EMBALIGN_THREADS", "0")) or max(1, (os.cpu_count() or 2) // 2),
                    help="torch intra-op threads (default: physical cores, i.e. logical/2 — measured "
                         "fastest; the torch default and all-logical-cores are 20-500%% slower)")
    args = ap.parse_args()
    if not args.serve:
        ap.error("--serve is the only mode")
    method = [m.strip() for m in args.methods.split(",") if m.strip()][0]

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
                )
                results = [{"pairs": p} for p in pairs_list]
                print(json.dumps({"results": results}), flush=True)
            else:
                pairs = align_pair(model, tokenizer, req["src"], req["tgt"], args.layer, method)
                print(json.dumps({"pairs": pairs}), flush=True)
        except Exception as e:  # keep serving; the caller decides what to do
            print(json.dumps({"error": f"{type(e).__name__}: {e}"}), flush=True)


if __name__ == "__main__":
    main()
