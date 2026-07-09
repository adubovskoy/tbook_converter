#!/usr/bin/env python3
"""Local embedding word aligner (SimAlign-style) for --align-mode emb|hybrid.

Serves internal/embalign over JSONL on stdin/stdout: after the model loads it
prints {"ready": true}; then for each request line
{"src": [words...], "tgt": [words...]} it answers {"pairs": [[s, t], ...]}
(word-index pairs into the request lists) or {"error": "..."}. EOF on stdin
exits. Progress/diagnostics go to stderr.

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
import sys

import numpy as np
import torch
from transformers import AutoModel, AutoTokenizer


@torch.no_grad()
def subword_vectors(model, tokenizer, words, layer):
    """Encode pre-split words; return (unit vectors [n_sub, dim], word_id per subword)."""
    enc = tokenizer(
        words,
        is_split_into_words=True,
        return_tensors="pt",
        truncation=True,
        max_length=512,
    )
    out = model(**enc, output_hidden_states=True)
    hid = out.hidden_states[layer][0]  # [seq, dim]
    vecs, wids = [], []
    for i, wid in enumerate(enc.word_ids(0)):
        if wid is None:  # CLS/SEP
            continue
        vecs.append(hid[i])
        wids.append(wid)
    v = torch.stack(vecs).float()
    v = torch.nn.functional.normalize(v, dim=-1)
    return v.numpy(), wids


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
    s_map = [i for i, w in enumerate(src_words) if w.strip()]
    t_map = [i for i, w in enumerate(tgt_words) if w.strip()]
    if not s_map or not t_map:
        return []
    sv, s_wids = subword_vectors(model, tokenizer, [src_words[i] for i in s_map], layer)
    tv, t_wids = subword_vectors(model, tokenizer, [tgt_words[i] for i in t_map], layer)
    sim = sv @ tv.T
    sub = itermax_pairs(sim) if method == "itermax" else argmax_pairs(sim)
    return [[s_map[s], t_map[t]] for s, t in to_word_pairs(sub, s_wids, t_wids)]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--serve", action="store_true", help="JSONL request/response server on stdin/stdout")
    ap.add_argument("--model", default="sentence-transformers/LaBSE")
    ap.add_argument("--layer", type=int, default=8)
    ap.add_argument("--methods", default="argmax", help="argmax | itermax (first entry is used)")
    args = ap.parse_args()
    if not args.serve:
        ap.error("--serve is the only mode")
    method = [m.strip() for m in args.methods.split(",") if m.strip()][0]

    print(f"loading {args.model} ...", file=sys.stderr)
    tokenizer = AutoTokenizer.from_pretrained(args.model)
    model = AutoModel.from_pretrained(args.model)
    model.eval()

    print(json.dumps({"ready": True}), flush=True)
    for line in sys.stdin:
        if not line.strip():
            continue
        try:
            req = json.loads(line)
            pairs = align_pair(model, tokenizer, req["src"], req["tgt"], args.layer, method)
            print(json.dumps({"pairs": pairs}), flush=True)
        except Exception as e:  # keep serving; the caller decides what to do
            print(json.dumps({"error": f"{type(e).__name__}: {e}"}), flush=True)


if __name__ == "__main__":
    main()
