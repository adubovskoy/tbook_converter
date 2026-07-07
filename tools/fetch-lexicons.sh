#!/usr/bin/env bash
# Fetch OPUS OpenSubtitles word-alignment dictionaries and convert them into
# the converter's compact lexicon format (lexicons/<src>-<tgt>.tsv.gz), both
# directions per pair. Coverage: de en es fr it ru (15 pairs → 30 lexicons).
#
#   tools/fetch-lexicons.sh [pair ...]     # default: all pairs
#
# Data: https://opus.nlpl.eu (OpenSubtitles v2018 .dic files). The dictionaries
# are used only by the converter's static drift check (--lexcheck), never
# shipped inside a .tbook.
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p lexicons
PAIRS=${@:-"de-en de-es de-fr de-it de-ru en-es en-fr en-it en-ru es-fr es-it es-ru fr-it fr-ru it-ru"}
for pair in $PAIRS; do
  src=${pair%-*}; tgt=${pair#*-}
  if [[ -f lexicons/$src-$tgt.tsv.gz && -f lexicons/$tgt-$src.tsv.gz ]]; then
    echo "✓ $pair (cached)"; continue
  fi
  echo "… $pair"
  curl -fsS --retry 3 -o "/tmp/$pair.dic.gz" \
    "https://object.pouta.csc.fi/OPUS-OpenSubtitles/v2018/dic/$pair.dic.gz"
  python3 - "$src" "$tgt" "/tmp/$pair.dic.gz" <<'PY'
import gzip, collections, re, sys
src, tgt, path = sys.argv[1], sys.argv[2], sys.argv[3]
word = re.compile(r'^[^\W\d_]+$', re.UNICODE)
fwd, rev = collections.defaultdict(list), collections.defaultdict(list)
for line in gzip.open(path, 'rt', encoding='utf-8'):
    parts = line.rstrip('\n').split('\t')
    if len(parts) < 6:
        continue
    try:
        count = int(parts[0])
    except ValueError:
        continue
    s, t = parts[2].strip().lower(), parts[3].strip().lower()
    if count < 3 or not word.match(s) or not word.match(t):
        continue
    fwd[s].append((count, t)); rev[t].append((count, s))
def dump(d, p, k=12):
    with gzip.open(p, 'wt', encoding='utf-8') as f:
        for s in sorted(d):
            tops = [t for _, t in sorted(d[s], reverse=True)[:k]]
            f.write(s + '\t' + '|'.join(dict.fromkeys(tops)) + '\n')
dump(fwd, f'lexicons/{src}-{tgt}.tsv.gz'); dump(rev, f'lexicons/{tgt}-{src}.tsv.gz')
print(f'  {src}-{tgt}: {len(fwd)} / {tgt}-{src}: {len(rev)} headwords')
PY
  rm -f "/tmp/$pair.dic.gz"
done
