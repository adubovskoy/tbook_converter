# bench-quality

Benchmark arms (`*.tbook` with their `*-stats.jsonl`), the probe sets and
scripts that produced them, and the write-ups in `reports/`.

## Learner-visible metrics every arm should carry

Two offline, zero-API tools score an arm next to its judged numbers. Both read
`.tbook` version 1 and version 2 and dispatch on `manifest.formatVersion`.

### `cmd/tapledger` — what one tap returns

```bash
go run ./cmd/tapledger bench-quality/full-kimi.tbook [--lang ru,de] [--content-only] [--json out.json]
```

Classifies every (source word, target language) tap as the reference consumers
render it:

| class | the learner sees |
|---|---|
| `clean` | one chunk, exclusive to the tapped word, one target token |
| `clean-multi` | ≥ 2 exclusive chunks — fr «ne … pas» |
| `shared` | a lit token that also answers another source word's question |
| `smear` | ≥ 3 target tokens light: a phrase-level highlight |
| `none` | nothing lights, with no reason given |
| `dead-sentence` | the translation is there, its alignment is empty: every tap is silent |

Priority: `dead-sentence` > `none` > `smear` > `shared` > `clean-multi` >
`clean`. Taps in a sentence with no translation are not counted.

Report per arm: **clean %** and **content-word silent %** (content word = ≥ 4
letters and not in the tool's stoplist; `--content-only` restricts the whole
ledger to those taps). The coarse three-class collapse printed underneath
(clean / shared / dead over every tap, untranslated ones included) is the shape
used in the panel's phase-3 table.

The panel also cross-tabbed each tap with the consumers' `.tdict` lookup; that
part is not ported — the tool needs no dictionary.

### `cmd/consensus` — which language disagrees

```bash
go run ./cmd/consensus android/app/src/main/assets/sample.tbook [--min-langs 6] [--content-only] [--json out.json]
```

For a book with ≥ 3 targets, the pivot design already holds N independent
alignments of the same sentence. Per source-word slot the tool reports who
highlights it: **unanimous %**, **solo misses per language** (exactly one
language fails where all others succeed — the per-language aligner regression
signal) and **unanimous misses** (nobody highlights it: usually a hard sense
every language translated and none could place). `--min-langs` is how many
aligned languages a sentence needs before it votes; `--content-only` drops
closed-class words, which otherwise dominate solo misses. Books with fewer than
three targets are refused — the signal does not exist for them.

It says nothing about a fluent mistranslation; it finds the class the judge
calls *wrong-mapping* and coverage cannot see.

### Reference numbers (regression guard)

`internal/learner` asserts these against the two books in the workspace; the
tests skip when a book is absent. Sources: panel `phase2/critic-learner-ux.md`
T0, `phase3/judge-learner-first.md` §1, `phase2/critic-producer-economics.md`
PE-10.

`sample.tbook`, 8 targets — clean / clean-multi / shared / smear / none /
dead-sentence / content-word silent:

| lang | clean | clean-multi | shared | smear | none | dead | cw-silent |
|---|---|---|---|---|---|---|---|
| ru | 51.34 | 3.27 | 40.58 | 0.66 | 4.16 | 0.00 | 2.84 |
| uk | 52.00 | 2.80 | 41.22 | 0.46 | 3.52 | 0.00 | 2.91 |
| tr | 33.23 | 3.74 | 56.63 | 0.57 | 5.83 | 0.00 | 3.57 |
| de | 72.87 | 4.33 | 18.96 | 0.60 | 3.10 | 0.14 | 3.07 |
| fr | 64.92 | 6.69 | 23.56 | 1.64 | 3.18 | 0.00 | 2.74 |
| es | 62.90 | 6.22 | 25.54 | 1.30 | 3.00 | 1.04 | 3.57 |
| it | 58.60 | 5.86 | 31.33 | 1.01 | 3.19 | 0.01 | 2.83 |
| pt | 63.19 | 5.46 | 27.90 | 0.94 | 2.50 | 0.00 | 2.19 |

`full-kimi.tbook` (ru): 52.21 / 4.21 / 39.34 / 0.88 / 3.35 / 0.00, content-word
silent 2.27 %; coarse collapse 56.95 / 39.69 / 3.35.

`sample.tbook` consensus at `--min-langs 6`: 82.59 % unanimous, 11,863 solo
misses (tr 3665, ru 1526, de 1331, fr 1275, it 1242, uk 1146, es 982, pt 696),
36 unanimous misses; `--content-only`: 4,618 solo misses of 51,684 slots, 17
unanimous misses.

Two notes on reading those numbers against the panel's:

- The panel scored chapter paragraphs only; these tools also score table cells
  and footnotes, which moves a share by ≤ 0.02 pp on `sample.tbook` (and adds
  the handful of slots by which the solo-miss counts differ).
- Migrating `sample.tbook` to version 2 and re-scoring it moves no class by more
  than **0.05 pp** in any language: version 2 makes digit runs tappable, which
  adds ~80 mostly-silent taps per language.
