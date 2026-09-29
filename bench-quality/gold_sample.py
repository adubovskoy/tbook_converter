#!/usr/bin/env python3
"""Build the GOLD-lite draft sets from one excerpt converted to several targets.

Walks the .tbook in converter order (manifest chapter order -> paragraphs ->
sentences), keeps prose only, applies the sampling rule and writes one JSONL per
pair: {id, chapter, para, sent, src, ref, ref_model, status}.

Usage:
  gold_sample.py --arm ru=a2/holmes4-g37-ru.tbook --arm de=... --arm fr=... \
      --n 300 --min-words 4 --model google/gemini-3.7-flash --out-dir gold
"""
import argparse, json, os, zipfile


def prose(path, lang):
    """Prose sentences in converter order. Table cells, figure captions and
    footnote bodies are NOT walked (they are separate chapter members)."""
    rows = []
    with zipfile.ZipFile(path) as z:
        man = json.loads(z.read('manifest.json'))
        for ci, c in enumerate(man['chapters'], 1):
            ch = json.loads(z.read(c['file']))
            for pi, para in enumerate(ch['paragraphs']):
                for si, s in enumerate(para):
                    src = (s.get('src') or '').strip()
                    tr = ((s.get('tr') or {}).get(lang) or {})
                    rows.append({'chapter': ci, 'para': pi, 'sent': si,
                                 'src': src, 'ref': (tr.get('text') or '').strip()})
    return rows


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--arm', action='append', required=True,
                    help='LANG=path.tbook (repeatable); the first is the pivot order')
    ap.add_argument('--n', type=int, default=300)
    ap.add_argument('--min-words', type=int, default=4)
    ap.add_argument('--model', required=True)
    ap.add_argument('--out-dir', required=True)
    ap.add_argument('--prefix', default='h4')
    args = ap.parse_args()

    arms = [a.split('=', 1) for a in args.arm]
    walks = {lang: prose(p, lang) for lang, p in arms}
    langs = [l for l, _ in arms]
    base = walks[langs[0]]
    for l in langs[1:]:
        assert len(walks[l]) == len(base), f'{l}: sentence count differs'
        for a, b in zip(walks[l], base):
            assert a['src'] == b['src'], f'{l}: source order differs at {a}'

    # Candidate rule (recorded in the report):
    #  - prose paragraphs only (no table cells, captions, notes)
    #  - >= min-words source words
    #  - a non-empty translation in EVERY arm
    #  - first occurrence of a duplicate source only
    seen, cand = set(), []
    for i, r in enumerate(base):
        if len(r['src'].split()) < args.min_words:
            continue
        if any(not walks[l][i]['ref'] for l in langs):
            continue
        if r['src'] in seen:
            continue
        seen.add(r['src'])
        cand.append(i)

    n = min(args.n, len(cand))
    picks = [cand[(k * len(cand)) // n] for k in range(n)]
    assert len(set(picks)) == n, 'stride collided'

    os.makedirs(args.out_dir, exist_ok=True)
    for l in langs:
        path = f'{args.out_dir}/en-{l}.jsonl'
        with open(path, 'w', encoding='utf-8') as f:
            for k, i in enumerate(picks, 1):
                b, w = base[i], walks[l][i]
                f.write(json.dumps({
                    'id': f'{args.prefix}-{k:03d}',
                    'chapter': b['chapter'], 'para': b['para'], 'sent': b['sent'],
                    'src': b['src'], 'ref': w['ref'],
                    'ref_model': args.model, 'status': 'draft',
                }, ensure_ascii=False) + '\n')
        print(f'{path}: {n} lines')
    print(f'walked {len(base)} prose sentences; {len(cand)} candidates; '
          f'stride {len(cand)/n:.3f} (pick k -> cand[floor(k*{len(cand)}/{n})])')


if __name__ == '__main__':
    main()
