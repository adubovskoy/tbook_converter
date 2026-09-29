#!/usr/bin/env python3
"""prepare_pairs.py with a --lang flag (the original reads tr['ru'] only).

Same arguments as prepare_pairs.py plus --lang CODE (default ru). Everything
else — sampling, seeding, both presentation orders, key.json — is the original
code, imported, so the two stay in step.

  prepare_pairs_lang.py A.tbook LA B.tbook LB --lang de --n 400 --out dir

--only-src FILE restricts the sample to the `src` values of a JSONL file (the
A3 gold subset), so screening rounds judge exactly the same 300 sentences.
"""
import json, sys, zipfile
import prepare_pairs as pp

LANG = 'ru'
ONLY = None   # optional set of source sentences the sample is restricted to


def texts_lang(path):
    if path.endswith('.json'):
        return pp.texts(path)
    out = {}
    with zipfile.ZipFile(path) as z:
        for n in sorted(z.namelist()):
            if n.startswith('chapters/') and n.endswith('.json'):
                d = json.loads(z.read(n))
                for para in d['paragraphs']:
                    for s in para:
                        tr = s.get('tr', {}).get(LANG, {})
                        t = (tr.get('text') or '').strip()
                        if t and s.get('src'):
                            out[s['src'].strip()] = t
    if ONLY is not None:
        out = {k: v for k, v in out.items() if k in ONLY}
    return out


if __name__ == '__main__':
    argv = sys.argv[1:]
    if '--lang' in argv:
        i = argv.index('--lang')
        LANG = argv[i + 1]
        del argv[i:i + 2]
    if '--only-src' in argv:
        i = argv.index('--only-src')
        path = argv[i + 1]
        del argv[i:i + 2]
        ONLY = {json.loads(l)['src'].strip()
                for l in open(path, encoding='utf-8') if l.strip()}
    pp.texts = texts_lang
    sys.argv = [sys.argv[0]] + argv
    pp.main()
