#!/usr/bin/env python3
"""Prepare blind pairwise A/B judging batches from two .tbook files.

Usage:
  prepare_pairs.py A.tbook LABEL_A B.tbook LABEL_B --n 300 --seed 1 \
      --min-words 6 --batch 10 --out pairs-dir

Emits into --out:
  key.json           {id: {"src":..., "a_label":..., "swap": bool}}  (secret key)
  batch-<k>-o1.json  [{id, src, X, Y}]  presentation order 1
  batch-<k>-o2.json  [{id, src, X, Y}]  presentation order 2 (X/Y swapped)

Judges see only X/Y; swap in key.json says whether X (in order 1) is arm B.
Each pair must be judged in BOTH orders; disagreement counts as a tie.
"""
import argparse, json, random, sys, zipfile


def texts(path):
    """src -> translation, from a .tbook or from an arm's JSON [{src, tr}] dump
    (what gloss_scale_probe.py writes — those arms are not .tbook files)."""
    if path.endswith('.json'):
        rows = json.load(open(path, encoding='utf-8'))
        return {r['src'].strip(): (r.get('tr') or '').strip()
                for r in rows if r.get('src') and (r.get('tr') or '').strip()}
    out = {}
    with zipfile.ZipFile(path) as z:
        for n in sorted(z.namelist()):
            if n.startswith('chapters/') and n.endswith('.json'):
                d = json.loads(z.read(n))
                for para in d['paragraphs']:
                    for s in para:
                        tr = s.get('tr', {}).get('ru', {})
                        t = (tr.get('text') or '').strip()
                        if t and s.get('src'):
                            out[s['src'].strip()] = t
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('a'); ap.add_argument('la')
    ap.add_argument('b'); ap.add_argument('lb')
    ap.add_argument('--n', type=int, default=300)
    ap.add_argument('--seed', type=int, default=1)
    ap.add_argument('--skip', type=int, default=0,
                    help='drop the first N sampled pairs — a second round with '
                         'the same seed and --skip N is disjoint from the first')
    ap.add_argument('--min-words', type=int, default=6)
    ap.add_argument('--batch', type=int, default=10)
    ap.add_argument('--out', required=True)
    args = ap.parse_args()

    ta, tb = texts(args.a), texts(args.b)
    common = [s for s in ta if s in tb and len(s.split()) >= args.min_words
              and ta[s] != tb[s]]
    same = sum(1 for s in ta if s in tb and ta[s] == tb[s])
    rng = random.Random(args.seed)
    rng.shuffle(common)
    sample = sorted(common[args.skip:args.skip + args.n])  # deterministic id order
    rng2 = random.Random(args.seed + 1 + args.skip)

    import os
    os.makedirs(args.out, exist_ok=True)
    key, items = {}, []
    for i, src in enumerate(sample, 1):
        sid = str(i)
        swap = rng2.random() < 0.5
        x, y = (tb[src], ta[src]) if swap else (ta[src], tb[src])
        key[sid] = {'src': src, 'swap': swap}
        items.append({'id': sid, 'src': src, 'X': x, 'Y': y})
    json.dump({'a_label': args.la, 'b_label': args.lb, 'pairs': key},
              open(f'{args.out}/key.json', 'w'), ensure_ascii=False, indent=1)
    nb = 0
    for k in range(0, len(items), args.batch):
        nb += 1
        chunk = items[k:k + args.batch]
        json.dump(chunk, open(f'{args.out}/batch-{nb:02d}-o1.json', 'w'),
                  ensure_ascii=False, indent=1)
        sw = [{'id': it['id'], 'src': it['src'], 'X': it['Y'], 'Y': it['X']}
              for it in chunk]
        json.dump(sw, open(f'{args.out}/batch-{nb:02d}-o2.json', 'w'),
                  ensure_ascii=False, indent=1)
    print(f'common differing: {len(common)} (+{same} identical, excluded); '
          f'sampled {len(items)}; {nb} batches x2 orders -> {args.out}')


if __name__ == '__main__':
    main()
