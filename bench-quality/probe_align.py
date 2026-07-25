#!/usr/bin/env python3
"""Measure and judge alignment of multi-word expressions in a .tbook.

Two modes:
  --cover      per-expression tap coverage (how many words of the expression
               highlight anything) + aggregate report
  --emit DIR   write judging batches: for each probe, the expression's source
               words and the target fragments each one highlights, so a judge
               can say whether the highlight is the CORRECT rendering

Coverage answers "does every word get markup"; judging answers "is the markup
right" — coverage is blind to wrong-but-present mappings.

Usage:
  probe_align.py book.tbook -t ru --probe probe-dev.json --cover
  probe_align.py book.tbook -t ru --probe probe-dev.json --emit judge-dir --label armname
"""
import argparse, json, os, zipfile

# Word spans come straight from the .tbook (produced by segment.Tokenize), so
# there is no second tokenizer here to disagree with the producer.



def load(path, lang):
    """{src: {"words": [[s,e]…], "text": tr, "align": [{t:[s,e], w:[idx]}…]}}"""
    out = {}
    with zipfile.ZipFile(path) as z:
        for n in sorted(z.namelist()):
            if not (n.startswith('chapters/') and n.endswith('.json')):
                continue
            for para in json.loads(z.read(n))['paragraphs']:
                for s in para:
                    tr = (s.get('tr') or {}).get(lang) or {}
                    if not s.get('src') or not tr.get('text'):
                        continue
                    out[s['src'].strip()] = {
                        'words': s.get('words') or [],
                        'text': tr['text'],
                        'align': tr.get('align') or [],
                    }
    return out


def expr_word_idx(src, words, expr):
    """Indices of the words of expr inside src, matched case-insensitively."""
    lo = src.lower().find(expr.lower())
    if lo < 0:
        return None
    hi = lo + len(expr)
    idx = [i for i, (s, e) in enumerate(words) if s < hi and e > lo]
    return idx or None


def frag(text, span):
    return text[span[0]:span[1]]


def analyze(rec, expr):
    """Per-expression view: which expr words are tapped, and onto what."""
    idx = expr_word_idx(rec['src'], rec['words'], expr) if 'src' in rec else None
    if idx is None:
        return None
    per_word = {i: [] for i in idx}
    for ch in rec['align']:
        w = ch.get('w') or []
        hit = [i for i in idx if i in w]
        if hit:
            f = frag(rec['text'], ch['t'])
            for i in hit:
                per_word[i].append(f)
    covered = sum(1 for i in idx if per_word[i])
    return {'idx': idx, 'per_word': per_word, 'covered': covered, 'total': len(idx)}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('book')
    ap.add_argument('-t', '--target', default='ru')
    ap.add_argument('--probe', required=True)
    ap.add_argument('--cover', action='store_true')
    ap.add_argument('--emit')
    ap.add_argument('--label', default='arm')
    ap.add_argument('--batch', type=int, default=25)
    args = ap.parse_args()

    book = load(args.book, args.target)
    probes = json.load(open(args.probe))
    rows, missing = [], 0
    for p in probes:
        src = p['src'].strip()
        if src not in book:
            missing += 1
            continue
        rec = dict(book[src], src=src)
        a = analyze(rec, p['expr'])
        if a is None:
            missing += 1
            continue
        rows.append((p, rec, a))

    if args.cover or not args.emit:
        full = sum(1 for _, _, a in rows if a['covered'] == a['total'])
        none = sum(1 for _, _, a in rows if a['covered'] == 0)
        part = len(rows) - full - none
        words_tot = sum(a['total'] for _, _, a in rows)
        words_cov = sum(a['covered'] for _, _, a in rows)
        print(f'{args.book}  probes matched {len(rows)} (skipped {missing})')
        print(f'  expression WORD tap coverage: {words_cov}/{words_tot} '
              f'({words_cov/words_tot:.1%})')
        print(f'  expressions fully tapped: {full} ({full/len(rows):.1%})   '
              f'partial: {part} ({part/len(rows):.1%})   none: {none} '
              f'({none/len(rows):.1%})')
        by = {}
        for p, _, a in rows:
            k = p.get('kind', '?')
            d = by.setdefault(k, [0, 0])
            d[0] += a['covered']; d[1] += a['total']
        print('  by kind: ' + '  '.join(
            f'{k} {c}/{t} ({c/t:.0%})' for k, (c, t) in sorted(by.items())))

    if args.emit:
        os.makedirs(args.emit, exist_ok=True)
        items = []
        for p, rec, a in rows:
            words = [rec['src'][s:e] for s, e in rec['words']]
            taps = [{'word': words[i],
                     'highlights': a['per_word'][i]} for i in a['idx']]
            items.append({'id': str(len(items) + 1), 'src': rec['src'],
                          'expr': p['expr'], 'gloss': p.get('gloss', ''),
                          'tr': rec['text'], 'taps': taps})
        n = 0
        for k in range(0, len(items), args.batch):
            n += 1
            json.dump(items[k:k + args.batch],
                      open(f'{args.emit}/batch-{n:02d}.json', 'w'),
                      ensure_ascii=False, indent=1)
        json.dump({'label': args.label, 'count': len(items)},
                  open(f'{args.emit}/meta.json', 'w'), indent=1)
        print(f'  emitted {len(items)} items in {n} batches -> {args.emit}')


if __name__ == '__main__':
    main()
